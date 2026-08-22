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

package util

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

const testAad = "admin/provider1"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "test-secret"
	plaintext := "sk-1234567890abcdef"

	ciphertext, err := EncryptWithKey(secret, plaintext, testAad)
	if err != nil {
		t.Fatalf("EncryptWithKey failed: %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext equals plaintext; value was not encrypted")
	}
	if !IsEncrypted(ciphertext) {
		t.Fatalf("ciphertext is missing the encryption marker: %q", ciphertext)
	}

	got, err := DecryptWithKey(secret, ciphertext, testAad)
	if err != nil {
		t.Fatalf("DecryptWithKey failed: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestEncryptIsNonDeterministic(t *testing.T) {
	secret, plaintext := "s", "same-value"
	a, _ := EncryptWithKey(secret, plaintext, testAad)
	b, _ := EncryptWithKey(secret, plaintext, testAad)
	if a == b {
		t.Fatal("two encryptions produced identical ciphertext; nonce is not random")
	}
}

func TestEncryptOptOut(t *testing.T) {
	if got, _ := EncryptWithKey("", "plain", testAad); got != "plain" {
		t.Fatalf("empty secret should pass through, got %q", got)
	}
	if got, _ := EncryptWithKey("secret", "", testAad); got != "" {
		t.Fatalf("empty value should pass through, got %q", got)
	}
}

func TestEncryptIdempotent(t *testing.T) {
	secret := "s"
	once, _ := EncryptWithKey(secret, "value", testAad)
	twice, err := EncryptWithKey(secret, once, testAad)
	if err != nil {
		t.Fatalf("re-encrypt failed: %v", err)
	}
	if twice != once {
		t.Fatal("re-encrypting an already-encrypted value changed it")
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	if got, err := DecryptWithKey("", "legacy-plain-key", testAad); err != nil || got != "legacy-plain-key" {
		t.Fatalf("legacy plaintext passthrough failed: got %q, err %v", got, err)
	}
	if got, err := DecryptWithKey("secret", "legacy-plain-key", testAad); err != nil || got != "legacy-plain-key" {
		t.Fatalf("legacy plaintext passthrough with key failed: got %q, err %v", got, err)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	ciphertext, _ := EncryptWithKey("right-key", "secret-value", testAad)
	if _, err := DecryptWithKey("wrong-key", ciphertext, testAad); err == nil {
		t.Fatal("decrypting with the wrong key should fail")
	}
}

func TestDecryptEncryptedWithoutKeyFails(t *testing.T) {
	ciphertext, _ := EncryptWithKey("key", "secret-value", testAad)
	if _, err := DecryptWithKey("", ciphertext, testAad); err == nil {
		t.Fatal("decrypting ciphertext without a configured key should fail")
	}
}

func TestDecryptWrongAadFails(t *testing.T) {
	ciphertext, _ := EncryptWithKey("key", "secret-value", "admin/provider1")
	if _, err := DecryptWithKey("key", ciphertext, "admin/provider2"); err == nil {
		t.Fatal("decrypting with a different aad should fail")
	}
	if got, err := DecryptWithKey("key", ciphertext, "admin/provider1"); err != nil || got != "secret-value" {
		t.Fatalf("decrypting with the original aad failed: got %q, err %v", got, err)
	}
}

func TestDecryptV1WithoutAad(t *testing.T) {
	secret, plaintext := "key", "secret-value"

	gcm, err := newGCM(secret)
	if err != nil {
		t.Fatalf("newGCM failed: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("nonce failed: %v", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	v1 := encPrefixV1 + base64.StdEncoding.EncodeToString(sealed)

	if !IsEncrypted(v1) {
		t.Fatal("a v1 value should still be recognized as encrypted")
	}
	got, err := DecryptWithKey(secret, v1, testAad)
	if err != nil {
		t.Fatalf("decrypting a v1 value failed: %v", err)
	}
	if got != plaintext {
		t.Fatalf("v1 round trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestNeedsReEncryption(t *testing.T) {
	v2, _ := EncryptWithKey("key", "value", testAad)

	cases := []struct {
		name   string
		secret string
		value  string
		want   bool
	}{
		{"encryption off", "", "plaintext", false},
		{"empty value", "key", "", false},
		{"legacy plaintext", "key", "plaintext", true},
		{"v1 ciphertext", "key", encPrefixV1 + "whatever", true},
		{"current format", "key", v2, false},
	}
	for _, tc := range cases {
		if got := NeedsReEncryption(tc.secret, tc.value); got != tc.want {
			t.Errorf("%s: NeedsReEncryption() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The api_key column in object.Provider is sized against this growth.
func TestCiphertextFitsColumn(t *testing.T) {
	const maxKeyChars, columnWidth = 500, 1000

	ciphertext, err := EncryptWithKey("key", strings.Repeat("a", maxKeyChars), testAad)
	if err != nil {
		t.Fatalf("EncryptWithKey failed: %v", err)
	}
	if len(ciphertext) > columnWidth {
		t.Fatalf("a %d character key encrypts to %d characters, which overflows the varchar(%d) api_key column", maxKeyChars, len(ciphertext), columnWidth)
	}
}
