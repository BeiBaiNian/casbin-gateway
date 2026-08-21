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
	"os"
	"path/filepath"
)

// defaultConfigMode is what a config file Gateway creates is born with. These
// files carry API keys and tokens, so a new one is not world-readable.
const defaultConfigMode = os.FileMode(0o600)

// readFile returns empty content for a file that does not exist yet, which is
// what an agent with no configuration of that kind looks like.
func readFile(path string) ([]byte, os.FileMode, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, defaultConfigMode, nil
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode().Perm(), nil
}

// writeFile replaces path in one step. An agent may be reading its own config
// while Gateway rewrites it, so the new content is staged in the same directory
// and renamed over the old file rather than truncating it in place.
func writeFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	staged, err := os.CreateTemp(dir, "."+filepath.Base(path)+".gateway-*")
	if err != nil {
		return err
	}
	name := staged.Name()
	defer os.Remove(name)

	if _, err := staged.Write(data); err != nil {
		staged.Close()
		return err
	}
	if err := staged.Sync(); err != nil {
		staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// checkName rejects a name that would escape the directory it is looked up in.
// Names come from an API request, and a skill is a directory that gets deleted
// or copied whole.
func checkName(name string) error {
	if name == "" {
		return fmt.Errorf("the name is empty")
	}
	if name != filepath.Base(name) || name == "." || name == ".." ||
		filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return fmt.Errorf("invalid name: %q", name)
	}
	return nil
}
