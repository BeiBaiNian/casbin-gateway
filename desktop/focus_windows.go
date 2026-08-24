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
	"sync"
	"syscall"
	"unsafe"
)

// webviewClass is the window class go-webview2 registers. The window is looked
// up by class rather than by "is it visible", because the whole point is to
// find it again once closing has hidden it.
const webviewClass = "webview"

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9
)

var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")

	procSetAppId            = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowThreadPid  = user32.NewProc("GetWindowThreadProcessId")
	procGetClassNameW       = user32.NewProc("GetClassNameW")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

func setAppUserModelId() {
	id, err := syscall.UTF16PtrFromString(appId)
	if err != nil {
		return
	}
	_, _, _ = procSetAppId.Call(uintptr(unsafe.Pointer(id)))
}

// focusWindow shows the window process's own window and raises it, so that
// clicking the tray brings back the window the user closed — the same one, with
// the page still loaded — rather than starting over.
func focusWindow(pid int) {
	hwnd := findWindow(pid)
	if hwnd == 0 {
		return
	}

	show := uintptr(swShow)
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		show = swRestore
	}
	_, _, _ = procShowWindow.Call(hwnd, show)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
}

// search is package level because syscall.NewCallback allocates a callback that
// is never freed, so the enumeration cannot make one per click.
var search struct {
	sync.Mutex
	pid   uint32
	found uintptr
}

var enumWindowsCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	var owner uint32
	_, _, _ = procGetWindowThreadPid.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
	if owner != search.pid || windowClass(hwnd) != webviewClass {
		return 1
	}
	search.found = hwnd
	return 0
})

func findWindow(pid int) uintptr {
	search.Lock()
	defer search.Unlock()

	search.pid = uint32(pid)
	search.found = 0
	_, _, _ = procEnumWindows.Call(enumWindowsCallback, 0)
	return search.found
}

func windowClass(hwnd uintptr) string {
	var name [64]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	return syscall.UTF16ToString(name[:n])
}
