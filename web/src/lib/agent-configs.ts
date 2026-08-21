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

import * as React from "react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import type {AgentConfigInventory, AgentConfigItem, AgentConfigKind} from "@/types";

/** Skills and MCP servers belong to an account, so an id alone is not unique. */
export function inventoryKey(inventory: Pick<AgentConfigInventory, "agentId" | "owner">) {
  return `${inventory.owner}:${inventory.agentId}`;
}

export function itemsOf(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  return kind === "skill" ? inventory.skills : inventory.mcpServers;
}

export function supports(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  return kind === "skill" ? inventory.skillsSupported : inventory.mcpSupported;
}

/** Where the items of one kind live, shown under the agent it belongs to. */
export function locationOf(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  return (kind === "skill" ? inventory.skillsDir : inventory.mcpFile) ?? "";
}

/**
 * Why an agent cannot be a copy target, in the reader's words. Empty when it
 * can be one.
 */
export function blockedReason(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  if (!supports(inventory, kind)) {
    return i18next.t("agentConfig:Gateway does not know where this agent keeps these");
  }
  if (kind === "mcp" && !inventory.mcpWritable) {
    return inventory.mcpReadOnly || i18next.t("agentConfig:This file is read-only for Gateway");
  }
  return "";
}

/** Which agents already have an item of each name, keyed by name. */
export function presenceOf(inventories: AgentConfigInventory[], kind: AgentConfigKind) {
  const presence = new Map<string, Set<string>>();
  inventories.forEach(inventory => {
    itemsOf(inventory, kind).forEach(item => {
      const holders = presence.get(item.name) ?? new Set<string>();
      holders.add(inventoryKey(inventory));
      presence.set(item.name, holders);
    });
  });
  return presence;
}

/**
 * Picks the singular or the plural wording. English needs a different noun for
 * one item; the other locales map both keys to the same string.
 */
export function counted(count: number, oneKey: string, manyKey: string, token = "{count}") {
  return count === 1 ? i18next.t(oneKey) : i18next.t(manyKey).replace(token, String(count));
}

export function formatBytes(bytes: number | undefined) {
  if (!bytes) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

/** The one-line summary of an MCP server: what it runs, or what it connects to. */
export function endpointOf(item: AgentConfigItem) {
  return item.command || item.url || "";
}

/**
 * useAgentConfigs owns the host scan behind the Skills & MCP page. Deleting and
 * copying both change files this page has already read, so every one of them
 * ends by calling refresh().
 */
export function useAgentConfigs() {
  const [inventories, setInventories] = React.useState<AgentConfigInventory[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [scanned, setScanned] = React.useState(false);

  const refresh = React.useCallback((forceRefresh = false) => {
    setLoading(true);
    setError("");
    AgentConfigBackend.getAgentConfigs(forceRefresh)
      .then(res => {
        if (res.status === "ok") {
          setInventories(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to read agent configuration"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => {
        setLoading(false);
        setScanned(true);
      });
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  return {inventories, loading, error, scanned, refresh};
}
