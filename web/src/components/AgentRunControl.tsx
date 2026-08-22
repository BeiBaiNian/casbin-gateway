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

import {Play, Square} from "lucide-react";
import i18next from "i18next";

import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {Agent, AgentRuntime} from "@/types";

/** Whether the installation has live processes, with its pids on hover. */
export function RunBadge({status}: {status?: AgentRuntime}) {
  if (!status) {
    return <span className="text-muted-foreground">-</span>;
  }
  if (!status.running) {
    return (
      <SimpleTooltip title={status.detail}>
        <span>
          <Badge variant="muted">{i18next.t("agent:Stopped")}</Badge>
        </span>
      </SimpleTooltip>
    );
  }
  return (
    <SimpleTooltip title={`${i18next.t("agent:Processes")}: ${status.pids.join(", ")}`}>
      <span>
        <Badge variant="success">{i18next.t("agent:Running")}</Badge>
      </span>
    </SimpleTooltip>
  );
}

/**
 * The start/stop control. Starting is harmless enough to happen on the click,
 * while stopping ends work in progress and is confirmed first.
 */
export function RunButton({
  agent,
  status,
  busy,
  onToggle,
}: {
  agent: Agent;
  status?: AgentRuntime;
  busy: boolean;
  onToggle: (agent: Agent, running: boolean) => void;
}) {
  if (status?.running) {
    return (
      <ConfirmDialog
        title={`${i18next.t("agent:Stop")} ${agent.name}?`}
        description={i18next.t("agent:Stop hint")}
        confirmText={i18next.t("agent:Stop")}
        onConfirm={() => onToggle(agent, true)}
      >
        <Button size="sm" variant="outline" loading={busy}>
          <Square />
          {i18next.t("agent:Stop")}
        </Button>
      </ConfirmDialog>
    );
  }

  const startable = status?.canStart === true;
  return (
    <SimpleTooltip title={startable ? "" : status?.detail}>
      <span>
        <Button
          size="sm"
          variant="outline"
          disabled={!startable}
          loading={busy}
          onClick={() => onToggle(agent, false)}
        >
          <Play />
          {i18next.t("agent:Start")}
        </Button>
      </span>
    </SimpleTooltip>
  );
}
