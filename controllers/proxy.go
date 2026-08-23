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
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

const (
	// proxyResponseHeaderTimeout bounds the wait for the upstream response
	// headers. It deliberately does not cover the response body: a completion
	// can legitimately take many minutes to finish streaming, so the body is
	// bounded by proxyIdleTimeout instead.
	proxyResponseHeaderTimeout = 60 * time.Second
	// proxyIdleTimeout is the longest gap tolerated between two chunks of the
	// upstream response body before the request is aborted.
	proxyIdleTimeout = 120 * time.Second
)

// hopByHopHeaders are connection-scoped, so a proxy must not pass them on.
// See RFC 7230 section 6.1.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// proxyTarget is one upstream API this proxy relays to: the wire format it
// speaks and the endpoint the request lands on.
type proxyTarget struct {
	protocol string
	endpoint string
	// clientEndpoint names the API the client called when it is not the one the
	// upstream serves, which means both bodies are translated on the way.
	clientEndpoint string
}

// responsesEndpoint is the Responses API front end. Recent Codex versions speak
// nothing else, while almost every provider behind the gateway serves chat
// completions, so requests and responses are translated in both directions.
const responsesEndpoint = "/responses"

var (
	openAiChat           = proxyTarget{protocol: object.ProtocolOpenAi, endpoint: "/chat/completions"}
	openAiResponses      = proxyTarget{protocol: object.ProtocolOpenAi, endpoint: "/chat/completions", clientEndpoint: responsesEndpoint}
	anthropicMessages    = proxyTarget{protocol: object.ProtocolAnthropic, endpoint: "/v1/messages"}
	anthropicCountTokens = proxyTarget{protocol: object.ProtocolAnthropic, endpoint: "/v1/messages/count_tokens"}
)

// proxyRoute is one client request being relayed. Everything except model and
// stream (messages, temperature, ...) is forwarded as-is to the upstream.
type proxyRoute struct {
	target proxyTarget
	body   []byte
	model  string
	stream bool
	// source describes how the providers were chosen, for the error a client
	// sees when none of them can be used.
	source string
	start  time.Time
	// record accumulates what is written to the LLM record of this request. It
	// is nil while recording is off.
	record *object.LlmRecord
}

// routingFields are the only fields read out of the request body. Both the
// OpenAI and the Anthropic body carry them under the same names.
type routingFields struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

// proxyErrorDetail is the {message, type} object both APIs report errors in.
type proxyErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// openaiErrorResponse follows the OpenAI API error format.
type openaiErrorResponse struct {
	Error proxyErrorDetail `json:"error"`
}

// anthropicErrorResponse follows the Anthropic API error format, which wraps
// the same detail in a typed envelope.
type anthropicErrorResponse struct {
	Type  string           `json:"type"`
	Error proxyErrorDetail `json:"error"`
}

// proxyClient is a shared HTTP client for upstream requests.
// Reusing a single instance allows TCP connection pooling across requests.
// It has no overall Timeout on purpose, see proxyResponseHeaderTimeout.
var proxyClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport: &http.Transport{
		Proxy: proxy.Proxy,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: proxyResponseHeaderTimeout,
	},
}

// ChatCompletions is the OpenAI-compatible chat completions proxy endpoint.
// It matches the upstream providers by model name and forwards the request and
// response body as-is (pass-through), trying the providers in priority order
// until one of them answers. Supports SSE streaming when stream=true in the
// request body.
// This endpoint does NOT require Casdoor authentication (auth deferred to milestone 1.3).
func (c *ApiController) ChatCompletions() {
	c.proxyByModel(openAiChat)
}

// Responses is the OpenAI Responses API entry point. The request is translated
// to chat completions on the way out and the answer back on the way in, so an
// upstream that never served the Responses API can still answer it.
func (c *ApiController) Responses() {
	c.proxyByModel(openAiResponses)
}

// Messages is the Anthropic-compatible counterpart of ChatCompletions, for the
// agents that speak that API rather than OpenAI's.
func (c *ApiController) Messages() {
	c.proxyByModel(anthropicMessages)
}

// CountTokens relays the Anthropic token-counting endpoint, which clients call
// alongside Messages to size their context.
func (c *ApiController) CountTokens() {
	c.proxyByModel(anthropicCountTokens)
}

