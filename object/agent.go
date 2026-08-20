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

// Agent is the Gateway-side configuration of one kind of AI agent. Installations
// are discovered by scanning the host, so this table holds only what is chosen.
type Agent struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// Channel is the "owner/name" id of the bound channel, empty when unbound.
	Channel string `xorm:"varchar(200)" json:"channel"`
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

// GetAgentChannels maps every configured agent id to its channel id.
func GetAgentChannels() (map[string]string, error) {
	agents := []*Agent{}
	if err := ormer.Engine.Where("owner = ?", AgentOwner).Find(&agents); err != nil {
		return nil, err
	}

	channels := map[string]string{}
	for _, agent := range agents {
		if agent.Channel != "" {
			channels[agent.Name] = agent.Channel
		}
	}
	return channels, nil
}

// SetAgentChannel binds an agent to a channel, or unbinds it when channelId is
// empty. The channel is resolved here so a typo fails at the form, not at runtime.
func SetAgentChannel(agentId string, channelId string) error {
	if agentId == "" {
		return errors.New("the agent id is empty")
	}

	if channelId != "" {
		channel, err := getChannelById(channelId)
		if err != nil {
			return err
		}
		if channel == nil {
			return fmt.Errorf("the channel does not exist: %s", channelId)
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
		})
		return err
	}

	// Cols() is what writes an empty channel: xorm skips zero values otherwise.
	_, err = ormer.Engine.ID(core.PK{AgentOwner, agentId}).
		Cols("channel", "updated_time").
		Update(&Agent{Channel: channelId, UpdatedTime: now})
	return err
}

// GetChannelByAgent resolves the channel one agent's requests go to. An unusable
// binding fails rather than falling back to model routing, so a request never
// reaches a different upstream than the one configured.
func GetChannelByAgent(agentId string) (*Channel, error) {
	agent, err := GetAgent(agentId)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Channel == "" {
		return nil, fmt.Errorf("%w: %s", ErrAgentNoChannel, agentId)
	}

	channel, err := getChannelById(agent.Channel)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("the channel bound to agent %s no longer exists: %s", agentId, agent.Channel)
	}
	if channel.Status != "enabled" {
		return nil, fmt.Errorf("the channel bound to agent %s is disabled: %s", agentId, agent.Channel)
	}
	return channel, nil
}

// getChannelById is GetChannel() without its panic on a malformed id.
func getChannelById(channelId string) (*Channel, error) {
	tokens := strings.Split(channelId, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return nil, fmt.Errorf("invalid channel ID: %s", channelId)
	}
	return getChannel(tokens[0], tokens[1])
}
