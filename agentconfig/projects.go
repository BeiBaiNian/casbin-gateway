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

package agentconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// maxProjects bounds how many checkouts one listing looks into. An agent
// remembers every directory it has ever been started in, and that list only
// grows.
const maxProjects = 500

// jsonProjects lists the directories an agent has worked in, from the projects
// object it keeps in its own JSON configuration. A directory that is no longer
// there is dropped: the agent remembers projects that have since been moved or
// deleted, and Gateway lists what is on disk.
func jsonProjects(segments ...string) func(string) []string {
	return func(home string) []string {
		raw, err := os.ReadFile(filepath.Join(append([]string{home}, segments...)...))
		if err != nil {
			return nil
		}

		document := struct {
			Projects map[string]json.RawMessage `json:"projects"`
		}{}
		if err := json.Unmarshal(raw, &document); err != nil {
			return nil
		}

		projects := make([]string, 0, len(document.Projects))
		for path := range document.Projects {
			if path == "" || path == home || !isDir(path) {
				continue
			}
			projects = append(projects, filepath.Clean(path))
		}
		sort.Strings(projects)
		return dedupe(projects, maxProjects)
	}
}

// comparablePath is what two spellings of one directory share, so a path can be
// used as a map key. Windows and macOS do not distinguish the case of a path,
// and a trailing separator never makes a different directory.
func comparablePath(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// dedupe keeps the first of each path and stops at limit. Two projects can
// differ only in case or in a trailing separator and still be one directory.
func dedupe(paths []string, limit int) []string {
	kept := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		key := comparablePath(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, path)
		if len(kept) >= limit {
			break
		}
	}
	return kept
}
