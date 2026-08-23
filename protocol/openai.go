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

// This file speaks the OpenAI chat completions API, which is what nearly every
// provider reachable through the gateway serves.

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type openAiCodec struct{}

func init() {
	register(openAiCodec{})
}

func (openAiCodec) Name() string { return OpenAi }

func (openAiCodec) Endpoint() string { return "/chat/completions" }

// ---------------------------------------------------------------------------
// The wire types
// ---------------------------------------------------------------------------

type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	Stream              bool            `json:"stream,omitempty"`
	StreamOptions       *chatStreamOpts `json:"stream_options,omitempty"`
	Tools               []chatTool      `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
}

type chatStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content,omitempty"`
	// ReasoningContent is what the vendors serving a thinking model add beside
	// the answer. It is not part of the API, but they all spell it alike.
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallId       string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Index    *int             `json:"index,omitempty"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatTool struct {
	Type     string      `json:"type"`
	Function chatToolDef `json:"function"`
}

type chatToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageUrl *struct {
		Url string `json:"url"`
	} `json:"image_url"`
}

type cachedTokens struct {
	CachedTokens int `json:"cached_tokens"`
}

type reasoningTokens struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type chatUsage struct {
	PromptTokens            int              `json:"prompt_tokens"`
	CompletionTokens        int              `json:"completion_tokens"`
	TotalTokens             int              `json:"total_tokens"`
	PromptTokensDetails     *cachedTokens    `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *reasoningTokens `json:"completion_tokens_details,omitempty"`
}

type chatCompletion struct {
	Id      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatChunk struct {
	Id      string      `json:"id"`
	Model   string      `json:"model"`
	Choices []chatDelta `json:"choices"`
	Usage   *chatUsage  `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type chatDelta struct {
	Delta struct {
		Content          json.RawMessage `json:"content"`
		ReasoningContent string          `json:"reasoning_content"`
		ToolCalls        []chatToolCall  `json:"tool_calls"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

func (openAiCodec) DecodeRequest(raw []byte) (*Request, error) {
	var body chatRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("invalid request body")
	}

	request := &Request{
		Model:             body.Model,
		Stream:            body.Stream,
		ToolChoice:        chatToolChoiceOf(body.ToolChoice),
		ParallelToolCalls: body.ParallelToolCalls,
		Temperature:       body.Temperature,
		TopP:              body.TopP,
		MaxTokens:         firstInt(body.MaxCompletionTokens, body.MaxTokens),
		StopSequences:     stringsOf(body.Stop),
		Format:            chatFormatOf(body.ResponseFormat),
	}
	if body.ReasoningEffort != "" {
		request.Reasoning = &Reasoning{Effort: body.ReasoningEffort}
	}
	for _, tool := range body.Tools {
		if tool.Function.Name == "" {
			continue
		}
		request.Tools = append(request.Tools, Tool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		})
	}

	for _, message := range body.Messages {
		switch message.Role {
		case "system", "developer":
			request.System = append(request.System, chatText(message.Content))
		case "tool":
			request.Messages = append(request.Messages, Message{
				Role: RoleUser,
				Content: []Content{{Kind: KindToolResult, ToolResult: &ToolResult{
					Id: message.ToolCallId, Text: chatText(message.Content),
				}}},
			})
		default:
			content := chatContentOf(message)
			if len(content) == 0 {
				continue
			}
			role := RoleUser
			if message.Role == RoleAssistant {
				role = RoleAssistant
			}
			request.Messages = append(request.Messages, Message{Role: role, Content: content})
		}
	}
	return request, nil
}

// chatContentOf reads the blocks of one user or assistant message.
func chatContentOf(message chatMessage) []Content {
	content := []Content{}
	if message.ReasoningContent != "" {
		content = append(content, Content{Kind: KindThinking, Text: message.ReasoningContent})
	}
	content = append(content, chatBlocks(message.Content)...)
	for _, call := range message.ToolCalls {
		content = append(content, Content{Kind: KindToolUse, ToolUse: &ToolUse{
			Id: call.Id, Name: call.Function.Name, Arguments: call.Function.Arguments,
		}})
	}
	return content
}

