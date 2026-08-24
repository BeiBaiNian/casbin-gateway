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

//go:build darwin || linux

package main

import (
	"errors"

	webview "github.com/webview/webview_go"
)

// runWindow shows the UI in the platform webview: WebKit on macOS, WebKitGTK on
// Linux. Failing here is not fatal for the desktop as a whole — the tray notices
// this process die and falls back to the browser.
func runWindow(url string) error {
	view := webview.New(debugEnabled())
	if view == nil {
		return errors.New("no system webview available")
	}
	defer view.Destroy()

	view.SetTitle(windowTitle)
	view.SetSize(windowWidth, windowHeight, webview.HintNone)
	view.Navigate(url)
	view.Run()
	return nil
}
