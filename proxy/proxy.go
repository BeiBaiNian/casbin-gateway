// Copyright 2021 The casbin Authors. All Rights Reserved.
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

package proxy

import (
	"net/http"
	"net/url"

	"github.com/beego/beego"
)

var ProxyHttpClient *http.Client

func InitHttpClient() {
	ProxyHttpClient = &http.Client{Transport: NewTransport()}
}

// NewTransport returns a standard transport configured for outbound traffic.
// An app.conf SOCKS5 proxy takes precedence over environment proxy variables.
func NewTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ConfigureTransport(transport)
	return transport
}

// ConfigureTransport applies the outbound proxy policy to transport.
func ConfigureTransport(transport *http.Transport) {
	httpProxy := beego.AppConfig.String("httpProxy")
	if httpProxy == "" {
		transport.Proxy = http.ProxyFromEnvironment
		return
	}

	transport.Proxy = http.ProxyURL(&url.URL{Scheme: "socks5", Host: httpProxy})
	beego.Trace("using SOCKS5 proxy for outbound traffic:", httpProxy)
}
