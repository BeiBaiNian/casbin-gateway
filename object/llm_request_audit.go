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
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

// LlmRequestAudit is a sanitized OpenAI-compatible request snapshot. It is
// intentionally request-only: response transcript and usage capture belong to
// a later feature and must not change this proxy's pass-through behaviour.
type LlmRequestAudit struct {
	Id            int64  `xorm:"int notnull pk autoincr" json:"id"`
	CreatedTime   string `xorm:"varchar(100) notnull index" json:"createdTime"`
	Model         string `xorm:"varchar(255) notnull index" json:"model"`
	Channel       string `xorm:"varchar(201)" json:"channel"`
	ClientIp      string `xorm:"varchar(100)" json:"clientIp"`
	Stream        bool   `xorm:"bool" json:"stream"`
	Payload       string `xorm:"mediumtext" json:"payload"`
	Truncated     bool   `xorm:"bool" json:"truncated"`
	OriginalBytes int    `xorm:"int" json:"originalBytes"`
}

type llmRequestAuditWriter struct {
	queue       chan llmRequestAuditTask
	done        chan struct{}
	once        sync.Once
	queueMutex  sync.RWMutex
	stopped     bool
	dropMutex   sync.Mutex
	dropped     uint64
	lastDropLog time.Time
}

type llmRequestAuditTask struct {
	rawBody       []byte
	model         string
	channel       string
	clientIP      string
	stream        bool
	originalBytes int
	bodyOmitted   bool
}

var requestAuditWriter llmRequestAuditWriter

// StartLlmRequestAuditWriter starts the best-effort persistent writer. It is
// deliberately opt-in so installations that do not request content retention
// neither retain prompts nor pay any proxy-path cost.
func StartLlmRequestAuditWriter() {
	if !conf.IsLlmRequestAuditEnabled() {
		return
	}
	requestAuditWriter.once.Do(func() {
		requestAuditWriter.queue = make(chan llmRequestAuditTask, conf.GetLlmRequestAuditQueueCapacity())
		requestAuditWriter.done = make(chan struct{})
		go requestAuditWriter.run()
	})
}

// StopLlmRequestAuditWriter drains accepted entries during graceful shutdown.
func StopLlmRequestAuditWriter() {
	requestAuditWriter.queueMutex.Lock()
	if requestAuditWriter.queue == nil || requestAuditWriter.stopped {
		requestAuditWriter.queueMutex.Unlock()
		return
	}
	queue, done := requestAuditWriter.queue, requestAuditWriter.done
	requestAuditWriter.stopped = true
	close(queue)
	requestAuditWriter.queueMutex.Unlock()
	<-done
}

// EnqueueLlmRequestAudit never parses, sanitizes, or writes the request on the
// proxy goroutine. Oversized bodies are represented by safe metadata so the
// bounded queue cannot retain arbitrary amounts of request memory.
func EnqueueLlmRequestAudit(rawBody []byte, model, channel, clientIP string, stream bool) {
	task := llmRequestAuditTask{
		rawBody: rawBody, model: model, channel: channel, clientIP: clientIP, stream: stream,
		originalBytes: len(rawBody),
	}
	if len(rawBody) > auditutil.MaxPayloadBytes {
		task.rawBody, task.bodyOmitted = nil, true
	}
	requestAuditWriter.queueMutex.RLock()
	defer requestAuditWriter.queueMutex.RUnlock()
	if requestAuditWriter.queue == nil || requestAuditWriter.stopped {
		return
	}
	select {
	case requestAuditWriter.queue <- task:
	default:
		requestAuditWriter.noteDrop()
	}
}

func (writer *llmRequestAuditWriter) noteDrop() {
	writer.dropMutex.Lock()
	defer writer.dropMutex.Unlock()
	writer.dropped++
	if time.Since(writer.lastDropLog) < time.Minute {
		return
	}
	beego.Error("LLM request audit queue is full; dropped audit records:", writer.dropped)
	writer.lastDropLog = time.Now()
	writer.dropped = 0
}

