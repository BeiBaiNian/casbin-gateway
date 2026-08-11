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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"

	"github.com/apache/casbin-gateway/conf"
)

func getAESKey() []byte {
	key := os.Getenv("CASGATEWAY_AES_KEY")
	if key == "" {
		key = conf.GetConfigString("aesEncryptionKey")
	}
	if key == "" {
		panic("aesEncryptionKey not configured: set CASGATEWAY_AES_KEY env or aesEncryptionKey in app.conf")
	}

	keyBytes := []byte(key)
	if len(keyBytes) == 32 {
		return keyBytes
	}

	hashed := sha256.Sum256(keyBytes)
	return hashed[:]
}

// EncryptApiKey encrypts a plaintext API key using AES-256-GCM.
// Returns the base64-encoded ciphertext (nonce + encrypted data).
func EncryptApiKey(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key := getAESKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptApiKey decrypts a base64-encoded ciphertext using AES-256-GCM.
// Returns the plaintext API key.
func DecryptApiKey(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	key := getAESKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", io.ErrUnexpectedEOF
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
