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
	"fmt"
	"os"
	"time"
)

const (
	// portReleaseTimeout bounds the wait for a killed process to let go of its
	// port. The kill is asynchronous, so the port stays busy for a short while
	// after the process itself is gone.
	portReleaseTimeout = 3 * time.Second
	// portReleasePollInterval is how often the port is retried while waiting.
	portReleasePollInterval = 100 * time.Millisecond
)

// ForeignPortError reports a port held by a process that is not Casbin Gateway.
// That process keeps running: the port may well be its own, and taking it by
// force would stop something the rest of the machine depends on.
type ForeignPortError struct {
	Port   int
	Holder string
}

func (e *ForeignPortError) Error() string {
	return fmt.Sprintf("port %d is held by %s, which is not Casbin Gateway", e.Port, e.Holder)
}

// StopOldInstance stops a previous Casbin Gateway still listening on the port,
// so that a restart never has to wait for it to be shut down by hand.
//
// Only another copy of this executable is ever stopped. A port held by anything
// else belongs to that program - an nginx on 443, a dev server on 17000 - so it
// is left alone and returned as a ForeignPortError; the caller reports the
// conflict instead of taking the port by force.
//
// Only sockets in the LISTEN state count, so a process that merely holds a
// connection to a remote port with the same number is never touched.
func StopOldInstance(port int) error {
	holder := LookupPortHolder(port)
	if holder == nil || holder.Pid == os.Getpid() {
		return nil
	}

	if !holder.Ours {
		return &ForeignPortError{Port: port, Holder: holder.String()}
	}

	process, err := os.FindProcess(holder.Pid)
	if err != nil {
		return err
	}

	err = process.Kill()
	if err != nil {
		return err
	}

	fmt.Printf("Casbin Gateway: stopped the previous Gateway (%s), which was holding port %d\n", holder, port)

	return waitForPortRelease(port)
}

// waitForPortRelease blocks until the port can be bound again, because a killed
// process releases it a moment after it disappears. It gives up quietly on
// timeout and leaves the caller's own bind to produce the error.
func waitForPortRelease(port int) error {
	deadline := time.Now().Add(portReleaseTimeout)
	for {
		if err := CheckPortAvailable(port); err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return nil
		}

		time.Sleep(portReleasePollInterval)
	}
}
