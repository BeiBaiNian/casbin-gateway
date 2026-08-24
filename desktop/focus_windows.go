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
	"syscall"
	"unsafe"
)

var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")

	procSetAppId            = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowThreadPid  = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

const swRestore = 9

func setAppUserModelId() {
	id, err := syscall.UTF16PtrFromString(appId)
	if err != nil {
		return
	}
	_, _, _ = procSetAppId.Call(uintptr(unsafe.Pointer(id)))
}

// focusWindow raises the window process's own window, so that clicking the tray
// while it is already open brings it to the front instead of doing nothing.
func focusWindow(pid int) {
	var found uintptr

	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		var owner uint32
		_, _, _ = procGetWindowThreadPid.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != uint32(pid) {
			return 1
		}
		found = hwnd
		return 0
	})

	_, _, _ = procEnumWindows.Call(callback, 0)
	if found == 0 {
		return
	}

	_, _, _ = procShowWindow.Call(found, swRestore)
	_, _, _ = procSetForegroundWindow.Call(found)
}
