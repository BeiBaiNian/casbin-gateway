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
import * as Setting from "@/Setting";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {TargetPicker} from "@/components/agent-config/target-picker";
import {Field} from "@/components/shared/form-dialog";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import {Textarea} from "@/components/ui/textarea";
import {counted} from "@/lib/agent-configs";
import {parsePairs} from "@/lib/pairs";
import type {AgentConfigInventory, AgentConfigPlanItem, McpTransport} from "@/types";

/**
 * Setting up one MCP server from Gateway: it is written into every agent picked
 * here, in that agent's own format and file, so the agents that should share a
 * server are configured once instead of one file at a time.
 */
export function AddMcpDialog({
  open,
  onOpenChange,
  inventories,
  source,
  onDone,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  inventories: AgentConfigInventory[];
  source: AgentConfigInventory;
  onDone: () => void;
}) {
  const [name, setName] = React.useState("");
  const [transport, setTransport] = React.useState<McpTransport>("stdio");
  const [command, setCommand] = React.useState("");
  const [args, setArgs] = React.useState<string[]>([]);
  const [env, setEnv] = React.useState("");
  const [url, setUrl] = React.useState("");
  const [headers, setHeaders] = React.useState("");
  const [targets, setTargets] = React.useState<string[]>([]);
  const [overwrite, setOverwrite] = React.useState(false);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  // The server is written into one account's home directory, so the agents
  // offered are the ones reading that same home - which is also what makes one
  // owner enough for the whole request.
  const candidates = inventories.filter(inventory => inventory.home === source.home);
  const defaultTarget = source.mcpWritable ? source.agentId : "";

  React.useEffect(() => {
    if (open) {
      setName("");
      setTransport("stdio");
      setCommand("");
      setArgs([]);
      setEnv("");
      setUrl("");
      setHeaders("");
      setTargets(defaultTarget === "" ? [] : [defaultTarget]);
      setOverwrite(false);
      setResult(null);
      setError("");
    }
  }, [open, defaultTarget]);

  const toggleTarget = (agentId: string) => {
    setTargets(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );
  };

  const ready =
    name.trim() !== "" &&
    targets.length > 0 &&
    (transport === "http" ? url.trim() !== "" : command.trim() !== "");

  const add = () => {
    setBusy(true);
    setError("");
    AgentConfigBackend.addAgentConfigMcp({
      owner: source.owner,
      to: targets,
      name: name.trim(),
      transport: transport,
      command: transport === "stdio" ? command.trim() : undefined,
      args: transport === "stdio" ? args : undefined,
      env: transport === "stdio" ? parsePairs(env) : undefined,
      url: transport === "http" ? url.trim() : undefined,
      headers: transport === "http" ? parsePairs(headers) : undefined,
      overwrite: overwrite,
    })
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to add this MCP server"));
          return;
        }

        const written = res.data ?? [];
        const done = written.filter(item => item.action === "create" || item.action === "overwrite").length;
        const failed = written.filter(item => item.action === "failed").length;
        setResult(written);
        Setting.showMessage(
          failed > 0 || done === 0 ? "error" : "success",
          failed > 0
            ? i18next
              .t("agentConfig:Added to {done}, {failed} failed")
              .replace("{done}", String(done))
              .replace("{failed}", String(failed))
            : counted(done, "agentConfig:Added to 1 agent", "agentConfig:Added to {done} agents", "{done}"),
        );
        onDone();
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const nameOf = (agentId: string) =>
    candidates.find(inventory => inventory.agentId === agentId)?.name ?? agentId;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-4 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agentConfig:Add MCP server")}</DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Add MCP server hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 overflow-y-auto">
          <Field label={i18next.t("general:Name")} htmlFor="mcp-name" required>
            <Input
              id="mcp-name"
              value={name}
              placeholder="context7"
              disabled={result !== null}
              onChange={event => setName(event.target.value)}
            />
          </Field>

          <Field label={i18next.t("agentConfig:Transport")}>
            <SimpleSelect
              value={transport}
              disabled={result !== null}
              options={[
                {label: i18next.t("agentConfig:Local command"), value: "stdio"},
                {label: i18next.t("agentConfig:HTTP endpoint"), value: "http"},
              ]}
              onChange={value => setTransport(value as McpTransport)}
            />
          </Field>

          {transport === "stdio" ? (
            <>
              <Field
                label={i18next.t("agentConfig:Command")}
                htmlFor="mcp-command"
                hint={i18next.t("agentConfig:Command hint")}
                required
              >
                <Input
                  id="mcp-command"
                  value={command}
                  placeholder="npx"
                  disabled={result !== null}
                  onChange={event => setCommand(event.target.value)}
                />
              </Field>
              <Field label={i18next.t("agentConfig:Arguments")}>
                <TagsInput
                  value={args}
                  placeholder="-y, @upstash/context7-mcp"
                  disabled={result !== null}
                  onChange={setArgs}
                />
              </Field>
              <Field
                label={i18next.t("agentConfig:Environment")}
                htmlFor="mcp-env"
                hint={i18next.t("agentConfig:Pairs hint")}
              >
                <Textarea
                  id="mcp-env"
                  value={env}
                  rows={2}
                  placeholder="API_KEY=..."
                  disabled={result !== null}
                  onChange={event => setEnv(event.target.value)}
                />
              </Field>
            </>
          ) : (
            <>
              <Field label={i18next.t("agentConfig:URL")} htmlFor="mcp-url" required>
                <Input
                  id="mcp-url"
                  value={url}
                  placeholder="https://mcp.example.com/mcp"
                  disabled={result !== null}
                  onChange={event => setUrl(event.target.value)}
                />
              </Field>
              <Field
                label={i18next.t("agentConfig:Headers")}
                htmlFor="mcp-headers"
                hint={i18next.t("agentConfig:Pairs hint")}
              >
                <Textarea
                  id="mcp-headers"
                  value={headers}
                  rows={2}
                  placeholder="Authorization=Bearer ..."
                  disabled={result !== null}
                  onChange={event => setHeaders(event.target.value)}
                />
              </Field>
            </>
          )}

          <Field label={i18next.t("agentConfig:Add to")} hint={i18next.t("agentConfig:Add to hint")}>
            <TargetPicker
              candidates={candidates}
              kind="mcp"
              selected={targets}
              onToggle={toggleTarget}
              disabled={result !== null}
            />
          </Field>

          {result === null ? (
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={overwrite} onCheckedChange={setOverwrite} />
              <span>{i18next.t("agentConfig:Replace items that already exist")}</span>
            </label>
          ) : null}

          {error ? <MessageAlert description={error} /> : null}

          {result === null ? null : (
            <ul className="divide-y rounded-md border">
              {result.map(item => (
                <li
                  key={`${item.agentId}/${item.name}`}
                  className="flex items-center justify-between gap-3 px-3 py-1.5 text-sm"
                >
                  <span className="truncate">{nameOf(item.agentId)}</span>
                  <span className="flex shrink-0 items-center gap-2">
                    {item.reason ? <span className="text-muted-foreground text-xs">{item.reason}</span> : null}
                    <ActionBadge action={item.action} />
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {result === null ? i18next.t("general:Cancel") : i18next.t("general:Close")}
          </Button>
          {result === null ? (
            <Button onClick={add} disabled={busy || !ready} loading={busy}>
              {i18next.t("agentConfig:Add MCP server")}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
