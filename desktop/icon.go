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

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/apache/casbin-gateway/desktop/internal/assets"
)

// writeAppIcon puts the icon on disk next to the Gateway, where the window can
// load it and a shortcut can point at it. The alternative is a linker resource,
// which would mean a mingw toolchain in the build for one image.
func writeAppIcon() (string, error) {
	name, data := "casbin-gateway.png", assets.AppIconPng
	switch runtime.GOOS {
	case "windows":
		name, data = "casbin-gateway.ico", assets.AppIcon
	case "darwin":
		// The application bundle wants an icon container, not a PNG.
		name, data = "casbin-gateway.icns", assets.AppIconIcns
	}

	path := filepath.Join(gatewayHome(), name)
	if existing, err := os.ReadFile(path); err == nil && len(existing) == len(data) {
		return path, nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
