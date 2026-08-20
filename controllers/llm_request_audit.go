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

// GetLlmRequestAudits returns retained, sanitized model requests. Prompt
// contents are sensitive operational data, so unlike the public model endpoint
// this endpoint is always limited to Gateway administrators.
func (c *ApiController) GetLlmRequestAudits() {
	if c.RequireAdmin() {
		return
	}
	page, _ := strconv.Atoi(c.Input().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Input().Get("limit"))
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	records, count, err := object.GetLlmRequestAudits((page-1)*limit, limit)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(records, count)
}
