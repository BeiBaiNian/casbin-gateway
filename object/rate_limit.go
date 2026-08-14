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

// rateLimitWindow is the length of one rate limiting window. It is a variable
// so tests can shrink it and exercise the window reset path.
var rateLimitWindow = time.Minute

// cleanupInterval is how often the stale entry sweep may run at most.
var cleanupInterval = time.Minute

type rateLimitEntry struct {
	// mu guards count and resetAt: sync.Map only protects the map itself, not
	// the fields of the entries it stores. Without the mutex, concurrent
	// requests would race on count++ and resetAt and silently drop counts.
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

var (
	rateLimitMap sync.Map

	// cleanupMu guards lastCleanupAt so the sweep itself does not race.
	cleanupMu     sync.Mutex
	lastCleanupAt time.Time
)

// CheckRateLimit returns true if the request is allowed (within rate limit).
// limit <= 0 means no rate limiting.
func CheckRateLimit(tokenOwner, tokenName string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("%s/%s", tokenOwner, tokenName)
	now := time.Now()

	val, _ := rateLimitMap.LoadOrStore(key, &rateLimitEntry{
		resetAt: now.Add(rateLimitWindow),
	})
	entry := val.(*rateLimitEntry)

	entry.mu.Lock()
	if now.After(entry.resetAt) {
		// The window has expired: this request opens a fresh window.
		entry.count = 1
		entry.resetAt = now.Add(rateLimitWindow)
		entry.mu.Unlock()
		cleanupStaleEntries(now)
		return true, nil
	}

	entry.count++
	allowed := entry.count <= limit
	entry.mu.Unlock()
	if allowed {
		cleanupStaleEntries(now)
	}
	return allowed, nil
}

// cleanupStaleEntries deletes entries whose window ended more than one window
// ago. It runs at most once per cleanupInterval: sweeping the whole map on
// every request would be an unnecessary O(N) scan, so it is throttled by a
// timestamp and stale entries at most linger for an extra interval.
func cleanupStaleEntries(now time.Time) {
	cleanupMu.Lock()
	if now.Sub(lastCleanupAt) < cleanupInterval {
		cleanupMu.Unlock()
		return
	}
	lastCleanupAt = now
	cleanupMu.Unlock()

	rateLimitMap.Range(func(k, v interface{}) bool {
		e := v.(*rateLimitEntry)
		e.mu.Lock()
		stale := now.After(e.resetAt.Add(rateLimitWindow))
		e.mu.Unlock()
		if stale {
			rateLimitMap.Delete(k)
		}
		return true
	})
}
