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
	"testing"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

func readStoredApiKey(t *testing.T, owner, name string) string {
	t.Helper()

	results, err := ormer.Engine.Table("provider").
		Where("owner = ? and name = ?", owner, name).QueryString()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 stored provider, got %d", len(results))
	}
	return results[0]["api_key"]
}

func newTestProvider(name, apiKey string) *Provider {
	return &Provider{
		Owner:  "admin",
		Name:   name,
		Type:   "openai",
		Status: "enabled",
		Models: []string{"gpt-4o"},
		ApiKey: apiKey,
	}
}

func TestProviderApiKeyIsEncryptedAtRest(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	const plainKey = "sk-1234567890abcdef"
	if _, err := AddProvider(newTestProvider("encrypted", plainKey)); err != nil {
		t.Fatal(err)
	}

	stored := readStoredApiKey(t, "admin", "encrypted")
	if stored == plainKey {
		t.Fatal("the api key was written to the database as plaintext")
	}
	if !util.IsEncrypted(stored) {
		t.Fatalf("the stored api key is missing the encryption marker: %q", stored)
	}

	provider, err := GetProvider("admin/encrypted")
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != plainKey {
		t.Fatalf("reading back the provider gave %q, want %q", provider.ApiKey, plainKey)
	}
}

func TestProviderApiKeyStaysPlaintextWithoutKey(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": ""})

	const plainKey = "sk-not-encrypted"
	if _, err := AddProvider(newTestProvider("plain", plainKey)); err != nil {
		t.Fatal(err)
	}

	if stored := readStoredApiKey(t, "admin", "plain"); stored != plainKey {
		t.Fatalf("with encryption off the key should be stored as-is, got %q", stored)
	}
}

func TestLegacyPlaintextIsUpgradedOnRead(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": ""})

	const plainKey = "sk-legacy-plaintext"
	if _, err := AddProvider(newTestProvider("legacy", plainKey)); err != nil {
		t.Fatal(err)
	}
	if stored := readStoredApiKey(t, "admin", "legacy"); stored != plainKey {
		t.Fatalf("setup failed, the key should still be plaintext, got %q", stored)
	}

	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	provider, err := GetProvider("admin/legacy")
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != plainKey {
		t.Fatalf("a legacy plaintext key should still be readable, got %q", provider.ApiKey)
	}

	stored := readStoredApiKey(t, "admin", "legacy")
	if !util.IsEncrypted(stored) {
		t.Fatalf("the legacy key should have been re-written as ciphertext, got %q", stored)
	}

	provider, err = GetProvider("admin/legacy")
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != plainKey {
		t.Fatalf("after the upgrade the key read back as %q, want %q", provider.ApiKey, plainKey)
	}
}

func TestUpgradeOnlyTouchesTheApiKeyColumn(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": ""})

	if _, err := AddProvider(newTestProvider("untouched", "sk-legacy")); err != nil {
		t.Fatal(err)
	}
	before, err := GetProvider("admin/untouched")
	if err != nil {
		t.Fatal(err)
	}

	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})
	if _, err = GetProvider("admin/untouched"); err != nil {
		t.Fatal(err)
	}

	after, err := GetProvider("admin/untouched")
	if err != nil {
		t.Fatal(err)
	}
	if after.UpdatedTime != before.UpdatedTime {
		t.Fatalf("the upgrade changed UpdatedTime: %q -> %q", before.UpdatedTime, after.UpdatedTime)
	}
	if len(after.Models) != 1 || after.Models[0] != "gpt-4o" {
		t.Fatalf("the upgrade damaged the models column: %v", after.Models)
	}
	if after.Status != before.Status || after.Type != before.Type {
		t.Fatal("the upgrade changed columns other than api_key")
	}
}

func TestCiphertextIsBoundToItsProvider(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	if _, err := AddProvider(newTestProvider("victim", "sk-victims-key")); err != nil {
		t.Fatal(err)
	}
	if _, err := AddProvider(newTestProvider("thief", "sk-thiefs-key")); err != nil {
		t.Fatal(err)
	}

	victimCipher := readStoredApiKey(t, "admin", "victim")
	if _, err := ormer.Engine.ID(core.PK{"admin", "thief"}).
		Cols("api_key").Update(&Provider{ApiKey: victimCipher}); err != nil {
		t.Fatal(err)
	}

	thief, err := GetProvider("admin/thief")
	if err != nil {
		t.Fatal(err)
	}
	if thief.ApiKey == "sk-victims-key" {
		t.Fatal("a ciphertext moved to another provider decrypted, it is not bound to its row")
	}
	if !util.IsEncrypted(thief.ApiKey) {
		t.Fatalf("a value that cannot be decrypted should be left alone, got %q", thief.ApiKey)
	}

	victim, err := GetProvider("admin/victim")
	if err != nil {
		t.Fatal(err)
	}
	if victim.ApiKey != "sk-victims-key" {
		t.Fatalf("the victim provider read back as %q", victim.ApiKey)
	}
}

func TestUpdateProviderKeepsMaskedApiKey(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	const plainKey = "sk-keep-me"
	if _, err := AddProvider(newTestProvider("masked", plainKey)); err != nil {
		t.Fatal(err)
	}

	edited := newTestProvider("masked", ApiKeyMask)
	edited.DisplayName = "renamed"
	if _, err := UpdateProvider("admin/masked", edited); err != nil {
		t.Fatal(err)
	}

	provider, err := GetProvider("admin/masked")
	if err != nil {
		t.Fatal(err)
	}
	if provider.ApiKey != plainKey {
		t.Fatalf("the stored key changed on a masked update: got %q, want %q", provider.ApiKey, plainKey)
	}
	if provider.DisplayName != "renamed" {
		t.Fatalf("the update did not apply: displayName is %q", provider.DisplayName)
	}
}
