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
	"encoding/json"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego/utils/pagination"
)

func (c *ApiController) channelAccess(owner string) bool {
	user := c.GetSessionUser()
	return user != nil && (user.IsAdmin || owner == c.GetSessionUsername())
}

// GetChannels returns a paginated list of channels with tenant isolation.
func (c *ApiController) GetChannels() {
	if c.RequireSignedIn() {
		return
	}

	user := c.GetSessionUser()
	owner := c.Input().Get("owner")

	if !user.IsAdmin || owner == "" || owner == "admin" {
		if !user.IsAdmin {
			owner = c.GetSessionUsername()
		} else if owner == "" || owner == "admin" {
			owner = ""
		}
	}

	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	if limit == "" || page == "" {
		channels, err := object.GetChannels(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(object.GetMaskedChannels(channels))
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetChannelCount(owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	p := pagination.SetPaginator(c.Ctx, limitInt, count)
	channels, err := object.GetPaginationChannels(owner, p.Offset(), limitInt)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedChannels(channels), p.Nums())
}

// GetChannel returns a single channel by owner/name id.
func (c *ApiController) GetChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _ := util.GetOwnerAndNameFromId(id)
	if !c.channelAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	channel, err := object.GetChannel(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedChannel(channel))
}

// AddChannel creates a new channel.
func (c *ApiController) AddChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	channel.Owner = c.GetSessionUsername()
	c.Data["json"] = wrapActionResponse(object.AddChannel(&channel))
	c.ServeJSON()
}

// UpdateChannel updates an existing channel.
func (c *ApiController) UpdateChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _ := util.GetOwnerAndNameFromId(id)
	if !c.channelAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateChannel(id, &channel))
	c.ServeJSON()
}

// DeleteChannel deletes a channel.
func (c *ApiController) DeleteChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !c.channelAccess(channel.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteChannel(&channel))
	c.ServeJSON()
}

// TestChannel tests connectivity to an upstream channel.
func (c *ApiController) TestChannel() {
	if c.RequireSignedIn() {
		return
	}

	var request struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !c.channelAccess(request.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	channel := &object.Channel{Owner: request.Owner, Name: request.Name}
	ok, code, msg := object.TestChannelConnectivity(channel, "")
	c.ResponseOk(map[string]interface{}{"success": ok, "statusCode": code, "message": msg})
}