// chatBlocks reads message content, which is either a plain string or the parts
// a message with a picture in it is made of.
func chatBlocks(raw json.RawMessage) []Content {
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

	var parts []chatPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}

	content := []Content{}
	for _, part := range parts {
		switch {
		case part.ImageUrl != nil && part.ImageUrl.Url != "":
			content = append(content, Content{Kind: KindImage, Image: imageOfUrl(part.ImageUrl.Url)})
		case part.Text != "":
			content = append(content, Content{Kind: KindText, Text: part.Text})
		}
	}
	return content
}

// chatText is the plain text of message content, for the places that take
// nothing else.
func chatText(raw json.RawMessage) string {
	parts := []string{}
	for _, content := range chatBlocks(raw) {
		if content.Kind == KindText {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "")
}

func (openAiCodec) EncodeRequest(request *Request) ([]byte, error) {
	body := chatRequest{
		Model:             request.Model,
		Messages:          []chatMessage{},
		Stream:            request.Stream,
		ParallelToolCalls: request.ParallelToolCalls,
		Temperature:       request.Temperature,
		TopP:              request.TopP,
		MaxTokens:         request.MaxTokens,
		ReasoningEffort:   request.Reasoning.EffortOf(),
	}
	if request.Stream {
		// The token counts only reach a streaming client when asked for.
		body.StreamOptions = &chatStreamOpts{IncludeUsage: true}
	}
	if len(request.System) > 0 {
		body.Messages = append(body.Messages, chatMessage{
			Role: "system", Content: rawString(strings.Join(request.System, "\n\n")),
		})
	}
	for _, message := range request.Messages {
		body.Messages = append(body.Messages, chatMessagesOf(message)...)
	}
	if len(body.Messages) == 0 {
		return nil, errors.New("the request carries no message")
	}

	for _, tool := range request.Tools {
		body.Tools = append(body.Tools, chatTool{
			Type: "function",
			Function: chatToolDef{
				Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
			},
		})
	}
	if choice := request.ToolChoice; choice != nil {
		switch choice.Mode {
		case ChoiceTool:
			body.ToolChoice = rawValue(map[string]any{
				"type": "function", "function": map[string]any{"name": choice.Name},
			})
		case ChoiceAuto, ChoiceNone, ChoiceRequired:
			body.ToolChoice = rawString(choice.Mode)
		}
	}
	if len(request.StopSequences) > 0 {
		body.Stop = rawValue(request.StopSequences)
	}
	if format := request.Format; format != nil {
		if format.Kind == "json_schema" {
			schema := map[string]any{"name": emptyAs(format.Name, "response"), "schema": format.Schema}
			if format.Strict != nil {
				schema["strict"] = *format.Strict
			}
			body.ResponseFormat = rawValue(map[string]any{"type": "json_schema", "json_schema": schema})
		} else if format.Kind != "" {
			body.ResponseFormat = rawValue(map[string]any{"type": format.Kind})
		}
	}
	return json.Marshal(body)
}

// chatMessagesOf writes one canonical message out. A tool result has a message
// of its own in this format rather than being content of the next one, so a
// message carrying results turns into several.
func chatMessagesOf(message Message) []chatMessage {
	messages := []chatMessage{}
	parts := []any{}
	text := []string{}
	hasImage := false
	calls := []chatToolCall{}

	for _, content := range message.Content {
		switch content.Kind {
		case KindText:
			text = append(text, content.Text)
			parts = append(parts, map[string]any{"type": "text", "text": content.Text})
		case KindImage:
			if content.Image == nil {
				continue
			}
			hasImage = true
			parts = append(parts, map[string]any{
				"type": "image_url", "image_url": map[string]any{"url": content.Image.Link()},
			})
		case KindToolUse:
			if content.ToolUse == nil {
				continue
			}
			calls = append(calls, chatToolCall{
				Id:   content.ToolUse.Id,
				Type: "function",
				Function: chatToolFunction{
					Name: content.ToolUse.Name, Arguments: emptyAsObject(content.ToolUse.Arguments),
				},
			})
		case KindToolResult:
			if content.ToolResult == nil {
				continue
			}
			messages = append(messages, chatMessage{
				Role:       "tool",
				ToolCallId: content.ToolResult.Id,
				Content:    rawString(content.ToolResult.Text),
			})
		}
		// A thinking block has no chat completions form, and replaying it would
		// only confuse an upstream that never produced it.
	}

	if len(text) == 0 && !hasImage && len(calls) == 0 {
		return messages
	}

	written := chatMessage{Role: message.Role, ToolCalls: calls}
	if hasImage {
		written.Content = rawValue(parts)
	} else if len(text) > 0 {
		written.Content = rawString(strings.Join(text, ""))
	}
	return append(messages, written)
}

// ---------------------------------------------------------------------------
// Response
// ---------------------------------------------------------------------------

func (openAiCodec) DecodeResponse(raw []byte) (*Response, error) {
	var body chatCompletion
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("the upstream answered with an unreadable body")
	}

	response := &Response{Id: body.Id, Model: body.Model, StopReason: StopEnd}
	if body.Usage != nil {
		response.Usage = body.Usage.canonical()
	}
	if len(body.Choices) > 0 {
		choice := body.Choices[0]
		response.Content = chatContentOf(choice.Message)
		if stop := stopReasonOfFinish(choice.FinishReason); stop != "" {
			response.StopReason = stop
		}
	}
	return response, nil
}

