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

package controllers

import (
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
)

// PlanAgentProvider renders what a switch would write, without touching a file.
func (c *ApiController) PlanAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	endpoint, err := agentEndpoint(target.AgentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	files, err := agentprovider.Plan(providerTarget(target), endpoint)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(files)
}

// ApplyAgentProvider writes the bound provider into the agent's own configuration
// file, in that agent's format.
func (c *ApiController) ApplyAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	endpoint, err := agentEndpoint(target.AgentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := agentprovider.Apply(providerTarget(target), endpoint); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentprovider.StatusOf(providerTarget(target)))
}

// RestoreAgentProvider puts back the provider settings the agent had before
// Gateway first switched it.
func (c *ApiController) RestoreAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentprovider.Restore(providerTarget(target)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentprovider.StatusOf(providerTarget(target)))
}

// GetProviderHealth reports what the proxy has seen of each provider, which is
// what says why a request went to a fallback rather than to the bound provider.
func (c *ApiController) GetProviderHealth() {
	if c.RequireAdmin() {
		return
	}
	c.ResponseOk(object.GetProviderHealth())
}

// checkAgentProtocol rejects providers the agent could never talk to. The proxy
// relays a request in the wire format it arrived in, so an agent speaking one
// API cannot be served by a provider speaking the other.
func checkAgentProtocol(agentId string, providerIds []string) error {
	protocol := agentprovider.ProtocolOf(agentId)
	if protocol == "" {
		return nil
	}

	for _, id := range providerIds {
		if id == "" {
			continue
		}
		provider, err := object.GetProvider(id)
		if err != nil {
			return err
		}
		// A provider that does not exist is reported by the routing itself.
		if provider == nil {
			continue
		}
		if spoken := object.ProviderProtocol(provider); spoken != protocol {
			return fmt.Errorf("%s speaks the %s API, but provider %s speaks %s: bind a provider that speaks %s instead",
				agentId, protocol, id, spoken, protocol)
		}
	}
	return nil
}

// agentEndpoint resolves where one agent should be pointed. In gateway mode
// that is the local proxy, which is what makes a later switch take effect
// without rewriting a file; in direct mode it is the provider's own upstream.
func agentEndpoint(agentId string) (agentprovider.Endpoint, error) {
	endpoint := agentprovider.Endpoint{Mode: object.ModeGateway}

	stored, err := object.GetAgent(agentId)
	if err != nil {
		return endpoint, err
	}
	if stored != nil && stored.Mode != "" {
		endpoint.Mode = stored.Mode
	}

	provider, err := object.GetProviderByAgent(agentId)
	if err != nil {
		return endpoint, err
	}

	endpoint.Provider = provider.GetId()
	endpoint.Protocol = object.ProviderProtocol(provider)
	if len(provider.Models) > 0 {
		endpoint.Model = provider.Models[0]
	}

	if endpoint.Mode == object.ModeDirect {
		endpoint.BaseUrl = provider.BaseUrl
		endpoint.ApiKey = provider.ApiKey
		endpoint.ServesResponsesApi = object.ServesResponsesApi(provider)
		return endpoint, nil
	}

	endpoint.BaseUrl = gatewayAgentUrl(agentId)
	endpoint.ServesResponsesApi = true
	// A client-auth provider forwards whatever the agent sends, so it must keep
	// sending its own credentials: a placeholder token written into the agent's
	// configuration would replace the sign-in it already has.
	if !object.UsesClientAuth(provider) {
		endpoint.ApiKey = conf.GetRelayToken()
	}
	return endpoint, nil
}

// gatewayAgentUrl is the loopback base URL an agent reaches its own provider at.
// One URL serves both wire formats: an OpenAI client appends /chat/completions
// to it, an Anthropic one appends /v1/messages.
func gatewayAgentUrl(agentId string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/v1/agents/%s", conf.GetHttpPort(), agentId)
}

// reapplyAgentProvider rewrites the configuration of every installation Gateway
// already switched, which is what a routing change means for an agent pointed
// straight at a provider. It reports what it could not rewrite rather than
// failing the routing change, which is already stored by then.
func reapplyAgentProvider(agentId string) string {
	installations, err := agent.Scan(false)
	if err != nil {
		return err.Error()
	}

	endpoint, endpointErr := agentEndpoint(agentId)
	failures := []string{}
	for _, installation := range installations {
		if installation.AgentId != agentId {
			continue
		}
		target := providerTarget(targetOf(installation))
		if !agentprovider.StatusOf(target).Applied {
			continue
		}
		if endpointErr != nil {
			failures = append(failures, endpointErr.Error())
			break
		}
		if err := agentprovider.Apply(target, endpoint); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return strings.Join(failures, "; ")
}

func providerTarget(target agentpatch.Target) agentprovider.Target {
	return agentprovider.Target{AgentId: target.AgentId, Path: target.Path, Owner: target.Owner}
}
