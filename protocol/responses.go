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

// This file speaks the OpenAI Responses API, the only wire format recent Codex
// versions know. It is a client format only: the gateway reads requests in it
// and writes answers back in it, but no provider type serves it, so a request
// that arrives here is always translated for whichever upstream answers it.

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type responsesCodec struct{}

func init() {
	register(responsesCodec{})
}

func (responsesCodec) Name() string { return Responses }

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

// responsesRequest is the part of a Responses body that has a canonical
// counterpart. The rest (store, include, previous_response_id) describes a
// conversation this API keeps for itself, which no upstream here holds.
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
	Reasoning *struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// responsesInput is one element of the input array. Its fields are a union over
// the item types, which is how the API itself is shaped.
type responsesInput struct {
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

func (responsesCodec) DecodeRequest(raw []byte) (*Request, error) {
	var body responsesRequest
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("invalid request body")
	}

	items, err := responsesItemsOf(body.Input)
	if err != nil {
		return nil, errors.New("invalid input")
	}

	request := &Request{
		Model:             body.Model,
		Stream:            body.Stream,
		ToolChoice:        responsesToolChoiceOf(body.ToolChoice),
		ParallelToolCalls: body.ParallelToolCalls,
		Temperature:       body.Temperature,
		TopP:              body.TopP,
		MaxTokens:         body.MaxOutputTokens,
	}
	if body.Instructions != "" {
		request.System = []string{body.Instructions}
	}
	if body.Reasoning != nil && body.Reasoning.Effort != "" {
		request.Reasoning = &Reasoning{Effort: body.Reasoning.Effort}
	}
	if body.Text != nil {
		request.Format = responsesFormatOf(body.Text.Format)
	}
	for _, tool := range body.Tools {
		// Only plain function tools cross over: the hosted ones (web_search,
		// local_shell) are run by OpenAI itself, and no upstream here has them.
		if tool.Type != "function" || tool.Name == "" {
			continue
		}
		request.Tools = append(request.Tools, Tool{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters,
		})
	}

	for _, item := range items {
		request.Messages = appendResponsesItem(request.Messages, item)
	}
	if len(request.Messages) == 0 {
		return nil, errors.New("input is required")
	}
	return request, nil
}