// AgentChatCompletions is the per-agent entry point of the same proxy: an agent
// pointed at ".../v1/agents/<agentId>" reaches the provider bound to it rather
// than one chosen by model name.
func (c *ApiController) AgentChatCompletions() {
	c.proxyByAgent(openAiChat)
}

// AgentResponses is the per-agent entry point of the Responses API, which is
// the one Codex reaches the gateway on.
func (c *ApiController) AgentResponses() {
	c.proxyByAgent(openAiResponses)
}

// AgentMessages is the per-agent entry point for Anthropic clients. One base URL
// serves both APIs: an OpenAI client appends /chat/completions to it, while an
// Anthropic one appends /v1/messages.
func (c *ApiController) AgentMessages() {
	c.proxyByAgent(anthropicMessages)
}

// AgentCountTokens is the per-agent Anthropic token-counting endpoint.
func (c *ApiController) AgentCountTokens() {
	c.proxyByAgent(anthropicCountTokens)
}

// proxyByModel forwards to the providers that serve the model the request names.
func (c *ApiController) proxyByModel(target proxyTarget) {
	route, ok := c.readProxyRoute(target)
	if !ok {
		return
	}
	route.source = "model: " + route.model
	// Every way out of a proxy entry point ends the client request, which is
	// what a record describes, so this is the only place one is written.
	defer c.finishLlmRecord(route)

	// Match the providers globally, without an owner filter.
	providers, err := object.GetProvidersByModel(route.model)
	if err != nil {
		if errors.Is(err, object.ErrNoProviderAvailable) {
			route.recordOutcome(http.StatusBadRequest, err.Error())
			c.writeProxyError(target.protocol, http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("provider lookup failed:", err)
			route.recordOutcome(http.StatusBadGateway, "provider lookup failed")
			c.writeProxyError(target.protocol, http.StatusBadGateway, "server_error", "provider lookup failed")
		}
		return
	}

	c.forwardToProviders(providers, route)
}

// proxyByAgent forwards to the provider chain bound to the agent in the path.
func (c *ApiController) proxyByAgent(target proxyTarget) {
	route, ok := c.readProxyRoute(target)
	if !ok {
		return
	}
	agentId := c.Ctx.Input.Param(":agentId")
	route.source = "agent: " + agentId
	if route.record != nil {
		route.record.Agent = agentId
	}
	defer c.finishLlmRecord(route)

	// The whole chain is forwarded to, so a bound provider that is down fails
	// over to the agent's fallbacks instead of failing the request.
	providers, err := object.GetProvidersByAgent(agentId)
	if err != nil {
		if errors.Is(err, object.ErrAgentNoProvider) {
			route.recordOutcome(http.StatusBadRequest, err.Error())
			c.writeProxyError(target.protocol, http.StatusBadRequest, "invalid_request_error", err.Error())
		} else {
			beego.Error("agent provider lookup failed:", err)
			route.recordOutcome(http.StatusBadGateway, err.Error())
			c.writeProxyError(target.protocol, http.StatusBadGateway, "server_error", err.Error())
		}
		return
	}

	c.forwardToProviders(providers, route)
}

func (c *ApiController) readProxyRoute(target proxyTarget) (*proxyRoute, bool) {
	if !c.allowRelay() {
		c.writeProxyError(target.protocol, http.StatusUnauthorized, "authentication_error",
			"this relay is reachable from the network, so it needs the token shown next to the provider in Casbin Gateway")
		return nil, false
	}

	rawBody := c.Ctx.Input.RequestBody

	var fields routingFields
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		c.writeProxyError(target.protocol, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return nil, false
	}
	if fields.Model == "" {
		c.writeProxyError(target.protocol, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, false
	}

	body := rawBody
	if target.clientEndpoint == responsesEndpoint {
		converted, err := responsesToChat(rawBody)
		if err != nil {
			c.writeProxyError(target.protocol, http.StatusBadRequest, "invalid_request_error", err.Error())
			return nil, false
		}
		body = converted
	}

	route := &proxyRoute{target: target, body: body, model: fields.Model, stream: fields.Stream, start: time.Now()}
	if object.IsLlmRecording() {
		route.record = &object.LlmRecord{
			Protocol: target.protocol,
			Endpoint: emptyAsValue(target.clientEndpoint, target.endpoint),
			Model:    fields.Model,
			ClientIp: util.GetClientIp(c.Ctx.Request),
			Stream:   fields.Stream,
		}
	}
	return route, true
}

// forwardToProviders relays the request to the first provider that answers.
func (c *ApiController) forwardToProviders(providers []*object.Provider, route *proxyRoute) {
	// Drop the providers this proxy cannot talk to before forwarding, so that
	// the last usable provider is known and its response can be relayed as-is.
	usableProviders := []*object.Provider{}
	skipReason := ""
	for _, provider := range providers {
		if reason := c.providerUnusableReason(provider, route.target.protocol); reason != "" {
			beego.Error("skipped provider", provider.GetId()+":", reason)
			skipReason = reason
			continue
		}
		usableProviders = append(usableProviders, provider)
	}
	if len(usableProviders) == 0 {
		message := fmt.Sprintf("no usable provider for %s", route.source)
		if skipReason != "" {
			message = skipReason
		}
		route.recordOutcome(http.StatusBadGateway, message)
		c.writeProxyError(route.target.protocol, http.StatusBadGateway, "server_error", message)
		return
	}

	// A provider inside its failure cooldown goes last, so a dead upstream stops
	// costing every request the time it takes to time out.
	usableProviders = object.SortProvidersByHealth(usableProviders)

	// Fail over to the next provider as long as nothing has been written to the
	// client yet. The last provider is never retried, so the client gets the
	// real upstream answer instead of a synthesized error.
	lastStatus, lastMessage := http.StatusBadGateway, "upstream connection failed"
	for i, provider := range usableProviders {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up, there is nobody left to fail over for.
			route.recordOutcome(0, "client disconnected")
			return
		}

		status, message, written := c.forwardToProvider(provider, route, i == len(usableProviders)-1)
		if written {
			return
		}
		lastStatus, lastMessage = status, message
	}

	route.recordOutcome(lastStatus, lastMessage)
	c.writeProxyError(route.target.protocol, lastStatus, "server_error", lastMessage)
}

// forwardToProvider sends the request to a single provider's upstream. It reports
// whether the client response was already written, and when it was not, the
// status and message describing the failure so that the caller can fail over to
// the next provider. The response of the last provider is always relayed, even
// when its status would otherwise be retried.
func (c *ApiController) forwardToProvider(provider *object.Provider, route *proxyRoute, isLast bool) (int, string, bool) {
	route.recordAttempt(provider.GetId())

	upstreamUrl, err := object.BuildProviderUrl(provider.BaseUrl, route.target.protocol, route.target.endpoint)
	if err != nil {
		object.ReportProviderFailure(provider.GetId(), err.Error())
		return http.StatusBadGateway, err.Error(), false
	}
	upstreamUrl = object.AppendQuery(upstreamUrl, c.Ctx.Request.URL.RawQuery)

	// The context is cancelled when this function returns, which happens only
	// after the response body has been relayed to the client.
	ctx, cancel := context.WithCancel(c.Ctx.Request.Context())
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamUrl, bytes.NewReader(route.body))
	if err != nil {
		return http.StatusBadGateway, "upstream connection failed", false
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	object.SetProviderAuth(upstreamReq.Header, provider)
	if object.UsesClientAuth(provider) {
		c.copyClientAuthHeaders(upstreamReq.Header)
	}
	if route.target.protocol == object.ProtocolAnthropic {
		c.copyAnthropicHeaders(upstreamReq.Header)
	}

	upstreamResp, err := proxyClient.Do(upstreamReq)
	if err != nil {
		if c.Ctx.Request.Context().Err() != nil {
			// The client hung up mid-request, there is nothing left to answer.
			route.recordOutcome(0, "client disconnected")
			return 0, "", true
		}

		beego.Error("upstream request to provider", provider.GetId(), "failed:", err)
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			object.ReportProviderFailure(provider.GetId(), "upstream timeout")
			return http.StatusGatewayTimeout, "upstream timeout", false
		}
		object.ReportProviderFailure(provider.GetId(), "upstream connection failed")
		return http.StatusBadGateway, "upstream connection failed", false
	}
	defer upstreamResp.Body.Close()

	reportProviderStatus(provider, upstreamResp.StatusCode)

	if !isLast && isRetryableStatus(upstreamResp.StatusCode) {
		beego.Error("provider", provider.GetId(), "returned a retryable status:", upstreamResp.Status)
		// Drain a bounded amount so the connection can be pooled and reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(upstreamResp.Body, 4096))
		return http.StatusBadGateway, fmt.Sprintf("upstream returned %s", upstreamResp.Status), false
	}

	// Abort the request when the upstream goes silent, rather than capping the
	// total duration, which would cut off long but healthy streams.
	body := newIdleTimeoutReader(upstreamResp.Body, proxyIdleTimeout, cancel)
	defer body.Stop()

	route.recordOutcome(upstreamResp.StatusCode, "")
	if route.record == nil {
		c.relayResponse(route, upstreamResp, body, route.stream && isEventStream(upstreamResp))
		return 0, "", true
	}

	tap := &usageTap{reader: body}
	c.relayResponse(route, upstreamResp, tap, route.stream && isEventStream(upstreamResp))
	route.recordUsage(tap.tail)
	return 0, "", true
}

