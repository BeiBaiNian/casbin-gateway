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

//go:build darwin

package agentprocess

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

const psPath = "/bin/ps"

// list joins two ps listings: "comm" is the executable's full path on macOS,
// while "command" carries the arguments an interpreted agent is named in.
func list(ctx context.Context, withCommands bool) []Process {
	paths := psImages(ctx)
	if len(paths) == 0 {
		return nil
	}

	commands := map[int]string{}
	if withCommands {
		commands = psCommands(ctx)
	}
	result := make([]Process, 0, len(paths))
	for pid, path := range paths {
		result = append(result, Process{Pid: pid, Path: path, Command: commands[pid]})
	}
	return result
}

func psImages(ctx context.Context) map[int]string {
	paths := map[int]string{}
	for _, line := range psLines(ctx, "pid=,comm=") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		// A path may hold spaces, so the image is everything after the pid.
		paths[pid] = strings.Join(fields[1:], " ")
	}
	return paths
}

func psCommands(ctx context.Context) map[int]string {
	commands := map[int]string{}
	for _, line := range psLines(ctx, "pid=,command=") {
		fields := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(fields) < 2 {
			continue
		}
		if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
			commands[pid] = fields[1]
		}
	}
	return commands
}

func psLines(ctx context.Context, format string) []string {
	output, err := exec.CommandContext(ctx, psPath, "-axww", "-o", format).Output()
	if err != nil {
		return nil
	}
	return strings.Split(string(output), "\n")
}
