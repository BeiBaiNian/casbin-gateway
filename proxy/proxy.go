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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/beego/beego"
	"golang.org/x/net/proxy"
)

var DefaultHttpClient *http.Client
var ProxyHttpClient *http.Client

func InitHttpClient() {
	// not use proxy
	DefaultHttpClient = http.DefaultClient

	// use proxy
	ProxyHttpClient = GetProxyHttpClient()
}

func isAddressOpen(address string) bool {
	timeout := time.Millisecond * 100
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		// cannot connect to address, proxy is not active
		return false
	}

	if conn != nil {
		defer conn.Close()
		fmt.Printf("Socks5 proxy enabled: %s\n", address)
		return true
	}

	return false
}

// GetProxyHttpClient returns an HTTP client that routes outbound connections
// through the proxy configured in app.conf ("httpProxy") when one is set, or
// a plain client otherwise.
//
// Two proxy formats are supported:
//   - http://host:port or https://host:port: an HTTP(S) proxy (the common
//     case, e.g. Clash at http://127.0.0.1:7899)
//   - host:port: a SOCKS5 proxy (legacy behavior, kept for compatibility)
func GetProxyHttpClient() *http.Client {
	httpProxy := beego.AppConfig.String("httpProxy")
	if httpProxy == "" {
		return &http.Client{}
	}

	if !isAddressOpen(proxyAddress(httpProxy)) {
		return &http.Client{}
	}

	if strings.HasPrefix(httpProxy, "http://") || strings.HasPrefix(httpProxy, "https://") {
		proxyUrl, err := url.Parse(httpProxy)
		if err != nil {
			panic(err)
		}
		return &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
		}
	}

	// https://stackoverflow.com/questions/33585587/creating-a-go-socks5-client
	dialer, err := proxy.SOCKS5("tcp", httpProxy, nil, proxy.Direct)
	if err != nil {
		panic(err)
	}

	tr := &http.Transport{Dial: dialer.Dial}
	return &http.Client{
		Transport: tr,
	}
}

// proxyAddress strips the scheme from an http://host:port proxy setting so
// the address can be dialed directly during the liveness check.
func proxyAddress(httpProxy string) string {
	if strings.HasPrefix(httpProxy, "http://") || strings.HasPrefix(httpProxy, "https://") {
		if u, err := url.Parse(httpProxy); err == nil {
			return u.Host
		}
	}
	return httpProxy
}

func GetProxyDialer() *net.Dialer {
	httpProxy := beego.AppConfig.String("httpProxy")
	if httpProxy == "" {
		return nil
	}

	if !isAddressOpen(httpProxy) {
		return nil
	}

	// https://stackoverflow.com/questions/33585587/creating-a-go-socks5-client
	dialer, err := proxy.SOCKS5("tcp", httpProxy, nil, proxy.Direct)
	if err != nil {
		panic(err)
	}

	return dialer.(*net.Dialer)
}