// reportProviderStatus feeds the breaker that decides in which order providers are
// tried. A status the upstream itself rejected the request with counts as a
// provider failure: a wrong key or an exhausted quota is not something the next
// request will do better.
func reportProviderStatus(provider *object.Provider, statusCode int) {
	switch {
	case isRetryableStatus(statusCode):
		object.ReportProviderFailure(provider.GetId(), fmt.Sprintf("upstream returned %d", statusCode))
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden ||
		statusCode == http.StatusPaymentRequired:
		object.ReportProviderFailure(provider.GetId(), fmt.Sprintf("upstream rejected the credentials with %d", statusCode))
	default:
		object.ReportProviderSuccess(provider.GetId())
	}
}

// allowRelay decides whether a request may use the providers stored here. A
// request from this machine always may — that is the whole point of a local
// gateway, and a client-auth provider carries the caller's own vendor
// credential in the same header a token would use. Anything off-box has to
// present the relay token instead.
func (c *ApiController) allowRelay() bool {
	if util.IsLoopbackRequest(c.Ctx.Request) {
		return true
	}

	token := conf.GetRelayToken()
	return token != "" && c.relayCredential() == token
}

// relayCredential is the token the client sent, in either of the two headers
// the OpenAI and Anthropic clients use.
func (c *ApiController) relayCredential() string {
	header := c.Ctx.Request.Header
	if key := strings.TrimSpace(header.Get("X-Api-Key")); key != "" {
		return key
	}

	authorization := strings.TrimSpace(header.Get("Authorization"))
	return strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
}

