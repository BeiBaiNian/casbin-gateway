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

package agentprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/apache/casbin-gateway/conf"
)

// state is what one installation was switched to, and what it looked like
// before the first switch. Previous holds only the keys Gateway writes: a key
// missing from it was not set, and is deleted again on restore.
type state struct {
	AgentId  string            `json:"agentId"`
	Provider string            `json:"provider"`
	Mode     string            `json:"mode"`
	BaseUrl  string            `json:"baseUrl"`
	Time     string            `json:"time"`
	Files    []string          `json:"files"`
	Previous map[string]string `json:"previous"`
}

func stateDir() string {
	return filepath.Join(conf.GetAgentPatchStateDir(), "providers")
}

func statePath(target Target) string {
	sum := sha256.Sum256([]byte(target.AgentId + "|" + target.Owner + "|" + target.Path))
	return filepath.Join(stateDir(), target.AgentId+"-"+hex.EncodeToString(sum[:])[:16]+".json")
}

func loadState(target Target) (*state, error) {
	data, err := os.ReadFile(statePath(target))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	saved := &state{}
	if err := json.Unmarshal(data, saved); err != nil {
		return nil, err
	}
	return saved, nil
}

func saveState(target Target, saved *state) error {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(target), append(data, '\n'), 0o600)
}

func clearState(target Target) error {
	err := os.Remove(statePath(target))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func nowString() string {
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}
