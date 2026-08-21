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
	"strings"
)

// How a new server is reached. Anything else an agent supports can still be
// copied from another agent, which is what the migration path is for.
const (
	TransportStdio = "stdio"
	TransportHttp  = "http"
)

// McpRequest is one new MCP server, described the way every host spells it, to
// be written into each agent listed in To.
type McpRequest struct {
	Owner     string            `json:"owner"`
	To        []string          `json:"to"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Url       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Overwrite bool              `json:"overwrite"`
}

// AddMcp writes one server into every target agent's own configuration file.
// Like Copy, one target's failure is recorded against that target and the rest
// still run, so a half-finished round is reported agent by agent.
func AddMcp(request McpRequest) ([]*PlanItem, error) {
	name := strings.TrimSpace(request.Name)
	if name == ManagedEntryName {
		return nil, errors.New("this name belongs to Gateway's agent monitoring")
	}
	entry, err := request.entry(name)
	if err != nil {
		return nil, err
	}
	if len(request.To) == 0 {
		return nil, errors.New("no agent was picked to add this server to")
	}

	planned := []*PlanItem{}
	for _, agentId := range request.To {
		item := &PlanItem{AgentId: agentId, Name: name}
		planned = append(planned, item)

		existing, err := targetNames(agentId, request.Owner, KindMcp)
		switch {
		case err != nil:
			item.Action, item.Reason = ActionSkip, err.Error()
			continue
		case !existing[name]:
			item.Action = ActionCreate
		case request.Overwrite:
			item.Action = ActionOverwrite
		default:
			item.Action, item.Reason = ActionSkip, "already exists"
			continue
		}

		path, err := writeMcp(request.Owner, agentId, name, entry)
		if err != nil {
			item.Action, item.Reason = ActionFailed, err.Error()
			continue
		}
		item.Path = path
	}
	return planned, nil
}

// entry is the server as it goes into the file. A stdio server is left without
// a type field, which every host reads as "spawn the command", and is also what
// the TOML half of Codex expects.
func (request McpRequest) entry(name string) (map[string]any, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}

	entry := map[string]any{}
	switch request.Transport {
	case TransportHttp:
		url := strings.TrimSpace(request.Url)
		if url == "" {
			return nil, errors.New("an HTTP MCP server needs a URL")
		}
		entry["type"] = TransportHttp
		entry["url"] = url
		if headers := pairs(request.Headers); len(headers) > 0 {
			entry["headers"] = headers
		}
	case TransportStdio, "":
		command := strings.TrimSpace(request.Command)
		if command == "" {
			return nil, errors.New("a stdio MCP server needs a command to run")
		}
		entry["command"] = command
		if args := list(request.Args); len(args) > 0 {
			entry["args"] = args
		}
		if env := pairs(request.Env); len(env) > 0 {
			entry["env"] = env
		}
	default:
		return nil, fmt.Errorf("unknown transport: %q", request.Transport)
	}
	return entry, nil
}

func writeMcp(owner string, agentId string, name string, entry map[string]any) (string, error) {
	found, home, err := resolve(agentId, owner, KindMcp)
	if err != nil {
		return "", err
	}
	if found.mcp.readOnly != "" {
		return "", errors.New(found.mcp.readOnly)
	}

	file := found.mcp.path(home)
	return file, found.mcp.store.write(file, name, entry)
}

// The two stores take the same shapes a parsed JSON file has, so the request's
// own types are converted rather than passed through.
func list(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func pairs(values map[string]string) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		if key = strings.TrimSpace(key); key != "" {
			result[key] = value
		}
	}
	return result
}
