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

func TestSetAgentProviderBindsAndUnbinds(t *testing.T) {
	initSqliteOrmer(t)

	provider := newTestProvider("codex-upstream", "sk-test")
	provider.BaseUrl = "https://api.openai.com/v1"
	if _, err := AddProvider(provider); err != nil {
		t.Fatal(err)
	}

	if err := SetAgentProvider("codex", "admin/codex-upstream"); err != nil {
		t.Fatal(err)
	}
	agents, err := GetAgents()
	if err != nil {
		t.Fatal(err)
	}
	if agents["codex"].Provider != "admin/codex-upstream" {
		t.Fatalf("expected the binding to be stored, got %q", agents["codex"].Provider)
	}

	if err := SetAgentProvider("codex", ""); err != nil {
		t.Fatal(err)
	}
	agents, err = GetAgents()
	if err != nil {
		t.Fatal(err)
	}
	if agents["codex"].Provider != "" {
		t.Fatal("expected an empty provider to unbind the agent")
	}
}

func TestSetAgentProviderRejectsAMissingProvider(t *testing.T) {
	initSqliteOrmer(t)

	if err := SetAgentProvider("codex", "admin/nope"); err == nil {
		t.Fatal("expected binding to a provider that does not exist to fail")
	}
	if err := SetAgentProvider("codex", "nope"); err == nil {
		t.Fatal("expected a malformed provider id to fail")
	}
}

func TestGetProviderByAgent(t *testing.T) {
	initSqliteOrmer(t)

	provider := newTestProvider("bound", "sk-test")
	provider.BaseUrl = "https://api.openai.com/v1"
	if _, err := AddProvider(provider); err != nil {
		t.Fatal(err)
	}

	if _, err := GetProviderByAgent("codex"); !errors.Is(err, ErrAgentNoProvider) {
		t.Fatalf("expected ErrAgentNoProvider for an unbound agent, got %v", err)
	}

	if err := SetAgentProvider("codex", "admin/bound"); err != nil {
		t.Fatal(err)
	}
	resolved, err := GetProviderByAgent("codex")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ApiKey != "sk-test" {
		t.Fatalf("expected the decrypted API key, got %q", resolved.ApiKey)
	}

	if _, err := DeleteProvider(provider); err != nil {
		t.Fatal(err)
	}
	if _, err := GetProviderByAgent("codex"); err == nil {
		t.Fatal("expected a binding pointing at a deleted provider to fail")
	}
}

func TestGetProviderByAgentRejectsADisabledProvider(t *testing.T) {
	initSqliteOrmer(t)

	provider := newTestProvider("off", "sk-test")
	provider.BaseUrl = "https://api.openai.com/v1"
	provider.Status = "disabled"
	if _, err := AddProvider(provider); err != nil {
		t.Fatal(err)
	}
	if err := SetAgentProvider("codex", "admin/off"); err != nil {
		t.Fatal(err)
	}

	if _, err := GetProviderByAgent("codex"); err == nil {
		t.Fatal("expected a disabled provider to be rejected")
	}
}