func (openAiCodec) EncodeResponse(response *Response) ([]byte, error) {
	message := chatMessage{Role: RoleAssistant, Content: rawString(response.Text())}
	for _, use := range response.ToolUses() {
		message.ToolCalls = append(message.ToolCalls, chatToolCall{
			Id:       use.Id,
			Type:     "function",
			Function: chatToolFunction{Name: use.Name, Arguments: emptyAsObject(use.Arguments)},
		})
	}
	for _, content := range response.Content {
		if content.Kind == KindThinking {
			message.ReasoningContent += content.Text
		}
	}

	return json.Marshal(chatCompletion{
		Id:      emptyAs(response.Id, "chatcmpl-"+newToken()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   response.Model,
		Choices: []chatChoice{{
			Index: 0, Message: message, FinishReason: finishReasonOfStop(response.StopReason),
		}},
		Usage: usageOfCanonical(response.Usage),
	})
}

func (openAiCodec) DecodeStream(reader io.Reader, fn func(Event) bool) error {
	stop := ""
	usage := Usage{}
	running := true

	err := forEachEvent(reader, func(data []byte) bool {
		if strings.TrimSpace(string(data)) == "[DONE]" {
			return true
		}

		var chunk chatChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return true
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			running = fn(Event{Kind: EventFailure, Failure: &Failure{
				Kind: emptyAs(chunk.Error.Type, "server_error"), Message: chunk.Error.Message,
			}})
			return running
		}
		if chunk.Usage != nil {
			usage = chunk.Usage.canonical()
		}

		for _, choice := range chunk.Choices {
			if delta := choice.Delta.ReasoningContent; delta != "" {
				if running = fn(Event{Kind: EventThinking, Text: delta, Model: chunk.Model}); !running {
					return false
				}
			}
			if delta := chatText(choice.Delta.Content); delta != "" {
				if running = fn(Event{Kind: EventText, Text: delta, Model: chunk.Model}); !running {
					return false
				}
			}
			for index, call := range choice.Delta.ToolCalls {
				if call.Index != nil {
					index = *call.Index
				}
				running = fn(Event{Kind: EventToolUse, Model: chunk.Model, Tool: &ToolDelta{
					Index: index, Id: call.Id, Name: call.Function.Name, Arguments: call.Function.Arguments,
				}})
				if !running {
					return false
				}
			}
			if choice.FinishReason != "" {
				stop = stopReasonOfFinish(choice.FinishReason)
			}
		}
		return true
	})

	if running {
		fn(Event{Kind: EventDone, StopReason: stop, Usage: &usage})
	}
	return err
}

