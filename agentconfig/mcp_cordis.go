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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// cordisMcpPlugin is the plugin one MCP server is mounted through.
	cordisMcpPlugin = "@deepseek-ai/dsh-mcp-client"
	// cordisRowPrefix names the rows Gateway adds, so a server is recognisable
	// in a file the owner also edits.
	cordisRowPrefix = "mcp-"
)

// cordisServerName is what dsh accepts as a server namespace; a name outside it
// fails the whole plugin instance at load rather than only that server.
var cordisServerName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// cordisStore holds MCP servers in a Cordis patch list. dsh has no map of
// servers: it mounts one plugin instance per server, and the user's patch file
// is where instances are added to the composed tree.
type cordisStore struct{}

func (store *cordisStore) read(file string) (map[string]map[string]any, error) {
	_, rows, err := store.load(file)
	if err != nil {
		return nil, err
	}

	entries := map[string]map[string]any{}
	for _, row := range rows {
		name := yamledit.String(row, "config", "serverName")
		if name == "" || entries[name] != nil {
			continue
		}
		entries[name] = configEntry(yamledit.Get(row, "config"))
	}
	return entries, nil
}

func (store *cordisStore) write(file string, name string, entry map[string]any) error {
	if err := checkName(name); err != nil {
		return err
	}
	if !cordisServerName.MatchString(name) {
		return fmt.Errorf("DeepSeek Harness only accepts letters, digits, %q and %q in a server name, so %q cannot be added", "-", "_", name)
	}
	if entry == nil {
		return fmt.Errorf("the definition of MCP server %q is empty", name)
	}

	document, rows, err := store.load(file)
	if err != nil {
		return err
	}
	config, err := yamledit.Node(cordisConfig(name, entry))
	if err != nil {
		return err
	}

	for _, row := range rows {
		if yamledit.String(row, "config", "serverName") != name {
			continue
		}
		if err := yamledit.Set(row, config, "config"); err != nil {
			return err
		}
		return store.save(file, document)
	}

	row, err := yamledit.Node(map[string]any{"id": cordisRowPrefix + name, "name": cordisMcpPlugin})
	if err != nil {
		return err
	}
	if err := yamledit.Set(row, config, "config"); err != nil {
		return err
	}
	if err := store.insert(document, row); err != nil {
		return err
	}
	return store.save(file, document)
}

func (store *cordisStore) remove(file string, name string) error {
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("%s does not exist", file)
	}

	document, _, err := store.load(file)
	if err != nil {
		return err
	}
	patches, err := document.Sequence()
	if err != nil {
		return err
	}

	removed := false
	kept := patches.Content[:0]
	for _, patch := range patches.Content {
		inserts := yamledit.Get(patch, "insert")
		if inserts != nil && inserts.Kind == yaml.SequenceNode {
			rows := inserts.Content[:0]
			for _, row := range inserts.Content {
				if isMcpRow(row) && yamledit.String(row, "config", "serverName") == name {
					removed = true
					continue
				}
				rows = append(rows, row)
			}
			inserts.Content = rows
			// A patch whose only purpose was the server it no longer inserts is
			// not a patch worth keeping.
			if len(rows) == 0 && len(patch.Content) == 2 {
				continue
			}
		}
		kept = append(kept, patch)
	}
	patches.Content = kept

	if !removed {
		return fmt.Errorf("no MCP server named %q in %s", name, file)
	}
	return store.save(file, document)
}

// insert adds a row to the patch list, reusing the untargeted insert patch the
// file already has so one server per patch does not accumulate.
func (store *cordisStore) insert(document *yamledit.Document, row *yaml.Node) error {
	patches, err := document.Sequence()
	if err != nil {
		return err
	}
	for _, patch := range patches.Content {
		inserts := yamledit.Get(patch, "insert")
		if inserts == nil || inserts.Kind != yaml.SequenceNode || yamledit.Get(patch, "id") != nil {
			continue
		}
		inserts.Style = 0
		inserts.Content = append(inserts.Content, row)
		return nil
	}

	patch, err := yamledit.Node(map[string]any{"insert": []any{}})
	if err != nil {
		return err
	}
	inserts := yamledit.Get(patch, "insert")
	inserts.Style = 0
	inserts.Content = append(inserts.Content, row)
	patches.Style = 0
	patches.Content = append(patches.Content, patch)
	return nil
}

// load returns the document and every MCP row in it, in file order.
func (store *cordisStore) load(file string) (*yamledit.Document, []*yaml.Node, error) {
	data, _, err := readFile(file)
	if err != nil {
		return nil, nil, err
	}
	document, err := yamledit.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", file, err)
	}
	patches, err := document.Sequence()
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", file, err)
	}

	var rows []*yaml.Node
	for _, patch := range patches.Content {
		inserts := yamledit.Get(patch, "insert")
		if inserts == nil || inserts.Kind != yaml.SequenceNode {
			continue
		}
		for _, row := range inserts.Content {
			if isMcpRow(row) {
				rows = append(rows, row)
			}
		}
	}
	return document, rows, nil
}

func (store *cordisStore) save(file string, document *yamledit.Document) error {
	data, err := document.Bytes()
	if err != nil {
		return err
	}
	_, mode, err := readFile(file)
	if err != nil {
		return err
	}
	return writeFile(file, data, mode)
}

func isMcpRow(row *yaml.Node) bool {
	return row.Kind == yaml.MappingNode && yamledit.String(row, "name") == cordisMcpPlugin
}

// cordisConfig is one server as the plugin's own config spells it: the name is
// a field rather than a key, and the transport is named rather than implied.
//
// The values dsh types are spelled as strings, and a config it refuses stops
// the whole agent from starting rather than only that server, so an entry
// written for an agent that allows a number here is narrowed on the way in.
func cordisConfig(name string, entry map[string]any) map[string]any {
	config := map[string]any{}
	for key, value := range entry {
		config[key] = value
	}

	transport := cordisTransport(config)
	delete(config, "type")
	config["serverName"] = name
	config["transport"] = transport
	if arguments, ok := config["args"].([]any); ok {
		config["args"] = stringList(arguments)
	}
	for _, key := range []string{"env", "headers"} {
		if values, ok := config[key].(map[string]any); ok {
			config[key] = stringMap(values)
		}
	}
	return config
}

func stringList(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, fmt.Sprint(value))
	}
	return result
}

func stringMap(values map[string]any) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = fmt.Sprint(value)
	}
	return result
}

// cordisTransport maps the spellings other agents use onto the two dsh takes,
// falling back to how the server is reached when the entry names neither.
func cordisTransport(entry map[string]any) string {
	switch declared := strings.ToLower(strings.TrimSpace(stringField(entry, "transport", "type"))); declared {
	case "stdio":
		return "stdio"
	case "":
		if stringField(entry, "command") != "" {
			return "stdio"
		}
		return "streamable-http"
	default:
		return "streamable-http"
	}
}

// configEntry reads a server's config back. A field written as a Cordis
// expression cannot be read as data, and is left out rather than failing the
// listing of every server beside it.
func configEntry(config *yaml.Node) map[string]any {
	entry := map[string]any{}
	if config == nil || config.Kind != yaml.MappingNode {
		return entry
	}
	for index := 0; index+1 < len(config.Content); index += 2 {
		var value any
		if config.Content[index+1].Decode(&value) == nil {
			entry[config.Content[index].Value] = value
		}
	}
	return entry
}
