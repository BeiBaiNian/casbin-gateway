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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// skillFile is the manifest that makes a directory a skill rather than
	// whatever else the operator keeps next to their skills.
	skillFile = "SKILL.md"
	// maxSkillFileBytes bounds what the detail view loads: a skill is meant to
	// be read by a model, but nothing stops one from carrying a large file.
	maxSkillFileBytes = 256 * 1024
	// maxSkillFiles bounds the file list shown beside a skill's manifest.
	maxSkillFiles = 200
)

// readSkills lists the skill folders in dir. A directory that does not exist is
// an agent with no skills yet, not an error.
func readSkills(agentId string, owner string, dir string) ([]*Item, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Item{}, nil
	}
	if err != nil {
		return []*Item{}, err
	}

	items := []*Item{}
	for _, entry := range entries {
		// A dot directory is the agent's own bookkeeping, such as the manifest
		// Cursor writes beside the skills it manages.
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		manifest, ok := manifestPath(path)
		if !ok {
			continue
		}

		item := &Item{
			AgentId: agentId,
			Owner:   owner,
			Kind:    KindSkill,
			Name:    entry.Name(),
			Path:    path,
		}
		if raw, err := os.ReadFile(manifest); err == nil {
			_, item.Description = parseFrontMatter(string(raw))
		}
		item.Files, item.Bytes = measure(path)
		items = append(items, item)
	}
	sortItems(items)
	return items, nil
}

// skillDetail loads one skill's manifest and the names of the files shipped
// with it.
func skillDetail(agentId string, owner string, dir string, name string) (*Detail, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name)
	manifest, ok := manifestPath(path)
	if !ok {
		return nil, fmt.Errorf("no %s in %s", skillFile, path)
	}

	raw, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	content := string(raw)
	if len(raw) > maxSkillFileBytes {
		content = string(raw[:maxSkillFileBytes]) + "\n\n... truncated by Gateway ..."
	}

	_, description := parseFrontMatter(content)
	files, bytes := measure(path)
	item := &Item{
		AgentId:     agentId,
		Owner:       owner,
		Kind:        KindSkill,
		Name:        name,
		Description: description,
		Path:        path,
		Files:       files,
		Bytes:       bytes,
	}
	return &Detail{Item: item, Content: content, Files: listFiles(path)}, nil
}

// deleteSkill removes one skill folder. It refuses a directory without a
// manifest, so a mistyped name cannot delete something that is not a skill.
func deleteSkill(dir string, name string) error {
	if err := checkName(name); err != nil {
		return err
	}

	path := filepath.Join(dir, name)
	if _, ok := manifestPath(path); !ok {
		return fmt.Errorf("%s is not a skill folder", path)
	}
	return os.RemoveAll(path)
}

// copySkill copies one skill folder into another agent's skills directory,
// replacing what is there. The two agents Gateway supports keep skills in the
// same format, so the copy is the folder itself and nothing is converted.
func copySkill(from string, dir string, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	if _, ok := manifestPath(from); !ok {
		return "", fmt.Errorf("%s is not a skill folder", from)
	}

	to := filepath.Join(dir, name)
	same, err := samePath(from, to)
	if err != nil {
		return "", err
	}
	if same {
		return "", fmt.Errorf("%s is already the source of this copy", to)
	}

	staged := to + ".gateway-copy"
	if err := os.RemoveAll(staged); err != nil {
		return "", err
	}
	if err := copyTree(from, staged); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	if err := os.RemoveAll(to); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	if err := os.Rename(staged, to); err != nil {
		os.RemoveAll(staged)
		return "", err
	}
	return to, nil
}

// manifestPath finds the skill manifest inside a folder. The name is matched
// case-insensitively because the agents that write it do not agree on the case
// on the file systems that care.
func manifestPath(dir string) (string, bool) {
	path := filepath.Join(dir, skillFile)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), skillFile) {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

func measure(dir string) (int, int64) {
	files := 0
	var bytes int64
	filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		files++
		if info, err := entry.Info(); err == nil {
			bytes += info.Size()
		}
		return nil
	})
	return files, bytes
}

// listFiles names what a skill ships besides its manifest, in slash form so the
// list reads the same on every platform.
func listFiles(dir string) []string {
	files := []string{}
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || len(files) >= maxSkillFiles {
			return nil
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil || strings.EqualFold(relative, skillFile) {
			return nil
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(files)
	return files
}

func copyTree(from string, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// A symbolic link in a skill folder would point at the source agent's
		// directory, so it is left behind rather than copied as a broken link.
		if !entry.Type().IsRegular() {
			return nil
		}
		return copyOneFile(path, target, entry)
	})
}

func copyOneFile(path string, target string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}

	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}
	return destination.Close()
}

// samePath compares two paths by what they resolve to, so a copy cannot be
// asked to overwrite its own source through a link or a differently spelled
// path to the same directory.
func samePath(left string, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, err
	}
	rightInfo, err := os.Stat(right)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(leftInfo, rightInfo), nil
}
