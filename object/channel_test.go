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

package object

import "testing"

func TestBuildOpenAiUrl(t *testing.T) {
	cases := []struct {
		name     string
		baseUrl  string
		endpoint string
		expected string
	}{
		{"bare host", "https://api.openai.com", "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"bare host with trailing slash", "https://api.openai.com/", "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"with /v1 prefix", "https://api.openai.com/v1", "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"with /v1 prefix and trailing slash", "https://api.openai.com/v1/", "/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"subpath ending in /v1", "https://oneapi.example.com/api/v1", "/chat/completions", "https://oneapi.example.com/api/v1/chat/completions"},
		{"non-v1 subpath is not mistaken for /v1", "https://x.com/api/v1beta", "/chat/completions", "https://x.com/api/v1beta/v1/chat/completions"},
		{"endpoint parameterization", "https://api.openai.com", "/models", "https://api.openai.com/v1/models"},
	}

	for _, tc := range cases {
		if got := BuildOpenAiUrl(tc.baseUrl, tc.endpoint); got != tc.expected {
			t.Errorf("%s: BuildOpenAiUrl(%q, %q) = %q, expected %q", tc.name, tc.baseUrl, tc.endpoint, got, tc.expected)
		}
	}
}
