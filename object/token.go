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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
	"golang.org/x/crypto/bcrypt"
)

// Token is an API access token for the LLM gateway. (Milestone 1.3)
type Token struct {
	Owner           string   `xorm:"varchar(100) notnull pk" json:"owner"`
	Name            string   `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime     string   `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime     string   `xorm:"varchar(100)" json:"updatedTime"`
	DisplayName     string   `xorm:"varchar(100)" json:"displayName"`
	SecretKeyHash   string   `xorm:"varchar(255)" json:"-"`
	SecretKeyPrefix string   `xorm:"varchar(20)" json:"secretKeyPrefix"`
	ExpireTime      string   `xorm:"varchar(100)" json:"expireTime"`
	AllowedModels   []string `xorm:"varchar(500)" json:"allowedModels"`
	RateLimit       int      `xorm:"int" json:"rateLimit"`
	Status          string   `xorm:"varchar(100)" json:"status"`
}

func GetTokens(owner string) ([]*Token, error) {
	tokens := []*Token{}
	engine := ormer.Engine.Desc("created_time")
	if owner != "" {
		engine = engine.Where("owner = ?", owner)
	}
	err := engine.Find(&tokens)
	return tokens, err
}

func GetTokenCount(owner string) (int64, error) {
	if owner == "" {
		return ormer.Engine.Count(&Token{})
	}
	return ormer.Engine.Where("owner = ?", owner).Count(&Token{})
}

func GetPaginationTokens(owner string, offset, limit int) ([]*Token, error) {
	tokens := []*Token{}
	engine := ormer.Engine.Desc("created_time").Limit(limit, offset)
	if owner != "" {
		engine = engine.Where("owner = ?", owner)
	}
	err := engine.Find(&tokens)
	return tokens, err
}

func getToken(owner, name string) (*Token, error) {
	token := &Token{Owner: owner, Name: name}
	ok, err := ormer.Engine.Get(token)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return token, nil
}

func GetToken(id string) (*Token, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getToken(owner, name)
}

func GetMaskedToken(token *Token) *Token {
	if token != nil {
		token.SecretKeyHash = ""
	}
	return token
}

func GetMaskedTokens(tokens []*Token) []*Token {
	for _, token := range tokens {
		GetMaskedToken(token)
	}
	return tokens
}

func validateToken(token *Token) error {
	if token.Status == "" {
		token.Status = "enabled"
	}
	if token.AllowedModels == nil {
		token.AllowedModels = []string{}
	}

	if token.Status != "enabled" && token.Status != "disabled" {
		return fmt.Errorf("invalid token status")
	}
	if len(token.AllowedModels) > 0 {
		modelJSON, err := json.Marshal(token.AllowedModels)
		if err != nil {
			return fmt.Errorf("failed to marshal allowed models: %v", err)
		}
		if len(modelJSON) > 500 {
			return fmt.Errorf("allowed models exceed 500 characters")
		}
	}

	return nil
}

const tokenCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateSecretKey() (string, error) {
	length := 48
	b := make([]byte, length)
	for i := range b {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(tokenCharset))))
		if err != nil {
			return "", err
		}
		b[i] = tokenCharset[index.Int64()]
	}
	return string(b), nil
}

// AddToken creates a new token. It generates a secret key, stores its bcrypt hash,
// and returns the plaintext secretKey separately (not stored in the Token struct).
// Returns: (affected bool, secretKey string, error)
func AddToken(token *Token) (bool, string, error) {
	if err := validateToken(token); err != nil {
		return false, "", err
	}

	now := util.GetCurrentTime()
	if token.CreatedTime == "" {
		token.CreatedTime = now
	}
	token.UpdatedTime = now

	randomPart, err := generateSecretKey()
	if err != nil {
		return false, "", err
	}
	secretKey := "sk-" + randomPart

	hash, err := bcrypt.GenerateFromPassword([]byte(secretKey), bcrypt.DefaultCost)
	if err != nil {
		return false, "", err
	}
	token.SecretKeyHash = string(hash)

	if len(secretKey) >= 7 {
		token.SecretKeyPrefix = secretKey[:7] + "****"
	} else {
		token.SecretKeyPrefix = secretKey + "****"
	}

	n, err := ormer.Engine.Insert(token)
	return n != 0, secretKey, err
}

func UpdateToken(id string, token *Token) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)

	if err := validateToken(token); err != nil {
		return false, err
	}

	token.Owner = owner
	token.Name = name
	token.UpdatedTime = util.GetCurrentTime()

	n, err := ormer.Engine.ID(core.PK{owner, name}).Omit("secret_key_hash", "secret_key_prefix").AllCols().Update(token)
	return n != 0, err
}

func DeleteToken(token *Token) (bool, error) {
	n, err := ormer.Engine.ID(core.PK{token.Owner, token.Name}).Delete(&Token{})
	return n != 0, err
}
