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
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// tomlStore holds MCP servers as sub-tables of one TOML table, which is how
// Codex keeps them in config.toml.
//
// Reading decodes the file, but writing edits its text: config.toml is a
// hand-written file with comments and formatting in it, and re-encoding a
// decoded document would hand it back stripped of both.
type tomlStore struct {
	table string
}

func (store *tomlStore) read(file string) (map[string]map[string]any, error) {
	data, _, err := readFile(file)
	if err != nil {
		return nil, err
	}

	document := map[string]any{}
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := toml.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
	}

	entries := map[string]map[string]any{}
	servers, ok := document[store.table].(map[string]any)
	if !ok {
		return entries, nil
	}
	for name, value := range servers {
		if entry, ok := value.(map[string]any); ok {
			entries[name] = entry
		}
	}
	return entries, nil
}

func (store *tomlStore) write(file string, name string, entry map[string]any) error {
	if err := checkName(name); err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("the definition of MCP server %q is empty", name)
	}

	block, err := store.render(name, entry)
	if err != nil {
		return err
	}

	data, mode, err := readFile(file)
	if err != nil {
		return err
	}
	text, _ := store.cut(string(data), name)

	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	return writeFile(file, []byte(text+block), mode)
}

func (store *tomlStore) remove(file string, name string) error {
	data, mode, err := readFile(file)
	if err != nil {
		return err
	}

	text, found := store.cut(string(data), name)
	if !found {
		return fmt.Errorf("no MCP server named %q in %s", name, file)
	}
	return writeFile(file, []byte(text), mode)
}

// cut removes one server's table and its sub-tables from the document text,
// leaving every other line, comment and blank line exactly as it was.
func (store *tomlStore) cut(text string, name string) (string, bool) {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropping := false
	found := false

	for _, line := range lines {
		header, isHeader := tableHeader(line)
		if isHeader {
			dropping = store.owns(header, name)
			found = found || dropping
		}
		if !dropping {
			kept = append(kept, line)
		}
	}

	result := strings.Join(kept, "\n")
	if found {
		result = strings.TrimRight(result, "\n") + "\n"
	}
	return result, found
}

// owns reports whether a table header belongs to one server: [mcp_servers.name]
// itself, or any sub-table of it such as [mcp_servers.name.env].
func (store *tomlStore) owns(header []string, name string) bool {
	return len(header) >= 2 && header[0] == store.table && header[1] == name
}

// render writes one server as TOML text. Scalars come first and sub-tables
// after, because everything following a sub-table header belongs to it.
func (store *tomlStore) render(name string, entry map[string]any) (string, error) {
	scalars := &strings.Builder{}
	tables := &strings.Builder{}

	fmt.Fprintf(scalars, "[%s.%s]\n", tomlKey(store.table), tomlKey(name))
	for _, key := range sortedKeys(entry) {
		value := entry[key]
		if nested, ok := value.(map[string]any); ok {
			fmt.Fprintf(tables, "\n[%s.%s.%s]\n", tomlKey(store.table), tomlKey(name), tomlKey(key))
			for _, nestedKey := range sortedKeys(nested) {
				text, err := tomlValue(nested[nestedKey])
				if err != nil {
					return "", fmt.Errorf("%s.%s: %w", key, nestedKey, err)
				}
				fmt.Fprintf(tables, "%s = %s\n", tomlKey(nestedKey), text)
			}
			continue
		}

		text, err := tomlValue(value)
		if err != nil {
			return "", fmt.Errorf("%s: %w", key, err)
		}
		fmt.Fprintf(scalars, "%s = %s\n", tomlKey(key), text)
	}
	return scalars.String() + tables.String(), nil
}

// tableHeader parses a [a.b.c] line into its dotted parts. Anything else, an
// array-of-tables header included, is not a header this package edits.
func tableHeader(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return nil, false
	}
	end := strings.LastIndex(trimmed, "]")
	if end <= 0 {
		return nil, false
	}
	// A comment may follow the header, but nothing else may.
	if rest := strings.TrimSpace(trimmed[end+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
		return nil, false
	}
	return splitTomlKey(trimmed[1:end]), true
}

// splitTomlKey splits a dotted key, honouring quoted parts and unquoting them,
// so ["my-server"] and [my-server] name the same table.
func splitTomlKey(key string) []string {
	parts := []string{}
	current := &strings.Builder{}
	quote := rune(0)

	for _, char := range key {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
				continue
			}
			current.WriteRune(char)
		case char == '"' || char == '\'':
			quote = char
		case char == '.':
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(char)
		}
	}
	return append(parts, strings.TrimSpace(current.String()))
}

// tomlKey quotes a key that is not a bare TOML key.
func tomlKey(key string) string {
	for _, char := range key {
		bare := char == '-' || char == '_' ||
			(char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
		if !bare {
			return quoteToml(key)
		}
	}
	if key == "" {
		return `""`
	}
	return key
}

func tomlValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return quoteToml(typed), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		// JSON has no integers, so a whole number arriving from a JSON config
		// has to go back out as one: Codex rejects 120.0 for a timeout.
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10), nil
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, err := tomlValue(item)
			if err != nil {
				return "", err
			}
			items = append(items, text)
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	case nil:
		return "", fmt.Errorf("a null value has no TOML equivalent")
	default:
		return "", fmt.Errorf("a %T value has no TOML equivalent", value)
	}
}

func quoteToml(value string) string {
	escaped := &strings.Builder{}
	escaped.WriteByte('"')
	for _, char := range value {
		switch char {
		case '"':
			escaped.WriteString(`\"`)
		case '\\':
			escaped.WriteString(`\\`)
		case '\n':
			escaped.WriteString(`\n`)
		case '\r':
			escaped.WriteString(`\r`)
		case '\t':
			escaped.WriteString(`\t`)
		default:
			if char < 0x20 {
				fmt.Fprintf(escaped, `\u%04X`, char)
				continue
			}
			escaped.WriteRune(char)
		}
	}
	escaped.WriteByte('"')
	return escaped.String()
}

func sortedKeys(entry map[string]any) []string {
	keys := make([]string, 0, len(entry))
	for key := range entry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
