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

package object

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/apache/casbin-gateway/auditutil"
)

func TestNewLlmRequestAuditCapturesOpenAICompatibleRequest(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"stream":true,
		"messages":[
			{"role":"system","content":"You are a helpful assistant."},
			{"role":"user","content":"Hello"}
		],
		"tools":[{"type":"function","function":{"name":"weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
		"tool_choice":"auto"
	}`)
	record := NewLlmRequestAudit(raw, "gpt-test", "admin/default", "127.0.0.1", true)
	if record == nil {
		t.Fatal("request audit was not created")
	}
	if !record.Stream || record.Model != "gpt-test" || record.Channel != "admin/default" || record.ClientIp != "127.0.0.1" {
		t.Errorf("metadata = %#v", record)
	}
	if record.OriginalBytes != len(raw) {
		t.Errorf("originalBytes = %d, expected raw body length %d", record.OriginalBytes, len(raw))
	}
	for _, expected := range []string{"You are a helpful assistant.", "Hello", "weather", "tool_choice"} {
		if !strings.Contains(record.Payload, expected) {
			t.Errorf("payload did not preserve %q: %s", expected, record.Payload)
		}
	}
}

func TestLlmRequestAuditTaskOmitsOversizedRawBody(t *testing.T) {
	task := llmRequestAuditTask{
		model: "gpt-test", channel: "admin/default", clientIP: "127.0.0.1", originalBytes: auditutil.MaxPayloadBytes + 1,
		bodyOmitted: true,
	}
	record := task.record()
	if record == nil || !record.Truncated || record.OriginalBytes != auditutil.MaxPayloadBytes+1 {
		t.Errorf("unexpected oversized task record: %#v", record)
	}
	if strings.Contains(record.Payload, "rawBody") || !strings.Contains(record.Payload, "capture limit") {
		t.Errorf("oversized task payload was not safe metadata: %s", record.Payload)
	}
}

func TestNewLlmRequestAuditRedactsCredentials(t *testing.T) {
	record := NewLlmRequestAudit([]byte(`{
		"model":"gpt-test","messages":[{"role":"user","content":"Bearer sk-abcdefghijklmnopqrstuvwxyz"}],
		"tools":[{"type":"function","function":{"name":"x","parameters":{"api_key":"top-secret","nested":{"authorization":"secret"}}}}]
	}`), "gpt-test", "admin/default", "127.0.0.1", false)
	if record == nil {
		t.Fatal("request audit was not created")
	}
	if strings.Contains(record.Payload, "top-secret") || strings.Contains(record.Payload, "abcdefghijklmnopqrstuvwxyz") || strings.Contains(record.Payload, `"authorization":"secret"`) {
		t.Errorf("credential was retained: %s", record.Payload)
	}
	if !strings.Contains(record.Payload, "[REDACTED]") {
		t.Errorf("sanitized payload has no redaction marker: %s", record.Payload)
	}
}

func TestNewLlmRequestAuditBoundsOversizedPayload(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"model":    "gpt-test",
		"messages": []map[string]string{{"role": "system", "content": strings.Repeat("x", auditutil.MaxPayloadBytes*2)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	record := NewLlmRequestAudit(raw, "gpt-test", "admin/default", "127.0.0.1", false)
	if record == nil || !record.Truncated || record.OriginalBytes <= auditutil.MaxPayloadBytes || len(record.Payload) > auditutil.MaxPayloadBytes {
		t.Errorf("unexpected bounded record: %#v", record)
	}
}

func TestNewLlmRequestAuditRejectsInvalidJSON(t *testing.T) {
	if record := NewLlmRequestAudit([]byte(`{"model":`), "gpt-test", "admin/default", "127.0.0.1", false); record != nil {
		t.Errorf("invalid JSON created an audit record: %#v", record)
	}
}
