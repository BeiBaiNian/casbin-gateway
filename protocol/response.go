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

// This file holds the canonical answer: the whole one, the events a streamed
// one is made of, and the two adapters between them.

package protocol

import "strings"

// Response is one whole answer, in the form every format is translated through.
type Response struct {
	Id    string
	Model string
	// Content is what the model produced, in the order it produced it.
	Content    []Content
	StopReason string
	Usage      Usage
	// Failure is set when the answer ended in an error the upstream reported
	// inside an otherwise successful stream.
	Failure *Failure
}

// Why the model stopped.
const (
	StopEnd       = "end"
	StopMaxTokens = "max_tokens"
	StopToolUse   = "tool_use"
	StopSequence  = "stop_sequence"
	StopFilter    = "content_filter"
)

// Usage is what the request cost. InputTokens covers the whole prompt, cached
// part included; a format that reports the two apart takes CacheReadTokens back
// out when it writes them.
type Usage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
}

func (usage Usage) IsZero() bool {
	return usage == Usage{}
}

type Failure struct {
	Kind    string
	Message string
}

// The pieces a streamed answer arrives in.
const (
	EventText     = "text"
	EventThinking = "thinking"
	EventToolUse  = "tool_use"
	// EventDone carries the stop reason and the counters, once, at the end of
	// the answer.
	EventDone    = "done"
	EventFailure = "failure"
)

// Event is one piece of a streamed answer.
type Event struct {
	Kind string
	// Text is the delta of a text or thinking event.
	Text string
	Tool *ToolDelta
	// Signature closes a thinking block Anthropic signed.
	Signature  string
	Model      string
	Usage      *Usage
	StopReason string
	Failure    *Failure
}

// ToolDelta is one piece of a tool call. Index identifies the call being
// assembled: its name and id arrive once, its arguments in as many pieces as
// the upstream cares to send.
type ToolDelta struct {
	Index     int
	Id        string
	Name      string
	Arguments string
}

// Collector assembles a stream of events back into one whole answer, for a
// client that asked for a body rather than a stream.
type Collector struct {
	response Response
	// blocks maps a tool call index onto the content block being assembled for
	// it, so pieces that arrive interleaved still land in the right one.
	blocks map[int]int
}

func NewCollector(model string) *Collector {
	return &Collector{response: Response{Model: model}, blocks: map[int]int{}}
}

func (collector *Collector) Add(event Event) {
	response := &collector.response
	if event.Model != "" {
		response.Model = event.Model
	}

	switch event.Kind {
	case EventText:
		collector.append(KindText, event.Text)
	case EventThinking:
		block := collector.append(KindThinking, event.Text)
		if event.Signature != "" {
			block.Signature = event.Signature
		}
	case EventToolUse:
		collector.tool(event.Tool)
	case EventDone:
		if event.StopReason != "" {
			response.StopReason = event.StopReason
		}
		if event.Usage != nil {
			response.Usage = *event.Usage
		}
	case EventFailure:
		response.Failure = event.Failure
	}
}

// append adds a delta to the block being built, or opens one when the answer
// moved on to another kind of content.
func (collector *Collector) append(kind string, text string) *Content {
	blocks := collector.response.Content
	if last := len(blocks) - 1; last >= 0 && blocks[last].Kind == kind {
		blocks[last].Text += text
		return &blocks[last]
	}
	collector.response.Content = append(blocks, Content{Kind: kind, Text: text})
	return &collector.response.Content[len(collector.response.Content)-1]
}

func (collector *Collector) tool(delta *ToolDelta) {
	if delta == nil {
		return
	}

	index, ok := collector.blocks[delta.Index]
	if !ok {
		index = len(collector.response.Content)
		collector.blocks[delta.Index] = index
		collector.response.Content = append(collector.response.Content,
			Content{Kind: KindToolUse, ToolUse: &ToolUse{}})
	}

	use := collector.response.Content[index].ToolUse
	if delta.Id != "" {
		use.Id = delta.Id
	}
	use.Name += delta.Name
	use.Arguments += delta.Arguments
}

func (collector *Collector) Response() *Response {
	response := collector.response
	if response.StopReason == "" {
		response.StopReason = StopEnd
	}
	return &response
}

// WriteStream writes a whole answer out as a stream, for a client that asked
// for one from an upstream that answered in a single piece.
func WriteStream(writer StreamWriter, response *Response) {
	writer.Open()
	toolIndex := 0
	for _, content := range response.Content {
		switch content.Kind {
		case KindText:
			writer.Write(Event{Kind: EventText, Text: content.Text})
		case KindThinking:
			writer.Write(Event{Kind: EventThinking, Text: content.Text, Signature: content.Signature})
		case KindToolUse:
			if content.ToolUse == nil {
				continue
			}
			writer.Write(Event{Kind: EventToolUse, Tool: &ToolDelta{
				Index:     toolIndex,
				Id:        content.ToolUse.Id,
				Name:      content.ToolUse.Name,
				Arguments: content.ToolUse.Arguments,
			}})
			toolIndex++
		}
	}
	if response.Failure != nil {
		writer.Write(Event{Kind: EventFailure, Failure: response.Failure})
	}
	writer.Write(Event{Kind: EventDone, StopReason: response.StopReason, Usage: &response.Usage})
	writer.Close()
}

// Text is the plain text of an answer, with the thinking and the tool calls
// left out.
func (response *Response) Text() string {
	parts := []string{}
	for _, content := range response.Content {
		if content.Kind == KindText {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "")
}

// ToolUses are the calls the model asked for, in order.
func (response *Response) ToolUses() []*ToolUse {
	uses := []*ToolUse{}
	for _, content := range response.Content {
		if content.Kind == KindToolUse && content.ToolUse != nil {
			uses = append(uses, content.ToolUse)
		}
	}
	return uses
}
