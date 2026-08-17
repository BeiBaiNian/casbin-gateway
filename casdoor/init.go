// Copyright 2023 The casbin Authors. All Rights Reserved.
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

package casdoor

import (
	_ "embed"

	"github.com/apache/casbin-gateway/auth"
	"github.com/apache/casbin-gateway/conf"
)

//go:embed token_jwt_key.pem
var JwtPublicKey string

// InitCasdoorConfig wires up the Casdoor SDK when "casdoorEndpoint" is set in
// app.conf. Casdoor is optional: with no endpoint configured, Gateway signs
// users in against its own local user table instead, and every Casdoor-backed
// feature degrades via conf.IsCasdoorAvailable().
func InitCasdoorConfig() {
	casdoorEndpoint := conf.GetConfigString("casdoorEndpoint")
	if casdoorEndpoint == "" {
		conf.SetCasdoorAvailable(false)
		return
	}

	clientId := conf.GetConfigString("clientId")
	clientSecret := conf.GetConfigString("clientSecret")
	casdoorOrganization := conf.GetConfigString("casdoorOrganization")
	casdoorApplication := conf.GetConfigString("casdoorApplication")
	auth.InitConfig(casdoorEndpoint, clientId, clientSecret, JwtPublicKey, casdoorOrganization, casdoorApplication)
	conf.SetCasdoorAvailable(true)
}
