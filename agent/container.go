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
	"strings"
	"sync"
)

var (
	containerOnce sync.Once
	container     bool
)

// InContainer reports whether Gateway runs inside a container, where a scan
// reads the container's own filesystem instead of the host's and therefore
// finds none of the agents the user actually installed.
func InContainer() bool {
	containerOnce.Do(func() {
		container = detectContainer()
	})
	return container
}

func detectContainer() bool {
	// Set by our own compose file, and by Podman inside every container.
	if os.Getenv("RUNNING_IN_DOCKER") == "true" || os.Getenv("container") != "" {
		return true
	}

	// Docker leaves the first file behind, Podman the second.
	for _, path := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	for _, marker := range []string{"docker", "libpod", "kubepods", "containerd"} {
		if strings.Contains(string(data), marker) {
			return true
		}
	}
	return false
}
