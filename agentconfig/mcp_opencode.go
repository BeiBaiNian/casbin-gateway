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

package agentconfig

import (
	"errors"
	"fmt"
)

// opencodeStore holds MCP servers in the "mcp" object of opencode's config.
// opencode spells an entry its own way: the transport is always named, a local
// server's command and arguments are one list, and its environment is called
// something else. Entries are translated on the way in and back on the way out,
// so a server copied from another agent arrives in a shape opencode reads.
type opencodeStore struct {
	json jsonStore
}

func newOpencodeStore() *opencodeStore {
	return &opencodeStore{json: jsonStore{paths: [][]string{{"mcp"}}, relaxed: true}}
}

func (store *opencodeStore) read(file string) (map[string]map[string]any, error) {
	entries, err := store.json.read(file)
	if err != nil {
		return nil, err
	}
	for name, entry := range entries {
		entries[name] = opencodeCommonEntry(entry)
	}
	return entries, nil
}

func (store *opencodeStore) write(file string, name string, entry map[string]any) error {
	converted, err := opencodeEntry(entry)
	if err != nil {
		return err
	}
	return store.json.write(file, name, converted)
}

func (store *opencodeStore) remove(file string, name string) error {
	return store.json.remove(file, name)
}

// opencodeEntry is one server as opencode's config spells it.
func opencodeEntry(entry map[string]any) (map[string]any, error) {
	if entry == nil {
		return nil, errors.New("the definition of this MCP server is empty")
	}

	result := map[string]any{"enabled": true}
	if transportOf(entry) == TransportStdio {
		command := stringField(entry, "command")
		if command == "" {
			return nil, errors.New("a local MCP server needs a command to run")
		}
		arguments, _ := entry["args"].([]any)
		result["type"] = "local"
		result["command"] = append([]any{command}, arguments...)
		if environment, ok := entry["env"].(map[string]any); ok && len(environment) > 0 {
			result["environment"] = environment
		}
		return result, nil
	}

	url := stringField(entry, "url", "serverUrl", "endpoint")
	if url == "" {
		return nil, errors.New("a remote MCP server needs a URL")
	}
	result["type"] = "remote"
	result["url"] = url
	if headers, ok := entry["headers"].(map[string]any); ok && len(headers) > 0 {
		result["headers"] = headers
	}
	return result, nil
}

// opencodeCommonEntry is the same server in the spelling every other agent
// shares, which is what a listing and a copy to another agent read.
func opencodeCommonEntry(entry map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range entry {
		result[key] = value
	}

	switch stringField(entry, "type") {
	case "local":
		result["type"] = TransportStdio
		delete(result, "command")
		if command, ok := entry["command"].([]any); ok && len(command) > 0 {
			result["command"] = fmt.Sprint(command[0])
			if len(command) > 1 {
				result["args"] = command[1:]
			}
		}
		if environment, ok := result["environment"]; ok {
			result["env"] = environment
			delete(result, "environment")
		}
	case "remote":
		result["type"] = TransportHttp
	}
	return result
}
