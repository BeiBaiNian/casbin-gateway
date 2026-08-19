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

	results, err := ormer.Engine.Table("channel").
		Where("owner = ? and name = ?", owner, name).QueryString()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 stored channel, got %d", len(results))
	}
	return results[0]["api_key"]
}

func newTestChannel(name, apiKey string) *Channel {
	return &Channel{
		Owner:  "admin",
		Name:   name,
		Type:   "openai",
		Status: "enabled",
		Models: []string{"gpt-4o"},
		ApiKey: apiKey,
	}
}

func TestChannelApiKeyIsEncryptedAtRest(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	const plainKey = "sk-1234567890abcdef"
	if _, err := AddChannel(newTestChannel("encrypted", plainKey)); err != nil {
		t.Fatal(err)
	}

	stored := readStoredApiKey(t, "admin", "encrypted")
	if stored == plainKey {
		t.Fatal("the api key was written to the database as plaintext")
	}
	if !util.IsEncrypted(stored) {
		t.Fatalf("the stored api key is missing the encryption marker: %q", stored)
	}

	channel, err := GetChannel("admin/encrypted")
	if err != nil {
		t.Fatal(err)
	}
	if channel.ApiKey != plainKey {
		t.Fatalf("reading back the channel gave %q, want %q", channel.ApiKey, plainKey)
	}
}

func TestChannelApiKeyStaysPlaintextWithoutKey(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": ""})

	const plainKey = "sk-not-encrypted"
	if _, err := AddChannel(newTestChannel("plain", plainKey)); err != nil {
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
	if _, err := AddChannel(newTestChannel("legacy", plainKey)); err != nil {
		t.Fatal(err)
	}
	if stored := readStoredApiKey(t, "admin", "legacy"); stored != plainKey {
		t.Fatalf("setup failed, the key should still be plaintext, got %q", stored)
	}

	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	channel, err := GetChannel("admin/legacy")
	if err != nil {
		t.Fatal(err)
	}
	if channel.ApiKey != plainKey {
		t.Fatalf("a legacy plaintext key should still be readable, got %q", channel.ApiKey)
	}

	stored := readStoredApiKey(t, "admin", "legacy")
	if !util.IsEncrypted(stored) {
		t.Fatalf("the legacy key should have been re-written as ciphertext, got %q", stored)
	}

	channel, err = GetChannel("admin/legacy")
	if err != nil {
		t.Fatal(err)
	}
	if channel.ApiKey != plainKey {
		t.Fatalf("after the upgrade the key read back as %q, want %q", channel.ApiKey, plainKey)
	}
}

func TestUpgradeOnlyTouchesTheApiKeyColumn(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": ""})

	if _, err := AddChannel(newTestChannel("untouched", "sk-legacy")); err != nil {
		t.Fatal(err)
	}
	before, err := GetChannel("admin/untouched")
	if err != nil {
		t.Fatal(err)
	}

	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})
	if _, err = GetChannel("admin/untouched"); err != nil {
		t.Fatal(err)
	}

	after, err := GetChannel("admin/untouched")
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

func TestCiphertextIsBoundToItsChannel(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	if _, err := AddChannel(newTestChannel("victim", "sk-victims-key")); err != nil {
		t.Fatal(err)
	}
	if _, err := AddChannel(newTestChannel("thief", "sk-thiefs-key")); err != nil {
		t.Fatal(err)
	}

	victimCipher := readStoredApiKey(t, "admin", "victim")
	if _, err := ormer.Engine.ID(core.PK{"admin", "thief"}).
		Cols("api_key").Update(&Channel{ApiKey: victimCipher}); err != nil {
		t.Fatal(err)
	}

	thief, err := GetChannel("admin/thief")
	if err != nil {
		t.Fatal(err)
	}
	if thief.ApiKey == "sk-victims-key" {
		t.Fatal("a ciphertext moved to another channel decrypted, it is not bound to its row")
	}
	if !util.IsEncrypted(thief.ApiKey) {
		t.Fatalf("a value that cannot be decrypted should be left alone, got %q", thief.ApiKey)
	}

	victim, err := GetChannel("admin/victim")
	if err != nil {
		t.Fatal(err)
	}
	if victim.ApiKey != "sk-victims-key" {
		t.Fatalf("the victim channel read back as %q", victim.ApiKey)
	}
}

func TestUpdateChannelKeepsMaskedApiKey(t *testing.T) {
	initSqliteOrmer(t)
	setEnv(t, map[string]string{"apiKeyEncryptionKey": "test-encryption-key"})

	const plainKey = "sk-keep-me"
	if _, err := AddChannel(newTestChannel("masked", plainKey)); err != nil {
		t.Fatal(err)
	}

	edited := newTestChannel("masked", ApiKeyMask)
	edited.DisplayName = "renamed"
	if _, err := UpdateChannel("admin/masked", edited); err != nil {
		t.Fatal(err)
	}

	channel, err := GetChannel("admin/masked")
	if err != nil {
		t.Fatal(err)
	}
	if channel.ApiKey != plainKey {
		t.Fatalf("the stored key changed on a masked update: got %q, want %q", channel.ApiKey, plainKey)
	}
	if channel.DisplayName != "renamed" {
		t.Fatalf("the update did not apply: displayName is %q", channel.DisplayName)
	}
}