func (writer *llmRequestAuditWriter) run() {
	defer close(writer.done)
	writer.prune()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case task, ok := <-writer.queue:
			if !ok {
				return
			}
			record := task.record()
			if record == nil {
				beego.Error("LLM request audit serialization failed")
				continue
			}
			if _, err := ormer.Engine.Insert(record); err != nil {
				beego.Error("LLM request audit write failed:", err)
			}
		case <-ticker.C:
			writer.prune()
		}
	}
}

func (writer *llmRequestAuditWriter) prune() {
	if ormer == nil || ormer.Engine == nil {
		return
	}
	cutoff := util.FormatTime(time.Now().AddDate(0, 0, -conf.GetLlmRequestAuditRetentionDays()))
	if _, err := ormer.Engine.Where("created_time < ?", cutoff).Delete(&LlmRequestAudit{}); err != nil {
		beego.Error("LLM request audit retention cleanup failed:", err)
		return
	}

	count, err := ormer.Engine.Count(&LlmRequestAudit{})
	if err != nil {
		beego.Error("LLM request audit retention count failed:", err)
		return
	}
	if count <= int64(conf.GetLlmRequestAuditMaxRecords()) {
		return
	}
	excess := int(count) - conf.GetLlmRequestAuditMaxRecords()
	oldest := make([]LlmRequestAudit, 0, excess)
	if err := ormer.Engine.Asc("id").Limit(excess).Find(&oldest); err != nil {
		beego.Error("LLM request audit retention lookup failed:", err)
		return
	}
	ids := make([]int64, 0, len(oldest))
	for _, record := range oldest {
		ids = append(ids, record.Id)
	}
	if len(ids) > 0 {
		if _, err := ormer.Engine.In("id", ids).Delete(&LlmRequestAudit{}); err != nil {
			beego.Error("LLM request audit retention deletion failed:", err)
		}
	}
}

func (task llmRequestAuditTask) record() *LlmRequestAudit {
	if task.bodyOmitted {
		return &LlmRequestAudit{
			CreatedTime: util.GetCurrentTime(), Model: task.model, Channel: task.channel, ClientIp: task.clientIP,
			Stream: task.stream, Truncated: true, OriginalBytes: task.originalBytes,
			Payload: auditutil.EncodeBoundedJSON(map[string]any{
				"truncated": true, "originalBytes": task.originalBytes,
				"reason": "request body exceeds the audit capture limit",
			}, auditutil.MaxPayloadBytes),
		}
	}
	return NewLlmRequestAudit(task.rawBody, task.model, task.channel, task.clientIP, task.stream)
}

// NewLlmRequestAudit sanitizes a queued request body on the writer goroutine.
// This is the sole body-retention boundary: headers, especially inbound
// authorization credentials, are never copied into an audit record.
func NewLlmRequestAudit(rawBody []byte, model, channel, clientIP string, stream bool) *LlmRequestAudit {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return nil
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil
	}
	sanitized := auditutil.SanitizeValue("", decoded)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return nil
	}
	record := &LlmRequestAudit{
		CreatedTime: util.GetCurrentTime(), Model: model, Channel: channel, ClientIp: clientIP,
		Stream: stream, OriginalBytes: len(rawBody),
	}
	if len(encoded) <= auditutil.MaxPayloadBytes {
		record.Payload = string(encoded)
		return record
	}
	record.Truncated = true
	record.Payload = auditutil.EncodeBoundedJSON(sanitized, auditutil.MaxPayloadBytes)
	return record
}

func GetLlmRequestAudits(offset, limit int) ([]*LlmRequestAudit, int64, error) {
	records := []*LlmRequestAudit{}
	count, err := ormer.Engine.Count(&LlmRequestAudit{})
	if err != nil {
		return nil, 0, err
	}
	err = ormer.Engine.Desc("id").Limit(limit, offset).Find(&records)
	return records, count, err
}
