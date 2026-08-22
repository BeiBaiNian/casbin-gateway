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

/** "KEY=VALUE" per line, which is how env vars and headers are written in the
 * forms here. */
export function parsePairs(text: string) {
  const pairs: Record<string, string> = {};
  text.split("\n").forEach(line => {
    const separator = line.indexOf("=");
    if (separator <= 0) {
      return;
    }
    const key = line.slice(0, separator).trim();
    if (key !== "") {
      pairs[key] = line.slice(separator + 1).trim();
    }
  });
  return pairs;
}

export function formatPairs(pairs: Record<string, string> | undefined) {
  return Object.entries(pairs ?? {})
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}
