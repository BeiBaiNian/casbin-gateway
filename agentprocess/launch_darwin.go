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

//go:build darwin

package agentprocess

import (
	"path/filepath"
	"strings"
)

func start(target Target) error {
	if target.Desktop {
		if bundle := appBundle(target.Executable); bundle != "" {
			return spawn("open", "-a", bundle)
		}
		return spawn(target.Executable)
	}
	return spawn("open", "-a", "Terminal", target.Executable)
}

// appBundle is the .app an executable lives in, empty outside a bundle. Opening
// the bundle rather than the binary inside it is what registers the app with the
// window server.
func appBundle(executable string) string {
	for dir := filepath.Dir(executable); ; {
		if strings.HasSuffix(dir, ".app") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
