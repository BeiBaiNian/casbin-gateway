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

// Package agentprovider writes the selected upstream provider into the
// configuration file each agent CLI reads on startup, in that CLI's own format.
package agentprovider

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotSupported reports an agent whose configuration format Gateway cannot
// write yet. Those agents are still usable through the environment variables
// the UI shows.
var ErrNotSupported = errors.New("switching the provider of this agent is not supported yet")

// The two ways an agent reaches its provider.
const (
	// ModeGateway points the agent at the local proxy, so changing the bound
	// channel afterwards takes effect without touching a file again.
	ModeGateway = "gateway"
	// ModeDirect writes the provider's own base URL and key into the config,
	// which is what a switcher without a proxy does.
	ModeDirect = "direct"
)

// Target identifies one discovered agent installation.
type Target struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
}

// Endpoint is the upstream one agent is switched to, already resolved: in
// gateway mode BaseUrl is the local proxy, in direct mode the channel's own.
type Endpoint struct {
	Channel  string `json:"channel"`
	Protocol string `json:"protocol"`
	BaseUrl  string `json:"baseUrl"`
	ApiKey   string `json:"apiKey"`
	Model    string `json:"model"`
	Mode     string `json:"mode"`
}

// File is one configuration file a switch writes, with the section it will
// contain afterwards.
type File struct {
	Path    string `json:"path"`
	Format  string `json:"format"`
	Preview string `json:"preview"`
}

// Status is what the UI shows beside an installation: whether Gateway owns its
// provider configuration, and which one it points at.
type Status struct {
	Supported bool     `json:"supported"`
	Applied   bool     `json:"applied"`
	Channel   string   `json:"channel"`
	Mode      string   `json:"mode"`
	BaseUrl   string   `json:"baseUrl"`
	Time      string   `json:"time"`
	Files     []string `json:"files"`
	Detail    string   `json:"detail"`
}

type writer interface {
	AgentId() string
	// Protocol is the wire format this agent's client speaks.
	Protocol() string
	// Plan renders what Apply would write, without touching a file.
	Plan(Target, Endpoint) ([]File, error)
	// Apply writes the endpoint and returns the previous values of the keys it
	// owns, which is what Restore puts back.
	Apply(Target, Endpoint) (map[string]string, error)
	Restore(Target, map[string]string) error
	// Current is the base URL the files point at right now, empty when the
	// agent has no provider configured.
	Current(Target) (string, error)
}

var (
	writers     = map[string]writer{}
	writerMutex sync.Mutex
)

func register(value writer) {
	writers[value.AgentId()] = value
}

// Plan is the preview shown before a switch is confirmed.
func Plan(target Target, endpoint Endpoint) ([]File, error) {
	value, err := writerFor(target, endpoint)
	if err != nil {
		return nil, err
	}
	return value.Plan(target, endpoint)
}

// Apply switches one installation to endpoint. The previous values of the keys
// it overwrites are saved first, so Restore can put the agent back on its own
// provider.
func Apply(target Target, endpoint Endpoint) error {
	value, err := writerFor(target, endpoint)
	if err != nil {
		return err
	}

	writerMutex.Lock()
	defer writerMutex.Unlock()

	files, err := value.Plan(target, endpoint)
	if err != nil {
		return err
	}

	// Only the first switch records what the agent looked like before Gateway
	// touched it; a later one would otherwise save Gateway's own values.
	saved, err := loadState(target)
	if err != nil {
		return err
	}

	previous, err := value.Apply(target, endpoint)
	if err != nil {
		return err
	}
	if saved != nil {
		previous = saved.Previous
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return saveState(target, &state{
		AgentId:  target.AgentId,
		Channel:  endpoint.Channel,
		Mode:     endpoint.Mode,
		BaseUrl:  endpoint.BaseUrl,
		Time:     nowString(),
		Files:    paths,
		Previous: previous,
	})
}

// Restore puts back the provider settings the agent had before the first Apply
// and forgets the installation. It is a no-op when Gateway never switched it.
func Restore(target Target) error {
	value, ok := writers[target.AgentId]
	if !ok {
		return fmt.Errorf("%s: %w", target.AgentId, ErrNotSupported)
	}

	writerMutex.Lock()
	defer writerMutex.Unlock()

	saved, err := loadState(target)
	if err != nil || saved == nil {
		return err
	}
	if err := value.Restore(target, saved.Previous); err != nil {
		return err
	}
	return clearState(target)
}

// StatusOf reports the provider state of one installation. A configuration file
// that cannot be read is reported as detail rather than as an error, so the
// agent list stays usable.
func StatusOf(target Target) Status {
	value, ok := writers[target.AgentId]
	if !ok {
		return Status{
			Files:  []string{},
			Detail: "Gateway cannot write this agent's provider configuration yet",
		}
	}

	status := Status{Supported: true, Files: []string{}}
	saved, err := loadState(target)
	if err != nil {
		status.Detail = err.Error()
		return status
	}

	current, err := value.Current(target)
	if err != nil {
		status.Detail = err.Error()
		return status
	}

	if saved == nil {
		if current == "" {
			status.Detail = "This agent uses its own provider configuration"
		} else {
			status.Detail = "Configured outside Gateway: " + current
		}
		return status
	}

	status.Applied = true
	status.Channel = saved.Channel
	status.Mode = saved.Mode
	status.BaseUrl = saved.BaseUrl
	status.Time = saved.Time
	status.Files = saved.Files
	if current != saved.BaseUrl {
		status.Applied = false
		status.Detail = "Changed outside Gateway, the agent now points at " + emptyAs(current, "no provider")
	} else if saved.Mode == ModeGateway {
		status.Detail = "Routed through the local proxy, switching the channel needs no restart"
	} else {
		status.Detail = "Written directly into the agent configuration"
	}
	return status
}

func writerFor(target Target, endpoint Endpoint) (writer, error) {
	if target.AgentId == "" {
		return nil, errors.New("agentId is required")
	}
	if endpoint.Channel == "" {
		return nil, errors.New("no channel is bound to this agent")
	}
	if endpoint.BaseUrl == "" {
		return nil, errors.New("the base URL of the bound channel is empty")
	}

	value, ok := writers[target.AgentId]
	if !ok {
		return nil, fmt.Errorf("%s: %w", target.AgentId, ErrNotSupported)
	}
	if value.Protocol() != endpoint.Protocol {
		return nil, fmt.Errorf("%s speaks the %s API, but channel %s speaks %s",
			target.AgentId, value.Protocol(), endpoint.Channel, endpoint.Protocol)
	}
	return value, nil
}

func emptyAs(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// maskSecret is what a preview shows in place of a key: enough to recognize it,
// not enough to use it.
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}
