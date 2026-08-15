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

package agentmonitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	codexMonitorStateFile = "codex-rollout-monitor.json"
	maxCodexRolloutLine   = 8 * 1024 * 1024
)

type codexClaim struct {
	AgentID   string `json:"agentId"`
	Path      string `json:"path"`
	Owner     string `json:"owner"`
	CodexHome string `json:"codexHome"`
}

type codexPendingCall struct {
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	TurnID    string    `json:"turnId,omitempty"`
	Object    string    `json:"object,omitempty"`
}

type codexCursor struct {
	Path       string                      `json:"path"`
	Root       string                      `json:"root"`
	Offset     int64                       `json:"offset"`
	SessionKey string                      `json:"sessionKey,omitempty"`
	AgentID    string                      `json:"agentId,omitempty"`
	Model      string                      `json:"model,omitempty"`
	TurnID     string                      `json:"turnId,omitempty"`
	Pending    map[string]codexPendingCall `json:"pending,omitempty"`
}

type codexSavedState struct {
	Claims  []codexClaim            `json:"claims"`
	Cursors map[string]*codexCursor `json:"cursors"`
}

// codexRootSummary is what one sessions directory looked like after the last
// poll. Status answers from it so that reading the UI never waits on disk and
// never touches cursor state the poller is mutating.
type codexRootSummary struct {
	agentFiles  map[string]int
	unknown     int
	lastPolled  time.Time
	everSampled bool
}

// codexMonitorManager owns two locks with a strict order: pollMutex is taken
// before mutex, never the other way round. pollMutex serializes the file IO and
// the cursor mutation it performs; mutex only guards the maps, so status stays
// responsive while a poll is reading disk.
type codexMonitorManager struct {
	pollMutex sync.Mutex

	mutex     sync.Mutex
	statePath string
	addRecord func(*Record)
	claims    map[string]codexClaim
	cursors   map[string]*codexCursor
	lastErr   map[string]error
	summaries map[string]codexRootSummary
	dirty     bool
	stop      chan struct{}
	done      chan struct{}
}

var codexMonitor = newCodexMonitorManager("", AddRecord)

func newCodexMonitorManager(statePath string, addRecord func(*Record)) *codexMonitorManager {
	return &codexMonitorManager{
		statePath: statePath,
		addRecord: addRecord,
		claims:    map[string]codexClaim{},
		cursors:   map[string]*codexCursor{},
		lastErr:   map[string]error{},
		summaries: map[string]codexRootSummary{},
	}
}

func (manager *codexMonitorManager) start() error {
	// pollMutex is always taken before mutex; see codexMonitorManager.
	manager.pollMutex.Lock()
	defer manager.pollMutex.Unlock()
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.stop != nil {
		return nil
	}
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(codexMonitorStateFile)
	}
	if err := manager.loadLocked(); err != nil {
		return err
	}
	manager.startPollerLocked()
	return nil
}

