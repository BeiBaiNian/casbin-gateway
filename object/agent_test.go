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
	"testing"
)

func TestSetAgentChannelBindsAndUnbinds(t *testing.T) {
	initSqliteOrmer(t)

	channel := newTestChannel("codex-upstream", "sk-test")
	channel.BaseUrl = "https://api.openai.com/v1"
	if _, err := AddChannel(channel); err != nil {
		t.Fatal(err)
	}

	if err := SetAgentChannel("codex", "admin/codex-upstream"); err != nil {
		t.Fatal(err)
	}
	agents, err := GetAgents()
	if err != nil {
		t.Fatal(err)
	}
	if agents["codex"].Channel != "admin/codex-upstream" {
		t.Fatalf("expected the binding to be stored, got %q", agents["codex"].Channel)
	}

	if err := SetAgentChannel("codex", ""); err != nil {
		t.Fatal(err)
	}
	agents, err = GetAgents()
	if err != nil {
		t.Fatal(err)
	}
	if agents["codex"].Channel != "" {
		t.Fatal("expected an empty channel to unbind the agent")
	}
}

func TestSetAgentChannelRejectsAMissingChannel(t *testing.T) {
	initSqliteOrmer(t)

	if err := SetAgentChannel("codex", "admin/nope"); err == nil {
		t.Fatal("expected binding to a channel that does not exist to fail")
	}
	if err := SetAgentChannel("codex", "nope"); err == nil {
		t.Fatal("expected a malformed channel id to fail")
	}
}

func TestGetChannelByAgent(t *testing.T) {
	initSqliteOrmer(t)

	channel := newTestChannel("bound", "sk-test")
	channel.BaseUrl = "https://api.openai.com/v1"
	if _, err := AddChannel(channel); err != nil {
		t.Fatal(err)
	}

	if _, err := GetChannelByAgent("codex"); !errors.Is(err, ErrAgentNoChannel) {
		t.Fatalf("expected ErrAgentNoChannel for an unbound agent, got %v", err)
	}

	if err := SetAgentChannel("codex", "admin/bound"); err != nil {
		t.Fatal(err)
	}
	resolved, err := GetChannelByAgent("codex")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ApiKey != "sk-test" {
		t.Fatalf("expected the decrypted API key, got %q", resolved.ApiKey)
	}

	if _, err := DeleteChannel(channel); err != nil {
		t.Fatal(err)
	}
	if _, err := GetChannelByAgent("codex"); err == nil {
		t.Fatal("expected a binding pointing at a deleted channel to fail")
	}
}

func TestGetChannelByAgentRejectsADisabledChannel(t *testing.T) {
	initSqliteOrmer(t)

	channel := newTestChannel("off", "sk-test")
	channel.BaseUrl = "https://api.openai.com/v1"
	channel.Status = "disabled"
	if _, err := AddChannel(channel); err != nil {
		t.Fatal(err)
	}
	if err := SetAgentChannel("codex", "admin/off"); err != nil {
		t.Fatal(err)
	}

	if _, err := GetChannelByAgent("codex"); err == nil {
		t.Fatal("expected a disabled channel to be rejected")
	}
}
