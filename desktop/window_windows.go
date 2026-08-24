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
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

const (
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	imageIcon      = 1
	lrLoadFromFile = 0x0010
	smCxIcon       = 11
	smCyIcon       = 12
	smCxSmIcon     = 49
	smCySmIcon     = 50
)

var (
	procLoadImageW       = user32.NewProc("LoadImageW")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

func runWindow(url string) error {
	// Without an explicit AppUserModelID the window is grouped under whatever
	// launched it, so this is what makes Casbin Gateway its own taskbar button
	// and lets a pinned shortcut point at it.
	setAppUserModelId()

	// Both of these have to happen before the window exists: DPI awareness is
	// fixed at the first window, and the size below is in physical pixels.
	enableDpiAwareness()
	scale := systemDpi()

	view := webview2.NewWithOptions(webview2.WebViewOptions{
		// Keeping the profile in the Gateway directory means uninstalling is
		// still "delete the directory".
		DataPath:  filepath.Join(gatewayHome(), "tmp", "webview"),
		AutoFocus: true,
		Debug:     debugEnabled(),
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  windowWidth * scale / defaultDpi,
			Height: windowHeight * scale / defaultDpi,
			Center: true,
		},
	})
	if view == nil {
		return errors.New("the Microsoft Edge WebView2 runtime is missing, install it from https://developer.microsoft.com/microsoft-edge/webview2/")
	}
	defer view.Destroy()

	setWindowIcon(uintptr(view.Window()))
	view.Navigate(url)
	view.Run()
	return nil
}

// setWindowIcon gives the title bar and the taskbar button the Casbin icon. A
// window whose class was registered without one keeps the generic icon
// otherwise, and that is the icon the user sees in the taskbar.
func setWindowIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	path, err := writeAppIcon()
	if err != nil {
		return
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}

	for _, size := range []struct {
		which  uintptr
		cx, cy uintptr
	}{
		{iconSmall, smCxSmIcon, smCySmIcon},
		{iconBig, smCxIcon, smCyIcon},
	} {
		cx, _, _ := procGetSystemMetrics.Call(size.cx)
		cy, _, _ := procGetSystemMetrics.Call(size.cy)
		icon, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(pathPtr)), imageIcon, cx, cy, lrLoadFromFile)
		if icon == 0 {
			continue
		}
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, size.which, icon)
	}
}
