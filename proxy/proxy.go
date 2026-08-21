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

// Package proxy routes every outbound request through the proxy configured as
// httpProxy in conf/app.conf, falling back to the HTTP_PROXY / HTTPS_PROXY
// environment variables when that setting is empty.
package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/conf"
)

// ProxyHttpClient sends its requests through the configured proxy.
var ProxyHttpClient *http.Client

var (
	proxyUrlOnce sync.Once
	proxyUrl     *url.URL

	transportOnce sync.Once
	transport     *http.Transport
)

func InitHttpClient() {
	proxyUrlOnce.Do(initProxyUrl)
	ProxyHttpClient = &http.Client{Transport: Transport()}
}

// Proxy picks the proxy to reach req through. It is meant to be assigned to
// http.Transport.Proxy, which calls it per request, so the httpProxy setting is
// read on first use instead of at package initialization: in -tags embed builds
// the embedded conf/app.conf is only loaded once main's init() runs, after every
// imported package has already initialized its own variables.
func Proxy(req *http.Request) (*url.URL, error) {
	proxyUrlOnce.Do(initProxyUrl)
	if proxyUrl == nil {
		return http.ProxyFromEnvironment(req)
	}
	return proxyUrl, nil
}

// Transport returns the shared transport for outbound requests. Sharing one
// instance keeps the connection pool shared as well.
func Transport() *http.Transport {
	transportOnce.Do(func() {
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = Proxy
	})
	return transport
}

// initProxyUrl parses the httpProxy setting. A bare "host:port" means SOCKS5,
// which is what the setting has always been taken to mean; "socks5://",
// "socks5h://", "http://" and "https://" addresses are honoured as written, and
// may carry credentials.
func initProxyUrl() {
	httpProxy := strings.TrimSpace(conf.GetConfigStringUnquoted("httpProxy"))
	if httpProxy == "" {
		return
	}

	if !strings.Contains(httpProxy, "://") {
		httpProxy = "socks5://" + httpProxy
	}

	parsedUrl, err := url.Parse(httpProxy)
	if err != nil || parsedUrl.Host == "" {
		fmt.Printf("httpProxy is not a valid proxy address, outbound traffic is left unproxied: %s\n", httpProxy)
		return
	}

	proxyUrl = parsedUrl
	fmt.Printf("Proxy enabled for outbound traffic: %s\n", proxyUrl.Redacted())
}
