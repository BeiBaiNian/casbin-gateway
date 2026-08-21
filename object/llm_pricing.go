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
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
)

// LlmPrice is what one million tokens of each kind costs, in US dollars. APIs
// that discount cached input instead of charging for the write leave
// CacheWrite at zero.
type LlmPrice struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cacheWrite"`
	CacheRead  float64 `json:"cacheRead"`
}

// llmPrices are published list prices, keyed by the part of a model name that
// identifies the model. They go stale, so llmPricingFile overrides them.
var llmPrices = map[string]LlmPrice{
	"claude-opus-4":     {15, 75, 18.75, 1.5},
	"claude-3-opus":     {15, 75, 18.75, 1.5},
	"claude-sonnet-4":   {3, 15, 3.75, 0.3},
	"claude-3-7-sonnet": {3, 15, 3.75, 0.3},
	"claude-3-5-sonnet": {3, 15, 3.75, 0.3},
	"claude-haiku-4-5":  {1, 5, 1.25, 0.1},
	"claude-3-5-haiku":  {0.8, 4, 1, 0.08},
	"claude-3-haiku":    {0.25, 1.25, 0.3, 0.03},

	"gpt-4o":       {2.5, 10, 0, 1.25},
	"gpt-4o-mini":  {0.15, 0.6, 0, 0.075},
	"gpt-4.1":      {2, 8, 0, 0.5},
	"gpt-4.1-mini": {0.4, 1.6, 0, 0.1},
	"gpt-4.1-nano": {0.1, 0.4, 0, 0.025},
	"o3":           {2, 8, 0, 0.5},
	"o3-mini":      {1.1, 4.4, 0, 0.55},
	"o4-mini":      {1.1, 4.4, 0, 0.275},

	"deepseek-chat":     {0.27, 1.1, 0, 0.07},
	"deepseek-reasoner": {0.55, 2.19, 0, 0.14},
}

var (
	pricingOnce sync.Once
	pricingKeys []string
)

// loadLlmPrices merges the override file over the built-in table, longest key
// first so a specific model wins over the family it belongs to.
func loadLlmPrices() {
	path := conf.GetLlmPricingFile()
	if path != "" {
		if data, err := os.ReadFile(path); err != nil {
			if !os.IsNotExist(err) {
				beego.Error("LLM pricing file could not be read:", err)
			}
		} else {
			overrides := map[string]LlmPrice{}
			if err := json.Unmarshal(data, &overrides); err != nil {
				beego.Error("LLM pricing file is not valid JSON:", err)
			} else {
				for model, price := range overrides {
					llmPrices[strings.ToLower(model)] = price
				}
			}
		}
	}

	pricingKeys = make([]string, 0, len(llmPrices))
	for key := range llmPrices {
		pricingKeys = append(pricingKeys, key)
	}
	sort.Slice(pricingKeys, func(i, j int) bool { return len(pricingKeys[i]) > len(pricingKeys[j]) })
}

// GetLlmPrice matches a model name in any of the shapes it arrives in, e.g.
// "claude-sonnet-4-20250514" or "us.anthropic.claude-sonnet-4-v1:0". It reports
// false when nothing matches, so an unpriced model is not costed at zero.
func GetLlmPrice(model string) (LlmPrice, bool) {
	pricingOnce.Do(loadLlmPrices)

	name := strings.ToLower(strings.TrimSpace(model))
	for _, key := range pricingKeys {
		if strings.Contains(name, key) {
			return llmPrices[key], true
		}
	}
	return LlmPrice{}, false
}

// GetLlmCost is what one recorded request cost, in US dollars.
func GetLlmCost(model string, promptTokens int, completionTokens int, cacheWriteTokens int, cacheReadTokens int) (float64, bool) {
	price, ok := GetLlmPrice(model)
	if !ok {
		return 0, false
	}

	cost := float64(promptTokens)*price.Input +
		float64(completionTokens)*price.Output +
		float64(cacheWriteTokens)*price.CacheWrite +
		float64(cacheReadTokens)*price.CacheRead
	return cost / 1e6, true
}