func (manager *codexMonitorManager) stopMonitor() {
	manager.mutex.Lock()
	stop, done := manager.stop, manager.done
	manager.stop, manager.done = nil, nil
	manager.mutex.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

// ResolveCodexHome finds the Codex state directory belonging to an installed
// Codex application. CODEX_HOME is meaningful only for the current user.
func ResolveCodexHome(agentPath, ownerName string) (string, error) {
	ownerName = strings.TrimSpace(ownerName)
	current, _ := user.Current()
	if current != nil && strings.EqualFold(accountName(ownerName), accountName(current.Username)) {
		if configured := strings.TrimSpace(os.Getenv("CODEX_HOME")); configured != "" {
			if !filepath.IsAbs(configured) {
				return "", errors.New("CODEX_HOME must be an absolute path")
			}
			return filepath.Clean(configured), nil
		}
		if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
			return filepath.Join(home, ".codex"), nil
		}
	}

	candidates := []string{ownerName}
	if index := strings.LastIndexAny(ownerName, `\\/`); index >= 0 && index+1 < len(ownerName) {
		candidates = append(candidates, ownerName[index+1:])
	}
	for _, candidate := range candidates {
		account, err := user.Lookup(candidate)
		if err == nil && filepath.IsAbs(account.HomeDir) {
			return filepath.Join(account.HomeDir, ".codex"), nil
		}
	}

	normalized := strings.ReplaceAll(filepath.Clean(agentPath), `\`, "/")
	lower := strings.ToLower(normalized)
	if index := strings.Index(lower, "/users/"); index >= 0 {
		remainder := normalized[index+len("/users/"):]
		if slash := strings.Index(remainder, "/"); slash > 0 {
			return filepath.Clean(normalized[:index+len("/users/")+slash] + "/.codex"), nil
		}
	}
	return "", fmt.Errorf("cannot resolve a home directory for owner %q", ownerName)
}

func accountName(value string) string {
	return filepath.Base(strings.ReplaceAll(strings.TrimSpace(value), `\`, "/"))
}

// EnableCodexMonitor declares one Codex installation for read-only rollout
// tailing. Existing rollout history is skipped on the first enable.
func EnableCodexMonitor(agentID, path, ownerName, codexHome string) error {
	return codexMonitor.enable(codexClaim{
		AgentID: canonicalAgentID(agentID),
		Path:    filepath.Clean(path), Owner: strings.TrimSpace(ownerName),
		CodexHome: filepath.Clean(codexHome),
	})
}

func (manager *codexMonitorManager) enable(claim codexClaim) error {
	if claim.AgentID != "codex" && claim.AgentID != "codex-cli" {
		return fmt.Errorf("unsupported Codex rollout agent %q", claim.AgentID)
	}
	if claim.Path == "" || claim.Owner == "" || !filepath.IsAbs(claim.CodexHome) {
		return errors.New("agent path, owner and an absolute CODEX_HOME are required")
	}

	// Seeding reads the sessions directory and writes cursors, so it takes
	// pollMutex to stay the single writer, then mutex for the maps.
	manager.pollMutex.Lock()
	defer manager.pollMutex.Unlock()

	manager.mutex.Lock()
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(codexMonitorStateFile)
	}
	key := codexClaimKey(claim.AgentID, claim.Path, claim.Owner)
	_, exists := manager.claims[key]
	root := codexSessionsRoot(claim.CodexHome)
	seeded := manager.hasClaimForRootLocked(root)
	manager.mutex.Unlock()

	if exists {
		return nil
	}
	if !seeded {
		if err := manager.seedRoot(root); err != nil {
			return err
		}
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.claims[key] = claim
	manager.dirty = true
	if err := manager.saveLocked(); err != nil {
		delete(manager.claims, key)
		return err
	}
	return nil
}

// DisableCodexMonitor removes a Codex rollout declaration.
func DisableCodexMonitor(agentID, path, ownerName string) error {
	return codexMonitor.disable(canonicalAgentID(agentID), path, ownerName)
}

func (manager *codexMonitorManager) disable(agentID, path, ownerName string) error {
	manager.pollMutex.Lock()
	defer manager.pollMutex.Unlock()
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	key := codexClaimKey(agentID, path, ownerName)
	claim, exists := manager.claims[key]
	if !exists {
		return nil
	}
	delete(manager.claims, key)
	if len(manager.claims) == 0 {
		manager.cursors = map[string]*codexCursor{}
	}
	manager.dirty = true
	if err := manager.saveLocked(); err != nil {
		manager.claims[key] = claim
		return err
	}
	return nil
}

// CodexMonitorStatus reports the declaration and local tailing state.
func CodexMonitorStatus(agentID, path, ownerName string) (bool, string) {
	return codexMonitor.status(canonicalAgentID(agentID), path, ownerName)
}

// status answers entirely from the last poll's summary. It performs no disk IO,
// so the UI can refresh it as often as it likes without competing with the
// poller for the session directories.
func (manager *codexMonitorManager) status(agentID, path, ownerName string) (bool, string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	claim, found := manager.claims[codexClaimKey(agentID, path, ownerName)]
	if !found {
		return false, "not patched"
	}
	key := codexPathKey(codexSessionsRoot(claim.CodexHome))
	if err := manager.lastErr[key]; err != nil {
		return true, "monitor error: " + err.Error()
	}
	summary, sampled := manager.summaries[key]
	if !sampled || !summary.everSampled {
		return true, "waiting for the first scan of the sessions directory"
	}

	matchingFiles := summary.agentFiles[claim.AgentID]
	knownFiles := 0
	for _, count := range summary.agentFiles {
		knownFiles += count
	}
	if knownFiles == 0 && summary.unknown > 0 {
		return true, fmt.Sprintf("unsupported source: %d rollout file(s) did not identify as Codex", summary.unknown)
	}
	if knownFiles == 0 {
		return true, "waiting for activity: no rollout files"
	}
	if matchingFiles == 0 {
		return true, "waiting for activity: no matching source"
	}
	return true, fmt.Sprintf("active: monitoring %d rollout file(s); last scan %s",
		matchingFiles, summary.lastPolled.Format(time.RFC3339))
}

func (manager *codexMonitorManager) startPollerLocked() {
	stop := make(chan struct{})
	done := make(chan struct{})
	manager.stop, manager.done = stop, done
	go manager.run(stop, done)
}

func (manager *codexMonitorManager) run(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	manager.poll()
	ticker := time.NewTicker(monitorPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			manager.poll()
		case <-stop:
			return
		}
	}
}

// poll rescans every claimed sessions directory. It holds pollMutex for the
// whole pass so cursor state has a single writer, and takes mutex only in short
// bursts so the status endpoint is never blocked behind disk IO.
func (manager *codexMonitorManager) poll() {
	manager.pollMutex.Lock()
	defer manager.pollMutex.Unlock()

	for key, root := range manager.claimedRoots() {
		files, _, err := codexRolloutFiles(root)
		if err != nil {
			manager.setRootError(key, err)
			continue
		}
		manager.setRootError(key, nil)
		// Rollout directories only ever grow, so cursors for files that have been
		// rotated away are dropped here instead of accumulating forever.
		manager.pruneCursors(root, files)

		for _, path := range files {
			cursor, err := manager.cursorForFile(root, path, false)
			if err != nil {
				manager.setRootError(key, err)
				continue
			}
			if err := manager.consumeFile(cursor); err != nil {
				manager.setRootError(key, err)
			}
		}
		manager.refreshSummary(key, root)
	}
	manager.flushState()
}

func (manager *codexMonitorManager) claimedRoots() map[string]string {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	roots := map[string]string{}
	for _, claim := range manager.claims {
		root := codexSessionsRoot(claim.CodexHome)
		roots[codexPathKey(root)] = root
	}
	return roots
}

func (manager *codexMonitorManager) setRootError(key string, err error) {
	manager.mutex.Lock()
	if err == nil {
		delete(manager.lastErr, key)
	} else {
		manager.lastErr[key] = err
	}
	manager.mutex.Unlock()
}

// pruneCursors drops cursors under root whose file no longer exists.
func (manager *codexMonitorManager) pruneCursors(root string, files []string) {
	live := make(map[string]struct{}, len(files))
	for _, path := range files {
		live[codexPathKey(path)] = struct{}{}
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	rootKey := codexPathKey(root)
	for key, cursor := range manager.cursors {
		if codexPathKey(cursor.Root) != rootKey {
			continue
		}
		if _, found := live[key]; !found {
			delete(manager.cursors, key)
			manager.dirty = true
		}
	}
}

// refreshSummary records what status should report for root, so status never
// reads cursor fields the poller may be writing.
func (manager *codexMonitorManager) refreshSummary(key, root string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	summary := codexRootSummary{agentFiles: map[string]int{}, lastPolled: time.Now(), everSampled: true}
	rootKey := codexPathKey(root)
	for _, cursor := range manager.cursors {
		if codexPathKey(cursor.Root) != rootKey {
			continue
		}
		if cursor.AgentID == "" {
			summary.unknown++
			continue
		}
		summary.agentFiles[cursor.AgentID]++
	}
	manager.summaries[key] = summary
}

func (manager *codexMonitorManager) flushState() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if !manager.dirty {
		return
	}
	if err := manager.saveLocked(); err != nil {
		for key := range manager.summaries {
			manager.lastErr[key] = err
		}
	}
}

// seedRoot skips the rollout history that already exists when a root is first
// claimed. It must be called with pollMutex held.
func (manager *codexMonitorManager) seedRoot(root string) error {
	files, _, err := codexRolloutFiles(root)
	if err != nil {
		return err
	}
	for _, path := range files {
		if _, err := manager.cursorForFile(root, path, true); err != nil {
			return err
		}
	}
	return nil
}

// cursorForFile resolves the cursor for one rollout file. It must be called
// with pollMutex held: file metadata is read outside mutex, and only the map
// lookup and insert take it.
func (manager *codexMonitorManager) cursorForFile(root, path string, seedEOF bool) (*codexCursor, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key := codexPathKey(path)

	manager.mutex.Lock()
	cursor := manager.cursors[key]
	manager.mutex.Unlock()

	if cursor == nil {
		meta, err := codexFileHeader(path)
		if err != nil {
			return nil, err
		}
		cursor = &codexCursor{
			Path: path, Root: root, SessionKey: meta.SessionKey, AgentID: meta.AgentID,
			Pending: map[string]codexPendingCall{},
		}
		manager.mutex.Lock()
		manager.cursors[key] = cursor
		manager.dirty = true
		manager.mutex.Unlock()
	}

	if seedEOF {
		cursor.Offset = info.Size()
		resetCodexCursorTurn(cursor)
		manager.markDirty()
	} else if info.Size() < cursor.Offset {
		cursor.Offset = 0
		cursor.SessionKey = ""
		cursor.AgentID = ""
		resetCodexCursorTurn(cursor)
		manager.markDirty()
	}
	if cursor.AgentID == "" && cursor.Offset == 0 && info.Size() > 0 {
		meta, err := codexFileHeader(path)
		if err != nil {
			return nil, err
		}
		cursor.SessionKey, cursor.AgentID = meta.SessionKey, meta.AgentID
	}
	return cursor, nil
}

func resetCodexCursorTurn(cursor *codexCursor) {
	cursor.Model = ""
	cursor.TurnID = ""
	cursor.Pending = map[string]codexPendingCall{}
}

func (manager *codexMonitorManager) markDirty() {
	manager.mutex.Lock()
	manager.dirty = true
	manager.mutex.Unlock()
}

// consumeFile reads the bytes appended since the last poll. It must be called
// with pollMutex held; the read itself deliberately runs without mutex.
func (manager *codexMonitorManager) consumeFile(cursor *codexCursor) error {
	info, err := os.Stat(cursor.Path)
	if err != nil {
		return err
	}
	if info.Size() == cursor.Offset {
		return nil
	}

	file, err := os.Open(cursor.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(cursor.Offset, io.SeekStart); err != nil {
		return err
	}

	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, size, complete, readErr := readCompleteBoundedLine(reader, maxCodexRolloutLine)
		if readErr != nil {
			return readErr
		}
		if !complete {
			return nil
		}
		cursor.Offset += size
		manager.markDirty()
		if line != nil {
			claim := manager.claimForCursor(cursor)
			for _, record := range parseCodexRolloutLine(bytes.TrimSpace(line), cursor, claim) {
				manager.addRecord(record)
			}
		}
	}
}

func (manager *codexMonitorManager) claimForCursor(cursor *codexCursor) *codexClaim {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for _, claim := range manager.claims {
		if claim.AgentID == cursor.AgentID && codexPathKey(codexSessionsRoot(claim.CodexHome)) == codexPathKey(cursor.Root) {
			return &claim
		}
	}
	return nil
}

func (manager *codexMonitorManager) hasClaimForRootLocked(root string) bool {
	for _, claim := range manager.claims {
		if codexPathKey(codexSessionsRoot(claim.CodexHome)) == codexPathKey(root) {
			return true
		}
	}
	return false
}

func codexRolloutFiles(root string) ([]string, bool, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, true, fmt.Errorf("Codex sessions root is not a directory: %s", root)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, true, err
	}

	files := []string{}
	err = filepath.WalkDir(canonicalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, true, err
}

type codexHeaderMeta struct {
	SessionKey string
	AgentID    string
}

func codexFileHeader(path string) (codexHeaderMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return codexHeaderMeta{}, err
	}
	defer file.Close()
	line, _, complete, err := readCompleteBoundedLine(bufio.NewReaderSize(file, 64*1024), maxCodexRolloutLine)
	if err != nil {
		return codexHeaderMeta{}, err
	}
	if !complete || line == nil {
		return codexHeaderMeta{}, nil
	}
	return parseCodexHeader(bytes.TrimSpace(line)), nil
}

func codexSessionsRoot(codexHome string) string {
	return filepath.Join(filepath.Clean(codexHome), "sessions")
}

func codexClaimKey(agentID, path, ownerName string) string {
	return strings.ToLower(canonicalAgentID(agentID) + "\x00" + filepath.Clean(path) + "\x00" + strings.TrimSpace(ownerName))
}

func codexPathKey(path string) string {
	key := filepath.Clean(path)
	if os.PathSeparator == '\\' {
		key = strings.ToLower(key)
	}
	return key
}

func (manager *codexMonitorManager) loadLocked() error {
	data, err := os.ReadFile(manager.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved codexSavedState
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("cannot parse Codex rollout monitor state: %w", err)
	}
	for _, claim := range saved.Claims {
		manager.claims[codexClaimKey(claim.AgentID, claim.Path, claim.Owner)] = claim
	}
	if saved.Cursors != nil {
		manager.cursors = saved.Cursors
	}
	for key, cursor := range manager.cursors {
		if cursor == nil {
			delete(manager.cursors, key)
			continue
		}
		if cursor.Pending == nil {
			cursor.Pending = map[string]codexPendingCall{}
		}
	}
	return nil
}

func (manager *codexMonitorManager) saveLocked() error {
	if len(manager.claims) == 0 {
		if err := os.Remove(manager.statePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		manager.dirty = false
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(manager.statePath), 0o700); err != nil {
		return err
	}
	saved := codexSavedState{Claims: make([]codexClaim, 0, len(manager.claims)), Cursors: manager.cursors}
	for _, claim := range manager.claims {
		saved.Claims = append(saved.Claims, claim)
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manager.statePath, append(data, '\n'), 0o600); err != nil {
		return err
	}
	manager.dirty = false
	return nil
}
