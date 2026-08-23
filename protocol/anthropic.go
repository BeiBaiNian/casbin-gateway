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

// This file speaks the Anthropic Messages API, which Claude Code and the other
// Claude clients talk, and which the Claude providers serve.

package protocol

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type anthropicCodec struct{}

func init() {
	register(anthropicCodec{})
}

func (anthropicCodec) Name() string { return Anthropic }

func (anthropicCodec) Endpoint() string { return "/v1/messages" }

// defaultMaxTokens is what a request that named no limit is sent with, since
// this API requires one. It is high enough for a long answer and low enough for
// every current Claude model to accept it.
const defaultMaxTokens = 8192

// ---------------------------------------------------------------------------
// The wire types
// ---------------------------------------------------------------------------

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	System        json.RawMessage    `json:"system,omitempty"`
	Messages      []anthropicMessage `json:"messages"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    json.RawMessage    `json:"tool_choice,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Thinking      *anthropicThinking `json:"thinking,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// anthropicBlock is one content block. Its fields are a union over the block
// types, which is how the API itself is shaped.
type anthropicBlock struct {
	Type      string           `json:"type"`
	Text      string           `json:"text,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`
	Signature string           `json:"signature,omitempty"`
	Source    *anthropicSource `json:"source,omitempty"`
	Id        string           `json:"id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Input     json.RawMessage  `json:"input,omitempty"`
	ToolUseId string           `json:"tool_use_id,omitempty"`
	Content   json.RawMessage  `json:"content,omitempty"`
	IsError   bool             `json:"is_error,omitempty"`
}

type anthropicSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	Url       string `json:"url,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

type anthropicResponse struct {
	Id         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Model      string           `json:"model"`
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Usage      *anthropicUsage  `json:"usage,omitempty"`
}

// anthropicEvent is one event of a streamed answer, in the union the API sends
// them as.
type anthropicEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index"`
	Message      *anthropicResponse `json:"message"`
	ContentBlock *anthropicBlock    `json:"content_block"`
	Delta        *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
		PartialJson string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *anthropicUsage `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

func (anthropicCodec) DecodeRequest(raw []byte) (*Request, error) {
	var body anthropicRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("invalid request body")
	}

	request := &Request{
		Model:         body.Model,
		Stream:        body.Stream,
		Temperature:   body.Temperature,
		TopP:          body.TopP,
		StopSequences: body.StopSequences,
		ToolChoice:    anthropicToolChoiceOf(body.ToolChoice),
	}
	if body.MaxTokens > 0 {
		maxTokens := body.MaxTokens
		request.MaxTokens = &maxTokens
	}
	if system := anthropicText(body.System); system != "" {
		request.System = []string{system}
	}
	if body.Thinking != nil && body.Thinking.Type != "disabled" {
		request.Reasoning = &Reasoning{BudgetTokens: body.Thinking.BudgetTokens}
	}
	for _, tool := range body.Tools {
		if tool.Name == "" {
			continue
		}
		request.Tools = append(request.Tools, Tool{
			Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema,
		})
	}

	for _, message := range body.Messages {
		content := anthropicContentOf(message.Content)
		if len(content) == 0 {
			continue
		}
		role := RoleUser
		if message.Role == RoleAssistant {
			role = RoleAssistant
		}
		request.Messages = append(request.Messages, Message{Role: role, Content: content})
	}
	return request, nil
}

// anthropicContentOf reads message content, which is either a plain string or
// the blocks the message is made of.
func anthropicContentOf(raw json.RawMessage) []Content {
	if len(raw) == 0 {
		return nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil
		}
		return TextContent(text)
	}

	var blocks []anthropicBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}

	content := []Content{}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				content = append(content, Content{Kind: KindText, Text: block.Text})
			}
		case "thinking", "redacted_thinking":
			content = append(content, Content{
				Kind: KindThinking, Text: block.Thinking, Signature: block.Signature,
			})
		case "image", "document":
			if image := anthropicImageOf(block.Source); image != nil {
				content = append(content, Content{Kind: KindImage, Image: image})
			}
		case "tool_use":
			content = append(content, Content{Kind: KindToolUse, ToolUse: &ToolUse{
				Id: block.Id, Name: block.Name, Arguments: string(block.Input),
			}})
		case "tool_result":
			content = append(content, Content{Kind: KindToolResult, ToolResult: &ToolResult{
				Id: block.ToolUseId, Text: anthropicText(block.Content), IsError: block.IsError,
			}})
		}
	}
	return content
}

// anthropicText is the plain text of a system prompt or a tool result, both of
// which are either a string or blocks.
func anthropicText(raw json.RawMessage) string {
	parts := []string{}
	for _, content := range anthropicContentOf(raw) {
		if content.Kind == KindText {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "")
}

func anthropicImageOf(source *anthropicSource) *Image {
	if source == nil {
		return nil
	}
	if source.Url != "" {
		return &Image{Url: source.Url}
	}
	if source.Data == "" {
		return nil
	}
	return &Image{MediaType: source.MediaType, Data: source.Data}
}

func (anthropicCodec) EncodeRequest(request *Request) ([]byte, error) {
	body := anthropicRequest{
		Model:         request.Model,
		MaxTokens:     defaultMaxTokens,
		Messages:      []anthropicMessage{},
		Stream:        request.Stream,
		Temperature:   request.Temperature,
		TopP:          request.TopP,
		StopSequences: request.StopSequences,
	}
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		body.MaxTokens = *request.MaxTokens
	}
	if len(request.System) > 0 {
		body.System = rawString(strings.Join(request.System, "\n\n"))
	}
	if budget := request.Reasoning.BudgetOf(); budget > 0 {
		body.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: budget}
		// The budget is part of the answer, so the limit has to leave room for
		// it as well as for the answer itself.
		if body.MaxTokens <= budget {
			body.MaxTokens = budget + defaultMaxTokens
		}
	}

	for _, message := range request.Messages {
		blocks := anthropicBlocksOf(message)
		if len(blocks) == 0 {
			continue
		}
		// This API takes the two roles in turn, so what another format spread
		// over several messages is folded back into one.
		if last := len(body.Messages) - 1; last >= 0 && body.Messages[last].Role == message.Role {
			var previous []anthropicBlock
			if err := json.Unmarshal(body.Messages[last].Content, &previous); err == nil {
				body.Messages[last].Content = rawValue(append(previous, blocks...))
				continue
			}
		}
		body.Messages = append(body.Messages, anthropicMessage{
			Role: message.Role, Content: rawValue(blocks),
		})
	}
	if len(body.Messages) == 0 {
		return nil, errors.New("the request carries no message")
	}

	for _, tool := range request.Tools {
		schema := tool.Parameters
		if len(schema) == 0 {
			schema = rawValue(map[string]any{"type": "object", "properties": map[string]any{}})
		}
		body.Tools = append(body.Tools, anthropicTool{
			Name: tool.Name, Description: tool.Description, InputSchema: schema,
		})
	}
	if choice := request.ToolChoice; choice != nil {
		switch choice.Mode {
		case ChoiceTool:
			body.ToolChoice = rawValue(map[string]any{"type": "tool", "name": choice.Name})
		case ChoiceRequired:
			body.ToolChoice = rawValue(map[string]any{"type": "any"})
		case ChoiceAuto, ChoiceNone:
			body.ToolChoice = rawValue(map[string]any{"type": choice.Mode})
		}
	}
	// This API has no response_format: a schema the client asked for is added to
	// the system prompt instead, which is the only way to ask for it here.
	if format := request.Format; format != nil && len(format.Schema) > 0 {
		body.System = rawString(strings.TrimSpace(anthropicText(body.System) +
			"\n\nAnswer with JSON matching this schema, and with nothing else:\n" + string(format.Schema)))
	}
	return json.Marshal(body)
}

// anthropicBlocksOf writes one canonical message out as content blocks. A tool
// result is content of the next message in this format, which is where the
// canonical form keeps it too.
func anthropicBlocksOf(message Message) []anthropicBlock {
	blocks := []anthropicBlock{}
	for _, content := range message.Content {
		switch content.Kind {
		case KindText:
			if content.Text == "" {
				continue
			}
			blocks = append(blocks, anthropicBlock{Type: "text", Text: content.Text})
		case KindThinking:
			// A thinking block is only accepted back with the signature the
			// model signed it with, which another API never produced.
			if content.Signature == "" {
				continue
			}
			blocks = append(blocks, anthropicBlock{
				Type: "thinking", Thinking: content.Text, Signature: content.Signature,
			})
		case KindImage:
			if content.Image == nil {
				continue
			}
			blocks = append(blocks, anthropicBlock{Type: "image", Source: anthropicSourceOf(content.Image)})
		case KindToolUse:
			if content.ToolUse == nil {
				continue
			}
			blocks = append(blocks, anthropicBlock{
				Type: "tool_use", Id: content.ToolUse.Id, Name: content.ToolUse.Name,
				Input: json.RawMessage(emptyAsObject(content.ToolUse.Arguments)),
			})
		case KindToolResult:
			if content.ToolResult == nil {
				continue
			}
			blocks = append(blocks, anthropicBlock{
				Type: "tool_result", ToolUseId: content.ToolResult.Id,
				Content: rawString(content.ToolResult.Text), IsError: content.ToolResult.IsError,
			})
		}
	}
	return blocks
}

func anthropicSourceOf(image *Image) *anthropicSource {
	if image.Data != "" {
		return &anthropicSource{
			Type: "base64", MediaType: emptyAs(image.MediaType, "image/png"), Data: image.Data,
		}
	}
	return &anthropicSource{Type: "url", Url: image.Url}
}

// ---------------------------------------------------------------------------
// Response
// ---------------------------------------------------------------------------

func (anthropicCodec) DecodeResponse(raw []byte) (*Response, error) {
	var body anthropicResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("the upstream answered with an unreadable body")
	}

	response := &Response{
		Id:         body.Id,
		Model:      body.Model,
		StopReason: stopReasonOfAnthropic(body.StopReason),
		Content:    anthropicContentOf(rawValue(body.Content)),
	}
	if body.Usage != nil {
		response.Usage = body.Usage.canonical()
	}
	return response, nil
}

func (anthropicCodec) EncodeResponse(response *Response) ([]byte, error) {
	blocks := anthropicBlocksOf(Message{Role: RoleAssistant, Content: response.Content})
	// A thinking block without a signature is dropped on the way in, but it is
	// still worth showing to the client that asked for the thinking.
	for _, content := range response.Content {
		if content.Kind == KindThinking && content.Signature == "" && content.Text != "" {
			blocks = append([]anthropicBlock{{Type: "thinking", Thinking: content.Text}}, blocks...)
		}
	}

	return json.Marshal(anthropicResponse{
		Id:         emptyAs(response.Id, "msg_"+newToken()),
		Type:       "message",
		Role:       RoleAssistant,
		Model:      response.Model,
		Content:    blocks,
		StopReason: anthropicStopReason(response.StopReason),
		Usage:      anthropicUsageOf(response.Usage),
	})
}

func (anthropicCodec) DecodeStream(reader io.Reader, fn func(Event) bool) error {
	model := ""
	stop := ""
	usage := Usage{}
	running := true
	// tools maps a content block index onto the tool call being assembled in
	// it, so the pieces are numbered the way every other format numbers them.
	tools := map[int]int{}
	toolCount := 0

	err := forEachEvent(reader, func(data []byte) bool {
		var event anthropicEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return true
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				model = event.Message.Model
				if event.Message.Usage != nil {
					usage = event.Message.Usage.canonical()
				}
			}
		case "content_block_start":
			block := event.ContentBlock
			if block == nil {
				return true
			}
			if block.Type == "tool_use" {
				tools[event.Index] = toolCount
				running = fn(Event{Kind: EventToolUse, Model: model, Tool: &ToolDelta{
					Index: toolCount, Id: block.Id, Name: block.Name,
				}})
				toolCount++
				return running
			}
			if block.Text != "" {
				running = fn(Event{Kind: EventText, Text: block.Text, Model: model})
			}
		case "content_block_delta":
			delta := event.Delta
			if delta == nil {
				return true
			}
			switch delta.Type {
			case "text_delta":
				running = fn(Event{Kind: EventText, Text: delta.Text, Model: model})
			case "thinking_delta":
				running = fn(Event{Kind: EventThinking, Text: delta.Thinking, Model: model})
			case "signature_delta":
				running = fn(Event{Kind: EventThinking, Signature: delta.Signature, Model: model})
			case "input_json_delta":
				running = fn(Event{Kind: EventToolUse, Model: model, Tool: &ToolDelta{
					Index: tools[event.Index], Arguments: delta.PartialJson,
				}})
			}
		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				stop = stopReasonOfAnthropic(event.Delta.StopReason)
			}
			if event.Usage != nil {
				usage.merge(event.Usage.canonical())
			}
		case "error":
			if event.Error != nil {
				running = fn(Event{Kind: EventFailure, Failure: &Failure{
					Kind: emptyAs(event.Error.Type, "server_error"), Message: event.Error.Message,
				}})
			}
		}
		return running
	})

	if running {
		fn(Event{Kind: EventDone, StopReason: stop, Usage: &usage, Model: model})
	}
	return err
}

// ---------------------------------------------------------------------------
// Stream
// ---------------------------------------------------------------------------

// anthropicStreamWriter writes the events out. This format numbers the blocks
// of the answer and opens and closes each one, so the writer keeps track of the
// block the events are landing in.
type anthropicStreamWriter struct {
	events *eventWriter
	id     string
	model  string

	index int
	// open names the kind of block the events are landing in, empty when none
	// is open. openTool is the tool call it belongs to, for a tool_use block.
	open     string
	openTool int

	usage   Usage
	stop    string
	failure *Failure
}

func (anthropicCodec) NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter {
	return &anthropicStreamWriter{
		events: &eventWriter{writer: writer, flush: flush},
		id:     "msg_" + newToken(),
		model:  model,
	}
}

func (writer *anthropicStreamWriter) Open() {
	writer.events.send("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": writer.id, "type": "message", "role": RoleAssistant, "model": writer.model,
			"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	})
}

func (writer *anthropicStreamWriter) Write(event Event) {
	if event.Model != "" {
		writer.model = event.Model
	}

	switch event.Kind {
	case EventText:
		if event.Text == "" {
			return
		}
		writer.start(KindText, 0, map[string]any{"type": "text", "text": ""})
		writer.delta(map[string]any{"type": "text_delta", "text": event.Text})
	case EventThinking:
		writer.start(KindThinking, 0, map[string]any{"type": "thinking", "thinking": ""})
		if event.Signature != "" {
			writer.delta(map[string]any{"type": "signature_delta", "signature": event.Signature})
			return
		}
		writer.delta(map[string]any{"type": "thinking_delta", "thinking": event.Text})
	case EventToolUse:
		if event.Tool == nil {
			return
		}
		writer.start(KindToolUse, event.Tool.Index, map[string]any{
			"type": "tool_use", "id": emptyAs(event.Tool.Id, "toolu_"+newToken()),
			"name": event.Tool.Name, "input": map[string]any{},
		})
		if event.Tool.Arguments != "" {
			writer.delta(map[string]any{"type": "input_json_delta", "partial_json": event.Tool.Arguments})
		}
	case EventDone:
		writer.stop = event.StopReason
		if event.Usage != nil {
			writer.usage = *event.Usage
		}
	case EventFailure:
		writer.failure = event.Failure
	}
}

// start opens the block the next delta belongs in, closing the one before it
// when the answer has moved on.
func (writer *anthropicStreamWriter) start(kind string, tool int, block map[string]any) {
	if writer.open == kind && (kind != KindToolUse || writer.openTool == tool) {
		return
	}

	writer.stopBlock()
	writer.open = kind
	writer.openTool = tool
	writer.events.send("content_block_start", map[string]any{
		"type": "content_block_start", "index": writer.index, "content_block": block,
	})
}

func (writer *anthropicStreamWriter) delta(delta map[string]any) {
	writer.events.send("content_block_delta", map[string]any{
		"type": "content_block_delta", "index": writer.index, "delta": delta,
	})
}

func (writer *anthropicStreamWriter) stopBlock() {
	if writer.open == "" {
		return
	}
	writer.events.send("content_block_stop", map[string]any{
		"type": "content_block_stop", "index": writer.index,
	})
	writer.open = ""
	writer.index++
}

func (writer *anthropicStreamWriter) Close() {
	writer.stopBlock()

	if writer.failure != nil {
		writer.events.send("error", map[string]any{
			"type": "error",
			"error": map[string]any{
				"type": emptyAs(writer.failure.Kind, "api_error"), "message": writer.failure.Message,
			},
		})
		return
	}

	writer.events.send("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": anthropicStopReason(writer.stop), "stop_sequence": nil},
		"usage": anthropicUsageOf(writer.usage),
	})
	writer.events.send("message_stop", map[string]any{"type": "message_stop"})
}

func (anthropicCodec) EncodeError(kind string, message string) []byte {
	data, err := json.Marshal(map[string]any{
		"type": "error", "error": map[string]any{"type": kind, "message": message},
	})
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"api_error","message":"server error"}}`)
	}
	return data
}

