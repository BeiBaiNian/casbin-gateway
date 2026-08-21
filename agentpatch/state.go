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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/conf"
)

// gatewayEntryName is the identifier Gateway writes into every agent
// configuration it touches, so operators can find and remove it by hand. The
// skill and MCP listings recognize it by the same name.
const gatewayEntryName = agentconfig.ManagedEntryName

type changeKind string

const (
	changeFile changeKind = "file"
	changeDir  changeKind = "dir"
)

type change struct {
	Kind        changeKind  `json:"kind"`
	Path        string      `json:"path"`
	Backup      string      `json:"backup,omitempty"`
	Mode        os.FileMode `json:"mode,omitempty"`
	PatchedHash string      `json:"patchedHash,omitempty"`
	PatchedMode os.FileMode `json:"patchedMode,omitempty"`
}

type manifest struct {
	AgentId   string    `json:"agentId"`
	Target    Target    `json:"target"`
	PatchedAt time.Time `json:"patchedAt"`
	Changes   []change  `json:"changes"`
}

var stateMutex sync.Mutex

// ChangeSet records the files and directories a patch owns. Files are restored
// only when they still contain the patch's content, so Unpatch never overwrites
// a configuration an operator has changed since Patch.
type ChangeSet struct {
	manifest  *manifest
	backupDir string
}

// MkdirAll creates dir and records only directories this patch created.
func (c *ChangeSet) MkdirAll(dir string) error {
	var created []string
	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(current); err == nil {
			break
		}
		created = append(created, current)
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for index := len(created) - 1; index >= 0; index-- {
		c.manifest.Changes = append(c.manifest.Changes, change{Kind: changeDir, Path: created[index]})
	}
	return nil
}

// ReadFile reads a patch target, returning empty content for a file Patch will
// create.
func (c *ChangeSet) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// WriteFile backs up the pre-patch file and records the replacement it owns.
func (c *ChangeSet) WriteFile(path string, data []byte, perm os.FileMode) error {
	item := change{Kind: changeFile, Path: path}
	if previous, err := os.ReadFile(path); err == nil {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		item.Mode = info.Mode().Perm()
		perm = item.Mode
		if err := os.MkdirAll(c.backupDir, 0o700); err != nil {
			return err
		}
		item.Backup = fmt.Sprintf("%d-%s", len(c.manifest.Changes), filepath.Base(path))
		if err := os.WriteFile(filepath.Join(c.backupDir, item.Backup), previous, 0o600); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	item.PatchedHash = contentHash(data)
	item.PatchedMode = info.Mode().Perm()
	c.manifest.Changes = append(c.manifest.Changes, item)
	return nil
}

// Apply runs a file-changing patch transaction and persists the information
// needed for Unpatch. A failed patch is rolled back before its error returns.
func Apply(target Target, apply func(*ChangeSet) error) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// A previous patch whose files were edited externally must not block a fresh
	// one: revertLocked has already cleaned up the tracked state by that point.
	if err := revertLocked(target); err != nil && !errors.As(err, new(*PartialRevertError)) {
		return err
	}
	changes := &ChangeSet{
		manifest:  &manifest{AgentId: target.AgentId, Target: target, PatchedAt: time.Now()},
		backupDir: backupDir(target),
	}
	if err := apply(changes); err != nil {
		_ = rollback(changes.manifest, changes.backupDir)
		return err
	}
	if err := saveManifest(target, changes.manifest); err != nil {
		_ = rollback(changes.manifest, changes.backupDir)
		_ = os.Remove(manifestPath(target))
		_ = os.RemoveAll(changes.backupDir)
		return err
	}
	return nil
}

// Revert restores a patch's unchanged files. It is a no-op when the target is
// not currently tracked by Gateway.
func Revert(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	return revertLocked(target)
}

