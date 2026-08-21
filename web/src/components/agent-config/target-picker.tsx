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

import {AgentIcon} from "@/components/AgentIcon";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {blockedReason, inventoryKey} from "@/lib/agent-configs";
import type {AgentConfigInventory, AgentConfigKind} from "@/types";

/**
 * The agents a write can land on. One that cannot take this kind of item is
 * still listed, disabled and with the reason on it, because "Gateway will not
 * write that file" is what the reader came to find out.
 */
export function TargetPicker({
  candidates,
  kind,
  selected,
  onToggle,
  disabled = false,
}: {
  candidates: AgentConfigInventory[];
  kind: AgentConfigKind;
  selected: string[];
  onToggle: (agentId: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {candidates.map(inventory => {
        const blocked = blockedReason(inventory, kind);
        const active = selected.includes(inventory.agentId);
        const button = (
          <button
            key={inventoryKey(inventory)}
            type="button"
            disabled={Boolean(blocked) || disabled}
            onClick={() => onToggle(inventory.agentId)}
            className={cn(
              "flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors",
              active ? "border-primary bg-primary/10" : "hover:bg-accent",
              (blocked || disabled) && "cursor-not-allowed opacity-50",
            )}
          >
            <AgentIcon agent={inventory.name} size={16} />
            <span>{inventory.name}</span>
            {inventory.installed ? null : (
              <span className="text-muted-foreground text-xs">{i18next.t("agentConfig:Not installed")}</span>
            )}
          </button>
        );
        return blocked ? (
          <SimpleTooltip key={inventoryKey(inventory)} title={blocked}>
            <span className="inline-flex">{button}</span>
          </SimpleTooltip>
        ) : (
          button
        );
      })}
    </div>
  );
}
