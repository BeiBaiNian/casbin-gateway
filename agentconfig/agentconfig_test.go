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

// fakeHome points the whole package at a temporary directory, so a test reads
// and writes agent configuration without touching the account running it.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeFileAt(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeSkill(t *testing.T, dir string, name string, description string) {
	t.Helper()
	writeFileAt(t, filepath.Join(dir, name, skillFile),
		"---\nname: "+name+"\ndescription: "+description+"\n---\n# "+name+"\n")
}

func TestReadListsSkillsAndServers(t *testing.T) {
	home := fakeHome(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "pdf", "Work with PDFs")
	writeFileAt(t, filepath.Join(home, ".claude", "skills", "pdf", "references", "spec.md"), "spec")
	writeFileAt(t, filepath.Join(home, ".claude.json"),
		`{"projects":{"a":1},"mcpServers":{"files":{"command":"npx","args":["-y","server"]}}}`)

	inventory := Read("claude-code", "")
	if len(inventory.Errors) > 0 {
		t.Fatalf("read reported errors: %v", inventory.Errors)
	}
	if len(inventory.Skills) != 1 || inventory.Skills[0].Name != "pdf" {
		t.Fatalf("skills = %+v", inventory.Skills)
	}
	if inventory.Skills[0].Description != "Work with PDFs" || inventory.Skills[0].Files != 2 {
		t.Fatalf("skill = %+v", inventory.Skills[0])
	}
	if len(inventory.McpServers) != 1 {
		t.Fatalf("servers = %+v", inventory.McpServers)
	}
	server := inventory.McpServers[0]
	if server.Name != "files" || server.Transport != "stdio" || server.Command != "npx -y server" {
		t.Fatalf("server = %+v", server)
	}
}

func TestReadSkipsDirectoriesWithoutAManifest(t *testing.T) {
	home := fakeHome(t)
	skills := filepath.Join(home, ".claude", "skills")
	writeSkill(t, skills, "real", "kept")
	writeFileAt(t, filepath.Join(skills, "notes", "README.md"), "not a skill")
	writeFileAt(t, filepath.Join(skills, ".managed", skillFile), "---\nname: hidden\n---\n")

	inventory := Read("claude-code", "")
	if len(inventory.Skills) != 1 || inventory.Skills[0].Name != "real" {
		t.Fatalf("skills = %+v", inventory.Skills)
	}
}

func TestReadReportsAnUnparsableFileWithoutLosingSkills(t *testing.T) {
	home := fakeHome(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "pdf", "Work with PDFs")
	writeFileAt(t, filepath.Join(home, ".claude.json"), "{ not json")

	inventory := Read("claude-code", "")
	if len(inventory.Errors) != 1 {
		t.Fatalf("errors = %v", inventory.Errors)
	}
	if len(inventory.Skills) != 1 {
		t.Fatalf("a broken MCP file hid the skills: %+v", inventory.Skills)
	}
}

func TestOpenClawServersAreReadUnderEitherSpelling(t *testing.T) {
	home := fakeHome(t)
	writeFileAt(t, filepath.Join(home, ".openclaw", "openclaw.json"),
		`{"mcp":{"servers":{"a":{"command":"a"}}},"mcpServers":{"b":{"url":"https://b"}}}`)

	inventory := Read("openclaw", "")
	if len(inventory.McpServers) != 2 {
		t.Fatalf("servers = %+v", inventory.McpServers)
	}
	if inventory.McpServers[1].Transport != "http" {
		t.Fatalf("server = %+v", inventory.McpServers[1])
	}
}

func TestDeleteRemovesOneServerAndKeepsTheRestOfTheFile(t *testing.T) {
	home := fakeHome(t)
	path := filepath.Join(home, ".cursor", "mcp.json")
	writeFileAt(t, path, `{"other":{"keep":true},"mcpServers":{"a":{"command":"a"},"b":{"command":"b"}}}`)

	if err := Delete("cursor", "", KindMcp, "a"); err != nil {
		t.Fatal(err)
	}

	content := readBack(t, path)
	if strings.Contains(content, `"a"`) || !strings.Contains(content, `"b"`) {
		t.Fatalf("file after delete: %s", content)
	}
	if !strings.Contains(content, `"keep"`) {
		t.Fatalf("delete dropped an unrelated key: %s", content)
	}
}

func TestDeleteSkillRefusesADirectoryThatIsNotASkill(t *testing.T) {
	home := fakeHome(t)
	writeFileAt(t, filepath.Join(home, ".claude", "skills", "notes", "README.md"), "not a skill")

	if err := Delete("claude-code", "", KindSkill, "notes"); err == nil {
		t.Fatal("a directory without a manifest was deleted")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "notes")); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRejectsATraversingName(t *testing.T) {
	fakeHome(t)
	for _, name := range []string{"..", "../escape", `..\escape`, "a/b", ""} {
		if err := Delete("claude-code", "", KindSkill, name); err == nil {
			t.Fatalf("name %q was accepted", name)
		}
	}
}

