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

// This file translates between the OpenAI Responses API, the only wire format
// recent Codex versions speak, and the chat completions API, which is what
// nearly every provider reachable through the gateway serves.

package controllers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/beego/beego"
)

// ---------------------------------------------------------------------------
// Request: Responses -> chat completions
// ---------------------------------------------------------------------------

// responsesRequest is the part of a Responses body that has a chat completions
// counterpart. The rest (store, include, reasoning, previous_response_id) is
// dropped: an upstream serving chat completions has nothing to do with it.
type responsesRequest struct {
	Model             string          `json:"model"`
	Stream            bool            `json:"stream"`
	Instructions      string          `json:"instructions"`
	Input             json.RawMessage `json:"input"`
	Tools             []responsesTool `json:"tools"`
	ToolChoice        json.RawMessage `json:"tool_choice"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls"`
	Temperature       *float64        `json:"temperature"`
	TopP              *float64        `json:"top_p"`
	MaxOutputTokens   *int            `json:"max_output_tokens"`
	Text              *struct {
		Format json.RawMessage `json:"format"`
	} `json:"text"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// responsesItem is one element of the input array. Its fields are a union over
// the item types, which is how the API itself is shaped.
type responsesItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	CallId    string          `json:"call_id"`
	Output    json.RawMessage `json:"output"`
}

type responsesPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageUrl string `json:"image_url"`
}

type chatRequest struct {
	Model             string          `json:"model"`
	Messages          []chatMessage   `json:"messages"`
	Stream            bool            `json:"stream,omitempty"`
	StreamOptions     *chatStreamOpts `json:"stream_options,omitempty"`
	Tools             []chatTool      `json:"tools,omitempty"`
	ToolChoice        any             `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64        `json:"temperature,omitempty"`
	TopP              *float64        `json:"top_p,omitempty"`
	MaxTokens         *int            `json:"max_tokens,omitempty"`
	ResponseFormat    any             `json:"response_format,omitempty"`
}

type chatStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallId string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	Id       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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

// responsesToChat rewrites a Responses request as the chat completions request
// that asks the upstream for the same thing.
func responsesToChat(raw []byte) ([]byte, error) {
	var request responsesRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, errors.New("invalid request body")
	}

	items, err := responsesInputItems(request.Input)
	if err != nil {
		return nil, errors.New("invalid input")
	}

	messages := []chatMessage{}
	if request.Instructions != "" {
		messages = append(messages, chatMessage{Role: "system", Content: request.Instructions})
	}
	for _, item := range items {
		messages = appendResponsesItem(messages, item)
	}
	if len(messages) == 0 {
		return nil, errors.New("input is required")
	}

	chat := chatRequest{
		Model:             request.Model,
		Messages:          messages,
		Stream:            request.Stream,
		ToolChoice:        responsesToolChoice(request.ToolChoice),
		ParallelToolCalls: request.ParallelToolCalls,
		Temperature:       request.Temperature,
		TopP:              request.TopP,
		MaxTokens:         request.MaxOutputTokens,
	}
	if request.Stream {
		// The token counts only reach a streaming client when asked for.
		chat.StreamOptions = &chatStreamOpts{IncludeUsage: true}
	}
	for _, tool := range request.Tools {
		// Only plain function tools cross over: the hosted ones (web_search,
		// local_shell) have no chat completions form to send.
		if tool.Type != "function" || tool.Name == "" {
			continue
		}
		chat.Tools = append(chat.Tools, chatTool{
			Type:     "function",
			Function: chatToolDef{Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters},
		})
	}
	if request.Text != nil {
		chat.ResponseFormat = responsesFormat(request.Text.Format)
	}
	return json.Marshal(chat)
}

