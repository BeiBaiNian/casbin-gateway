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
	"net/http"
	"testing"
)

func TestBuildChannelUrl(t *testing.T) {
	cases := []struct {
		baseUrl  string
		protocol string
		endpoint string
		expected string
	}{
		{"https://api.openai.com/v1", ProtocolOpenAi, "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com", ProtocolOpenAi, "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://api.openai.com/v1/", ProtocolOpenAi, "/chat/completions", "https://api.openai.com/v1/chat/completions"},

		// An Anthropic base URL is bare and the endpoint supplies the /v1.
		{"https://api.anthropic.com", ProtocolAnthropic, "/v1/messages", "https://api.anthropic.com/v1/messages"},
		{"https://api.anthropic.com/", ProtocolAnthropic, "/v1/messages", "https://api.anthropic.com/v1/messages"},
		{"https://api.anthropic.com/v1", ProtocolAnthropic, "/v1/messages", "https://api.anthropic.com/v1/messages"},
		{"https://api.anthropic.com/v1/messages", ProtocolAnthropic, "/v1/messages", "https://api.anthropic.com/v1/messages"},
		{"https://relay.example.com/anthropic", ProtocolAnthropic, "/v1/messages", "https://relay.example.com/anthropic/v1/messages"},
		{"https://api.anthropic.com", ProtocolAnthropic, "/v1/messages/count_tokens", "https://api.anthropic.com/v1/messages/count_tokens"},
	}

	for _, tc := range cases {
		got, err := BuildChannelUrl(tc.baseUrl, tc.protocol, tc.endpoint)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.expected {
			t.Errorf("BuildChannelUrl(%s, %s, %s) = %s, expected %s", tc.baseUrl, tc.protocol, tc.endpoint, got, tc.expected)
		}
	}
}

func TestChannelProtocolAndAuth(t *testing.T) {
	for _, channelType := range []string{"openai", "custom", ""} {
		channel := &Channel{Type: channelType, ApiKey: "sk-test"}
		if got := ChannelProtocol(channel); got != ProtocolOpenAi {
			t.Errorf("ChannelProtocol(%q) = %s, expected %s", channelType, got, ProtocolOpenAi)
		}

		header := http.Header{}
		SetChannelAuth(header, channel)
		if header.Get("Authorization") != "Bearer sk-test" || header.Get("X-Api-Key") != "" {
			t.Errorf("openai auth header = %v", header)
		}
	}

	channel := &Channel{Type: "anthropic", ApiKey: "sk-ant-test"}
	if got := ChannelProtocol(channel); got != ProtocolAnthropic {
		t.Errorf("ChannelProtocol(anthropic) = %s", got)
	}

	header := http.Header{}
	SetChannelAuth(header, channel)
	if header.Get("X-Api-Key") != "sk-ant-test" || header.Get("Authorization") != "" {
		t.Errorf("anthropic auth header = %v", header)
	}
}
