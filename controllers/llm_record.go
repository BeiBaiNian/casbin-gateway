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

package controllers

import (
	"strconv"

	"github.com/apache/casbin-gateway/object"
)

// A record holds what a user asked a model, so unlike the proxy endpoints
// themselves these are limited to Gateway administrators.

// GetLlmRecords lists one page of relayed requests, without their bodies.
func (c *ApiController) GetLlmRecords() {
	if c.RequireAdmin() {
		return
	}

	page, _ := strconv.Atoi(c.Input().Get("p"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Input().Get("pageSize"))
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	filter := object.LlmRecordFilter{
		Model:    c.Input().Get("model"),
		Channel:  c.Input().Get("channel"),
		Agent:    c.Input().Get("agent"),
		ClientIp: c.Input().Get("clientIp"),
		Outcome:  c.Input().Get("outcome"),
	}
	records, count, err := object.GetLlmRecords(filter, (page-1)*limit, limit)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(records, count)
}

// GetLlmRecord returns one record with its stored request body.
func (c *ApiController) GetLlmRecord() {
	if c.RequireAdmin() {
		return
	}

	id, err := strconv.ParseInt(c.Input().Get("id"), 10, 64)
	if err != nil {
		c.ResponseError("invalid record id")
		return
	}
	record, err := object.GetLlmRecord(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(record)
}

// GetLlmRecordStatus reports the recorder's own settings and how many records
// it had to drop, so a gap in the list can be explained.
func (c *ApiController) GetLlmRecordStatus() {
	if c.RequireAdmin() {
		return
	}

	status, err := object.GetLlmRecordStatus()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(status)
}

func (c *ApiController) DeleteLlmRecord() {
	if c.RequireAdmin() {
		return
	}

	id, err := strconv.ParseInt(c.Input().Get("id"), 10, 64)
	if err != nil {
		c.ResponseError("invalid record id")
		return
	}
	if err := object.DeleteLlmRecord(id); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// ClearLlmRecords drops every record, for the operator who has to answer a
// request to erase retained prompts.
func (c *ApiController) ClearLlmRecords() {
	if c.RequireAdmin() {
		return
	}

	if err := object.ClearLlmRecords(); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}
