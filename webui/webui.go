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

// Package webui says which compiled frontend this process serves. The router
// and the startup summary both need the answer, and they must agree on it.
package webui

import "github.com/apache/casbin-gateway/util"

// BuildDirs lists the compiled web UIs, most preferred first: "web2" is the
// shadcn frontend and "web" the older antd one it replaces. Whichever is found
// first serves the whole UI, so a half-built tree is never quietly completed
// with files from the other.
var BuildDirs = []string{"web2/build", "web/build"}

// GetBuildDir returns the first compiled UI present on disk, or "" when none of
// them was built. A build made with -tags embed carries its own copy, which is
// used only in that last case.
func GetBuildDir() string {
	for _, dir := range BuildDirs {
		if util.FileExist(dir + "/index.html") {
			return dir
		}
	}

	return ""
}