// ---------------------------------------------------------------------------
// Shared pieces
// ---------------------------------------------------------------------------

// canonical lifts the counters onto the canonical form, where the cached part
// is counted inside the input total rather than beside it.
func (usage *anthropicUsage) canonical() Usage {
	return Usage{
		InputTokens:      usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens,
		OutputTokens:     usage.OutputTokens,
		CacheReadTokens:  usage.CacheReadInputTokens,
		CacheWriteTokens: usage.CacheCreationInputTokens,
	}
}

// merge keeps the higher of each counter: this API sends the input counts once
// and the output counts again at the end.
func (usage *Usage) merge(other Usage) {
	if other.InputTokens > usage.InputTokens {
		usage.InputTokens = other.InputTokens
	}
	if other.OutputTokens > usage.OutputTokens {
		usage.OutputTokens = other.OutputTokens
	}
	if other.CacheReadTokens > usage.CacheReadTokens {
		usage.CacheReadTokens = other.CacheReadTokens
	}
	if other.CacheWriteTokens > usage.CacheWriteTokens {
		usage.CacheWriteTokens = other.CacheWriteTokens
	}
	if other.ReasoningTokens > usage.ReasoningTokens {
		usage.ReasoningTokens = other.ReasoningTokens
	}
}

func anthropicUsageOf(usage Usage) *anthropicUsage {
	input := usage.InputTokens - usage.CacheReadTokens - usage.CacheWriteTokens
	if input < 0 {
		input = 0
	}
	return &anthropicUsage{
		InputTokens:              input,
		OutputTokens:             usage.OutputTokens,
		CacheReadInputTokens:     usage.CacheReadTokens,
		CacheCreationInputTokens: usage.CacheWriteTokens,
	}
}

func stopReasonOfAnthropic(reason string) string {
	switch reason {
	case "max_tokens":
		return StopMaxTokens
	case "tool_use", "pause_turn":
		return StopToolUse
	case "stop_sequence":
		return StopSequence
	case "refusal":
		return StopFilter
	case "":
		return ""
	}
	return StopEnd
}

func anthropicStopReason(stop string) string {
	switch stop {
	case StopMaxTokens:
		return "max_tokens"
	case StopToolUse:
		return "tool_use"
	case StopSequence:
		return "stop_sequence"
	case StopFilter:
		return "refusal"
	}
	return "end_turn"
}

func anthropicToolChoiceOf(raw json.RawMessage) *ToolChoice {
	if len(raw) == 0 {
		return nil
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil
	}
	switch choice.Type {
	case "tool":
		return &ToolChoice{Mode: ChoiceTool, Name: choice.Name}
	case "any":
		return &ToolChoice{Mode: ChoiceRequired}
	case ChoiceAuto, ChoiceNone:
		return &ToolChoice{Mode: choice.Type}
	}
	return nil
}
