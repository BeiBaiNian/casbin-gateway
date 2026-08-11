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

func (c *ApiController) tokenAccess(owner string) bool {
	user := c.GetSessionUser()
	return user != nil && (user.IsAdmin || owner == c.GetSessionUsername())
}

// GetTokens returns a paginated list of tokens with tenant isolation.
func (c *ApiController) GetTokens() {
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
		tokens, err := object.GetTokens(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(object.GetMaskedTokens(tokens))
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetTokenCount(owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	p := pagination.SetPaginator(c.Ctx, limitInt, count)
	tokens, err := object.GetPaginationTokens(owner, p.Offset(), limitInt)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedTokens(tokens), p.Nums())
}

// GetToken returns a single token by owner/name id.
func (c *ApiController) GetToken() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _ := util.GetOwnerAndNameFromId(id)
	if !c.tokenAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	token, err := object.GetToken(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedToken(token))
}

// AddToken creates a new token and returns the one-time secret key in the response.
func (c *ApiController) AddToken() {
	if c.RequireSignedIn() {
		return
	}

	var token object.Token
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &token); err != nil {
		c.ResponseError(err.Error())
		return
	}

	token.Owner = c.GetSessionUsername()
	affected, secretKey, err := object.AddToken(&token)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(map[string]interface{}{
		"affected":  affected,
		"secretKey": secretKey,
	})
}

// UpdateToken updates an existing token.
func (c *ApiController) UpdateToken() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _ := util.GetOwnerAndNameFromId(id)
	if !c.tokenAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	var token object.Token
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &token); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateToken(id, &token))
	c.ServeJSON()
}

// DeleteToken deletes a token.
func (c *ApiController) DeleteToken() {
	if c.RequireSignedIn() {
		return
	}

	var token object.Token
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &token); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !c.tokenAccess(token.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteToken(&token))
	c.ServeJSON()
}
