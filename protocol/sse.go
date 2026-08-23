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

package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// forEachEvent calls fn with the data of every event in an SSE stream, until fn
// asks it to stop. The event name is not passed on: every format the gateway
// reads repeats it inside the data.
func forEachEvent(reader io.Reader, fn func([]byte) bool) error {
	buffered := bufio.NewReaderSize(reader, 64*1024)
	data := []byte{}
	for {
		line, err := buffered.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(trimmed, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(trimmed, "data:"), " ")...)
		case trimmed == "" && len(data) > 0:
			if !fn(data) {
				return nil
			}
			data = data[:0]
		}
		if err != nil {
			if len(data) > 0 {
				fn(data)
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// eventWriter writes one SSE stream. A write that fails is dropped: the client
// has stopped reading, and there is nowhere left to report it to.
type eventWriter struct {
	writer io.Writer
	flush  func()
}

// send writes one event. An empty name leaves the event line out, which is what
// the OpenAI stream is made of.
func (writer *eventWriter) send(name string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if name == "" {
		writer.raw(fmt.Sprintf("data: %s\n\n", data))
		return
	}
	writer.raw(fmt.Sprintf("event: %s\ndata: %s\n\n", name, data))
}

func (writer *eventWriter) raw(text string) {
	if _, err := io.WriteString(writer.writer, text); err != nil {
		return
	}
	if writer.flush != nil {
		writer.flush()
	}
}
