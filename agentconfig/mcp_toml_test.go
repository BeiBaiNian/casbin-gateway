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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const codexConfig = `# the operator's own notes
model = "gpt-5"

[features]
js_repl = false

[mcp_servers.node_repl]
command = 'C:\bin\node_repl.exe'
args = []
startup_timeout_sec = 120

[mcp_servers.node_repl.env]
NODE_REPL_NODE_PATH = 'C:\bin\node.exe'

[desktop]
followUpQueueMode = "queue"
`

func codexFile(t *testing.T, content string) (*tomlStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return &tomlStore{table: "mcp_servers"}, path
}

func TestTomlReadsServersAndTheirEnvironment(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	entries, err := store.read(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := entries["node_repl"]
	if !ok {
		t.Fatalf("entries = %v", entries)
	}
	if entry["command"] != `C:\bin\node_repl.exe` {
		t.Fatalf("command = %v", entry["command"])
	}
	environment, ok := entry["env"].(map[string]any)
	if !ok || environment["NODE_REPL_NODE_PATH"] != `C:\bin\node.exe` {
		t.Fatalf("env = %v", entry["env"])
	}
}

func TestTomlRemoveKeepsCommentsAndOtherTables(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	if err := store.remove(path, "node_repl"); err != nil {
		t.Fatal(err)
	}

	content := readBack(t, path)
	if strings.Contains(content, "mcp_servers") {
		t.Fatalf("the table survived: %s", content)
	}
	for _, kept := range []string{"# the operator's own notes", `model = "gpt-5"`, "[features]", "[desktop]"} {
		if !strings.Contains(content, kept) {
			t.Fatalf("remove dropped %q: %s", kept, content)
		}
	}
}

func TestTomlRemoveReportsAMissingServer(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	if err := store.remove(path, "absent"); err == nil {
		t.Fatal("removing a server that is not there succeeded")
	}
	if readBack(t, path) != codexConfig {
		t.Fatal("a failed remove rewrote the file")
	}
}

func TestTomlWriteReplacesOneServerInPlace(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	entry := map[string]any{
		"command":             "npx",
		"args":                []any{"-y", "@modelcontextprotocol/server-files"},
		"startup_timeout_sec": float64(30),
		"env":                 map[string]any{"API_KEY": "secret"},
	}
	if err := store.write(path, "node_repl", entry); err != nil {
		t.Fatal(err)
	}

	content := readBack(t, path)
	if strings.Contains(content, "node_repl.exe") {
		t.Fatalf("the old definition survived: %s", content)
	}
	if !strings.Contains(content, "startup_timeout_sec = 30") {
		t.Fatalf("a whole number was written as a float: %s", content)
	}
	if !strings.Contains(content, "[desktop]") {
		t.Fatalf("write dropped a later table: %s", content)
	}

	entries, err := store.read(path)
	if err != nil {
		t.Fatal(err)
	}
	written := entries["node_repl"]
	if written["command"] != "npx" {
		t.Fatalf("command = %v", written["command"])
	}
	arguments, _ := written["args"].([]any)
	if len(arguments) != 2 || arguments[0] != "-y" {
		t.Fatalf("args = %v", written["args"])
	}
	environment, _ := written["env"].(map[string]any)
	if environment["API_KEY"] != "secret" {
		t.Fatalf("env = %v", written["env"])
	}
}

func TestTomlWriteCreatesAMissingFile(t *testing.T) {
	store := &tomlStore{table: "mcp_servers"}
	path := filepath.Join(t.TempDir(), "nested", "config.toml")

	if err := store.write(path, "files", map[string]any{"command": "npx"}); err != nil {
		t.Fatal(err)
	}
	entries, err := store.read(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries["files"]["command"] != "npx" {
		t.Fatalf("entries = %v", entries)
	}
}

func TestTomlQuotesANameThatIsNotABareKey(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	if err := store.write(path, "my server", map[string]any{"command": "npx"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readBack(t, path), `[mcp_servers."my server"]`) {
		t.Fatalf("content = %s", readBack(t, path))
	}

	if err := store.remove(path, "my server"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readBack(t, path), "my server") {
		t.Fatalf("a quoted name was not matched on remove: %s", readBack(t, path))
	}
}

func TestTomlRejectsAValueItCannotWrite(t *testing.T) {
	store, path := codexFile(t, codexConfig)

	err := store.write(path, "files", map[string]any{"command": "npx", "deep": map[string]any{
		"nested": map[string]any{"too": "far"},
	}})
	if err == nil {
		t.Fatal("a value with no TOML equivalent was accepted")
	}
	if readBack(t, path) != codexConfig {
		t.Fatal("a rejected write still changed the file")
	}
}

func TestFrontMatterReadsFoldedAndQuotedFields(t *testing.T) {
	name, description := parseFrontMatter("---\nname: babysit\ndescription: >-\n  Keep a PR merge-ready by\n  fixing CI in a loop.\n---\n# body\n")
	if name != "babysit" {
		t.Fatalf("name = %q", name)
	}
	if description != "Keep a PR merge-ready by fixing CI in a loop." {
		t.Fatalf("description = %q", description)
	}

	_, quoted := parseFrontMatter("---\nname: a\ndescription: 'Work with PDFs'\n---\n")
	if quoted != "Work with PDFs" {
		t.Fatalf("description = %q", quoted)
	}

	if _, none := parseFrontMatter("# no front matter\ndescription: not a field\n"); none != "" {
		t.Fatalf("description = %q", none)
	}
}