// responsesInputItems reads the input field, which is either a bare prompt or
// the conversation so far.
func responsesInputItems(raw json.RawMessage) ([]responsesItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var prompt string
	if err := json.Unmarshal(raw, &prompt); err == nil {
		content, _ := json.Marshal(prompt)
		return []responsesItem{{Type: "message", Role: "user", Content: content}}, nil
	}

	var items []responsesItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func appendResponsesItem(messages []chatMessage, item responsesItem) []chatMessage {
	switch {
	case item.Type == "function_call":
		call := chatToolCall{
			Id:       item.CallId,
			Type:     "function",
			Function: chatToolFunction{Name: item.Name, Arguments: emptyAsObject(item.Arguments)},
		}
		// Calls the model made in one turn belong to one assistant message,
		// which is how they were sent to the client in the first place.
		if last := len(messages) - 1; last >= 0 &&
			messages[last].Role == "assistant" && messages[last].Content == nil {
			messages[last].ToolCalls = append(messages[last].ToolCalls, call)
			return messages
		}
		return append(messages, chatMessage{Role: "assistant", ToolCalls: []chatToolCall{call}})

	case item.Type == "function_call_output":
		return append(messages, chatMessage{
			Role:       "tool",
			ToolCallId: item.CallId,
			Content:    responsesOutputText(item.Output),
		})

	case item.Type == "message" || (item.Type == "" && item.Role != ""):
		role := item.Role
		if role == "developer" {
			role = "system"
		}
		return append(messages, chatMessage{Role: role, Content: responsesContent(item.Content)})
	}

	// Reasoning and the hosted tool calls have no chat completions form, and
	// replaying them would only confuse the upstream.
	return messages
}

// responsesContent flattens message content. Text stays a plain string, which
// every OpenAI-compatible upstream accepts; an image forces the array form.
func responsesContent(raw json.RawMessage) any {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var parts []responsesPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}

	chatParts := []any{}
	texts := []string{}
	hasImage := false
	for _, part := range parts {
		if part.Type == "input_image" || part.Type == "image_url" {
			if part.ImageUrl == "" {
				continue
			}
			hasImage = true
			chatParts = append(chatParts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": part.ImageUrl},
			})
			continue
		}
		if part.Text != "" {
			texts = append(texts, part.Text)
			chatParts = append(chatParts, map[string]any{"type": "text", "text": part.Text})
		}
	}
	if hasImage {
		return chatParts
	}
	return strings.Join(texts, "")
}