// clientAuthHeaders are forwarded verbatim by a provider that authenticates with
// the caller's own credentials: the credential itself, plus what the vendors
// expect beside a token issued to a CLI rather than to an API account. It is an
// allowlist, so nothing else the client sent (a browser cookie, say) leaks
// upstream.
var clientAuthHeaders = []string{
	"Authorization",
	"X-Api-Key",
	"User-Agent",
	"X-App",
	"Openai-Beta",
	"Openai-Organization",
	"Openai-Project",
	"Chatgpt-Account-Id",
}

// hasClientCredentials reports whether the client request carries a credential
// a client-auth provider could forward.
func (c *ApiController) hasClientCredentials() bool {
	header := c.Ctx.Request.Header
	return header.Get("Authorization") != "" || header.Get("X-Api-Key") != ""
}

func (c *ApiController) copyClientAuthHeaders(dst http.Header) {
	for _, name := range clientAuthHeaders {
		dst.Del(name)
		for _, value := range c.Ctx.Request.Header.Values(name) {
			dst.Add(name, value)
		}
	}
}

// copyAnthropicHeaders passes the client's API version and beta opt-ins on to
// the upstream: they select response features, so dropping them would answer a
// different request than the one that was made.
func (c *ApiController) copyAnthropicHeaders(dst http.Header) {
	version := c.Ctx.Request.Header.Get("Anthropic-Version")
	if version == "" {
		version = object.AnthropicVersion
	}
	dst.Set("Anthropic-Version", version)

	for _, beta := range c.Ctx.Request.Header.Values("Anthropic-Beta") {
		dst.Add("Anthropic-Beta", beta)
	}
}

