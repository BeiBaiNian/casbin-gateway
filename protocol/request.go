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

// This file holds the canonical request: what every format is read into and
// written back out of.

package protocol

import (
	"encoding/json"
	"strings"
)

// Who a message is from. Tool results travel as user content, which is how the
// Anthropic API models them; a format with a role of its own for them splits
// them back out when it writes the request.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// The kinds of content a message and an answer are made of.
const (
	KindText       = "text"
	KindImage      = "image"
	KindThinking   = "thinking"
	KindToolUse    = "tool_use"
	KindToolResult = "tool_result"
)

// Request is one completion request, in the form every format is translated
// through. A field no format of a given request carries stays zero, and is then
// left out of what is written for the upstream.
type Request struct {
	Model    string
	Stream   bool
	System   []string
	Messages []Message

	Tools      []Tool
	ToolChoice *ToolChoice

	Temperature       *float64
	TopP              *float64
	MaxTokens         *int
	StopSequences     []string
	ParallelToolCalls *bool
	Format            *Format
	Reasoning         *Reasoning
}

type Message struct {
	Role    string
	Content []Content
}

// Content is one block of a message or of an answer. Only the field its Kind
// names is filled in.
type Content struct {
	Kind string
	// Text carries the text of a text, thinking or tool_result block.
	Text       string
	Image      *Image
	ToolUse    *ToolUse
	ToolResult *ToolResult
	// Signature is what Anthropic signs a thinking block with. It is carried
	// along so a block handed back to that API is accepted again.
	Signature string
}

// Image is a picture in a message, as a link or as bytes. The formats spell it
// differently: OpenAI sends one URL, which may be a data URI, and Anthropic a
// source object with the media type beside the data.
type Image struct {
	Url       string
	MediaType string
	Data      string
}

type ToolUse struct {
	Id   string
	Name string
	// Arguments is the JSON object the model produced, as text: it arrives in
	// pieces while streaming, so it cannot be held decoded.
	Arguments string
}

type ToolResult struct {
	Id      string
	Text    string
	IsError bool
}

type Tool struct {
	Name        string
	Description string
	// Parameters is the JSON schema of the tool's arguments.
	Parameters json.RawMessage
}

// How a request asks for its tools to be used.
const (
	ChoiceAuto     = "auto"
	ChoiceNone     = "none"
	ChoiceRequired = "required"
	ChoiceTool     = "tool"
)

type ToolChoice struct {
	Mode string
	// Name is the tool that has to be called, for ChoiceTool.
	Name string
}

// Format is the shape an answer is required to take.
type Format struct {
	// Kind is "json_object" or "json_schema".
	Kind   string
	Name   string
	Schema json.RawMessage
	Strict *bool
}

// Reasoning is how much thinking the model is asked to do, in both the ways the
// formats spell it: OpenAI as an effort level, Anthropic as a token budget.
type Reasoning struct {
	Effort       string
	BudgetTokens int
}

// The effort levels OpenAI takes.
const (
	EffortMinimal = "minimal"
	EffortLow     = "low"
	EffortMedium  = "medium"
	EffortHigh    = "high"
)

// The budgets an effort level is worth. They follow what the Anthropic docs
// suggest for a request of that size, and are only ever a translation of a
// level the client itself chose.
const (
	budgetLow    = 4096
	budgetMedium = 8192
	budgetHigh   = 16384
)

// BudgetOf is the thinking budget this request asks for, whichever way it was
// spelled. Zero means the client asked for no thinking at all.
func (reasoning *Reasoning) BudgetOf() int {
	if reasoning == nil {
		return 0
	}
	if reasoning.BudgetTokens > 0 {
		return reasoning.BudgetTokens
	}
	switch reasoning.Effort {
	case EffortLow:
		return budgetLow
	case EffortMedium:
		return budgetMedium
	case EffortHigh:
		return budgetHigh
	}
	return 0
}

// EffortOf is the effort level this request asks for, worked out from the
// budget when that is how it was spelled.
func (reasoning *Reasoning) EffortOf() string {
	if reasoning == nil {
		return ""
	}
	if reasoning.Effort != "" {
		return reasoning.Effort
	}
	switch {
	case reasoning.BudgetTokens == 0:
		return ""
	case reasoning.BudgetTokens <= budgetLow:
		return EffortLow
	case reasoning.BudgetTokens <= budgetMedium:
		return EffortMedium
	}
	return EffortHigh
}

// Text is the plain text of a message, with the non-text blocks left out.
func (message Message) Text() string {
	parts := []string{}
	for _, content := range message.Content {
		if content.Kind == KindText {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "")
}

// TextContent is the shorthand for the common message: one block of text.
func TextContent(text string) []Content {
	return []Content{{Kind: KindText, Text: text}}
}

// imageCharacters is what one picture is counted as, in the characters the
// estimate below works in.
const imageCharacters = 4 * 1500

// EstimateTokens is a rough token count of a request, for the client that asks
// the gateway to size its context against a provider whose API has no endpoint
// to ask. It is the usual four-characters-a-token rule, not a tokenizer.
func EstimateTokens(request *Request) int {
	characters := 0
	for _, system := range request.System {
		characters += len(system)
	}
	for _, message := range request.Messages {
		for _, content := range message.Content {
			characters += len(content.Text)
			if content.ToolUse != nil {
				characters += len(content.ToolUse.Name) + len(content.ToolUse.Arguments)
			}
			if content.ToolResult != nil {
				characters += len(content.ToolResult.Text)
			}
			if content.Image != nil {
				// A picture costs what it costs whatever its encoding weighs,
				// so its bytes are not counted as characters.
				characters += imageCharacters
			}
		}
	}
	for _, tool := range request.Tools {
		characters += len(tool.Name) + len(tool.Description) + len(tool.Parameters)
	}
	return characters / 4
}
