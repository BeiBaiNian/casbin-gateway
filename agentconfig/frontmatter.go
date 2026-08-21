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

package agentconfig

import "strings"

const maxDescriptionRunes = 400

// parseFrontMatter reads the name and description out of a SKILL.md header.
//
// The header is YAML, but a skill only ever declares flat string fields in it,
// and two of them are all a list needs. Reading those two directly keeps this a
// leaf package and keeps a header Gateway does not fully understand from
// costing the operator the whole listing.
func parseFrontMatter(raw string) (string, string) {
	lines, ok := frontMatterLines(raw)
	if !ok {
		return "", ""
	}

	name, description := "", ""
	for index := 0; index < len(lines); index++ {
		key, value := splitField(lines[index])
		if key != "name" && key != "description" {
			continue
		}
		if folded := strings.TrimSpace(value); folded == ">" || folded == ">-" || folded == "|" || folded == "|-" {
			var consumed int
			value, consumed = readBlock(lines[index+1:], strings.HasPrefix(folded, "|"))
			index += consumed
		}

		if key == "name" {
			name = unquote(value)
		} else {
			description = unquote(value)
		}
	}
	return name, truncate(description)
}

// frontMatterLines returns the lines between the opening and closing --- of a
// document that starts with one.
func frontMatterLines(raw string) ([]string, bool) {
	raw = strings.TrimLeft(raw, "\uFEFF \t\r\n")
	if !strings.HasPrefix(raw, "---") {
		return nil, false
	}

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for index := 1; index < len(lines); index++ {
		if strings.TrimRight(lines[index], " \t") == "---" {
			return lines[1:index], true
		}
	}
	return nil, false
}

// splitField returns a top-level "key: value" pair. An indented line belongs to
// the field above it and is not one itself.
func splitField(line string) (string, string) {
	if line == "" || line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(strings.TrimSpace(line), "#") {
		return "", ""
	}
	key, value, found := strings.Cut(line, ":")
	if !found {
		return "", ""
	}
	return strings.TrimSpace(key), strings.TrimSpace(value)
}

// readBlock collects the indented lines of a folded (>) or literal (|) scalar,
// and reports how many lines it consumed.
func readBlock(lines []string, literal bool) (string, int) {
	collected := []string{}
	consumed := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		consumed++
		collected = append(collected, strings.TrimSpace(line))
	}

	separator := " "
	if literal {
		separator = "\n"
	}
	return strings.TrimSpace(strings.Join(collected, separator)), consumed
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func truncate(value string) string {
	runes := []rune(value)
	if len(runes) <= maxDescriptionRunes {
		return value
	}
	return string(runes[:maxDescriptionRunes]) + "..."
}
