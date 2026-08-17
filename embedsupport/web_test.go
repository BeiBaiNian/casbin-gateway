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

package embedsupport

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestServeWeb(t *testing.T) {
	webFS = fstest.MapFS{
		"index.html":              {Data: []byte("<html>index</html>")},
		"static/js/main.abc.js":   {Data: []byte("console.log(1);")},
		"static/css/main.abc.css": {Data: []byte("body{}")},
	}
	defer func() { webFS = nil }()

	tests := []struct {
		name     string
		urlPath  string
		wantBody string
		wantType string
	}{
		{
			name:     "root serves index.html",
			urlPath:  "/",
			wantBody: "<html>index</html>",
			wantType: "text/html; charset=utf-8",
		},
		{
			name:     "asset is served from its own path",
			urlPath:  "/static/js/main.abc.js",
			wantBody: "console.log(1);",
		},
		{
			// A browser-side route of the single-page app: it has no file of
			// its own, and index.html has to answer or a reload 404s.
			name:     "unknown path falls back to index.html",
			urlPath:  "/sites/example.com",
			wantBody: "<html>index</html>",
			wantType: "text/html; charset=utf-8",
		},
		{
			// Anything that climbs out of the tree must not reach the caller's
			// filesystem; it lands on index.html like any other unknown path.
			name:     "path traversal cannot escape the embedded tree",
			urlPath:  "/../../conf/app.conf",
			wantBody: "<html>index</html>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ServeWeb(recorder, httptest.NewRequest(http.MethodGet, "http://localhost"+test.urlPath, nil), test.urlPath)

			if recorder.Code != http.StatusOK {
				t.Fatalf("got status %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Body.String(); got != test.wantBody {
				t.Errorf("got body %q, want %q", got, test.wantBody)
			}
			if test.wantType != "" {
				if got := recorder.Header().Get("Content-Type"); got != test.wantType {
					t.Errorf("got Content-Type %q, want %q", got, test.wantType)
				}
			}
		})
	}
}

// TestServeWebWithoutIndex checks the one case that has no fallback: an
// embedded tree so broken that even index.html is missing.
func TestServeWebWithoutIndex(t *testing.T) {
	webFS = fstest.MapFS{}
	defer func() { webFS = nil }()

	recorder := httptest.NewRecorder()
	ServeWeb(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/", nil), "/")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
