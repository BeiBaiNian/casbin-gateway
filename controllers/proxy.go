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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
	"golang.org/x/crypto/bcrypt"
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

// ChatCompletions is the OpenAI-compatible chat completions proxy endpoint.
// It matches an upstream channel by model name, validates the target URL for
// SSRF safety, and forwards the request and response body as-is (pass-through).
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

	// 1.3 — Token authentication.
	token, ok := c.authenticateToken(req.Model)
	if !ok {
		return
	}
	_ = token // reserved for milestone 1.4 (usage tracking)

	// 3.3.2 — Match channel globally (no owner filter; Judge Q1 → Plan A).
	channel, err := object.GetChannelByModel(req.Model)
	if err != nil {
		c.writeOpenAIError(http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	// 3.3.3 — SSRF safety: validate scheme and basic URL structure upfront.
	u, err := url.Parse(channel.BaseUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		c.writeOpenAIError(http.StatusBadGateway, "server_error", "upstream connection blocked by security policy")
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

	// 3.3.5 — Per-request timeout via context. proxy.DefaultHttpClient has no timeout.
	ctx, cancel := context.WithTimeout(c.Ctx.Request.Context(), 120*time.Second)
	defer cancel()
	upstreamReq = upstreamReq.WithContext(ctx)

	// 3.3.3 cont'd — Build SSRF-guarding HTTP client with port whitelist,
	// DNS rebinding protection (resolve then dial by IP), intranet IP block,
	// and redirect prohibition. Mirrors TestChannelConnectivity's pattern.
	host := u.Hostname()
	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			_, portStr, e := net.SplitHostPort(addr)
			if e != nil {
				return nil, fmt.Errorf("connection blocked by security policy")
			}
			p, e := strconv.Atoi(portStr)
			if e != nil || !object.IsAllowedPort(p) {
				return nil, fmt.Errorf("connection blocked by security policy")
			}
			// DNS rebinding protection: resolve first, then check each IP.
			ips, e := net.LookupIP(host)
			if e != nil || len(ips) == 0 {
				return nil, fmt.Errorf("connection blocked by security policy")
			}
			for _, ip := range ips {
				if util.IsIntranetIp(ip.String()) {
					return nil, fmt.Errorf("connection blocked by security policy")
				}
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(dialCtx, network, net.JoinHostPort(ips[0].String(), portStr))
		},
	}
	if u.Scheme == "https" {
		transport.TLSClientConfig = &tls.Config{ServerName: host}
	}

	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// Execute upstream request.
	upstreamResp, err := client.Do(upstreamReq)
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
	io.Copy(c.Ctx.ResponseWriter, upstreamResp.Body)
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
				// Log errors silently; upstream may close the connection at any time.
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

// authenticateToken validates a Bearer token from the Authorization header.
// It checks the secret key hash, status, expiration, allowed models, and rate limit.
// Returns (*Token, true) on success, or (nil, false) after writing an error response.
func (c *ApiController) authenticateToken(model string) (*object.Token, bool) {
	authHeader := c.Ctx.Request.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		c.writeOpenAIError(http.StatusUnauthorized, "authentication_error", "missing or invalid Authorization header")
		return nil, false
	}

	providedKey := strings.TrimPrefix(authHeader, "Bearer ")
	if providedKey == "" {
		c.writeOpenAIError(http.StatusUnauthorized, "authentication_error", "empty token")
		return nil, false
	}

	// Iterate all tokens and compare bcrypt hashes.
	tokens, err := object.GetTokens("")
	if err != nil {
		c.writeOpenAIError(http.StatusInternalServerError, "server_error", "failed to verify token")
		return nil, false
	}

	var matchedToken *object.Token
	for _, t := range tokens {
		if t.SecretKeyHash == "" {
			continue
		}
		err := bcrypt.CompareHashAndPassword([]byte(t.SecretKeyHash), []byte(providedKey))
		if err == nil {
			matchedToken = t
			break
		}
	}

	if matchedToken == nil {
		c.writeOpenAIError(http.StatusUnauthorized, "authentication_error", "invalid token")
		return nil, false
	}

	if matchedToken.Status != "enabled" {
		c.writeOpenAIError(http.StatusForbidden, "permission_error", "token is disabled")
		return nil, false
	}

	if matchedToken.ExpireTime != "" {
		expireTime, parseErr := time.Parse(time.RFC3339, matchedToken.ExpireTime)
		if parseErr == nil && time.Now().After(expireTime) {
			c.writeOpenAIError(http.StatusForbidden, "permission_error", "token has expired")
			return nil, false
		}
	}

	if len(matchedToken.AllowedModels) > 0 {
		allowed := false
		for _, m := range matchedToken.AllowedModels {
			if m == model {
				allowed = true
				break
			}
		}
		if !allowed {
			c.writeOpenAIError(http.StatusForbidden, "permission_error", "model not allowed for this token")
			return nil, false
		}
	}

	allowed, err := object.CheckRateLimit(matchedToken.Owner, matchedToken.Name, matchedToken.RateLimit)
	if err != nil {
		c.writeOpenAIError(http.StatusInternalServerError, "server_error", "rate limit check failed")
		return nil, false
	}
	if !allowed {
		c.writeOpenAIError(http.StatusTooManyRequests, "rate_limit_error", "rate limit exceeded")
		return nil, false
	}

	return matchedToken, true
}
