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

// Package assets holds the desktop icons, all rendered from web/public/logo512.png.
package assets

import _ "embed"

// AppIcon is the Windows icon, 16 to 256 pixels, used for the window, the
// shortcuts and the tray.
//
//go:embed appicon.ico
var AppIcon []byte

// AppIconPng is the 512-pixel icon a Linux desktop entry points at.
//
//go:embed appicon.png
var AppIconPng []byte

// AppIconIcns is the icon of the macOS application bundle.
//
//go:embed appicon.icns
var AppIconIcns []byte

// TrayIcon is what the macOS menu bar and the Linux tray show; both want a
// small PNG rather than an icon container.
//
//go:embed trayicon.png
var TrayIcon []byte
