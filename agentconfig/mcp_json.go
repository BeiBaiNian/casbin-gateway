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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/apache/casbin-gateway/internal/jsonc"
)

// jsonStore holds MCP servers in a JSON object somewhere in a JSON config file.
// paths are the places that object has been spelled; all of them are read, and
// the first is where a new entry goes when the file has none of them yet.
type jsonStore struct {
	paths [][]string
	// relaxed reads a file whose agent accepts comments and trailing commas.
	// Rewriting such a file drops the comments, which is the price of editing
	// it at all.
	relaxed bool
}

func (store *jsonStore) read(file string) (map[string]map[string]any, error) {
	config, _, _, err := store.load(file)
	if err != nil {
		return nil, err
	}

	entries := map[string]map[string]any{}
	for _, path := range store.paths {
		servers, ok := objectAt(config, path)
		if !ok {
			continue
		}
		for name, value := range servers {
			entry, ok := value.(map[string]any)
			if !ok || entries[name] != nil {
				continue
			}
			entries[name] = entry
		}
	}
	return entries, nil
}

func (store *jsonStore) write(file string, name string, entry map[string]any) error {
	if err := checkName(name); err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("the definition of MCP server %q is empty", name)
	}

	config, mode, _, err := store.load(file)
	if err != nil {
		return err
	}

	servers, err := ensureObject(config, store.writePath(config))
	if err != nil {
		return err
	}
	servers[name] = entry
	return store.save(file, config, mode)
}

func (store *jsonStore) remove(file string, name string) error {
	config, mode, exists, err := store.load(file)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s does not exist", file)
	}

	removed := false
	for _, path := range store.paths {
		servers, ok := objectAt(config, path)
		if !ok {
			continue
		}
		if _, found := servers[name]; found {
			delete(servers, name)
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("no MCP server named %q in %s", name, file)
	}
	return store.save(file, config, mode)
}

// writePath is the path a new entry goes to: the one the file already uses, so
// an edit does not add a second spelling the agent may not merge.
func (store *jsonStore) writePath(config map[string]any) []string {
	for _, path := range store.paths {
		if _, ok := objectAt(config, path); ok {
			return path
		}
	}
	return store.paths[0]
}

func (store *jsonStore) load(file string) (map[string]any, os.FileMode, bool, error) {
	data, mode, err := readFile(file)
	if err != nil {
		return nil, 0, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, mode, len(data) > 0, nil
	}
	if store.relaxed {
		data = jsonc.Strip(data)
	}

	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, 0, false, fmt.Errorf("parse %s: %w", file, err)
	}
	return config, mode, true, nil
}

func (store *jsonStore) save(file string, config map[string]any, mode os.FileMode) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(file, append(data, '\n'), mode)
}

func objectAt(config map[string]any, path []string) (map[string]any, bool) {
	current := config
	for index, key := range path {
		value, ok := current[key]
		if !ok {
			return nil, false
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		if index == len(path)-1 {
			return object, true
		}
		current = object
	}
	return nil, false
}

// ensureObject walks path, creating the objects that are missing. A key already
// holding something that is not an object is an error rather than a value to
// overwrite: that is somebody else's setting.
func ensureObject(config map[string]any, path []string) (map[string]any, error) {
	current := config
	for _, key := range path {
		value, ok := current[key]
		if !ok {
			created := map[string]any{}
			current[key] = created
			current = created
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not a JSON object", strings.Join(path, "."))
		}
		current = object
	}
	return current, nil
}
