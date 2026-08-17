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

package routers

import (
	"net/http"
	"strings"

	"github.com/apache/casbin-gateway/embedsupport"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
	"github.com/beego/beego/context"
)

// webBuildDir holds the compiled web UI. A build made with -tags embed also
// carries a copy of it inside the binary, used only when this directory has no
// index.html. The whole UI comes from one source or the other, so a partial
// build on disk is never quietly completed with files from the binary.
const webBuildDir = "web/build"

func TransparentStatic(ctx *context.Context) {
	urlPath := ctx.Request.URL.Path
	if strings.HasPrefix(urlPath, "/api/") || strings.HasPrefix(urlPath, "/v1/") {
		return
	}

	indexPath := webBuildDir + "/index.html"
	if !util.FileExist(indexPath) && embedsupport.HasWeb() {
		embedsupport.ServeWeb(ctx.ResponseWriter, ctx.Request, urlPath)
		return
	}

	path := webBuildDir
	if urlPath == "/" {
		path = indexPath
	} else {
		path += urlPath
	}

	if util.FileExist(path) {
		http.ServeFile(ctx.ResponseWriter, ctx.Request, path)
	} else {
		http.ServeFile(ctx.ResponseWriter, ctx.Request, indexPath)
	}
}

func ApiFilter(ctx *context.Context) {
	if beego.AppConfig.DefaultBool("isDemoMode", false) && !isAllowedInDemoMode(ctx.Request.Method, ctx.Request.URL.Path) {
		denyRequest(ctx)
	}
}

func isAllowedInDemoMode(method string, urlPath string) bool {
	if method == "POST" {
		return urlPath == "/api/signin" || urlPath == "/api/signout"
	}

	// If method equals GET
	return true
}
