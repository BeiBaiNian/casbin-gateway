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

package object

import (
	"sort"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/util"
)

const (
	// channelFailureThreshold is how many failures in a row take a channel out
	// of rotation. One failure is not enough: an upstream that answers 500 once
	// is still the channel the operator picked.
	channelFailureThreshold = 2
	channelCooldown         = 15 * time.Second
	channelMaxCooldown      = 5 * time.Minute
)

// ChannelHealth is what the proxy has seen of one channel. It lives in memory
// only: it describes this process's view of an upstream, not a stored setting.
type ChannelHealth struct {
	Channel     string `json:"channel"`
	Healthy     bool   `json:"healthy"`
	Successes   int64  `json:"successes"`
	Failures    int64  `json:"failures"`
	Consecutive int    `json:"consecutive"`
	LastError   string `json:"lastError"`
	LastFailure string `json:"lastFailure"`
	RetryTime   string `json:"retryTime"`
}

type channelHealth struct {
	successes   int64
	failures    int64
	consecutive int
	lastError   string
	lastFailure time.Time
	retryTime   time.Time
}

var (
	channelHealthMutex sync.Mutex
	channelHealthMap   = map[string]*channelHealth{}
)

// ReportChannelSuccess closes the breaker: an upstream that answered is back in
// rotation whatever it did before.
func ReportChannelSuccess(channelId string) {
	channelHealthMutex.Lock()
	defer channelHealthMutex.Unlock()

	health := healthOf(channelId)
	health.successes++
	health.consecutive = 0
	health.retryTime = time.Time{}
}

// ReportChannelFailure records an attempt that could not be relayed. Once a
// channel has failed channelFailureThreshold times in a row it is suspended for
// a window that doubles with every further failure, so a dead upstream stops
// costing every request the time it takes to time out.
func ReportChannelFailure(channelId string, reason string) {
	channelHealthMutex.Lock()
	defer channelHealthMutex.Unlock()

	health := healthOf(channelId)
	health.failures++
	health.consecutive++
	health.lastError = reason
	health.lastFailure = time.Now()

	if health.consecutive < channelFailureThreshold {
		return
	}
	cooldown := channelCooldown << (health.consecutive - channelFailureThreshold)
	if cooldown > channelMaxCooldown || cooldown <= 0 {
		cooldown = channelMaxCooldown
	}
	health.retryTime = time.Now().Add(cooldown)
}

// IsChannelSuspended reports whether a channel is inside its cooldown window.
// A suspended channel is tried last rather than dropped: it may be the only one
// the agent has.
func IsChannelSuspended(channelId string) bool {
	channelHealthMutex.Lock()
	defer channelHealthMutex.Unlock()

	health, ok := channelHealthMap[channelId]
	return ok && time.Now().Before(health.retryTime)
}

// ClearChannelHealth forgets what the proxy saw of a channel, which is what an
// edited channel deserves: its base URL or key may be the thing that was fixed.
func ClearChannelHealth(channelId string) {
	channelHealthMutex.Lock()
	defer channelHealthMutex.Unlock()
	delete(channelHealthMap, channelId)
}

// GetChannelHealth lists what is known about every channel used since startup.
func GetChannelHealth() []*ChannelHealth {
	channelHealthMutex.Lock()
	defer channelHealthMutex.Unlock()

	now := time.Now()
	result := make([]*ChannelHealth, 0, len(channelHealthMap))
	for id, health := range channelHealthMap {
		item := &ChannelHealth{
			Channel:     id,
			Healthy:     !now.Before(health.retryTime),
			Successes:   health.successes,
			Failures:    health.failures,
			Consecutive: health.consecutive,
			LastError:   health.lastError,
		}
		if !health.lastFailure.IsZero() {
			item.LastFailure = util.FormatTime(health.lastFailure)
		}
		if now.Before(health.retryTime) {
			item.RetryTime = util.FormatTime(health.retryTime)
		}
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Channel < result[j].Channel })
	return result
}

// SortChannelsByHealth puts the channels that are inside a cooldown window last,
// keeping the configured order among the rest.
func SortChannelsByHealth(channels []*Channel) []*Channel {
	ready := make([]*Channel, 0, len(channels))
	suspended := []*Channel{}
	for _, channel := range channels {
		if IsChannelSuspended(channel.GetId()) {
			suspended = append(suspended, channel)
			continue
		}
		ready = append(ready, channel)
	}
	return append(ready, suspended...)
}

func healthOf(channelId string) *channelHealth {
	health, ok := channelHealthMap[channelId]
	if !ok {
		health = &channelHealth{}
		channelHealthMap[channelId] = health
	}
	return health
}
