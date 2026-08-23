// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package controllers

import (
	"encoding/json"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agenthistory"
	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/object"
)

type discoveredAgent struct {
	agent.Installation
	agentpatch.Status

	// Provider is the "owner/name" id of the bound provider, and Fallbacks are the
	// providers tried after it. Installations are discovered per host while the
	// routing is stored per agent id, so they merge here.
	Provider  string   `json:"provider"`
	Fallbacks []string `json:"fallbacks"`
	Mode      string   `json:"mode"`
	// ProviderConfig is the state of the agent's own configuration file, which
	// is what the config orchestrator writes.
	ProviderConfig agentprovider.Status `json:"providerConfig"`
}

// GetAgents scans known installation locations and returns the AI agents
// installed in the environment where Casbin Gateway is running.
func (c *ApiController) GetAgents() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(c.GetString("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	agents, err := object.GetAgents()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	result := make([]*discoveredAgent, 0, len(installations))
	for _, installation := range installations {
		target := targetOf(installation)
		item := &discoveredAgent{
			Installation:   installation,
			Status:         agentpatch.StatusOf(target),
			Fallbacks:      []string{},
			Mode:           object.ModeGateway,
			ProviderConfig: agentprovider.StatusOf(providerTarget(target)),
		}
		if stored, ok := agents[installation.AgentId]; ok {
			item.Provider = stored.Provider
			item.Mode = stored.Mode
			if stored.Fallbacks != nil {
				item.Fallbacks = stored.Fallbacks
			}
		}
		result = append(result, item)
	}
	c.ResponseOk(result, agent.InContainer())
}

// UpdateAgentRouting binds one agent to the provider its requests are forwarded
// to, to the providers tried when that one cannot answer, and to the way it
// reaches them. The binding is per agent id.
//
// An installation whose configuration file Gateway already wrote is rewritten
// here: in gateway mode the file does not change, which is what makes a switch
// take effect without restarting the agent, but in direct mode the new provider
// only reaches the agent through its own configuration.
func (c *ApiController) UpdateAgentRouting() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId   string   `json:"agentId"`
		Provider  string   `json:"provider"`
		Fallbacks []string `json:"fallbacks"`
		Mode      string   `json:"mode"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	if err := object.SetAgentRouting(form.AgentId, form.Provider, form.Fallbacks, form.Mode); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if form.Provider != "" {
		if failure := reapplyAgentProvider(form.AgentId); failure != "" {
			c.ResponseError("the routing was saved, but the agent configuration was not rewritten: " + failure)
			return
		}
	}
	c.ResponseOk(form.Provider)
}

// PatchAgent enables monitoring for one discovered installation.
func (c *ApiController) PatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Patch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// UnpatchAgent disables monitoring and restores any configuration it changed.
func (c *ApiController) UnpatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Unpatch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// GetAgentRecords returns the current process's in-memory agent activity.
func (c *ApiController) GetAgentRecords() {
	if c.RequireAdmin() {
		return
	}

	limit := 200
	if value := c.Input().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		limit = parsed
	}
	c.ResponseOk(agentmonitor.ListRecords(agentmonitor.RecordQuery{
		Agent:     c.Input().Get("agent"),
		EventType: c.Input().Get("eventType"),
		Outcome:   c.Input().Get("outcome"),
		Session:   c.Input().Get("session"),
		Limit:     limit,
	}))
}

// GetAgentSessions groups the current in-memory records by agent session. The
// optional agent filter is what an agent's own detail page asks for.
func (c *ApiController) GetAgentSessions() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.Input().Get("agent")
	live := agentmonitor.ListSessions(agentmonitor.RecordQuery{Agent: agentId})

	// The transcripts on disk are the sessions that already happened, so they
	// are listed next to the monitored ones rather than only after Patch.
	sessions := make([]agenthistory.Session, 0, len(live))
	seen := map[string]bool{}
	for _, session := range live {
		seen[sessionSeenKey(session.Agent, session.SessionKey)] = true
		sessions = append(sessions, agenthistory.Session{
			Agent:       session.Agent,
			SessionKey:  session.SessionKey,
			Title:       session.Title,
			RecordCount: session.RecordCount,
			FirstTime:   session.FirstTime,
			LastTime:    session.LastTime,
		})
	}
	for _, session := range historicalSessions(agentId) {
		if seen[sessionSeenKey(session.Agent, session.SessionKey)] {
			continue
		}
		sessions = append(sessions, session)
	}

	sort.SliceStable(sessions, func(left, right int) bool {
		return sessions[left].LastTime > sessions[right].LastTime
	})
	c.ResponseOk(sessions)
}

// sessionSeenKey identifies one session across the two sources, so a session
// that monitoring already reported is not listed twice.
func sessionSeenKey(agentId string, sessionKey string) string {
	return agentId + "/" + sessionKey
}

// historicalSessions reads the transcripts of every account with an agent on
// this machine. A home Gateway cannot open is skipped: the page lists what it
// can read, and says nothing about the rest.
func historicalSessions(agentId string) []agenthistory.Session {
	installations, err := agent.Scan(false)
	if err != nil {
		return nil
	}

	sessions := []agenthistory.Session{}
	scanned := map[string]bool{}
	for _, installation := range installations {
		home, err := agenthome.Resolve(installation.Owner)
		if err != nil || scanned[home] {
			continue
		}
		scanned[home] = true
		for _, session := range agenthistory.Scan(home) {
			if agentId == "" || strings.EqualFold(session.Agent, agentId) {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions
}

// AddAgentRecord accepts reports from a hook or MCP process launched locally by
// Gateway. Those processes have no browser session, so they authenticate with
// the per-installation credential issued at Patch time. Loopback alone is not a
// trust boundary: behind a reverse proxy every caller looks local, and any web
// page the operator visits can reach 127.0.0.1.
func (c *ApiController) AddAgentRecord() {
	ip, ok := c.directLoopbackClient()
	if !ok {
		c.ResponseError("agent record ingestion is limited to direct loopback requests")
		return
	}
	agentId, ok := agentpatch.ValidateIngestToken(c.Ctx.Input.Header(agentmonitor.IngestTokenHeader))
	if !ok {
		c.ResponseError("agent record ingestion requires a valid installation token")
		return
	}

	var record agentmonitor.Record
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &record); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if record.Agent == "" {
		c.ResponseError("agent is required")
		return
	}
	// The token decides which agent a reporter may speak for, so a compromised
	// hook cannot attribute its activity to a different installation.
	if agentId != "" && !strings.EqualFold(record.Agent, agentId) {
		c.ResponseError("agent does not match the installation this token was issued for")
		return
	}
	record.ClientIp = ip.String()
	agentmonitor.AddRecord(&record)
	c.ResponseOk()
}

// directLoopbackClient reports the peer address, rejecting anything that was
// relayed by a proxy. A forwarding header means the real client is remote even
// though the socket is local.
func (c *ApiController) directLoopbackClient() (net.IP, bool) {
	for _, header := range []string{"X-Forwarded-For", "X-Real-Ip", "Forwarded"} {
		if c.Ctx.Input.Header(header) != "" {
			return nil, false
		}
	}
	remoteAddr := c.Ctx.Request.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, false
	}
	return ip, true
}

// readAgentInstallation resolves the request body against the installations that
// were actually discovered. Patching writes into the owner's home directory and
// starting runs a program, so an unverified body would let a caller name any
// account, and any file, on the host.
func (c *ApiController) readAgentInstallation() (agent.Installation, bool) {
	var requested agentpatch.Target
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &requested); err != nil {
		c.ResponseError(err.Error())
		return agent.Installation{}, false
	}

	installations, err := agent.Scan(false)
	if err != nil {
		c.ResponseError(err.Error())
		return agent.Installation{}, false
	}
	for _, installation := range installations {
		if matchesTarget(targetOf(installation), requested) {
			return installation, true
		}
	}
	c.ResponseError("no discovered agent installation matches this target")
	return agent.Installation{}, false
}

func (c *ApiController) readAgentPatchTarget() (agentpatch.Target, bool) {
	installation, ok := c.readAgentInstallation()
	if !ok {
		return agentpatch.Target{}, false
	}
	return targetOf(installation), true
}

func matchesTarget(discovered, requested agentpatch.Target) bool {
	return discovered.AgentId == requested.AgentId &&
		strings.EqualFold(filepath.Clean(discovered.Path), filepath.Clean(requested.Path)) &&
		strings.EqualFold(discovered.Owner, requested.Owner)
}

func targetOf(installation agent.Installation) agentpatch.Target {
	return agentpatch.Target{
		AgentId: installation.AgentId,
		Path:    installation.Path,
		Owner:   installation.Owner,
	}
}
