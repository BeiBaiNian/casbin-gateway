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

package agentpatch

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const ingestTokenFile = "ingest-tokens.json"

// ingestToken is the credential one patched installation presents when it
// reports a record.
type ingestToken struct {
	Token       string `json:"token"`
	AgentId     string `json:"agentId"`
	CreatedTime string `json:"createdTime"`
}

type ingestTokenStore struct {
	mutex  sync.Mutex
	loaded bool
	tokens map[string]ingestToken
}

// tokens is guarded by its own mutex: patchers call into it while they already
// hold stateMutex, so it must never reach back for that lock.
var tokens = &ingestTokenStore{tokens: map[string]ingestToken{}}

// IssueIngestToken returns the reporting credential for target, minting and
// persisting a new one when the target has none. Re-patching an installation
// deliberately keeps the existing token so that a hook left behind by a crashed
// patch run does not start failing.
func IssueIngestToken(target Target) (string, error) {
	return tokens.issue(target)
}

// RevokeIngestToken invalidates the credential of one installation. Records
// reported with it afterwards are rejected.
func RevokeIngestToken(target Target) error {
	return tokens.revoke(target)
}

// ValidateIngestToken reports whether presented matches a live installation
// credential, and returns the agent it was issued for.
func ValidateIngestToken(presented string) (string, bool) {
	return tokens.validate(presented)
}

func (store *ingestTokenStore) issue(target Target) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if err := store.loadLocked(); err != nil {
		return "", err
	}

	key := targetKey(target)
	if existing, found := store.tokens[key]; found && existing.Token != "" {
		return existing.Token, nil
	}
	secret, err := newSecret()
	if err != nil {
		return "", err
	}
	store.tokens[key] = ingestToken{
		Token:       secret,
		AgentId:     target.AgentId,
		CreatedTime: time.Now().Format(time.RFC3339),
	}
	if err := store.saveLocked(); err != nil {
		delete(store.tokens, key)
		return "", err
	}
	return secret, nil
}

func (store *ingestTokenStore) revoke(target Target) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if err := store.loadLocked(); err != nil {
		return err
	}

	key := targetKey(target)
	previous, found := store.tokens[key]
	if !found {
		return nil
	}
	delete(store.tokens, key)
	if err := store.saveLocked(); err != nil {
		store.tokens[key] = previous
		return err
	}
	return nil
}

func (store *ingestTokenStore) validate(presented string) (string, bool) {
	if presented == "" {
		return "", false
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()
	if err := store.loadLocked(); err != nil {
		return "", false
	}
	for _, issued := range store.tokens {
		if subtle.ConstantTimeCompare([]byte(issued.Token), []byte(presented)) == 1 {
			return issued.AgentId, true
		}
	}
	return "", false
}

func (store *ingestTokenStore) loadLocked() error {
	if store.loaded {
		return nil
	}
	data, err := os.ReadFile(ingestTokenPath())
	if os.IsNotExist(err) {
		store.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	var saved struct {
		Tokens map[string]ingestToken `json:"tokens"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("parse agent ingest tokens: %w", err)
	}
	if saved.Tokens != nil {
		store.tokens = saved.Tokens
	}
	store.loaded = true
	return nil
}

func (store *ingestTokenStore) saveLocked() error {
	path := ingestTokenPath()
	if len(store.tokens) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(map[string]any{"tokens": store.tokens}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func ingestTokenPath() string {
	return filepath.Join(stateDir(), ingestTokenFile)
}

func newSecret() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate agent ingest token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
