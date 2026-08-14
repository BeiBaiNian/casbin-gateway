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

package object

import (
	"strings"
	"testing"
)

func TestValidateToken(t *testing.T) {
	validToken := func() *Token {
		return &Token{Name: "t1", DisplayName: "Token 1"}
	}

	cases := []struct {
		desc    string
		mutate  func(*Token)
		wantErr string
	}{
		{"valid minimal token", func(t *Token) {}, ""},
		{"empty name", func(t *Token) { t.Name = "" }, "name cannot be empty"},
		{"empty display name", func(t *Token) { t.DisplayName = "" }, "display name cannot be empty"},
		{"invalid status", func(t *Token) { t.Status = "paused" }, "invalid token status"},
		{"malformed expire time", func(t *Token) { t.ExpireTime = "2026-08-14" }, "RFC3339"},
		{"malformed expire time garbage", func(t *Token) { t.ExpireTime = "abc" }, "RFC3339"},
		{"valid expire time", func(t *Token) { t.ExpireTime = "2026-08-14T10:30:00+08:00" }, ""},
		{"empty model name", func(t *Token) { t.AllowedModels = []string{"gpt-4", "  "} }, "model name cannot be empty"},
		{"too long model name", func(t *Token) { t.AllowedModels = []string{strings.Repeat("a", maxTokenModelChars+1)} }, "too long"},
	}

	for _, tc := range cases {
		token := validToken()
		tc.mutate(token)
		err := validateToken(token)
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s: expected no error, got: %v", tc.desc, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("%s: expected error containing %q, got: %v", tc.desc, tc.wantErr, err)
		}
	}

	// Too many models: the limit is about the model count, not a character
	// budget for the serialized column.
	manyModels := &Token{Name: "t1", DisplayName: "Token 1"}
	for i := 0; i <= maxTokenModels; i++ {
		manyModels.AllowedModels = append(manyModels.AllowedModels, "m")
	}
	if err := validateToken(manyModels); err == nil || !strings.Contains(err.Error(), "too many allowed models") {
		t.Errorf("expected too many models error, got: %v", err)
	}

	// Defaults are filled in.
	token := validToken()
	if err := validateToken(token); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.Status != "enabled" {
		t.Errorf("status default = %s, expected enabled", token.Status)
	}
	if token.AllowedModels == nil {
		t.Error("allowedModels should default to an empty slice, not nil")
	}
}

func TestGenerateSecretKey(t *testing.T) {
	key, err := generateSecretKey()
	if err != nil {
		t.Fatalf("generateSecretKey failed: %v", err)
	}
	if len(key) != 48 {
		t.Errorf("key length = %d, expected 48", len(key))
	}
	for _, c := range key {
		if !strings.ContainsRune(tokenCharset, c) {
			t.Errorf("key contains a character outside the charset: %q", c)
		}
	}

	key2, _ := generateSecretKey()
	if key == key2 {
		t.Error("two generated keys are identical")
	}
}

func TestGetMaskedToken(t *testing.T) {
	token := &Token{SecretKey: "sk-secret", SecretKeyPrefix: "sk-secr****"}
	masked := GetMaskedToken(token)
	if masked.SecretKey != "" {
		t.Errorf("masked token still carries the secret key: %q", masked.SecretKey)
	}
	if masked.SecretKeyPrefix != "sk-secr****" {
		t.Errorf("masked token prefix changed: %q", masked.SecretKeyPrefix)
	}

	// Nil stays nil.
	if GetMaskedToken(nil) != nil {
		t.Error("GetMaskedToken(nil) should return nil")
	}
}
