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

import {query, request} from "@/backend/request";
import type {
  AgentConfigDetail,
  AgentConfigInventory,
  AgentConfigKind,
  AgentConfigPlanItem,
} from "@/types";

export interface CopyRequest {
  owner: string;
  from: string;
  to: string[];
  kind: AgentConfigKind;
  names: string[];
  overwrite: boolean;
}

export function getAgentConfigs(forceRefresh = false) {
  return request<AgentConfigInventory[]>(`/api/get-agent-configs${forceRefresh ? "?refresh=true" : ""}`);
}

export function getAgentConfigItem(agentId: string, owner: string, kind: AgentConfigKind, name: string) {
  return request<AgentConfigDetail>(
    `/api/get-agent-config-item${query({agentId: agentId, owner: owner, kind: kind, name: name})}`,
  );
}

export function deleteAgentConfigItem(agentId: string, owner: string, kind: AgentConfigKind, name: string) {
  return request("/api/delete-agent-config-item", "POST", {
    agentId: agentId,
    owner: owner,
    kind: kind,
    name: name,
  });
}

/** What a copy would do, asked for before anything is written. */
export function planAgentConfigCopy(body: CopyRequest) {
  return request<AgentConfigPlanItem[]>("/api/plan-agent-config-copy", "POST", body);
}

export function copyAgentConfig(body: CopyRequest) {
  return request<AgentConfigPlanItem[]>("/api/copy-agent-config", "POST", body);
}