// responsesOutputText is the text of a function_call_output, which clients send
// either as a string or as the object their tool runner produced.
func responsesOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var object struct {
		Content json.RawMessage `json:"content"`
		Output  string          `json:"output"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		if value, ok := responsesContent(object.Content).(string); ok && value != "" {
			return value
		}
		if object.Output != "" {
			return object.Output
		}
	}
	return string(raw)
}

func responsesToolChoice(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}

	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return name
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err == nil && choice.Type == "function" && choice.Name != "" {
		return map[string]any{"type": "function", "function": map[string]any{"name": choice.Name}}
	}
	return nil
}

// responsesFormat maps text.format onto response_format: the same schema, one
// level deeper.
func responsesFormat(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}

	var format struct {
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
		Strict *bool           `json:"strict"`
	}
	if err := json.Unmarshal(raw, &format); err != nil {
		return nil
	}
	switch format.Type {
	case "json_object":
		return map[string]any{"type": "json_object"}
	case "json_schema":
		schema := map[string]any{"name": emptyAsValue(format.Name, "response"), "schema": format.Schema}
		if format.Strict != nil {
			schema["strict"] = *format.Strict
		}
		return map[string]any{"type": "json_schema", "json_schema": schema}
	}
	return nil
}

func emptyAsObject(arguments string) string {
	if strings.TrimSpace(arguments) == "" {
		return "{}"
	}
	return arguments
}

func emptyAsValue(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// ---------------------------------------------------------------------------
// Response: chat completions -> Responses
// ---------------------------------------------------------------------------

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatDeltaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatDeltaToolCall struct {
	Index    int               `json:"index"`
	Id       string            `json:"id"`
	Function chatDeltaFunction `json:"function"`
}

type chatStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          string              `json:"content"`
			ReasoningContent string              `json:"reasoning_content"`
			ToolCalls        []chatDeltaToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type chatCompletion struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *chatUsage `json:"usage"`
}

// responsesCall is one function call being assembled out of the deltas the
// upstream sends it in.
type responsesCall struct {
	itemId      string
	callId      string
	name        string
	arguments   strings.Builder
	outputIndex int
}

// responsesStream turns a chat completions response into the Responses events a
// client such as Codex expects.
type responsesStream struct {
	writer io.Writer
	flush  func()
	// token makes the item ids of this response unique without a counter of
	// their own: they are the response id plus what the item is.
	token    string
	id       string
	model    string
	created  int64
	sequence int
	// broken records that the client stopped reading, which is the end of the
	// response whatever the upstream still has to say.
	broken bool

	nextOutputIndex int
	messageId       string
	messageIndex    int
	messageStarted  bool
	text            strings.Builder

	reasoningId string

	calls     map[int]*responsesCall
	callOrder []int

	usage   *chatUsage
	failure string
}

func newResponsesStream(writer io.Writer, flush func(), model string) *responsesStream {
	now := time.Now()
	token := fmt.Sprintf("%d", now.UnixNano())
	return &responsesStream{
		writer:  writer,
		flush:   flush,
		token:   token,
		id:      "resp_" + token,
		model:   model,
		created: now.Unix(),
		calls:   map[int]*responsesCall{},
	}
}

func (s *responsesStream) emit(name string, payload map[string]any) {
	payload["type"] = name
	payload["sequence_number"] = s.sequence
	s.sequence++

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return
	}
	if s.flush != nil {
		s.flush()
	}
}

func (s *responsesStream) emitCreated() {
	s.emit("response.created", map[string]any{"response": s.response("in_progress", []any{})})
}

// appendText records one piece of the answer and sends it on.
func (s *responsesStream) appendText(delta string) {
	s.text.WriteString(delta)
	s.emit("response.output_text.delta", map[string]any{
		"item_id":       s.messageId,
		"output_index":  s.messageIndex,
		"content_index": 0,
		"delta":         delta,
	})
}

func (s *responsesStream) consume(data []byte) {
	var chunk chatStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return
	}
	if chunk.Error != nil && chunk.Error.Message != "" {
		s.failure = chunk.Error.Message
		return
	}
	if chunk.Model != "" {
		s.model = chunk.Model
	}
	if chunk.Usage != nil {
		s.usage = chunk.Usage
	}

	for _, choice := range chunk.Choices {
		if delta := choice.Delta.ReasoningContent; delta != "" {
			s.emit("response.reasoning_summary_text.delta", map[string]any{
				"item_id":       s.reasoningItemId(),
				"output_index":  0,
				"summary_index": 0,
				"delta":         delta,
			})
		}
		if delta := choice.Delta.Content; delta != "" {
			s.startMessage()
			s.appendText(delta)
		}
		for _, call := range choice.Delta.ToolCalls {
			s.accumulateCall(call)
		}
	}
}

func (s *responsesStream) reasoningItemId() string {
	if s.reasoningId == "" {
		s.reasoningId = "rs_" + s.token
	}
	return s.reasoningId
}

func (s *responsesStream) startMessage() {
	if s.messageStarted {
		return
	}
	s.messageStarted = true
	s.messageId = fmt.Sprintf("msg_%s_%d", s.token, s.nextOutputIndex)
	s.messageIndex = s.nextOutputIndex
	s.nextOutputIndex++
	s.emit("response.output_item.added", map[string]any{
		"output_index": s.messageIndex,
		"item": map[string]any{
			"type": "message", "id": s.messageId, "status": "in_progress",
			"role": "assistant", "content": []any{},
		},
	})
}

func (s *responsesStream) accumulateCall(delta chatDeltaToolCall) {
	call, ok := s.calls[delta.Index]
	if !ok {
		call = &responsesCall{
			itemId:      fmt.Sprintf("fc_%s_%d", s.token, s.nextOutputIndex),
			outputIndex: s.nextOutputIndex,
		}
		s.nextOutputIndex++
		s.calls[delta.Index] = call
		s.callOrder = append(s.callOrder, delta.Index)
	}
	if delta.Id != "" {
		call.callId = delta.Id
	}
	if delta.Function.Name != "" {
		call.name += delta.Function.Name
	}
	if delta.Function.Arguments != "" {
		call.arguments.WriteString(delta.Function.Arguments)
		s.emit("response.function_call_arguments.delta", map[string]any{
			"item_id":      call.itemId,
			"output_index": call.outputIndex,
			"delta":        delta.Function.Arguments,
		})
	}
}

// output is the item list a finished response carries, in the order the items
// were started.
func (s *responsesStream) output() []any {
	slots := make([]any, s.nextOutputIndex)
	if s.messageStarted {
		slots[s.messageIndex] = s.messageItem()
	}
	for _, index := range s.callOrder {
		call := s.calls[index]
		slots[call.outputIndex] = callItem(call)
	}

	items := []any{}
	for _, item := range slots {
		if item != nil {
			items = append(items, item)
		}
	}
	return items
}

func (s *responsesStream) messageItem() map[string]any {
	return map[string]any{
		"type": "message", "id": s.messageId, "status": "completed", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": s.text.String(), "annotations": []any{}}},
	}
}

func callItem(call *responsesCall) map[string]any {
	callId := call.callId
	if callId == "" {
		callId = call.itemId
	}
	return map[string]any{
		"type": "function_call", "id": call.itemId, "status": "completed",
		"call_id": callId, "name": call.name, "arguments": emptyAsObject(call.arguments.String()),
	}
}

func (s *responsesStream) response(status string, output []any) map[string]any {
	response := map[string]any{
		"id": s.id, "object": "response", "created_at": s.created,
		"status": status, "model": s.model, "output": output,
	}
	if s.usage != nil {
		response["usage"] = map[string]any{
			"input_tokens":  s.usage.PromptTokens,
			"output_tokens": s.usage.CompletionTokens,
			"total_tokens":  s.usage.TotalTokens,
		}
	}
	return response
}

func (s *responsesStream) finish() {
	if s.messageStarted {
		s.emit("response.output_text.done", map[string]any{
			"item_id": s.messageId, "output_index": s.messageIndex,
			"content_index": 0, "text": s.text.String(),
		})
		s.emit("response.output_item.done", map[string]any{
			"output_index": s.messageIndex, "item": s.messageItem(),
		})
	}
	for _, index := range s.callOrder {
		call := s.calls[index]
		s.emit("response.function_call_arguments.done", map[string]any{
			"item_id": call.itemId, "output_index": call.outputIndex,
			"arguments": emptyAsObject(call.arguments.String()),
		})
		s.emit("response.output_item.done", map[string]any{
			"output_index": call.outputIndex, "item": callItem(call),
		})
	}

	if s.failure != "" {
		response := s.response("failed", s.output())
		response["error"] = map[string]any{"code": "server_error", "message": s.failure}
		s.emit("response.failed", map[string]any{"response": response})
		return
	}
	s.emit("response.completed", map[string]any{"response": s.response("completed", s.output())})
}

// relayResponsesStream rewrites the upstream SSE stream as Responses events.
func (c *ApiController) relayResponsesStream(route *proxyRoute, body io.Reader) {
	stream := newResponsesStream(c.startEventStream(), c.Ctx.ResponseWriter.Flush, route.model)
	stream.emitCreated()

	err := forEachSseData(body, func(data []byte) bool {
		if !bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			stream.consume(data)
		}
		return !stream.broken
	})
	if err != nil && stream.failure == "" {
		stream.failure = err.Error()
	}
	stream.finish()
}

// relayResponsesBody rewrites a whole chat completion as a Responses answer. A
// client that asked for a stream still gets one, of a single turn: the upstream
// answering in one piece is no reason to hand it a body it cannot read.
func (c *ApiController) relayResponsesBody(route *proxyRoute, body io.Reader) {
	data, err := io.ReadAll(body)
	if err != nil {
		c.writeProxyError(route.target.protocol, http.StatusBadGateway, "server_error", "upstream read failed")
		return
	}

	var completion chatCompletion
	if err := json.Unmarshal(data, &completion); err != nil {
		c.writeProxyError(route.target.protocol, http.StatusBadGateway, "server_error", "upstream returned an unreadable body")
		return
	}

	stream := newResponsesStream(io.Discard, nil, route.model)
	if route.stream {
		stream.writer = c.startEventStream()
		stream.flush = c.Ctx.ResponseWriter.Flush
		stream.emitCreated()
	}

	if completion.Model != "" {
		stream.model = completion.Model
	}
	stream.usage = completion.Usage
	if len(completion.Choices) > 0 {
		message := completion.Choices[0].Message
		if message.Content != "" {
			stream.startMessage()
			stream.appendText(message.Content)
		}
		for index, call := range message.ToolCalls {
			stream.accumulateCall(chatDeltaToolCall{
				Index:    index,
				Id:       call.Id,
				Function: chatDeltaFunction{Name: call.Function.Name, Arguments: call.Function.Arguments},
			})
		}
	}

	if route.stream {
		stream.finish()
		return
	}

	response, err := json.Marshal(stream.response("completed", stream.output()))
	if err != nil {
		c.writeProxyError(route.target.protocol, http.StatusBadGateway, "server_error", "upstream returned an unreadable body")
		return
	}
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(http.StatusOK)
	if _, err := c.Ctx.ResponseWriter.Write(response); err != nil {
		beego.Error("responses body write failed:", err)
	}
}

// startEventStream begins an SSE response of this gateway's own making, which
// carries none of the upstream headers: the body relayed under them is gone.
func (c *ApiController) startEventStream() io.Writer {
	header := c.Ctx.ResponseWriter.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	c.Ctx.ResponseWriter.WriteHeader(http.StatusOK)
	return c.Ctx.ResponseWriter
}

// forEachSseData calls fn with the data of every event in an SSE stream, until
// fn asks it to stop.
func forEachSseData(reader io.Reader, fn func([]byte) bool) error {
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
