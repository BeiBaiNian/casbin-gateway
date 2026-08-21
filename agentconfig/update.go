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
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// originFile records where the skills Gateway copied came from, under the same
// home as the trash. A skill folder carries no provenance of its own, so
// without this a copy and a hand-written skill look alike a day later.
const originFile = "skill-origins.json"

// What a skill's content says about the copy it came from.
const (
	// UpdateCurrent is a skill holding exactly what its source holds.
	UpdateCurrent = "current"
	// UpdateAvailable is a source that has moved on while this copy has not.
	UpdateAvailable = "available"
	// UpdateModified is a copy edited here, with the source unchanged.
	UpdateModified = "modified"
	// UpdateDiverged is both of the above at once: updating would overwrite
	// edits made here.
	UpdateDiverged = "diverged"
	// UpdateUnknown is a source that is no longer on this machine.
	UpdateUnknown = "unknown"
)

// SkillUpdate is where one skill came from and whether that source still holds
// the same thing. It answers the question a folder of files cannot: is what is
// installed here the current version of it.
type SkillUpdate struct {
	State string `json:"state"`
	// Source is the folder this skill was copied from, and SourceAgentId the
	// agent that folder belongs to.
	Source        string `json:"source,omitempty"`
	SourceAgentId string `json:"sourceAgentId,omitempty"`
	SourceName    string `json:"sourceName,omitempty"`
	// Inferred marks a source Gateway matched by name rather than one it
	// recorded itself when copying.
	Inferred bool `json:"inferred,omitempty"`

	SourceDigest   string `json:"sourceDigest,omitempty"`
	SourceModified int64  `json:"sourceModified,omitempty"`
	CopiedAt       int64  `json:"copiedAt,omitempty"`
}

// skillOrigin is one recorded copy: what was copied, from where, and the two
// digests at the time. Both are needed to tell a source that has moved on from
// a copy that was edited here.
type skillOrigin struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	// Digest is what the source held when it was copied, and Written what
	// landed here.
	Digest   string `json:"digest,omitempty"`
	Written  string `json:"written,omitempty"`
	CopiedAt int64  `json:"copiedAt"`
}

// originLock serializes the read-modify-write of the origin file. Two copies
// running at once would otherwise each write back what they read.
var originLock sync.Mutex

// attachSkillUpdates tells each skill how it stands against the copy it came
// from. A plugin's skills are left alone: the plugin updates those, not Gateway.
func attachSkillUpdates(items []*Item, home string) {
	origins := readSkillOrigins(home)
	for _, item := range items {
		if item.Scope == ScopePlugin {
			continue
		}
		if origin, ok := origins[comparablePath(item.Path)]; ok {
			item.Update = compareToOrigin(item, origin)
		}
	}
	inferSkillUpdates(items)
}

// compareToOrigin measures the source again and reads the two digests against
// the ones recorded when the copy was made.
func compareToOrigin(item *Item, origin *skillOrigin) *SkillUpdate {
	update := &SkillUpdate{
		State:         UpdateUnknown,
		Source:        origin.Path,
		SourceAgentId: origin.AgentId,
		SourceName:    origin.Name,
		SourceDigest:  origin.Digest,
		CopiedAt:      origin.CopiedAt,
	}
	if _, ok := manifestPath(origin.Path); !ok {
		return update
	}

	stat := measure(origin.Path)
	update.SourceDigest, update.SourceModified = stat.digest, stat.modified

	sourceChanged := stat.digest != origin.Digest
	editedHere := origin.Written != "" && item.Digest != origin.Written
	switch {
	case stat.digest == item.Digest, !sourceChanged && !editedHere:
		update.State = UpdateCurrent
	case sourceChanged && editedHere:
		update.State = UpdateDiverged
	case sourceChanged:
		update.State = UpdateAvailable
	default:
		update.State = UpdateModified
	}
	return update
}

