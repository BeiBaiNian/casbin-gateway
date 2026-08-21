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
	"runtime"
	"sort"

	"github.com/apache/casbin-gateway/agenthome"
)

// ManagedEntryName is the MCP server Gateway registers for its own monitoring.
// It is listed like any other so an operator can see it, but it is never
// migrated or deleted from here: turning monitoring off is what removes it.
const ManagedEntryName = "casbin-gateway-agent-monitor"

// layout is where one agent keeps its skills and MCP servers. A nil half means
// the agent has no such location that Gateway knows about, which the UI shows
// as unsupported rather than as empty.
type layout struct {
	skills *skillLayout
	mcp    *mcpLayout
}

// skillLayout is a directory of skill folders, each with a SKILL.md. Every
// agent Gateway supports uses that same layout, which is what makes copying a
// skill between two of them a plain directory copy.
type skillLayout struct {
	segments []string
}

func (l *skillLayout) dir(home string) string {
	return filepath.Join(append([]string{home}, l.segments...)...)
}

// mcpLayout is one config file holding named MCP server entries.
type mcpLayout struct {
	file  func(home string) string
	store mcpStore
	// readOnly explains, in the operator's words, why Gateway will not write
	// this file. Empty for the files it may write.
	readOnly string
}

func (l *mcpLayout) path(home string) string {
	return l.file(home)
}

// mcpStore is the file format half: JSON objects for most agents, TOML tables
// for Codex. Entries are keyed by server name and hold that server's own
// fields, in whatever spelling the file used.
type mcpStore interface {
	read(file string) (map[string]map[string]any, error)
	write(file string, name string, entry map[string]any) error
	remove(file string, name string) error
}

// layouts is the whole of Gateway's per-agent knowledge. The Codex CLI, the
// Codex VS Code extension and the ChatGPT desktop app share one CODEX_HOME, and
// Cursor and its CLI share one ~/.cursor, so those ids share a layout.
var layouts = map[string]layout{
	"claude-code": {
		skills: &skillLayout{segments: []string{".claude", "skills"}},
		mcp:    &mcpLayout{file: under(".claude.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
	},
	"claude-desktop": {
		mcp: &mcpLayout{file: claudeDesktopConfig, store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
	},
	"codex":        codexLayout,
	"codex-cli":    codexLayout,
	"codex-vscode": codexLayout,
	"codex_vscode": codexLayout,
	"cursor":       cursorLayout,
	"cursor-agent": cursorLayout,
	"windsurf": {
		mcp: &mcpLayout{
			file:  under(".codeium", "windsurf", "mcp_config.json"),
			store: &jsonStore{paths: [][]string{{"mcpServers"}}},
		},
	},
	"openclaw": {
		skills: &skillLayout{segments: []string{".openclaw", "skills"}},
		mcp: &mcpLayout{
			file: under(".openclaw", "openclaw.json"),
			// OpenClaw has spelled this three ways across versions, and reads
			// all three; the first is where a new entry goes.
			store: &jsonStore{paths: [][]string{{"mcp", "servers"}, {"mcp", "mcpServers"}, {"mcpServers"}}},
		},
	},
}

var codexLayout = layout{
	skills: &skillLayout{segments: []string{".codex", "skills"}},
	mcp:    &mcpLayout{file: under(".codex", "config.toml"), store: &tomlStore{table: "mcp_servers"}},
}

var cursorLayout = layout{
	skills: &skillLayout{segments: []string{".cursor", "skills-cursor"}},
	mcp:    &mcpLayout{file: under(".cursor", "mcp.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
}

func layoutOf(agentId string) (layout, bool) {
	found, ok := layouts[agentId]
	return found, ok
}

// Supports reports whether Gateway knows where agentId keeps either kind of
// configuration, so a caller can skip an installation before resolving a home.
func Supports(agentId string) bool {
	_, ok := layouts[agentId]
	return ok
}

// McpConfigPath is where agentId keeps its MCP servers, for the account whose
// home directory is home. Callers outside this package use it to edit one entry
// of a file this package otherwise owns.
func McpConfigPath(agentId string, home string) (string, bool) {
	found, ok := layouts[agentId]
	if !ok || found.mcp == nil {
		return "", false
	}
	return found.mcp.path(home), true
}

func under(segments ...string) func(string) string {
	return func(home string) string {
		return filepath.Join(append([]string{home}, segments...)...)
	}
}

func claudeDesktopConfig(home string) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
}

func homeOf(owner string) (string, error) {
	return agenthome.Resolve(owner)
}

// KnownAgents lists the agent ids Gateway has a layout for, in a stable order.
func KnownAgents() []string {
	ids := make([]string, 0, len(layouts))
	for id := range layouts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Configured reports whether agentId has either of its locations on disk for
// owner. An agent can be set up on an account without Gateway recognizing an
// installation of it, and those skills are just as real as any other.
func Configured(agentId string, owner string) bool {
	found, ok := layouts[agentId]
	if !ok {
		return false
	}
	home, err := homeOf(owner)
	if err != nil {
		return false
	}

	if found.skills != nil && exists(found.skills.dir(home)) {
		return true
	}
	return found.mcp != nil && exists(found.mcp.path(home))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
