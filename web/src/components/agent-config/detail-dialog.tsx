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
import {Loading} from "@/components/shared/loading";
import {CodeText} from "@/components/shared/misc";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {endpointOf} from "@/lib/agent-configs";
import type {AgentConfigDetail, AgentConfigItem} from "@/types";

/** The item whose definition is on screen, and the agent it was read from. */
export interface DetailTarget {
  item: AgentConfigItem;
  agentName: string;
}

/**
 * Shows one skill's SKILL.md or one MCP server's entry, exactly as the agent
 * has it on disk. Credentials are masked by the server before they get here.
 */
export function DetailDialog({
  target,
  onOpenChange,
}: {
  target: DetailTarget | null;
  onOpenChange: (open: boolean) => void;
}) {
  const [detail, setDetail] = React.useState<AgentConfigDetail | null>(null);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  const item = target?.item;

  React.useEffect(() => {
    if (!item) {
      return;
    }

    let current = true;
    setDetail(null);
    setError("");
    setLoading(true);
    AgentConfigBackend.getAgentConfigItem(item.agentId, item.owner, item.kind, item.name)
      .then(res => {
        if (!current) {
          return;
        }
        if (res.status === "ok") {
          setDetail(res.data ?? null);
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to read this item"));
        }
      })
      .catch(err => current && setError(err.message || String(err)))
      .then(() => current && setLoading(false));

    return () => {
      current = false;
    };
  }, [item]);

  return (
    <Dialog open={Boolean(target)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-3 sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span className="truncate">{item?.name}</span>
            <Badge variant="muted">{target?.agentName}</Badge>
          </DialogTitle>
          <DialogDescription>
            {item?.description || detail?.item.description || (item ? endpointOf(item) : "")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3 overflow-y-auto">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="text-muted-foreground">{i18next.t("general:Path")}</span>
            <CodeText copyable>{item?.path}</CodeText>
          </div>

          {detail?.files?.length ? (
            <div className="flex flex-wrap items-center gap-1.5 text-xs">
              <span className="text-muted-foreground">{i18next.t("agentConfig:Bundled files")}</span>
              {detail.files.map(file => (
                <Badge key={file} variant="outline" className="font-mono font-normal">
                  {file}
                </Badge>
              ))}
            </div>
          ) : null}

          {error ? <MessageAlert description={error} /> : null}
          {loading ? <Loading /> : null}
          {detail ? (
            <pre className="bg-muted max-h-[50vh] overflow-auto rounded-md p-3 text-xs whitespace-pre-wrap">
              {detail.content}
            </pre>
          ) : null}
          {detail && item?.kind === "mcp" ? (
            <p className="text-muted-foreground text-xs">{i18next.t("agentConfig:Credentials are masked here")}</p>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
