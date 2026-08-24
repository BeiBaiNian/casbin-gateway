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

import "syscall"

const (
	wmClose     = 0x0010
	swHide      = 0
	swShow      = 5
	swRestore   = 9
	gwlpWndProc = ^uintptr(3) // GWLP_WNDPROC, which is -4

	// DWMWA_TRANSITIONS_FORCEDISABLED, which turns off the animation the window
	// manager plays when a window is shown, hidden or restored.
	dwmTransitionsForceDisabled = 3
)

var (
	dwmapi = syscall.NewLazyDLL("dwmapi.dll")

	procSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	procSetWindowLongW       = user32.NewProc("SetWindowLongW")
	procCallWindowProcW      = user32.NewProc("CallWindowProcW")
	procDwmSetWindowAttrib   = dwmapi.NewProc("DwmSetWindowAttribute")
	procIsIconic             = user32.NewProc("IsIconic")
	procBringWindowToTop     = user32.NewProc("BringWindowToTop")
	procAttachThreadInput    = user32.NewProc("AttachThreadInput")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetCurrentThreadId   = kernel32.NewProc("GetCurrentThreadId")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	previousWindowProc       uintptr
	hideOnCloseProcCallback  = syscall.NewCallback(hideOnClose)
)

// keepWindowAlive makes closing the window hide it instead of destroying it.
// The webview's own window procedure answers WM_CLOSE with DestroyWindow, which
// ends the run loop and the process with it — which is why reopening from the
// tray used to reload everything from scratch. Hiding keeps the page loaded, so
// the window comes back instantly.
func keepWindowAlive(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	disableWindowAnimations(hwnd)

	setLong := procSetWindowLongPtrW
	if setLong.Find() != nil {
		// 32-bit Windows has no pointer-sized variant.
		setLong = procSetWindowLongW
	}
	if setLong.Find() != nil {
		return
	}

	previousWindowProc, _, _ = setLong.Call(hwnd, gwlpWndProc, hideOnCloseProcCallback)
}

func hideOnClose(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmClose {
		_, _, _ = procShowWindow.Call(hwnd, swHide)
		return 0
	}

	result, _, _ := procCallWindowProcW.Call(previousWindowProc, hwnd, msg, wparam, lparam)
	return result
}

func disableWindowAnimations(hwnd uintptr) {
	if procDwmSetWindowAttrib.Find() != nil {
		return
	}

	disabled := int32(1)
	_, _, _ = procDwmSetWindowAttrib.Call(
		hwnd,
		dwmTransitionsForceDisabled,
		uintptr(unsafePointerTo(&disabled)),
		4,
	)
}
