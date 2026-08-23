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

package util

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	// EnvDaemonChildKey marks the copy started by "start", so it serves instead
	// of spawning another one.
	EnvDaemonChildKey = "CASBIN_GATEWAY_DAEMON"
	// daemonStartTimeout bounds the wait for the detached copy to answer.
	daemonStartTimeout = 30 * time.Second
	daemonPollInterval = 200 * time.Millisecond
)

// RunCommand handles the sub-commands that manage a background Gateway, so that
// running it does not have to occupy a terminal. It reports whether it handled
// os.Args, in which case the caller must not go on to serve.
func RunCommand(args []string, port int, logPath string) bool {
	if os.Getenv(EnvDaemonChildKey) == "1" {
		return false
	}

	switch commandOf(args) {
	case "start":
		startDetached(port, logPath)
	case "stop":
		stopDaemon(port)
	case "status":
		printStatus(port)
	default:
		return false
	}
	return true
}

func commandOf(args []string) string {
	for _, arg := range args[1:] {
		switch arg {
		case "start", "--daemon", "-d":
			return "start"
		case "stop", "--stop":
			return "stop"
		case "status", "--status":
			return "status"
		}
	}
	return ""
}

// startDetached launches a copy of this executable with no terminal attached and
// waits until it answers, so that a failed start is reported here rather than
// discovered later in a log nobody opened.
func startDetached(port int, logPath string) {
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Casbin Gateway: cannot find its own executable: %v\n", err)
		os.Exit(1)
	}

	if isServing(port) {
		fmt.Printf("Casbin Gateway is already running on http://localhost:%d\n", port)
		return
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Printf("Casbin Gateway: cannot create %s: %v\n", filepath.Dir(logPath), err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Printf("Casbin Gateway: cannot write %s: %v\n", logPath, err)
		os.Exit(1)
	}
	defer logFile.Close()

	cmd := exec.Command(executable)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.Env = append(os.Environ(), EnvDaemonChildKey+"=1")
	detach(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Printf("Casbin Gateway: could not start in the background: %v\n", err)
		os.Exit(1)
	}

	deadline := time.Now().Add(daemonStartTimeout)
	for time.Now().Before(deadline) {
		if isServing(port) {
			fmt.Printf("Casbin Gateway is running in the background on http://localhost:%d (pid %d)\n", port, cmd.Process.Pid)
			fmt.Printf("  Log: %s   Stop it with: casbin-gateway stop\n", logPath)
			return
		}
		time.Sleep(daemonPollInterval)
	}

	fmt.Printf("Casbin Gateway did not answer on port %d within %v. See %s\n", port, daemonStartTimeout, logPath)
	os.Exit(1)
}

func stopDaemon(port int) {
	if !isServing(port) {
		fmt.Printf("Casbin Gateway is not running on port %d\n", port)
		return
	}

	err := StopOldInstance(port)
	if err == nil {
		fmt.Println("Casbin Gateway stopped")
		return
	}

	// Answering on the port is not the same as being Gateway, so this is where
	// "stop" finds out that the port belongs to something else entirely.
	var foreign *ForeignPortError
	if errors.As(err, &foreign) {
		fmt.Printf("Port %d is held by %s, not by Casbin Gateway, so nothing was stopped\n", port, foreign.Holder)
		os.Exit(1)
	}

	fmt.Printf("Casbin Gateway: could not stop the process on port %d: %v\n", port, err)
	os.Exit(1)
}

func printStatus(port int) {
	if !isServing(port) {
		fmt.Printf("Casbin Gateway is not running on port %d\n", port)
		return
	}

	holder := LookupPortHolder(port)
	if holder == nil {
		fmt.Printf("Casbin Gateway is running on http://localhost:%d\n", port)
		return
	}

	if !holder.Ours {
		fmt.Printf("Port %d is held by %s, not by Casbin Gateway\n", port, holder)
		return
	}

	fmt.Printf("Casbin Gateway is running on http://localhost:%d (%s)\n", port, holder)
}

// isServing reports whether something already answers on the port, which is the
// only signal available to a process that did not start it.
func isServing(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
