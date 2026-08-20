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

import i18next from "i18next";

/** Labels are read at call time so a language switch re-translates them. */
export function getRuleTypes() {
  return [
    {value: "WAF", label: "WAF"},
    {value: "IP", label: "IP"},
    {value: "User-Agent", label: "User-Agent"},
    {value: "URL Path", label: "URL Path"},
    {value: "IP Rate Limiting", label: i18next.t("rule:IP Rate Limiting")},
    {value: "Compound", label: i18next.t("rule:Compound")},
  ];
}

export function getRuleActions() {
  return [
    {value: "Allow", label: i18next.t("rule:Allow")},
    {value: "Block", label: i18next.t("rule:Block")},
    {value: "CAPTCHA", label: i18next.t("rule:Captcha")},
  ];
}
