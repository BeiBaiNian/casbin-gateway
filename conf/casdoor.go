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

package conf

import "sync/atomic"

// casdoorAvailable records whether Casdoor is configured, i.e. whether the SDK
// may be called at all. Set by casdoor.InitCasdoorConfig at startup.
var casdoorAvailable atomic.Bool

// SetCasdoorAvailable records whether Casdoor was initialized successfully.
func SetCasdoorAvailable(available bool) {
	casdoorAvailable.Store(available)
}

// IsCasdoorAvailable reports whether Casdoor is configured and may be used for
// SDK calls. Casdoor is optional: when it is not configured, Gateway falls back
// to its own local user table for sign-in.
func IsCasdoorAvailable() bool {
	return casdoorAvailable.Load()
}