// responsesItemsOf reads the input field, which is either a bare prompt or the
// conversation so far.
func responsesItemsOf(raw json.RawMessage) ([]responsesInput, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var prompt string
	if err := json.Unmarshal(raw, &prompt); err == nil {
		return []responsesInput{{Type: "message", Role: RoleUser, Content: rawString(prompt)}}, nil
	}

	var items []responsesInput
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func appendResponsesItem(messages []Message, item responsesInput) []Message {
	switch {
	case item.Type == "function_call":
		use := Content{Kind: KindToolUse, ToolUse: &ToolUse{
			Id: item.CallId, Name: item.Name, Arguments: emptyAsObject(item.Arguments),
		}}
		// Calls the model made in one turn belong to one assistant message,
		// which is how they were sent to the client in the first place.
		if last := len(messages) - 1; last >= 0 && messages[last].Role == RoleAssistant {
			messages[last].Content = append(messages[last].Content, use)
			return messages
		}
		return append(messages, Message{Role: RoleAssistant, Content: []Content{use}})

	case item.Type == "function_call_output":
		return append(messages, Message{Role: RoleUser, Content: []Content{{
			Kind:       KindToolResult,
			ToolResult: &ToolResult{Id: item.CallId, Text: responsesOutputText(item.Output)},
		}}})

	case item.Type == "message" || (item.Type == "" && item.Role != ""):
		role := RoleUser
		if item.Role == RoleAssistant {
			role = RoleAssistant
		}
		content := responsesContentOf(item.Content)
		if len(content) == 0 {
			return messages
		}
		return append(messages, Message{Role: role, Content: content})
	}

	// A developer message is a system prompt sent as an item; anything else,
	// reasoning above all, describes a turn no other API would take back.
	if item.Role == "developer" || item.Role == "system" {
		return append(messages, Message{Role: RoleUser, Content: responsesContentOf(item.Content)})
	}
	return messages
}

func responsesContentOf(raw json.RawMessage) []Content {
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

	var parts []responsesPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}

	content := []Content{}
	for _, part := range parts {
		switch {
		case part.ImageUrl != "":
			content = append(content, Content{Kind: KindImage, Image: imageOfUrl(part.ImageUrl)})
		case part.Text != "":
			content = append(content, Content{Kind: KindText, Text: part.Text})
		}
	}
	return content
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

	var wrapper struct {
		Content json.RawMessage `json:"content"`
		Output  string          `json:"output"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		inner := Message{Content: responsesContentOf(wrapper.Content)}
		if value := inner.Text(); value != "" {
			return value
		}
		if wrapper.Output != "" {
			return wrapper.Output
		}
	}
	return string(raw)
}

func responsesToolChoiceOf(raw json.RawMessage) *ToolChoice {
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
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &choice); err == nil && choice.Type == "function" && choice.Name != "" {
		return &ToolChoice{Mode: ChoiceTool, Name: choice.Name}
	}
	return nil
}

// responsesFormatOf reads text.format, which is the response format one level
// less deep than chat completions spells it.
func responsesFormatOf(raw json.RawMessage) *Format {
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
	case "json_object", "json_schema":
		return &Format{Kind: format.Type, Name: format.Name, Schema: format.Schema, Strict: format.Strict}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Response
// ---------------------------------------------------------------------------

// responsesItem is one item of the answer this API reports as a list: the
// assistant message, or one function call.
type responsesItem struct {
	id    string
	index int
	// callId, name and arguments are what a function call item carries.
	callId    string
	name      string
	arguments strings.Builder
	text      strings.Builder
}

func (item *responsesItem) message(status string) map[string]any {
	return map[string]any{
		"type": "message", "id": item.id, "status": status, "role": RoleAssistant,
		"content": []any{map[string]any{
			"type": "output_text", "text": item.text.String(), "annotations": []any{},
		}},
	}
}

func (item *responsesItem) call(status string) map[string]any {
	return map[string]any{
		"type": "function_call", "id": item.id, "status": status,
		"call_id": emptyAs(item.callId, item.id), "name": item.name,
		"arguments": emptyAsObject(item.arguments.String()),
	}
}

// responsesAnswer builds the answer as this API reports it. The stream writer
// and the whole-body encoder share it, so a client reading the events and one
// reading the body are told the same thing.
type responsesAnswer struct {
	token   string
	id      string
	model   string
	created int64

	nextIndex int
	message   *responsesItem
	// thinkingId names the reasoning item the thinking deltas belong to. The
	// item itself is not part of the output: no other API signs one, and this
	// one only takes back what it produced.
	thinkingId string
	calls      map[int]*responsesItem
	callOrder  []int

	usage   Usage
	failure *Failure
}

func newResponsesAnswer(model string) *responsesAnswer {
	now := time.Now()
	token := fmt.Sprintf("%d", now.UnixNano())
	return &responsesAnswer{
		token:   token,
		id:      "resp_" + token,
		model:   model,
		created: now.Unix(),
		calls:   map[int]*responsesItem{},
	}
}

// startMessage opens the assistant message item, which every answer carrying
// text has exactly one of.
func (answer *responsesAnswer) startMessage() (*responsesItem, bool) {
	if answer.message != nil {
		return answer.message, false
	}
	answer.message = &responsesItem{
		id:    fmt.Sprintf("msg_%s_%d", answer.token, answer.nextIndex),
		index: answer.nextIndex,
	}
	answer.nextIndex++
	return answer.message, true
}

// startCall opens the item one tool call is assembled in.
func (answer *responsesAnswer) startCall(index int) (*responsesItem, bool) {
	if call, ok := answer.calls[index]; ok {
		return call, false
	}
	call := &responsesItem{
		id:    fmt.Sprintf("fc_%s_%d", answer.token, answer.nextIndex),
		index: answer.nextIndex,
	}
	answer.nextIndex++
	answer.calls[index] = call
	answer.callOrder = append(answer.callOrder, index)
	return call, true
}

func (answer *responsesAnswer) reasoningId() string {
	if answer.thinkingId == "" {
		answer.thinkingId = "rs_" + answer.token
	}
	return answer.thinkingId
}

// output is the item list a finished answer carries, in the order the items
// were started.
func (answer *responsesAnswer) output() []any {
	slots := make([]any, answer.nextIndex)
	if answer.message != nil {
		slots[answer.message.index] = answer.message.message("completed")
	}
	for _, index := range answer.callOrder {
		call := answer.calls[index]
		slots[call.index] = call.call("completed")
	}

	items := []any{}
	for _, item := range slots {
		if item != nil {
			items = append(items, item)
		}
	}
	return items
}

func (answer *responsesAnswer) response(status string, output []any) map[string]any {
	response := map[string]any{
		"id": answer.id, "object": "response", "created_at": answer.created,
		"status": status, "model": answer.model, "output": output,
	}
	if !answer.usage.IsZero() {
		response["usage"] = map[string]any{
			"input_tokens":  answer.usage.InputTokens,
			"output_tokens": answer.usage.OutputTokens,
			"total_tokens":  answer.usage.InputTokens + answer.usage.OutputTokens,
		}
	}
	if answer.failure != nil {
		response["error"] = map[string]any{
			"code": emptyAs(answer.failure.Kind, "server_error"), "message": answer.failure.Message,
		}
	}
	return response
}

// add records one event, and reports the item it landed in so a stream writer
// can send the same piece on.
func (answer *responsesAnswer) add(event Event) (*responsesItem, bool) {
	if event.Model != "" {
		answer.model = event.Model
	}

	switch event.Kind {
	case EventText:
		item, opened := answer.startMessage()
		item.text.WriteString(event.Text)
		return item, opened
	case EventToolUse:
		if event.Tool == nil {
			return nil, false
		}
		call, opened := answer.startCall(event.Tool.Index)
		if event.Tool.Id != "" {
			call.callId = event.Tool.Id
		}
		call.name += event.Tool.Name
		call.arguments.WriteString(event.Tool.Arguments)
		return call, opened
	case EventDone:
		if event.Usage != nil {
			answer.usage = *event.Usage
		}
	case EventFailure:
		answer.failure = event.Failure
	}
	return nil, false
}

func (responsesCodec) EncodeResponse(response *Response) ([]byte, error) {
	answer := newResponsesAnswer(response.Model)
	answer.usage = response.Usage
	answer.failure = response.Failure
	WriteStream(&responsesCollector{answer: answer}, response)

	status := "completed"
	if response.Failure != nil {
		status = "failed"
	}
	return json.Marshal(answer.response(status, answer.output()))
}

// responsesCollector fills an answer in without sending anything, for the
// client that asked for a body rather than a stream.
type responsesCollector struct {
	answer *responsesAnswer
}

func (collector *responsesCollector) Open() {}

func (collector *responsesCollector) Write(event Event) { collector.answer.add(event) }

func (collector *responsesCollector) Close() {}

// ---------------------------------------------------------------------------
// Stream
// ---------------------------------------------------------------------------

// responsesWriter turns canonical events into the Responses events a client
// such as Codex expects.
type responsesWriter struct {
	events   *eventWriter
	answer   *responsesAnswer
	sequence int
}

func (responsesCodec) NewStreamWriter(writer io.Writer, flush func(), model string) StreamWriter {
	return &responsesWriter{
		events: &eventWriter{writer: writer, flush: flush},
		answer: newResponsesAnswer(model),
	}
}

func (writer *responsesWriter) emit(name string, payload map[string]any) {
	payload["type"] = name
	payload["sequence_number"] = writer.sequence
	writer.sequence++
	writer.events.send(name, payload)
}

func (writer *responsesWriter) Open() {
	writer.emit("response.created", map[string]any{
		"response": writer.answer.response("in_progress", []any{}),
	})
}

func (writer *responsesWriter) Write(event Event) {
	if event.Kind == EventThinking {
		if event.Text == "" {
			return
		}
		writer.emit("response.reasoning_summary_text.delta", map[string]any{
			"item_id": writer.answer.reasoningId(), "output_index": 0,
			"summary_index": 0, "delta": event.Text,
		})
		return
	}

	item, opened := writer.answer.add(event)
	if item == nil {
		return
	}

	switch event.Kind {
	case EventText:
		if opened {
			writer.emit("response.output_item.added", map[string]any{
				"output_index": item.index,
				"item": map[string]any{
					"type": "message", "id": item.id, "status": "in_progress",
					"role": RoleAssistant, "content": []any{},
				},
			})
		}
		writer.emit("response.output_text.delta", map[string]any{
			"item_id": item.id, "output_index": item.index, "content_index": 0, "delta": event.Text,
		})
	case EventToolUse:
		if opened {
			writer.emit("response.output_item.added", map[string]any{
				"output_index": item.index,
				"item": map[string]any{
					"type": "function_call", "id": item.id, "status": "in_progress",
					"call_id": emptyAs(item.callId, item.id), "name": item.name, "arguments": "",
				},
			})
		}
		if event.Tool.Arguments != "" {
			writer.emit("response.function_call_arguments.delta", map[string]any{
				"item_id": item.id, "output_index": item.index, "delta": event.Tool.Arguments,
			})
		}
	}
}

func (writer *responsesWriter) Close() {
	answer := writer.answer
	if message := answer.message; message != nil {
		writer.emit("response.output_text.done", map[string]any{
			"item_id": message.id, "output_index": message.index,
			"content_index": 0, "text": message.text.String(),
		})
		writer.emit("response.output_item.done", map[string]any{
			"output_index": message.index, "item": message.message("completed"),
		})
	}
	for _, index := range answer.callOrder {
		call := answer.calls[index]
		writer.emit("response.function_call_arguments.done", map[string]any{
			"item_id": call.id, "output_index": call.index,
			"arguments": emptyAsObject(call.arguments.String()),
		})
		writer.emit("response.output_item.done", map[string]any{
			"output_index": call.index, "item": call.call("completed"),
		})
	}

	if answer.failure != nil {
		writer.emit("response.failed", map[string]any{
			"response": answer.response("failed", answer.output()),
		})
		return
	}
	writer.emit("response.completed", map[string]any{
		"response": answer.response("completed", answer.output()),
	})
}

func (responsesCodec) EncodeError(kind string, message string) []byte {
	return openAiCodec{}.EncodeError(kind, message)
}
