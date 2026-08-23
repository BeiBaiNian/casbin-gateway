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
	"github.com/apache/casbin-gateway/version"
)

// GetVersion tells the web UI which build this is and which one is published.
// "refresh=1" asks GitHub again instead of answering from the cache.
func (c *ApiController) GetVersion() {
	if c.RequireSignedIn() {
		return
	}

	c.ResponseOk(version.Describe(c.Input().Get("refresh") == "1"))
}

// UpdateGateway replaces this executable with the published build and restarts
// into it. It answers as soon as the download starts, because the rest takes
// long enough that the web UI follows it through GetUpdateStatus.
func (c *ApiController) UpdateGateway() {
	if c.RequireAdmin() {
		return
	}

	if err := version.StartUpdate(); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(version.UpdateStatus())
}

// GetUpdateStatus is polled while an update runs. Once the new Gateway is
// starting this stops answering at all, which is the web UI's cue to wait for
// the restarted one.
func (c *ApiController) GetUpdateStatus() {
	if c.RequireSignedIn() {
		return
	}

	c.ResponseOk(version.UpdateStatus())
}
