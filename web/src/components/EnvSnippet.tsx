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
import {Copy} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {envSnippet, shells, type Shell} from "@/lib/channels";

/**
 * The environment variables that point a client at one Gateway endpoint, ready
 * to paste into the shell the agent is started from.
 */
export function EnvSnippet({
  protocol,
  baseUrl,
  defaultShell = "bash",
  includeToken = true,
}: {
  protocol: string;
  baseUrl: string;
  defaultShell?: Shell;
  includeToken?: boolean;
}) {
  const [shell, setShell] = React.useState<Shell>(defaultShell);
  const snippet = envSnippet(protocol, baseUrl, shell, includeToken);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="inline-flex rounded-md border p-0.5">
          {shells.map(name => (
            <button
              key={name}
              onClick={() => setShell(name)}
              className={cn(
                "rounded px-2 py-0.5 text-xs transition-colors",
                shell === name
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent",
              )}
            >
              {name}
            </button>
          ))}
        </div>
        <SimpleTooltip title={i18next.t("general:Copy")}>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 shrink-0"
            aria-label={i18next.t("general:Copy")}
            onClick={() => {
              copy(snippet);
              Setting.showMessage("success", i18next.t("general:Copied to clipboard"));
            }}
          >
            <Copy className="h-3.5 w-3.5" />
          </Button>
        </SimpleTooltip>
      </div>
      <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">{snippet}</pre>
    </div>
  );
}
