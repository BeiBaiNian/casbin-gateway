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

// Package agentconfig reads and edits the skills and MCP servers that AI coding
// agents keep in their own configuration files, so the agents installed on one
// host can be compared and their configuration moved between them.
//
// Every agent stores the same two things in a different place and a different
// format. layout.go holds that per-agent knowledge, and nothing else does.
package agentconfig

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// The two kinds of configuration Gateway manages.
const (
	KindSkill = "skill"
	KindMcp   = "mcp"
)

// ErrUnsupported reports a kind an agent has no known location for, which is
// different from an agent that has one and it is empty.
var ErrUnsupported = errors.New("Gateway does not know where this agent keeps that configuration")

// Item is one skill or MCP server as it exists in an agent's own configuration.
type Item struct {
	AgentId string `json:"agentId"`
	Owner   string `json:"owner"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`

	Description string `json:"description,omitempty"`

	// Path is what an edit would change: the skill's own directory, or the
	// config file the MCP server is one entry of.
	Path string `json:"path"`

	// Transport, Command and Url summarize an MCP server in a list.
	Transport string `json:"transport,omitempty"`
	Command   string `json:"command,omitempty"`
	Url       string `json:"url,omitempty"`

	// Files and Bytes summarize a skill in a list.
	Files int   `json:"files,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`

	// Managed marks an entry Gateway wrote itself, which is not the operator's
	// to migrate and is removed by turning monitoring off instead.
	Managed bool `json:"managed,omitempty"`
}

// Inventory is everything Gateway can read for one installation. A location it
// cannot read leaves the corresponding list empty and adds to Errors, so one
// unreadable file never hides the rest of the host.
type Inventory struct {
	AgentId string `json:"agentId"`
	Owner   string `json:"owner"`
	// Home is the account directory the locations below were resolved under.
	// Two agents can be compared and copied between exactly when they share it.
	Home string `json:"home,omitempty"`

	SkillsDir string `json:"skillsDir,omitempty"`
	McpFile   string `json:"mcpFile,omitempty"`

	SkillsSupported bool `json:"skillsSupported"`
	McpSupported    bool `json:"mcpSupported"`
	// McpWritable is false for a config Gateway can read but must not rewrite,
	// because writing it back would lose something it cannot represent.
	McpWritable bool   `json:"mcpWritable"`
	McpReadOnly string `json:"mcpReadOnly,omitempty"`

	Skills     []*Item  `json:"skills"`
	McpServers []*Item  `json:"mcpServers"`
	Errors     []string `json:"errors,omitempty"`
}

// Detail is one item's full definition, for the viewer.
type Detail struct {
	Item *Item `json:"item"`
	// Content is the SKILL.md of a skill, or the JSON of one MCP server entry.
	Content string `json:"content"`
	// Files lists the other files a skill directory carries.
	Files []string `json:"files,omitempty"`
}

// source is one item of the migration source, read once per Copy rather than
// once per target.
type source struct {
	item  *Item
	entry map[string]any
}

// Read returns one installation's inventory. It never fails: an unreadable
// location is reported in Errors, because a page listing every agent on the
// host must stay useful when one of them has a broken config file.
func Read(agentId string, owner string) *Inventory {
	inventory := &Inventory{
		AgentId:    agentId,
		Owner:      owner,
		Skills:     []*Item{},
		McpServers: []*Item{},
	}

	found, ok := layoutOf(agentId)
	if !ok {
		return inventory
	}
	inventory.SkillsSupported = found.skills != nil
	inventory.McpSupported = found.mcp != nil
	if found.mcp != nil {
		inventory.McpWritable = found.mcp.readOnly == ""
		inventory.McpReadOnly = found.mcp.readOnly
	}

	home, err := homeOf(owner)
	if err != nil {
		inventory.Errors = append(inventory.Errors, err.Error())
		return inventory
	}
	inventory.Home = home

	if found.skills != nil {
		inventory.SkillsDir = found.skills.dir(home)
		skills, err := readSkills(agentId, owner, inventory.SkillsDir)
		if err != nil {
			inventory.Errors = append(inventory.Errors, err.Error())
		}
		inventory.Skills = skills
	}

	if found.mcp != nil {
		inventory.McpFile = found.mcp.path(home)
		entries, err := found.mcp.store.read(inventory.McpFile)
		if err != nil {
			inventory.Errors = append(inventory.Errors, err.Error())
		}
		inventory.McpServers = mcpItems(agentId, owner, inventory.McpFile, entries)
	}
	return inventory
}

