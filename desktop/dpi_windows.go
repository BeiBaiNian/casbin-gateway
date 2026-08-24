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

const (
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, which is (HANDLE)-4.
	dpiPerMonitorAwareV2 = ^uintptr(3)
	defaultDpi           = 96
)

var (
	procSetDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetDpiAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForSystem        = user32.NewProc("GetDpiForSystem")
)

// enableDpiAwareness stops Windows from rendering at 96 DPI and stretching the
// result, which is what a process that never says otherwise gets — and what
// made the UI blurry on a scaled display. Nothing declares it for us, since a
// Go binary carries no application manifest, so it is claimed here instead. It
// has to happen before the process owns any window, and it applies to the tray
// menu as much as to the webview.
func enableDpiAwareness() {
	if procSetDpiAwarenessContext.Find() == nil {
		if ok, _, _ := procSetDpiAwarenessContext.Call(dpiPerMonitorAwareV2); ok != 0 {
			return
		}
	}

	// Windows before 1703 knows only the one system-wide setting.
	if procSetDpiAware.Find() == nil {
		_, _, _ = procSetDpiAware.Call()
	}
}

// systemDpi is the DPI of the primary monitor, which is the one the window is
// centred on. It answers 96 to a process that is not DPI aware, so it is only
// meaningful after enableDpiAwareness.
func systemDpi() uint {
	if procGetDpiForSystem.Find() != nil {
		return defaultDpi
	}

	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return defaultDpi
	}
	return uint(dpi)
}
