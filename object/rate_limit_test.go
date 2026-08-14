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
	"testing"
	"time"
)

// withShortWindow temporarily shrinks the rate limit window and the cleanup
// interval so the window reset and cleanup paths can be exercised without
// waiting a full minute. The tests in this package are serial (no t.Parallel),
// so mutating the globals is safe here.
func withShortWindow(t *testing.T, f func()) {
	t.Helper()
	oldWindow, oldInterval := rateLimitWindow, cleanupInterval
	rateLimitWindow, cleanupInterval = 50*time.Millisecond, 1*time.Nanosecond
	defer func() {
		rateLimitWindow, cleanupInterval = oldWindow, oldInterval
	}()
	f()
}

func TestCheckRateLimitNoLimit(t *testing.T) {
	allowed, err := CheckRateLimit("owner", "unlimited", 0)
	if err != nil || !allowed {
		t.Errorf("limit <= 0 must always allow, got allowed=%v err=%v", allowed, err)
	}
}

func TestCheckRateLimitWindow(t *testing.T) {
	withShortWindow(t, func() {
		key := fmt.Sprintf("owner-%d", time.Now().UnixNano())

		allowed, _ := CheckRateLimit(key, "tok", 1)
		if !allowed {
			t.Fatal("first request should be allowed")
		}
		allowed, _ = CheckRateLimit(key, "tok", 1)
		if allowed {
			t.Fatal("second request within the same window should be rejected")
		}

		// After the window passes, the counter resets and the request is
		// allowed again.
		time.Sleep(80 * time.Millisecond)
		allowed, _ = CheckRateLimit(key, "tok", 1)
		if !allowed {
			t.Fatal("request after the window should be allowed again")
		}
	})
}

// TestCheckRateLimitConcurrent checks that concurrent requests do not drop
// counts. Exactly limit out of limit+1 concurrent calls must be allowed; with
// a data race on entry.count some of the increments would be lost and more
// calls would pass. Run with -race to catch the race itself.
func TestCheckRateLimitConcurrent(t *testing.T) {
	const limit = 50
	const callers = limit + 1

	// All goroutines share the same key so they contend on one entry.
	tokenName := fmt.Sprintf("tok-%d", time.Now().UnixNano())

	results := make([]bool, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			allowed, err := CheckRateLimit("concurrent-owner", tokenName, limit)
			if err != nil {
				t.Errorf("CheckRateLimit failed: %v", err)
				return
			}
			results[i] = allowed
		}(i)
	}
	wg.Wait()

	allowedCount := 0
	for _, allowed := range results {
		if allowed {
			allowedCount++
		}
	}
	if allowedCount != limit {
		t.Errorf("allowed %d out of %d concurrent calls, expected exactly %d (counts were dropped)", allowedCount, callers, limit)
	}
}

func TestCheckRateLimitStaleCleanup(t *testing.T) {
	withShortWindow(t, func() {
		key := fmt.Sprintf("cleanup-%d", time.Now().UnixNano())

		// Create an entry and let its window end.
		allowed, _ := CheckRateLimit(key, "tok", 10)
		if !allowed {
			t.Fatal("first request should be allowed")
		}
		time.Sleep(80 * time.Millisecond)

		// The next allowed request triggers the throttled sweep, which must
		// remove the stale entry so the counter starts over.
		allowed, _ = CheckRateLimit(key, "tok", 10)
		if !allowed {
			t.Fatal("request after cleanup should be allowed")
		}
	})
}
