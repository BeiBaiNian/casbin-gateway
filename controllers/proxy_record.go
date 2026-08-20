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

package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/apache/casbin-gateway/object"
)

// usageTailBytes is how much of the end of a response is kept to read the token
// usage out of. Both APIs report it last, so the tail is enough.
const usageTailBytes = 16 * 1024

// finishLlmRecord queues the record of one relayed request. Every path out of
// forwardToChannels ends the client request, so this is the only place it is
// called from.
func (c *ApiController) finishLlmRecord(route *proxyRoute) {
	if route.record == nil {
		return
	}

	record := route.record
	route.record = nil
	record.DurationMs = time.Since(route.start).Milliseconds()
	object.AddLlmRecord(record, route.body)
}

func (route *proxyRoute) recordAttempt(channelId string) {
	if route.record == nil {
		return
	}
	route.record.Attempts++
	route.record.Channel = channelId
}

func (route *proxyRoute) recordOutcome(status int, message string) {
	if route.record == nil {
		return
	}
	route.record.Status = status
	route.record.Error = message
}

func (route *proxyRoute) recordUsage(tail []byte) {
	if route.record == nil {
		return
	}

	usage := readUsage(tail)
	route.record.PromptTokens = higher(usage.PromptTokens, usage.InputTokens)
	route.record.CompletionTokens = higher(usage.CompletionTokens, usage.OutputTokens)
	route.record.TotalTokens = usage.TotalTokens
	if route.record.TotalTokens == 0 {
		route.record.TotalTokens = route.record.PromptTokens + route.record.CompletionTokens
	}
}

// usageTap keeps the tail of a relayed response. It never changes a byte of
// what the client receives, and never holds more than usageTailBytes.
type usageTap struct {
	reader io.Reader
	tail   []byte
}

func (tap *usageTap) Read(p []byte) (int, error) {
	n, err := tap.reader.Read(p)
	if n > 0 {
		tap.keep(p[:n])
	}
	return n, err
}

func (tap *usageTap) keep(chunk []byte) {
	if len(chunk) >= usageTailBytes {
		tap.tail = append(tap.tail[:0], chunk[len(chunk)-usageTailBytes:]...)
		return
	}
	if len(tap.tail)+len(chunk) > usageTailBytes {
		kept := usageTailBytes - len(chunk)
		copy(tap.tail, tap.tail[len(tap.tail)-kept:])
		tap.tail = tap.tail[:kept]
	}
	tap.tail = append(tap.tail, chunk...)
}

// llmUsage covers both spellings of the same counters: OpenAI reports
// prompt/completion tokens, Anthropic input/output ones.
type llmUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
}

func (usage *llmUsage) merge(other llmUsage) {
	usage.PromptTokens = higher(usage.PromptTokens, other.PromptTokens)
	usage.CompletionTokens = higher(usage.CompletionTokens, other.CompletionTokens)
	usage.TotalTokens = higher(usage.TotalTokens, other.TotalTokens)
	usage.InputTokens = higher(usage.InputTokens, other.InputTokens)
	usage.OutputTokens = higher(usage.OutputTokens, other.OutputTokens)
}

// readUsage merges every usage object in the tail: a stream splits the counters
// across several events, and a plain response carries them once.
func readUsage(tail []byte) llmUsage {
	usage := llmUsage{}
	marker := []byte(`"usage"`)
	for rest := tail; ; {
		index := bytes.Index(rest, marker)
		if index < 0 {
			return usage
		}
		rest = rest[index+len(marker):]

		var parsed llmUsage
		if value := balancedObject(rest); value != nil && json.Unmarshal(value, &parsed) == nil {
			usage.merge(parsed)
		}
	}
}

// balancedObject returns the JSON object that directly follows a key, or nil
// when the value is not one. Usage objects hold only numbers, so counting
// braces cannot be thrown off by one inside a string.
func balancedObject(data []byte) []byte {
	start := 0
	for start < len(data) && (data[start] == ':' || data[start] == ' ' || data[start] == '\t' || data[start] == '\r' || data[start] == '\n') {
		start++
	}
	if start >= len(data) || data[start] != '{' {
		return nil
	}

	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return nil
}

func higher(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
