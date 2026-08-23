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
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/service"
)

// GetGatewayStatus tells the web UI whether the reverse-proxy gateway is
// actually running. Without it, a Gateway whose reverse proxy is off accepts
// every site and rule and silently proxies nothing.
func (c *ApiController) GetGatewayStatus() {
	if c.RequireSignedIn() {
		return
	}

	c.ResponseOk(map[string]interface{}{
		"gatewayEnabled": conf.IsGatewayEnabled(),
		"gatewayRunning": service.IsGatewayRunning(),
		"gatewayError":   service.GatewayError(),
		"httpPort":       conf.GetGatewayHttpPort(),
		"httpsPort":      conf.GetGatewayHttpsPort(),
	})
}

// GetRelayToken hands the web UI the token an agent has to send to the relay,
// so the snippets it shows can be pasted as they are.
func (c *ApiController) GetRelayToken() {
	if c.RequireSignedIn() {
		return
	}

	c.ResponseOk(map[string]interface{}{
		"relayToken": conf.GetRelayToken(),
		"localOnly":  conf.IsHttpAddrLoopback(),
	})
}
