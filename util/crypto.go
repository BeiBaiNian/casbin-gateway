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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The prefix tells ciphertext apart from a legacy plaintext value, and carries
// a format version: v1 is not bound to anything, v2 feeds the caller's aad to
// GCM. v1 is accepted on read, never written any more.
const (
	encPrefixV1 = "enc:v1:"
	encPrefix   = "enc:v2:"
)

// IsEncrypted reports whether value carries a ciphertext marker of any version.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encPrefix) || strings.HasPrefix(value, encPrefixV1)
}

// NeedsReEncryption reports whether a stored value should be written back: it
// is either legacy plaintext or ciphertext from an older format version.
func NeedsReEncryption(secret, value string) bool {
	return secret != "" && value != "" && !strings.HasPrefix(value, encPrefix)
}

func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// EncryptWithKey returns an AES-256-GCM ciphertext (base64, prefixed) for value.
// Encryption is opt-in: an empty secret or empty value returns value unchanged,
// and an already-encrypted value is returned as-is so re-saving is idempotent.
//
// aad is authenticated but not encrypted, and DecryptWithKey has to be given the
// same value. Pass whatever identifies the place the ciphertext is stored, so
// that moving it elsewhere makes it undecryptable.
func EncryptWithKey(secret, value, aad string) (string, error) {
	if secret == "" || value == "" || IsEncrypted(value) {
		return value, nil
	}

	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Prepend the nonce to the ciphertext so DecryptWithKey can recover it.
	sealed := gcm.Seal(nonce, nonce, []byte(value), []byte(aad))
	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptWithKey reverses EncryptWithKey. A value without a marker is treated
// as legacy plaintext and returned unchanged, which keeps pre-encryption rows
// usable after the key is switched on.
func DecryptWithKey(secret, value, aad string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	if secret == "" {
		return "", errors.New("the value is encrypted but no encryption key is configured")
	}

	prefix := encPrefix
	if strings.HasPrefix(value, encPrefixV1) {
		prefix, aad = encPrefixV1, ""
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", err
	}

	gcm, err := newGCM(secret)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid ciphertext: too short")
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		// gcm.Open() gives the same answer for a wrong key and for a value moved
		// to another row.
		return "", fmt.Errorf("cannot decrypt the value, the encryption key may have changed: %w", err)
	}
	return string(plaintext), nil
}

func newGCM(secret string) (cipher.AEAD, error) {
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
