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

package agentpatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/apache/casbin-gateway/agent"
)

// Monitoring is on by default: an installation Gateway finds is patched without
// waiting for a click. Turning one off by hand leaves an opt-out marker beside
// its manifest, so the next scan leaves that installation alone.

func optOutPath(target Target) string {
	return filepath.Join(stateDir(), targetKey(target)+".off")
}

func isOptedOut(target Target) bool {
	_, err := os.Stat(optOutPath(target))
	return err == nil
}

func setOptOut(target Target, off bool) {
	if !off {
		_ = os.Remove(optOutPath(target))
		return
	}
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(optOutPath(target), []byte(time.Now().Format(time.RFC3339)), 0o600)
}

// EnsurePatched turns monitoring on for one installation, unless it cannot be
// monitored, already is, or was turned off by hand.
func EnsurePatched(target Target) error {
	patcher, ok := patchers[target.AgentId]
	if !ok || !patcher.Supported() || isOptedOut(target) {
		return nil
	}

	status, err := patcher.Status(target)
	if err != nil {
		return err
	}
	if status.Patched {
		return nil
	}
	return patcher.Patch(target)
}

// EnableAll turns monitoring on for every agent installed on this host. The
// failures are joined so one installation that cannot be patched neither hides
// the others nor stops them.
func EnableAll() error {
	installations, err := agent.Scan(false)
	if err != nil {
		return err
	}

	failures := []error{}
	for _, installation := range installations {
		target := Target{AgentId: installation.AgentId, Path: installation.Path, Owner: installation.Owner}
		if err := EnsurePatched(target); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", installation.AgentId, err))
		}
	}
	return errors.Join(failures...)
}
