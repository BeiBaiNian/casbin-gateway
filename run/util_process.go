// Copyright 2023 The casbin Authors. All Rights Reserved.
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

package run

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// A site runs in its own console window started from its repo folder. The
// "title" sets the window title so the user can tell the windows apart, and it
// is also what identifies the process in the command line listing below.
var reSiteNames = regexp.MustCompile(`title (\S+) & go run main\.go`)

func getRunCommand(name string) string {
	return fmt.Sprintf("title %s & go run main.go", name)
}

func parseSiteName(s string) string {
	res := reSiteNames.FindStringSubmatch(s)
	if res == nil {
		return ""
	}

	return res[1]
}

func getSiteNamesFromOutput(output string) map[string]int {
	siteNameMap := map[string]int{}

	output = strings.ReplaceAll(output, "\r", "")
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		name := parseSiteName(line)
		if name == "" {
			continue
		}

		tokens := strings.Fields(line)
		processId, err := strconv.Atoi(tokens[len(tokens)-1])
		if err != nil {
			continue
		}

		siteNameMap[name] = processId
	}

	return siteNameMap
}

func getPid(name string) (int, error) {
	name = getMappedName(name)

	psCommand := `Get-CimInstance Win32_Process -Filter "Name='cmd.exe'" | Select-Object CommandLine, ProcessId | ForEach-Object { "$($_.CommandLine) $($_.ProcessId)" }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("powershell command failed: %v, stderr: %s", err, stderr.String())
	}

	siteNameMap := getSiteNamesFromOutput(out.String())
	pid, ok := siteNameMap[name]
	if ok {
		return pid, nil
	} else {
		return 0, fmt.Errorf("getSiteNamesFromOutput() error, name = %s, siteNameMap = %v", name, siteNameMap)
	}
}

func startProcess(name string) error {
	fmt.Printf("startProcess(): [%s]\n", name)

	cmd := exec.Command("cmd", "/C", "start", "", "cmd", "/C", getRunCommand(getMappedName(name)))
	cmd.Dir = GetRepoPath(name)
	return cmd.Run()
}

func stopProcess(name string) error {
	fmt.Printf("stopProcess(): [%s]\n", name)

	pid, err := getPid(name)
	if err != nil {
		// Not running is not an error to report here.
		return nil
	}

	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}

func IsProcessActive(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, err
	}

	output := out.String()
	res := strings.Contains(output, strconv.Itoa(pid))
	return res, nil
}

// IsSiteProcessActive covers the window between "started" and "serving", while
// go run is still compiling and the port is not up yet.
func IsSiteProcessActive(name string) (bool, error) {
	_, err := getPid(name)
	if err != nil {
		return false, nil
	}

	return true, nil
}
