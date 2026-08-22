// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agentprovider

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/apache/casbin-gateway/agentmonitor"
)

const (
	// codexProviderName is the model provider Gateway owns in config.toml.
	// Everything else in the file, including other providers, is left alone.
	codexProviderName = "casbin-gateway"
	// codexAuthKey is where Codex reads the key of a provider whose env_key is
	// not exported in the shell.
	codexAuthKey = "OPENAI_API_KEY"
)

// The keys of the root table Gateway owns, remembered under these names.
const (
	codexModelProviderKey = "model_provider"
	codexModelKey         = "model"
	codexAuthStateKey     = "auth." + codexAuthKey
)

var codexProviderPath = []string{"model_providers", codexProviderName}

// errCodexNoKey rejects a provider that forwards the caller's own credentials:
// Codex reads its key from auth.json, and its own ChatGPT sign-in speaks a
// different API than the chat completions this provider entry points at.
var errCodexNoKey = errors.New("Codex needs an API key, so it cannot use a provider that forwards the credentials of the caller")

type codexWriter struct {
	id string
}

func init() {
	register(codexWriter{id: "codex"})
	register(codexWriter{id: "codex-cli"})
	// The VS Code integration drives the same CLI, and with it the same
	// ~/.codex directory.
	register(codexWriter{id: "codex_vscode"})
	register(codexWriter{id: "codex-vscode"})
}

func (w codexWriter) AgentId() string { return w.id }

func (codexWriter) Protocol() string { return "openai" }

func (w codexWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.ApiKey == "" {
		return nil, errCodexNoKey
	}

	home, err := agentmonitor.ResolveCodexHome(target.Path, target.Owner)
	if err != nil {
		return nil, err
	}

	config := tomlSetRootKey("", codexModelProviderKey, codexProviderName)
	if endpoint.Model != "" {
		config = tomlSetRootKey(config, codexModelKey, endpoint.Model)
	}
	config = tomlTidy(tomlAppend(config, w.providerTable(endpoint)))

	auth, err := encodeJSON(map[string]any{codexAuthKey: maskSecret(endpoint.ApiKey)})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: filepath.Join(home, "config.toml"), Format: "toml", Preview: config},
		{Path: filepath.Join(home, "auth.json"), Format: "json", Preview: string(auth)},
	}, nil
}

func (w codexWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.ApiKey == "" {
		return nil, errCodexNoKey
	}

	configPath, authPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	data, _, _, err := readFile(configPath)
	if err != nil {
		return nil, err
	}
	document, err := w.decode(configPath, data)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	if value, ok := document[codexModelProviderKey].(string); ok {
		previous[codexModelProviderKey] = value
	}
	if value, ok := document[codexModelKey].(string); ok {
		previous[codexModelKey] = value
	}

	text := tomlCutTable(string(data), codexProviderPath...)
	text = tomlSetRootKey(text, codexModelProviderKey, codexProviderName)
	if endpoint.Model != "" {
		text = tomlSetRootKey(text, codexModelKey, endpoint.Model)
	}
	text = tomlAppend(text, w.providerTable(endpoint))

	auth, _, err := readJSON(authPath)
	if err != nil {
		return nil, err
	}
	if value, ok := auth[codexAuthKey].(string); ok {
		previous[codexAuthStateKey] = value
	}
	auth[codexAuthKey] = endpoint.ApiKey
	authData, err := encodeJSON(auth)
	if err != nil {
		return nil, err
	}

	changes := &txn{}
	if err := changes.write(configPath, []byte(text)); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.write(authPath, authData); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w codexWriter) Restore(target Target, previous map[string]string) error {
	configPath, authPath, err := w.paths(target)
	if err != nil {
		return err
	}

	data, _, _, err := readFile(configPath)
	if err != nil {
		return err
	}
	text := tomlCutTable(string(data), codexProviderPath...)
	for _, key := range []string{codexModelProviderKey, codexModelKey} {
		if value, ok := previous[key]; ok {
			text = tomlSetRootKey(text, key, value)
		} else {
			text = tomlDeleteRootKey(text, key)
		}
	}
	text = strings.TrimLeft(text, "\n")

	auth, _, err := readJSON(authPath)
	if err != nil {
		return err
	}
	if value, ok := previous[codexAuthStateKey]; ok {
		auth[codexAuthKey] = value
	} else {
		delete(auth, codexAuthKey)
	}
	authData, err := encodeJSON(auth)
	if err != nil {
		return err
	}

	changes := &txn{}
	if err := changes.write(configPath, []byte(text)); err != nil {
		changes.abort()
		return err
	}
	if err := changes.write(authPath, authData); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w codexWriter) Current(target Target) (string, error) {
	configPath, _, err := w.paths(target)
	if err != nil {
		return "", err
	}
	data, _, exists, err := readFile(configPath)
	if err != nil || !exists {
		return "", err
	}
	document, err := w.decode(configPath, data)
	if err != nil {
		return "", err
	}

	selected, _ := document[codexModelProviderKey].(string)
	if selected == "" {
		return "", nil
	}
	providers, _ := document["model_providers"].(map[string]any)
	provider, _ := providers[selected].(map[string]any)
	if baseUrl, ok := provider["base_url"].(string); ok {
		return baseUrl, nil
	}
	// A provider without a base URL is one Codex knows itself, which is still
	// worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// providerTable is the [model_providers.casbin-gateway] block. The key lives in
// auth.json under env_key, so config.toml can be read without exposing it.
func (codexWriter) providerTable(endpoint Endpoint) string {
	return tomlTable(codexProviderPath,
		[]string{"name", "base_url", "wire_api", "env_key"},
		map[string]string{
			"name":     "Casbin Gateway",
			"base_url": endpoint.BaseUrl,
			"wire_api": "chat",
			"env_key":  codexAuthKey,
		})
}

func (codexWriter) decode(path string, data []byte) (map[string]any, error) {
	document := map[string]any{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return document, nil
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, nil
}

func (codexWriter) paths(target Target) (string, string, error) {
	home, err := agentmonitor.ResolveCodexHome(target.Path, target.Owner)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, "config.toml"), filepath.Join(home, "auth.json"), nil
}
