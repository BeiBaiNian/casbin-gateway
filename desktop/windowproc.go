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
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// windowFailWindow is how long a window child has to stay up before its exit
// counts as the user closing it rather than as a webview that could not start.
const windowFailWindow = 3 * time.Second

var window struct {
	sync.Mutex
	process *os.Process
	closing bool
}

// showWindow puts the management UI in front of the user. The window is a child
// process, so an open one is raised rather than replaced, and one that never
// came up — a Linux host without WebKitGTK, a Windows one without the WebView2
// runtime — falls back to the default browser.
func showWindow() {
	window.Lock()
	if window.process != nil {
		pid := window.process.Pid
		window.Unlock()
		focusWindow(pid)
		return
	}
	window.Unlock()

	// Clicking the tray while the server is still coming up, or after it failed
	// to, would open a window on a port nothing answers. The startup path opens
	// the window itself once the server is serving, so there is nothing to lose
	// by doing nothing here.
	if !isServing(httpPort()) {
		return
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
		return
	}

	cmd := exec.Command(executable, "window", gatewayUrl())
	cmd.Dir = gatewayHome()
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
		fallbackToBrowser()
		return
	}

	window.Lock()
	window.process = cmd.Process
	window.closing = false
	window.Unlock()

	go func() {
		startedAt := time.Now()
		waitErr := cmd.Wait()

		window.Lock()
		killed := window.closing
		window.process = nil
		window.closing = false
		window.Unlock()

		if killed || waitErr == nil || time.Since(startedAt) >= windowFailWindow {
			return
		}
		fmt.Fprintf(os.Stderr, "casbin-gateway-desktop: no window on this host (%v) %s\n",
			waitErr, strings.TrimSpace(stderr.String()))
		fallbackToBrowser()
	}()
}

func closeWindow() {
	window.Lock()
	process := window.process
	if process != nil {
		window.closing = true
	}
	window.Unlock()

	if process != nil {
		_ = process.Kill()
	}
}

func fallbackToBrowser() {
	if err := openBrowser(gatewayUrl()); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
	}
}
