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

package object

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ErrAgentNoChannel lets the proxy answer with a client error, not a gateway one.
var ErrAgentNoChannel = errors.New("no channel is bound to this agent")

// AgentOwner is the owner every agent row is stored under: an agent belongs to
// the host, and its proxy endpoint is reached without a session.
const AgentOwner = "admin"

// How an agent reaches its channel. The values are the ones agentprovider
// writes into the agent's own configuration file.
const (
	ModeGateway = "gateway"
	ModeDirect  = "direct"
)

// Agent is the Gateway-side configuration of one kind of AI agent. Installations
// are discovered by scanning the host, so this table holds only what is chosen.
type Agent struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// Channel is the "owner/name" id of the bound channel, empty when unbound.
	Channel string `xorm:"varchar(200)" json:"channel"`
	// Fallbacks are the channel ids tried, in order, when Channel cannot answer.
	// They are JSON-serialized by xorm, hence the text column.
	Fallbacks []string `xorm:"mediumtext" json:"fallbacks"`
	// Mode is how the agent reaches its channel: ModeGateway routes it through
	// the local proxy, ModeDirect writes the channel's own endpoint into the
	// agent's configuration file.
	Mode string `xorm:"varchar(20)" json:"mode"`
}

func (agent *Agent) GetId() string {
	return fmt.Sprintf("%s/%s", agent.Owner, agent.Name)
}

func GetAgent(agentId string) (*Agent, error) {
	agent := &Agent{Owner: AgentOwner, Name: agentId}
	existed, err := ormer.Engine.Get(agent)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return agent, nil
}

// GetAgents returns every configured agent, keyed by agent id. Installations
// are discovered per host while the routing is stored per agent id, so the two
// are merged by the caller.
func GetAgents() (map[string]*Agent, error) {
	agents := []*Agent{}
	if err := ormer.Engine.Where("owner = ?", AgentOwner).Find(&agents); err != nil {
		return nil, err
	}

	result := map[string]*Agent{}
	for _, agent := range agents {
		if agent.Mode == "" {
			agent.Mode = ModeGateway
		}
		result[agent.Name] = agent
	}
	return result, nil
}

// SetAgentRouting stores where one agent's requests go: the bound channel, the
// channels to fall over to when it cannot answer, and how the agent reaches
// them. Every channel is resolved here so a typo fails at the form rather than
// on the next relayed request.
func SetAgentRouting(agentId string, channelId string, fallbacks []string, mode string) error {
	if agentId == "" {
		return errors.New("the agent id is empty")
	}
	if mode == "" {
		mode = ModeGateway
	}
	if mode != ModeGateway && mode != ModeDirect {
		return fmt.Errorf("invalid agent mode: %s", mode)
	}

	fallbacks = normalizeFallbacks(channelId, fallbacks)
	for _, id := range append([]string{channelId}, fallbacks...) {
		if id == "" {
			continue
		}
		channel, err := getChannelById(id)
		if err != nil {
			return err
		}
		if channel == nil {
			return fmt.Errorf("the channel does not exist: %s", id)
		}
	}

	stored, err := GetAgent(agentId)
	if err != nil {
		return err
	}

	now := util.GetCurrentTime()
	if stored == nil {
		_, err = ormer.Engine.Insert(&Agent{
			Owner:       AgentOwner,
			Name:        agentId,
			CreatedTime: now,
			UpdatedTime: now,
			Channel:     channelId,
			Fallbacks:   fallbacks,
			Mode:        mode,
		})
		return err
	}

	// Cols() is what writes an empty channel: xorm skips zero values otherwise.
	_, err = ormer.Engine.ID(core.PK{AgentOwner, agentId}).
		Cols("channel", "fallbacks", "mode", "updated_time").
		Update(&Agent{Channel: channelId, Fallbacks: fallbacks, Mode: mode, UpdatedTime: now})
	return err
}

// SetAgentChannel binds an agent to a channel, or unbinds it when channelId is
// empty, leaving its fallbacks and mode as they are.
func SetAgentChannel(agentId string, channelId string) error {
	stored, err := GetAgent(agentId)
	if err != nil {
		return err
	}
	if stored == nil {
		return SetAgentRouting(agentId, channelId, nil, "")
	}
	return SetAgentRouting(agentId, channelId, stored.Fallbacks, stored.Mode)
}

// normalizeFallbacks drops empty entries, duplicates and the primary channel,
// so the chain never tries the same upstream twice.
func normalizeFallbacks(channelId string, fallbacks []string) []string {
	result := []string{}
	seen := map[string]bool{channelId: true, "": true}
	for _, id := range fallbacks {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// GetChannelsByAgent is the chain one agent's requests are tried against: the
// bound channel first, then its fallbacks. A missing or disabled entry is
// skipped rather than failing the request, which is what makes the fallbacks
// worth configuring; a chain with nothing left in it reports why.
func GetChannelsByAgent(agentId string) ([]*Channel, error) {
	agent, err := GetAgent(agentId)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Channel == "" {
		return nil, fmt.Errorf("%w: %s", ErrAgentNoChannel, agentId)
	}

	channels := []*Channel{}
	skipped := ""
	for _, id := range append([]string{agent.Channel}, agent.Fallbacks...) {
		channel, err := getChannelById(id)
		if err != nil {
			skipped = err.Error()
			continue
		}
		if channel == nil {
			skipped = fmt.Sprintf("the channel bound to agent %s no longer exists: %s", agentId, id)
			continue
		}
		if channel.Status != "enabled" {
			skipped = fmt.Sprintf("the channel bound to agent %s is disabled: %s", agentId, id)
			continue
		}
		channels = append(channels, channel)
	}

	if len(channels) == 0 {
		if skipped == "" {
			return nil, fmt.Errorf("%w: %s", ErrAgentNoChannel, agentId)
		}
		return nil, errors.New(skipped)
	}
	return channels, nil
}

// GetChannelByAgent resolves the first channel of an agent's chain.
func GetChannelByAgent(agentId string) (*Channel, error) {
	channels, err := GetChannelsByAgent(agentId)
	if err != nil {
		return nil, err
	}
	return channels[0], nil
}

// getChannelById is GetChannel() without its panic on a malformed id.
func getChannelById(channelId string) (*Channel, error) {
	tokens := strings.Split(channelId, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return nil, fmt.Errorf("invalid channel ID: %s", channelId)
	}
	return getChannel(tokens[0], tokens[1])
}
