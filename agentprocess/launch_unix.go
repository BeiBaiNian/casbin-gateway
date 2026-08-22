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

//go:build !windows

package agentprocess

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

const stopGrace = 2 * time.Second

// spawn runs a launcher in its own session, so the agent outlives Gateway.
func spawn(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// stop asks first and insists afterwards, so an agent gets to write out its
// session before it is killed.
func stop(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	deadline := time.Now().Add(stopGrace)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if process.Signal(syscall.Signal(0)) != nil {
			return nil
		}
	}
	return process.Signal(syscall.SIGKILL)
}
