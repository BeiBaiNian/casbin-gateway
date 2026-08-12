// Copyright 2023 The casbin Authors. All Rights Reserved.
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
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/beego/beego"
)

// chatCompletionsRequest contains only the fields needed for routing.
// All other fields (messages, temperature, etc.) are forwarded as-is to the upstream.
// (PM section 3.3.1)
type chatCompletionsRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// openaiErrorResponse follows the OpenAI API error format.
// (PM section 3.3.7)
type openaiErrorResponse struct {
	Error openaiErrorDetail `json:"error"`
}

type openaiErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// proxyClient is a shared HTTP client for upstream requests.
// Reusing a single instance allows TCP connection pooling across requests.
var proxyClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// ChatCompletions is the OpenAI-compatible chat completions proxy endpoint.
// It matches an upstream channel by model name and forwards the request
// and response body as-is (pass-through).
// Supports SSE streaming when stream=true in the request body.
// This endpoint does NOT require Casdoor authentication (auth deferred to milestone 1.3).
// (PM section 3.3)
func (c *ApiController) ChatCompletions() {
	rawBody := c.Ctx.Input.RequestBody

	// 3.3.1 — Parse only model and stream; forward everything else as-is.
	var req chatCompletionsRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	if req.Model == "" {
		c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}

	// 3.3.2 — Match channel globally (no owner filter; Judge Q1 → Plan A).
	channel, err := object.GetChannelByModel(req.Model)
	if err != nil {
		if errors.Is(err, object.ErrNoChannelAvailable) {
			c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("channel lookup failed:", err)
			c.writeOpenAIError(http.StatusBadGateway, "server_error", "channel lookup failed")
		}
		return
	}

	// Guard against misconfigured channels with empty base URL.
	if channel.BaseUrl == "" {
		beego.Error("channel has empty base URL:", channel.GetId())
		c.writeOpenAIError(http.StatusBadGateway, "server_error", "channel base URL is not configured")
		return
	}

	// 3.3.4 — Build upstream request. Path is hardcoded to OpenAI-compatible /v1/chat/completions.
	upstreamURL := strings.TrimRight(channel.BaseUrl, "/") + "/v1/chat/completions"
	upstreamReq, err := http.NewRequestWithContext(c.Ctx.Request.Context(), "POST", upstreamURL, bytes.NewReader(rawBody))
	if err != nil {
		c.writeOpenAIError(http.StatusBadGateway, "server_error", "upstream connection failed")
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+channel.ApiKey)

	// 3.3.5 — Per-request timeout via context.
	ctx, cancel := context.WithTimeout(c.Ctx.Request.Context(), 120*time.Second)
	defer cancel()
	upstreamReq = upstreamReq.WithContext(ctx)

	// Execute upstream request.
	upstreamResp, err := proxyClient.Do(upstreamReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c.writeOpenAIError(http.StatusGatewayTimeout, "server_error", "upstream timeout")
		} else {
			c.writeOpenAIError(http.StatusBadGateway, "server_error", "upstream connection failed")
		}
		return
	}
	defer upstreamResp.Body.Close()

	// 3.3.6 / 3.3.7 — Stream SSE or copy non-stream response as pass-through.
	if req.Stream {
		c.proxySSE(upstreamResp)
	} else {
		c.proxyNonStream(upstreamResp)
	}
}

// proxyNonStream copies the upstream response headers, status code, and body
// directly to the client without any transformation (pass-through mode).
// (PM section 3.3.7)
func (c *ApiController) proxyNonStream(upstreamResp *http.Response) {
	for k, vs := range upstreamResp.Header {
		for _, v := range vs {
			c.Ctx.ResponseWriter.Header().Add(k, v)
		}
	}
	c.Ctx.ResponseWriter.WriteHeader(upstreamResp.StatusCode)
	if _, err := io.Copy(c.Ctx.ResponseWriter, upstreamResp.Body); err != nil {
		beego.Error("proxy non-stream response copy failed:", err)
	}
}

// proxySSE forwards an SSE (Server-Sent Events) stream from the upstream
// to the client. Each chunk is written and flushed immediately so that
// the client receives events in real time.
// Monitors client disconnection via c.Ctx.Request.Context().Done().
// (PM section 3.3.6)
func (c *ApiController) proxySSE(upstreamResp *http.Response) {
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "text/event-stream")
	c.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	c.Ctx.ResponseWriter.Header().Set("Connection", "keep-alive")
	c.Ctx.ResponseWriter.WriteHeader(upstreamResp.StatusCode)

	buf := make([]byte, 4096)
	for {
		// Respect client disconnection to avoid goroutine leak.
		select {
		case <-c.Ctx.Request.Context().Done():
			return
		default:
		}

		n, err := upstreamResp.Body.Read(buf)
		if n > 0 {
			c.Ctx.ResponseWriter.Write(buf[:n])
			c.Ctx.ResponseWriter.Flush()
		}
		if err != nil {
			if err != io.EOF {
				beego.Error("SSE upstream read error:", err)
			}
			return
		}
	}
}

// writeOpenAIError writes an OpenAI-compatible JSON error response
// with the given HTTP status code, error type, and message.
// (PM section 3.3.7)
func (c *ApiController) writeOpenAIError(statusCode int, errType, message string) {
	resp := openaiErrorResponse{
		Error: openaiErrorDetail{
			Message: message,
			Type:    errType,
		},
	}
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(statusCode)
	json.NewEncoder(c.Ctx.ResponseWriter).Encode(resp)
}