// revertLocked restores what it can and always clears the tracked state, so an
// installation can never end up unable to unpatch and unable to re-patch.
func revertLocked(target Target) error {
	saved, err := loadManifest(target)
	if err != nil || saved == nil {
		return err
	}
	rollbackErr := rollback(saved, backupDir(target))
	if rollbackErr != nil && !errors.As(rollbackErr, new(*PartialRevertError)) {
		return rollbackErr
	}
	if err := os.Remove(manifestPath(target)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(backupDir(target)); err != nil {
		return err
	}
	return rollbackErr
}

// discardStateLocked drops a target's manifest and backups without restoring
// them. Patchers that own a precise entry rather than a whole file use it to
// clear state written by an earlier, backup-based release.
func discardStateLocked(target Target) {
	_ = os.Remove(manifestPath(target))
	_ = os.RemoveAll(backupDir(target))
}

// IsApplied reports whether Gateway has the manifest needed to restore target.
func IsApplied(target Target) bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	saved, err := loadManifest(target)
	return err == nil && saved != nil
}

// rollback restores the files this patch still owns. A file that changed after
// Patch is left exactly as the operator or the agent left it: refusing the whole
// unpatch instead would strand the installation, because Apply reverts first and
// so a stale manifest would block re-patching too. Skipped files are reported so
// the caller can tell the operator what to clean up by hand.
func rollback(saved *manifest, backups string) error {
	var first error
	var skipped []string
	for index := len(saved.Changes) - 1; index >= 0; index-- {
		item := saved.Changes[index]
		if item.Kind == changeDir {
			_ = os.Remove(item.Path)
			continue
		}
		if !patchStillOwns(item) {
			skipped = append(skipped, item.Path)
			continue
		}

		var err error
		if item.Backup == "" {
			err = os.Remove(item.Path)
			if os.IsNotExist(err) {
				err = nil
			}
		} else if content, readErr := os.ReadFile(filepath.Join(backups, item.Backup)); readErr != nil {
			err = readErr
		} else {
			err = os.WriteFile(item.Path, content, item.Mode)
			if err == nil {
				err = os.Chmod(item.Path, item.Mode)
			}
		}
		if err != nil && first == nil {
			first = fmt.Errorf("restore %s: %w", item.Path, err)
		}
	}
	if first == nil && len(skipped) != 0 {
		return &PartialRevertError{Paths: skipped}
	}
	return first
}

// PartialRevertError reports files that Unpatch deliberately left untouched
// because they changed after Patch. The patch state is still cleaned up, so the
// installation can be patched again.
type PartialRevertError struct {
	Paths []string
}

func (e *PartialRevertError) Error() string {
	return fmt.Sprintf("monitoring was disabled, but %s changed after Patch and was left unmodified; remove any remaining %q entry by hand",
		strings.Join(e.Paths, ", "), gatewayEntryName)
}

// patchStillOwns reports whether a file is byte-for-byte what Patch wrote.
func patchStillOwns(item change) bool {
	content, err := os.ReadFile(item.Path)
	if err != nil {
		// A file the patch created and something else deleted is already in the
		// desired post-unpatch state; a file we cannot read is not ours to touch.
		return os.IsNotExist(err) && item.Backup == ""
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		return false
	}
	return item.PatchedHash != "" && item.PatchedHash == contentHash(content) && item.PatchedMode == info.Mode().Perm()
}

func stateDir() string {
	return conf.GetAgentPatchStateDir()
}

func targetKey(target Target) string {
	sum := sha256.Sum256([]byte(target.AgentId + "|" + target.Owner + "|" + target.Path))
	return target.AgentId + "-" + hex.EncodeToString(sum[:])[:16]
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func manifestPath(target Target) string {
	return filepath.Join(stateDir(), targetKey(target)+".json")
}

func backupDir(target Target) string {
	return filepath.Join(stateDir(), targetKey(target))
}

func loadManifest(target Target) (*manifest, error) {
	data, err := os.ReadFile(manifestPath(target))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved manifest
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	return &saved, nil
}

func saveManifest(target Target, saved *manifest) error {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(target), append(data, '\n'), 0o600)
}
