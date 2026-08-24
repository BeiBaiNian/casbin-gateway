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

// Package jsonc reads the relaxed JSON some agents accept in their config
// files: comments and trailing commas, which encoding/json rejects.
package jsonc

// Strip returns data as plain JSON, blanking comments and dropping trailing
// commas. Byte offsets are preserved, so a parse error still points at the
// place in the file the operator sees.
func Strip(data []byte) []byte {
	result := make([]byte, len(data))
	copy(result, data)

	// The last comma seen outside a string, to be dropped if the next thing is
	// a closing bracket rather than another value.
	comma := -1
	for i := 0; i < len(result); i++ {
		switch result[i] {
		case '"':
			i = skipString(result, i)
			comma = -1
		case '/':
			if end := skipComment(result, i); end > i {
				blank(result, i, end)
				i = end - 1
			}
		case ',':
			comma = i
		case '}', ']':
			if comma >= 0 {
				result[comma] = ' '
			}
			comma = -1
		case ' ', '\t', '\r', '\n':
		default:
			comma = -1
		}
	}
	return result
}

// skipString returns the index of the closing quote of the string starting at
// start, or the end of the data when it is unterminated.
func skipString(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return len(data)
}

// skipComment returns the index just past the comment starting at start, or
// start when nothing there is a comment.
func skipComment(data []byte, start int) int {
	if start+1 >= len(data) {
		return start
	}
	switch data[start+1] {
	case '/':
		for i := start + 2; i < len(data); i++ {
			if data[i] == '\n' {
				return i
			}
		}
		return len(data)
	case '*':
		for i := start + 2; i+1 < len(data); i++ {
			if data[i] == '*' && data[i+1] == '/' {
				return i + 2
			}
		}
		return len(data)
	}
	return start
}

// blank replaces a comment with spaces, keeping the newlines so the line
// numbers of everything after it do not move.
func blank(data []byte, start, end int) {
	for i := start; i < end; i++ {
		if data[i] != '\n' {
			data[i] = ' '
		}
	}
}
