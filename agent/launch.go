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

package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Launch is how one installation is started.
type Launch struct {
	// Executable is the file to run, empty when none was resolved.
	Executable string
	// Desktop marks a windowed app, which needs no console of its own.
	Desktop bool
}

// LaunchOf resolves what to run for one installation.
func LaunchOf(installation Installation) Launch {
	launch := Launch{}
	execName := ""
	for i := range fingerprints {
		if fingerprints[i].ID == installation.AgentId {
			execName = fingerprints[i].ExecName
			launch.Desktop = fingerprints[i].Desktop
			break
		}
	}
	launch.Executable = executableOf(installation.Path, execName)
	return launch
}

// executableOf resolves the launcher of an installation. A package manager
// records the package directory rather than a program, so the shim it installed
// beside the tree is what runs the agent.
func executableOf(path, execName string) string {
	if isFile(path) {
		return path
	}
	if execName == "" {
		return ""
	}

	root := nodeModulesRoot(path)
	if root == "" {
		return ""
	}
	for _, candidate := range npmShims(root, execName) {
		if isFile(candidate) {
			return candidate
		}
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// nodeModulesRoot is the directory holding the node_modules tree path sits in.
func nodeModulesRoot(path string) string {
	for dir := filepath.Clean(path); ; {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if strings.EqualFold(filepath.Base(dir), "node_modules") {
			return parent
		}
		dir = parent
	}
}

// npmShims are the launcher layouts npm writes: beside the tree on Windows,
// under the prefix's bin/ elsewhere.
func npmShims(root, execName string) []string {
	local := filepath.Join(root, "node_modules", ".bin", execName)
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(root, execName+".cmd"),
			filepath.Join(root, execName+".exe"),
			local + ".cmd",
		}
	}

	prefix := root
	// A Unix global root is <prefix>/lib/node_modules.
	if filepath.Base(root) == "lib" {
		prefix = filepath.Dir(root)
	}
	return []string{filepath.Join(prefix, "bin", execName), local}
}
