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

package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// Filled in by the release build:
//
//	-ldflags "-X github.com/apache/casbin-gateway/version.buildVersion=nightly
//	          -X github.com/apache/casbin-gateway/version.buildCommit=<sha>
//	          -X github.com/apache/casbin-gateway/version.buildDate=<RFC3339>"
var (
	buildVersion string
	buildCommit  string
	buildDate    string
)

// DevVersion is what an unstamped build calls itself.
const DevVersion = "dev"

// Build is what this executable knows about itself.
type Build struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"shortCommit"`
	BuildTime   string `json:"buildTime"`
	// Modified marks a build made from a checkout with uncommitted changes,
	// which no commit alone can identify.
	Modified  bool   `json:"modified"`
	Os        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"goVersion"`
}

var (
	currentOnce sync.Once
	current     Build
)

// Current describes this executable. An unstamped build still reports a commit
// and a date: "go build" records the checkout it came from.
func Current() Build {
	currentOnce.Do(func() {
		current = Build{
			Version:   buildVersion,
			Commit:    buildCommit,
			BuildTime: buildDate,
			Os:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
		}

		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if current.Commit == "" {
						current.Commit = setting.Value
					}
				case "vcs.time":
					if current.BuildTime == "" {
						current.BuildTime = setting.Value
					}
				case "vcs.modified":
					current.Modified = setting.Value == "true"
				}
			}
		}

		if current.Version == "" {
			current.Version = DevVersion
		}
		current.ShortCommit = ShortCommit(current.Commit)
	})

	return current
}

// BuildTime is when this executable was built, zero when nothing recorded it.
func (b Build) Time() time.Time {
	moment, err := time.Parse(time.RFC3339, b.BuildTime)
	if err != nil {
		return time.Time{}
	}

	return moment
}

// String is the one-line form printed by "casbin-gateway version".
func (b Build) String() string {
	text := b.Version
	if b.ShortCommit != "" {
		text += " (" + b.ShortCommit
		if b.Modified {
			text += ", modified"
		}
		text += ")"
	}
	if b.BuildTime != "" {
		text += " built " + b.BuildTime
	}

	return fmt.Sprintf("%s %s/%s %s", text, b.Os, b.Arch, b.GoVersion)
}

func ShortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}

	return commit
}

// RunCommand answers "casbin-gateway version" and reports whether it did, so
// the caller knows not to go on and serve. The staged binary of an update is
// smoke-tested with this before it replaces the running one.
func RunCommand(args []string) bool {
	for _, arg := range args[1:] {
		switch arg {
		case "version", "--version", "-v":
			fmt.Println("Casbin Gateway", Current().String())
			return true
		}
	}

	return false
}
