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
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/apache/casbin-gateway/conf"
)

const (
	desktopLogPath = "./logs/casbin-gateway-desktop.out"
	devUiLogPath   = "./logs/web-dev.out"

	// devUiAddress is where "yarn dev" serves the frontend, fixed in
	// web/vite.config.ts.
	devUiAddress = "localhost:16002"

	startWait = 90 * time.Second
	startPoll = 300 * time.Millisecond
)

// TestDesktopApp starts Casbin Gateway as the desktop app — tray icon and
// native window — rather than the bare server "go run ." leaves in a terminal.
// It is a test only so that running it is one command:
//
//	go test -run TestDesktopApp -v .
//
// The window shows the Vite dev server on port 16002, so it runs the frontend
// in web/src with hot reload rather than the compiled copy in web/build, which
// is only as new as the last "yarn build". Set CASBIN_GATEWAY_URL to point the
// window elsewhere — at the server's own port to see the built UI instead.
//
// Everything is rebuilt from source and left running after the test returns:
// quit the app from the tray icon, which stops the server it started, and the
// dev server keeps serving for the next run. Every environment variable reaches
// the executables, so httpport=17001 runs this beside a Gateway already holding
// the configured port.
func TestDesktopApp(t *testing.T) {
	// Without this, "go test ./..." would launch the app and never come back.
	if pattern := flag.Lookup("test.run"); pattern == nil || pattern.Value.String() == "" {
		t.Skip("launching the desktop app is opt-in: go test -run TestDesktopApp -v .")
	}

	home, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	serverExe := filepath.Join(home, exeName("casbin-gateway"))
	desktopExe := filepath.Join(home, exeName("casbin-gateway-desktop"))
	port := conf.GetHttpPort()
	apiAddress := fmt.Sprintf("127.0.0.1:%d", port)

	// Both executables are about to be rebuilt, so a server running from the
	// old one has to go: Windows locks a running executable, and one still
	// answering on the port is one the tray would attach to instead of
	// starting the build under test.
	stopped := ""
	if _, err := os.Stat(serverExe); err == nil {
		output, _ := exec.Command(serverExe, "stop").CombinedOutput()
		stopped = strings.TrimSpace(string(output))
		if stopped != "" {
			t.Log(stopped)
		}
	}

	// Anything still on the port is something "stop" does not recognize as
	// Gateway. The tray would attach to it and show its UI, which looks like a
	// working desktop app running code that is not the one just built.
	if isServing(apiAddress) {
		t.Fatalf("port %d is still in use, so the build under test cannot serve it: quit what holds it, or run with httpport=<free port>\n%s", port, stopped)
	}

	goBuild(t, home, serverExe)
	goBuild(t, filepath.Join(home, "desktop"), desktopExe)

	url := strings.TrimSpace(os.Getenv("CASBIN_GATEWAY_URL"))
	if url == "" {
		url = startDevUi(t, home, port)
	}

	cmd := exec.Command(desktopExe)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), "CASBIN_GATEWAY_URL="+url)
	cmd.Stdout, cmd.Stderr = logWriter(t, desktopLogPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	waitForServing(t, "the desktop app", cmd, apiAddress, desktopLogPath)

	t.Logf("Casbin Gateway is running: UI %s, API http://%s (desktop pid %d, log %s)",
		url, apiAddress, cmd.Process.Pid, desktopLogPath)
	t.Log("it keeps running after this test; quit it from the tray icon")
}

// startDevUi brings up "yarn dev" and returns the URL the window should show.
// The dev server proxies /api and /v1 to the Gateway this run starts, which is
// what keeps one origin — and therefore the session cookie — for the window.
func startDevUi(t *testing.T, home string, backendPort int) string {
	t.Helper()

	url := "http://" + devUiAddress
	if isServing(devUiAddress) {
		t.Logf("reusing the dev server already on %s", url)
		if backendPort != 17000 {
			t.Logf("it proxies the API to whatever backend it was started with, not necessarily port %d", backendPort)
		}
		return url
	}

	webDir := filepath.Join(home, "web")
	if _, err := os.Stat(filepath.Join(webDir, "node_modules")); err != nil {
		t.Fatal("the frontend dependencies are missing: cd web && yarn install")
	}

	// --strictPort so that a port already taken is an error here rather than a
	// dev server quietly moving to the next one and a window showing nothing.
	cmd := exec.Command("yarn", "dev", "--strictPort")
	cmd.Dir = webDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("VITE_BACKEND_URL=http://127.0.0.1:%d", backendPort))
	cmd.Stdout, cmd.Stderr = logWriter(t, devUiLogPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start the dev server: %v", err)
	}

	waitForServing(t, "the dev server", cmd, devUiAddress, devUiLogPath)
	t.Logf("dev server on %s (log %s)", url, devUiLogPath)

	return url
}

// waitForServing blocks until the child answers, watching it for an early exit
// so that a server that failed to start reports its own reason instead of a
// timeout.
func waitForServing(t *testing.T, what string, cmd *exec.Cmd, address string, logPath string) {
	t.Helper()

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(startWait)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("%s exited (%v)\n%s", what, err, logTail(logPath))
		default:
		}

		if isServing(address) {
			return
		}
		time.Sleep(startPoll)
	}

	t.Fatalf("%s did not answer on %s within %v\n%s", what, address, startWait, logTail(logPath))
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func goBuild(t *testing.T, dir string, output string) {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", output, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		hint := ""
		if runtime.GOOS == "windows" {
			hint = " (Windows locks a running executable: quit Casbin Gateway from its tray icon first)"
		}
		t.Fatalf("building %s failed%s: %v\n%s", dir, hint, err, out)
	}
	t.Logf("built %s", output)
}

// logWriter is where a child that outlives this test writes. The file is closed
// here because Start() hands the child its own handle.
func logWriter(t *testing.T, path string) (*os.File, *os.File) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })

	return file, file
}

// isServing probes an address rather than a port because the two servers here
// do not bind the same one: Gateway listens on 127.0.0.1, while the dev server
// listens on whatever "localhost" resolves to, which on Windows is IPv6 first.
func isServing(address string) bool {
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func logTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.Join(lines, "\n")
}
