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

import (
	"fmt"
	"sync"
	"time"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

var rateLimitMap sync.Map

// CheckRateLimit returns true if the request is allowed (within rate limit).
// limit <= 0 means no rate limiting.
func CheckRateLimit(tokenOwner, tokenName string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("%s/%s", tokenOwner, tokenName)
	now := time.Now()

	val, _ := rateLimitMap.LoadOrStore(key, &rateLimitEntry{
		count:   0,
		resetAt: now.Add(time.Minute),
	})
	entry := val.(*rateLimitEntry)

	if now.After(entry.resetAt) {
		entry.count = 1
		entry.resetAt = now.Add(time.Minute)
		return true, nil
	}

	entry.count++
	if entry.count > limit {
		return false, nil
	}

	// Periodic lazy cleanup of stale entries (every 100 requests).
	// Avoids the O(N) Range cost on every single request.
	if entry.count%100 == 0 {
		rateLimitMap.Range(func(k, v interface{}) bool {
			e := v.(*rateLimitEntry)
			if now.After(e.resetAt.Add(time.Minute)) {
				rateLimitMap.Delete(k)
			}
			return true
		})
	}

	return true, nil
}