// ---------------------------------------------------------------------------
// Stream
// ---------------------------------------------------------------------------

type chatStreamWriter struct {
	events  *eventWriter
	id      string
	model   string
	created int64
	stop    string
	usage   Usage
	failure *Failure
}

func (openAiCodec) NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter {
	return &chatStreamWriter{
		events:  &eventWriter{writer: writer, flush: flush},
		id:      "chatcmpl-" + newToken(),
		model:   model,
		created: time.Now().Unix(),
	}
}

func (writer *chatStreamWriter) chunk(delta map[string]any, finish any) {
	choice := map[string]any{"index": 0, "delta": delta, "finish_reason": finish}
	writer.events.send("", map[string]any{
		"id": writer.id, "object": "chat.completion.chunk", "created": writer.created,
		"model": writer.model, "choices": []any{choice},
	})
}

func (writer *chatStreamWriter) Open() {
	writer.chunk(map[string]any{"role": RoleAssistant, "content": ""}, nil)
}

func (writer *chatStreamWriter) Write(event Event) {
	if event.Model != "" {
		writer.model = event.Model
	}

	switch event.Kind {
	case EventText:
		writer.chunk(map[string]any{"content": event.Text}, nil)
	case EventThinking:
		writer.chunk(map[string]any{"reasoning_content": event.Text}, nil)
	case EventToolUse:
		if event.Tool == nil {
			return
		}
		function := map[string]any{}
		if event.Tool.Name != "" {
			function["name"] = event.Tool.Name
		}
		if event.Tool.Arguments != "" {
			function["arguments"] = event.Tool.Arguments
		}
		call := map[string]any{"index": event.Tool.Index, "type": "function", "function": function}
		if event.Tool.Id != "" {
			call["id"] = event.Tool.Id
		}
		writer.chunk(map[string]any{"tool_calls": []any{call}}, nil)
	case EventDone:
		writer.stop = event.StopReason
		if event.Usage != nil {
			writer.usage = *event.Usage
		}
	case EventFailure:
		writer.failure = event.Failure
	}
}

func (writer *chatStreamWriter) Close() {
	if writer.failure != nil {
		// This format has no error event: an upstream failing mid-answer is
		// reported in a chunk of its own, which is what the clients read.
		writer.events.send("", map[string]any{
			"id": writer.id, "object": "chat.completion.chunk", "created": writer.created,
			"model": writer.model, "choices": []any{},
			"error": map[string]any{
				"message": writer.failure.Message, "type": emptyAs(writer.failure.Kind, "server_error"),
			},
		})
	}

	writer.chunk(map[string]any{}, finishReasonOfStop(writer.stop))
	if !writer.usage.IsZero() {
		writer.events.send("", map[string]any{
			"id": writer.id, "object": "chat.completion.chunk", "created": writer.created,
			"model": writer.model, "choices": []any{}, "usage": usageOfCanonical(writer.usage),
		})
	}
	writer.events.raw("data: [DONE]\n\n")
}

func (openAiCodec) EncodeError(kind string, message string) []byte {
	data, err := json.Marshal(map[string]any{
		"error": map[string]any{"message": message, "type": kind},
	})
	if err != nil {
		return []byte(`{"error":{"message":"server error","type":"server_error"}}`)
	}
	return data
}

// ---------------------------------------------------------------------------
// Shared pieces
// ---------------------------------------------------------------------------