func TestCopySkillsBetweenAgents(t *testing.T) {
	home := fakeHome(t)
	source := filepath.Join(home, ".claude", "skills")
	writeSkill(t, source, "pdf", "Work with PDFs")
	writeFileAt(t, filepath.Join(source, "pdf", "references", "spec.md"), "spec")
	writeSkill(t, source, "xlsx", "Work with spreadsheets")
	writeSkill(t, filepath.Join(home, ".codex", "skills"), "pdf", "the older copy")

	request := CopyRequest{
		From:  "claude-code",
		To:    []string{"codex-cli"},
		Kind:  KindSkill,
		Names: []string{"pdf", "xlsx"},
	}

	planned, err := Plan(request)
	if err != nil {
		t.Fatal(err)
	}
	if planned[0].Action != ActionSkip || planned[1].Action != ActionCreate {
		t.Fatalf("plan = %+v %+v", planned[0], planned[1])
	}

	request.Overwrite = true
	result, err := Copy(request)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Action != ActionOverwrite || result[1].Action != ActionCreate {
		t.Fatalf("result = %+v %+v", result[0], result[1])
	}

	target := filepath.Join(home, ".codex", "skills")
	if content := readBack(t, filepath.Join(target, "pdf", skillFile)); !strings.Contains(content, "Work with PDFs") {
		t.Fatalf("the existing skill was not replaced: %s", content)
	}
	if content := readBack(t, filepath.Join(target, "pdf", "references", "spec.md")); content != "spec" {
		t.Fatalf("reference file = %q", content)
	}
	if _, err := os.Stat(filepath.Join(target, "xlsx", skillFile)); err != nil {
		t.Fatal(err)
	}
}

func TestCopyLeavesTheSourceInPlace(t *testing.T) {
	home := fakeHome(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "pdf", "Work with PDFs")

	_, err := Copy(CopyRequest{From: "claude-code", To: []string{"cursor"}, Kind: KindSkill, Names: []string{"pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "pdf", skillFile)); err != nil {
		t.Fatal(err)
	}
}

func TestCopyReportsAFailureItemByItem(t *testing.T) {
	home := fakeHome(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "pdf", "Work with PDFs")

	result, err := Copy(CopyRequest{
		From:  "claude-code",
		To:    []string{"claude-desktop", "cursor"},
		Kind:  KindSkill,
		Names: []string{"pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Action != ActionSkip || !strings.Contains(result[1].Path, "skills-cursor") {
		t.Fatalf("result = %+v %+v", result[0], result[1])
	}
}

func TestCopyRejectsTheSourceAsItsOwnTarget(t *testing.T) {
	fakeHome(t)
	_, err := Copy(CopyRequest{From: "cursor", To: []string{"cursor"}, Kind: KindSkill, Names: []string{"pdf"}})
	if err == nil {
		t.Fatal("an agent was allowed to copy onto itself")
	}
}

func TestCopyServersIntoAJsonConfig(t *testing.T) {
	home := fakeHome(t)
	writeFileAt(t, filepath.Join(home, ".claude.json"),
		`{"mcpServers":{"files":{"command":"npx","args":["-y","server"],"env":{"API_KEY":"secret"}}}}`)
	writeFileAt(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{}}`)

	result, err := Copy(CopyRequest{From: "claude-code", To: []string{"cursor"}, Kind: KindMcp, Names: []string{"files"}})
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Action != ActionCreate {
		t.Fatalf("result = %+v", result[0])
	}

	content := readBack(t, filepath.Join(home, ".cursor", "mcp.json"))
	if !strings.Contains(content, `"API_KEY": "secret"`) {
		t.Fatalf("the copy lost the environment: %s", content)
	}
}

func TestDetailMasksCredentialsButCopyDoesNot(t *testing.T) {
	home := fakeHome(t)
	writeFileAt(t, filepath.Join(home, ".cursor", "mcp.json"),
		`{"mcpServers":{"files":{"command":"npx","env":{"API_KEY":"secret"}}}}`)

	detail, err := ReadDetail("cursor", "", KindMcp, "files")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(detail.Content, "secret") {
		t.Fatalf("the detail view leaked a credential: %s", detail.Content)
	}

	if _, err := Copy(CopyRequest{From: "cursor", To: []string{"claude-code"}, Kind: KindMcp, Names: []string{"files"}}); err != nil {
		t.Fatal(err)
	}
	if content := readBack(t, filepath.Join(home, ".claude.json")); !strings.Contains(content, "secret") {
		t.Fatalf("the copy wrote the masked value: %s", content)
	}
}

func TestManagedEntriesAreNotMigrated(t *testing.T) {
	home := fakeHome(t)
	writeFileAt(t, filepath.Join(home, ".claude.json"),
		`{"mcpServers":{"`+ManagedEntryName+`":{"command":"gateway"}}}`)

	planned, err := Plan(CopyRequest{
		From: "claude-code", To: []string{"cursor"}, Kind: KindMcp, Names: []string{ManagedEntryName},
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned[0].Action != ActionSkip {
		t.Fatalf("plan = %+v", planned[0])
	}
}

func TestUnsupportedKindIsAnError(t *testing.T) {
	fakeHome(t)
	if _, err := ReadDetail("windsurf", "", KindSkill, "pdf"); err == nil {
		t.Fatal("an agent with no skills directory answered a skill lookup")
	}
	if inventory := Read("windsurf", ""); inventory.SkillsSupported {
		t.Fatal("windsurf reported skill support")
	}
}

func readBack(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