// relayResponse copies the upstream status code, headers and body to the client
// without any transformation (pass-through mode). When flush is true, every
// chunk is written out as soon as it arrives so that SSE clients receive the
// events in real time. A client that called an endpoint the upstream does not
// serve gets the answer translated back into the API it asked in; an upstream
// error is passed through either way, since both APIs report errors alike.
func (c *ApiController) relayResponse(route *proxyRoute, upstreamResp *http.Response, body io.Reader, flush bool) {
	if route.target.clientEndpoint == responsesEndpoint && upstreamResp.StatusCode == http.StatusOK {
		if flush {
			c.relayResponsesStream(route, body)
		} else {
			c.relayResponsesBody(route, body)
		}
		return
	}

	copyResponseHeaders(c.Ctx.ResponseWriter.Header(), upstreamResp.Header)
	if flush {
		c.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	}
	c.Ctx.ResponseWriter.WriteHeader(upstreamResp.StatusCode)

	if !flush {
		if _, err := io.Copy(c.Ctx.ResponseWriter, body); err != nil {
			beego.Error("proxy response copy failed:", err)
		}
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := c.Ctx.ResponseWriter.Write(buf[:n]); writeErr != nil {
				beego.Error("proxy stream write failed:", writeErr)
				return
			}
			c.Ctx.ResponseWriter.Flush()
		}
		if err != nil {
			if err != io.EOF {
				beego.Error("proxy stream read failed:", err)
			}
			return
		}
	}
}

// copyResponseHeaders copies the upstream headers to the client, minus the
// hop-by-hop ones, which belong to the upstream connection and not to the
// response being relayed.
func copyResponseHeaders(dst http.Header, src http.Header) {
	for name, values := range src {
		if isHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func isHopByHopHeader(name string) bool {
	for _, header := range hopByHopHeaders {
		if strings.EqualFold(name, header) {
			return true
		}
	}
	return false
}

// isEventStream reports whether the upstream response really is an SSE stream.
// An upstream that rejects the request answers with a JSON body even when
// stream=true was asked for, and relaying that as text/event-stream would
// leave the client waiting for events that never come.
func isEventStream(upstreamResp *http.Response) bool {
	if upstreamResp.StatusCode < 200 || upstreamResp.StatusCode > 299 {
		return false
	}
	return strings.Contains(strings.ToLower(upstreamResp.Header.Get("Content-Type")), "text/event-stream")
}

// isRetryableStatus reports whether another provider is worth trying. A rate
// limit or an upstream-side error is transient or specific to that provider,
// while a 4xx caused by the request itself would fail the same way everywhere.
func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// providerUnusableReason reports why the proxy cannot forward to a provider, or
// an empty string when it can.
func (c *ApiController) providerUnusableReason(provider *object.Provider, protocol string) string {
	if !object.IsProviderTypeSupported(provider) {
		return fmt.Sprintf("the %s provider type is not supported", provider.Type)
	}
	if object.ProviderProtocol(provider) != protocol {
		return fmt.Sprintf("provider %s does not speak the %s API", provider.GetId(), protocol)
	}
	if provider.BaseUrl == "" {
		return "provider base URL is not configured"
	}
	// Without a credential to forward the upstream would answer 401, which
	// reads as a broken provider rather than a client that sent no key.
	if object.UsesClientAuth(provider) && !c.hasClientCredentials() {
		return fmt.Sprintf("provider %s forwards the credentials of the caller, but the request carries none", provider.GetId())
	}
	return ""
}

// idleTimeoutReader aborts the upstream request when no data arrives for the
// given duration. It takes the place of an overall request timeout, which would
// cut off a long but healthy streaming completion.
type idleTimeoutReader struct {
	reader  io.Reader
	timeout time.Duration
	timer   *time.Timer
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration, onIdle func()) *idleTimeoutReader {
	return &idleTimeoutReader{
		reader:  reader,
		timeout: timeout,
		timer:   time.AfterFunc(timeout, onIdle),
	}
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

func (r *idleTimeoutReader) Stop() {
	r.timer.Stop()
}

// writeProxyError writes a JSON error response in the format the client that
// made the request expects.
func (c *ApiController) writeProxyError(protocol string, statusCode int, errType, message string) {
	detail := proxyErrorDetail{Message: message, Type: errType}

	var resp any = openaiErrorResponse{Error: detail}
	if protocol == object.ProtocolAnthropic {
		resp = anthropicErrorResponse{Type: "error", Error: detail}
	}

	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/json")
	c.Ctx.ResponseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(c.Ctx.ResponseWriter).Encode(resp); err != nil {
		beego.Error("proxy error response write failed:", err)
	}
}
