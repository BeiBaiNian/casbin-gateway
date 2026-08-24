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

package version

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/beego/beego"
)

// macOsBundleLauncher is the copy the installer puts inside the application
// bundle, which is a copy because a bundle whose executable lives outside it
// gets neither the Dock icon nor the name.
const macOsBundleLauncher = "Applications/Casbin Gateway.app/Contents/MacOS/casbin-gateway-desktop"

func desktopLauncherName() string {
	if runtime.GOOS == "windows" {
		return "casbin-gateway-desktop.exe"
	}
	return "casbin-gateway-desktop"
}

// updateDesktopLauncher replaces the launcher next to the Gateway with the one
// from the same archive. It is best-effort and never fails the update: an
// installation without the desktop has nothing to replace, and one whose
// launcher is running out of a directory this cannot write to still ends up
// with an updated server.
func updateDesktopLauncher(archive string, staging string, installDir string) {
	launcher := filepath.Join(installDir, desktopLauncherName())
	if _, err := os.Stat(launcher); err != nil {
		return
	}

	staged := filepath.Join(staging, desktopLauncherName())
	if err := extractBinary(archive, staged); err != nil {
		beego.Error("the update holds no desktop launcher:", err)
		return
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		beego.Error("could not make the new desktop launcher executable:", err)
		return
	}

	if err := swapLauncher(launcher, staged); err != nil {
		beego.Error("the desktop launcher was left at the previous version:", err)
		return
	}

	refreshMacOsBundle(launcher)
}

// swapLauncher moves the new launcher into place. The old one is renamed rather
// than overwritten because it is very likely running — the tray icon is what
// started this Gateway — and a running executable can be renamed on every
// platform but replaced in place on none.
func swapLauncher(launcher string, staged string) error {
	backup := launcher + backupSuffix
	_ = os.Remove(backup)

	if err := os.Rename(launcher, backup); err != nil {
		return err
	}
	if err := os.Rename(staged, launcher); err != nil {
		_ = os.Rename(backup, launcher)
		return err
	}

	removeWhenUnlocked(backup)
	return nil
}

// refreshMacOsBundle keeps the copy inside the application bundle in step with
// the one that was just replaced. Without it the Dock icon would go on starting
// the previous launcher.
func refreshMacOsBundle(launcher string) {
	if runtime.GOOS != "darwin" {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	bundled := filepath.Join(home, macOsBundleLauncher)
	if _, err := os.Stat(bundled); err != nil {
		return
	}

	// Copied through a temporary name in the same directory so that the rename
	// onto the running executable is atomic.
	content, err := os.ReadFile(launcher)
	if err != nil {
		return
	}
	staged := bundled + ".new"
	if err := os.WriteFile(staged, content, 0o755); err != nil {
		beego.Error("could not stage the launcher inside the application bundle:", err)
		return
	}
	if err := swapLauncher(bundled, staged); err != nil {
		_ = os.Remove(staged)
		beego.Error("the application bundle was left at the previous version:", err)
	}
}