// ReadDetail returns one item's full definition.
func ReadDetail(agentId string, owner string, kind string, name string) (*Detail, error) {
	found, home, err := resolve(agentId, owner, kind)
	if err != nil {
		return nil, err
	}
	if kind == KindSkill {
		return skillDetail(agentId, owner, found.skills.dir(home), name)
	}

	file := found.mcp.path(home)
	entries, err := found.mcp.store.read(file)
	if err != nil {
		return nil, err
	}
	entry, ok := entries[name]
	if !ok {
		return nil, fmt.Errorf("%s: no MCP server named %q in %s", agentId, name, file)
	}
	return mcpDetail(agentId, owner, file, name, entry)
}

// Delete removes one item from the agent's own configuration.
func Delete(agentId string, owner string, kind string, name string) error {
	found, home, err := resolve(agentId, owner, kind)
	if err != nil {
		return err
	}
	if kind == KindSkill {
		return deleteSkill(found.skills.dir(home), name)
	}
	if found.mcp.readOnly != "" {
		return fmt.Errorf("%s: %s", agentId, found.mcp.readOnly)
	}
	return found.mcp.store.remove(found.mcp.path(home), name)
}

// CopyRequest is one migration: some of a source agent's items, into one or
// more other agents belonging to the same account.
type CopyRequest struct {
	Owner     string   `json:"owner"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Kind      string   `json:"kind"`
	Names     []string `json:"names"`
	Overwrite bool     `json:"overwrite"`
}

// What one planned copy would do to the target.
const (
	ActionCreate    = "create"
	ActionOverwrite = "overwrite"
	ActionSkip      = "skip"
	ActionFailed    = "failed"
)

// PlanItem is one item's fate at one target, decided before anything is written
// and reported again afterwards with what actually happened.
type PlanItem struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Action  string `json:"action"`
	Reason  string `json:"reason,omitempty"`
	Path    string `json:"path,omitempty"`
}

// Plan reports what Copy would do, without touching anything.
func Plan(request CopyRequest) ([]*PlanItem, error) {
	sources, err := readSources(request)
	if err != nil {
		return nil, err
	}
	return plan(request, sources), nil
}

func plan(request CopyRequest, sources map[string]*source) []*PlanItem {
	planned := []*PlanItem{}
	for _, agentId := range request.To {
		existing, err := targetNames(agentId, request.Owner, request.Kind)
		for _, name := range request.Names {
			item := &PlanItem{AgentId: agentId, Name: name}
			switch {
			case err != nil:
				item.Action, item.Reason = ActionSkip, err.Error()
			case sources[name] == nil:
				item.Action, item.Reason = ActionSkip, "not found in the source agent"
			case sources[name].item.Managed:
				item.Action, item.Reason = ActionSkip, "written by Gateway, not migrated"
			case !existing[name]:
				item.Action = ActionCreate
			case request.Overwrite:
				item.Action = ActionOverwrite
			default:
				item.Action, item.Reason = ActionSkip, "already exists"
			}
			planned = append(planned, item)
		}
	}
	return planned
}

// Copy writes the selected items into every target agent. One item's failure is
// recorded against that item and the rest still run, so a partly finished
// migration is reported item by item instead of hidden behind the first error.
func Copy(request CopyRequest) ([]*PlanItem, error) {
	sources, err := readSources(request)
	if err != nil {
		return nil, err
	}

	planned := plan(request, sources)
	for _, item := range planned {
		if item.Action != ActionCreate && item.Action != ActionOverwrite {
			continue
		}
		path, err := writeItem(request, sources[item.Name], item.AgentId)
		if err != nil {
			item.Action, item.Reason = ActionFailed, err.Error()
			continue
		}
		item.Path = path
	}
	return planned, nil
}

// writeItem puts one already-read source item into one target agent.
func writeItem(request CopyRequest, from *source, agentId string) (string, error) {
	found, home, err := resolve(agentId, request.Owner, request.Kind)
	if err != nil {
		return "", err
	}

	if request.Kind == KindSkill {
		return copySkill(from.item.Path, found.skills.dir(home), from.item.Name)
	}
	if found.mcp.readOnly != "" {
		return "", errors.New(found.mcp.readOnly)
	}
	file := found.mcp.path(home)
	return file, found.mcp.store.write(file, from.item.Name, from.entry)
}

// readSources validates the request and loads the source agent's items once.
func readSources(request CopyRequest) (map[string]*source, error) {
	switch {
	case request.From == "":
		return nil, errors.New("the source agent is empty")
	case len(request.To) == 0:
		return nil, errors.New("no target agent was selected")
	case len(request.Names) == 0:
		return nil, errors.New("nothing was selected to copy")
	case request.Kind != KindSkill && request.Kind != KindMcp:
		return nil, fmt.Errorf("unknown configuration kind: %s", request.Kind)
	}
	for _, agentId := range request.To {
		if agentId == request.From {
			return nil, errors.New("the source agent cannot also be a target")
		}
	}

	found, home, err := resolve(request.From, request.Owner, request.Kind)
	if err != nil {
		return nil, err
	}

	inventory := Read(request.From, request.Owner)
	if len(inventory.Errors) > 0 {
		return nil, errors.New(strings.Join(inventory.Errors, "; "))
	}

	entries := map[string]map[string]any{}
	if request.Kind == KindMcp {
		entries, err = found.mcp.store.read(found.mcp.path(home))
		if err != nil {
			return nil, err
		}
	}

	sources := map[string]*source{}
	for _, item := range itemsOf(inventory, request.Kind) {
		sources[item.Name] = &source{item: item, entry: entries[item.Name]}
	}
	return sources, nil
}

// targetNames is what the target already has, so planning can tell a new name
// from one that would be replaced.
func targetNames(agentId string, owner string, kind string) (map[string]bool, error) {
	if _, _, err := resolve(agentId, owner, kind); err != nil {
		return nil, err
	}

	inventory := Read(agentId, owner)
	if len(inventory.Errors) > 0 {
		return nil, errors.New(strings.Join(inventory.Errors, "; "))
	}

	names := map[string]bool{}
	for _, item := range itemsOf(inventory, kind) {
		names[item.Name] = true
	}
	return names, nil
}

func itemsOf(inventory *Inventory, kind string) []*Item {
	if kind == KindSkill {
		return inventory.Skills
	}
	return inventory.McpServers
}

// resolve is the check every read and write shares: the agent is known, it
// keeps that kind of configuration somewhere, and the owner's home resolves.
func resolve(agentId string, owner string, kind string) (layout, string, error) {
	found, ok := layoutOf(agentId)
	if !ok {
		return layout{}, "", fmt.Errorf("unknown agent: %s", agentId)
	}
	if (kind == KindSkill && found.skills == nil) || (kind == KindMcp && found.mcp == nil) {
		return layout{}, "", fmt.Errorf("%s: %w", agentId, ErrUnsupported)
	}
	home, err := homeOf(owner)
	if err != nil {
		return layout{}, "", err
	}
	return found, home, nil
}

func sortItems(items []*Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
}