func (usage *chatUsage) canonical() Usage {
	canonical := Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}
	if usage.PromptTokensDetails != nil {
		canonical.CacheReadTokens = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokensDetails != nil {
		canonical.ReasoningTokens = usage.CompletionTokensDetails.ReasoningTokens
	}
	return canonical
}

// usageOfCanonical writes the counters back out. This format counts the cached
// part inside the prompt total, which is where the canonical form keeps it too.
func usageOfCanonical(usage Usage) *chatUsage {
	if usage.IsZero() {
		return nil
	}

	written := &chatUsage{
		PromptTokens:     usage.InputTokens + usage.CacheWriteTokens,
		CompletionTokens: usage.OutputTokens,
	}
	written.TotalTokens = written.PromptTokens + written.CompletionTokens
	if usage.CacheReadTokens > 0 {
		written.PromptTokensDetails = &cachedTokens{CachedTokens: usage.CacheReadTokens}
	}
	if usage.ReasoningTokens > 0 {
		written.CompletionTokensDetails = &reasoningTokens{ReasoningTokens: usage.ReasoningTokens}
	}
	return written
}

func stopReasonOfFinish(finish string) string {
	switch finish {
	case "length":
		return StopMaxTokens
	case "tool_calls", "function_call":
		return StopToolUse
	case "content_filter":
		return StopFilter
	case "":
		return ""
	}
	return StopEnd
}

func finishReasonOfStop(stop string) string {
	switch stop {
	case StopMaxTokens:
		return "length"
	case StopToolUse:
		return "tool_calls"
	case StopFilter:
		return "content_filter"
	}
	return "stop"
}

func chatToolChoiceOf(raw json.RawMessage) *ToolChoice {
	if len(raw) == 0 {
		return nil
	}

	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case ChoiceAuto, ChoiceNone, ChoiceRequired:
			return &ToolChoice{Mode: mode}
		}
		return nil
	}

	var choice struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil
	}
	if name := emptyAs(choice.Function.Name, choice.Name); name != "" {
		return &ToolChoice{Mode: ChoiceTool, Name: name}
	}
	return nil
}

func chatFormatOf(raw json.RawMessage) *Format {
	if len(raw) == 0 {
		return nil
	}

	var format struct {
		Type   string `json:"type"`
		Schema struct {
			Name   string          `json:"name"`
			Schema json.RawMessage `json:"schema"`
			Strict *bool           `json:"strict"`
		} `json:"json_schema"`
	}
	if err := json.Unmarshal(raw, &format); err != nil || format.Type == "" {
		return nil
	}
	return &Format{
		Kind:   format.Type,
		Name:   format.Schema.Name,
		Schema: format.Schema.Schema,
		Strict: format.Schema.Strict,
	}
}

// Link is the picture as one URL, which is how this format carries it: bytes
// become a data URI.
func (image *Image) Link() string {
	if image.Url != "" {
		return image.Url
	}
	if image.Data == "" {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", emptyAs(image.MediaType, "image/png"), image.Data)
}

// imageOfUrl reads a picture sent as one URL, splitting a data URI back into
// the media type and the bytes an API that takes them apart needs.
func imageOfUrl(url string) *Image {
	if !strings.HasPrefix(url, "data:") {
		return &Image{Url: url}
	}

	head, data, found := strings.Cut(strings.TrimPrefix(url, "data:"), ",")
	if !found {
		return &Image{Url: url}
	}
	return &Image{MediaType: strings.TrimSuffix(head, ";base64"), Data: data}
}

func rawString(value string) json.RawMessage {
	return rawValue(value)
}

func rawValue(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}

// stringsOf reads a field that is either one string or a list of them.
func stringsOf(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		if one == "" {
			return nil
		}
		return []string{one}
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil
	}
	return many
}

func firstInt(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func emptyAs(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func emptyAsObject(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "{}"
	}
	return arguments
}

// newToken is what the ids the gateway makes up are told apart by, for the
// answers it writes itself rather than relays.
func newToken() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