// inferSkillUpdates pairs a skill the operator keeps with the plugin copy of
// the same name on this machine. A skill installed by hand from a plugin
// carries no record of it, and the plugin's copy is the published one, so the
// two digests still answer whether this one is current.
func inferSkillUpdates(items []*Item) {
	published := map[string]*Item{}
	for _, item := range items {
		if item.Scope == ScopePlugin {
			published[targetName(KindSkill, item.Name)] = item
		}
	}
	if len(published) == 0 {
		return
	}

	for _, item := range items {
		source := published[targetName(KindSkill, item.Name)]
		if item.Update != nil || item.Scope == ScopePlugin || source == nil || source.Digest == "" {
			continue
		}
		update := &SkillUpdate{
			State:          UpdateCurrent,
			Source:         source.Path,
			SourceAgentId:  source.AgentId,
			SourceName:     source.Name,
			SourceDigest:   source.Digest,
			SourceModified: source.Modified,
			Inferred:       true,
		}
		if source.Digest != item.Digest {
			// Without a record of when this copy was made, the newer of the two
			// is all that separates an update from a local edit.
			if source.Modified <= item.Modified {
				continue
			}
			update.State = UpdateAvailable
		}
		item.Update = update
	}
}

// UpdateSkill replaces one skill with the current content of the source it was
// copied from. The old content goes to the trash first, so an update that turns
// out to be unwanted is one restore away like a delete.
func UpdateSkill(agentId string, owner string, name string) (*Item, error) {
	_, home, err := resolve(agentId, owner, KindSkill)
	if err != nil {
		return nil, err
	}
	item, err := findItem(agentId, owner, KindSkill, name)
	if err != nil {
		return nil, err
	}

	if item.ReadOnly != "" {
		return nil, fmt.Errorf("%s: %s", name, item.ReadOnly)
	}
	if item.Update == nil || item.Update.Source == "" {
		return nil, fmt.Errorf("%s: Gateway does not know where this skill came from", name)
	}
	if item.Update.State == UpdateCurrent {
		return nil, fmt.Errorf("%s is already the version its source holds", name)
	}
	if _, ok := manifestPath(item.Update.Source); !ok {
		return nil, fmt.Errorf("%s is no longer a skill folder", item.Update.Source)
	}

	if err := trashSkill(home, item); err != nil {
		return nil, err
	}
	path, err := copySkill(item.Update.Source, filepath.Dir(item.Path), filepath.Base(item.Path))
	if err != nil {
		return nil, fmt.Errorf("%s; the previous version is in the recycle bin", err)
	}

	recordSkillOrigin(home, path, item.Update.SourceAgentId, item.Update.SourceName, item.Update.Source)
	updated, err := findItem(agentId, owner, KindSkill, name)
	if err != nil {
		return item, nil
	}
	return updated, nil
}

// recordSkillOrigin remembers that target was copied from source, so a later
// listing can say whether the source has moved on since. A record that cannot
// be written costs the update badge, not the copy, so nothing is reported.
func recordSkillOrigin(home string, target string, sourceAgentId string, sourceName string, source string) {
	originLock.Lock()
	defer originLock.Unlock()

	origins := readSkillOrigins(home)
	origins[comparablePath(target)] = &skillOrigin{
		AgentId:  sourceAgentId,
		Name:     sourceName,
		Path:     source,
		Digest:   measure(source).digest,
		Written:  measure(target).digest,
		CopiedAt: time.Now().Unix(),
	}

	// A skill that has been deleted since would keep its record forever, and
	// the next skill of that path would inherit it.
	for path, origin := range origins {
		if !exists(origin.Path) && !exists(path) {
			delete(origins, path)
		}
	}
	writeSkillOrigins(home, origins)
}

// forgetSkillOrigin drops the record of one path, for a skill that has left it.
func forgetSkillOrigin(home string, target string) {
	originLock.Lock()
	defer originLock.Unlock()

	origins := readSkillOrigins(home)
	if _, ok := origins[comparablePath(target)]; !ok {
		return
	}
	delete(origins, comparablePath(target))
	writeSkillOrigins(home, origins)
}

func readSkillOrigins(home string) map[string]*skillOrigin {
	origins := map[string]*skillOrigin{}
	raw, err := os.ReadFile(filepath.Join(home, trashRoot, originFile))
	if err != nil {
		return origins
	}
	if err := json.Unmarshal(raw, &origins); err != nil {
		return map[string]*skillOrigin{}
	}
	return origins
}

func writeSkillOrigins(home string, origins map[string]*skillOrigin) {
	raw, err := json.MarshalIndent(origins, "", "  ")
	if err != nil {
		return
	}
	writeFile(filepath.Join(home, trashRoot, originFile), raw, defaultConfigMode)
}
