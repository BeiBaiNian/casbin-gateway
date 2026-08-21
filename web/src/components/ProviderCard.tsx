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
import {FileCog, RotateCcw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import {EnvSnippet} from "@/components/EnvSnippet";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Loading} from "@/components/shared/loading";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Switch} from "@/components/ui/switch";
import {agentProxyBaseUrl, agentSetupNoteKey, directMode, gatewayMode} from "@/lib/agents";
import {channelProtocol, shellForPath, usesClientAuth} from "@/lib/channels";
import {cn} from "@/lib/utils";
import type {Agent, AgentProviderFile, Channel, ChannelHealth} from "@/types";

/** Radix rejects an empty item value, so "unbound" needs a stand-in. */
const noChannel = "-";

function channelIdOf(channel: Channel) {
  return `${channel.owner}/${channel.name}`;
}

/** The file preview, loaded when the dialog opens rather than on every render. */
function PreviewDialog({
  agent,
  open,
  onOpenChange,
}: {
  agent: Agent;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [files, setFiles] = React.useState<AgentProviderFile[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      return;
    }

    setLoading(true);
    setError("");
    AgentBackend.planAgentProvider({agentId: agent.agentId, path: agent.path, owner: agent.owner})
      .then(res => {
        if (res.status === "ok") {
          setFiles(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agent:Failed to write the agent configuration"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [open, agent.agentId, agent.path, agent.owner]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-4 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Configuration preview")}</DialogTitle>
          <DialogDescription>{i18next.t("agent:Preview hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 overflow-y-auto">
          {loading ? <Loading /> : null}
          {error ? <MessageAlert description={error} /> : null}
          {files.map(file => (
            <div key={file.path} className="space-y-1">
              <div className="flex items-center justify-between gap-2">
                <code className="min-w-0 truncate text-xs">{file.path}</code>
                <Badge variant="muted">{file.format}</Badge>
              </div>
              <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">{file.preview}</pre>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {i18next.t("general:Close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** One channel of the chain, with what the proxy last saw of it. */
function ChannelChip({
  label,
  active,
  disabled,
  health,
  onClick,
}: {
  label: string;
  active: boolean;
  disabled?: boolean;
  health?: ChannelHealth;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm transition-colors",
        active ? "border-primary bg-primary/10" : "hover:bg-accent",
        disabled && "cursor-not-allowed opacity-50",
      )}
    >
      <span>{label}</span>
      {health && !health.healthy ? (
        <Badge variant="warning">{i18next.t("agent:Cooling down")}</Badge>
      ) : null}
    </button>
  );
}

/**
 * Which upstream an agent's requests go to, which upstreams take over when it
 * cannot answer, and how that choice reaches the agent: through the local proxy,
 * or written into the agent's own configuration file.
 */
export function ProviderCard({
  agent,
  channels,
  health,
  busy,
  onRouting,
  onWrite,
}: {
  agent: Agent;
  channels: Channel[];
  health: ChannelHealth[];
  busy: boolean;
  onRouting: (routing: Partial<AgentBackend.AgentRouting>) => void;
  onWrite: (restore: boolean) => void;
}) {
  const [preview, setPreview] = React.useState(false);

  const bound = channels.find(channel => channelIdOf(channel) === agent.channel);
  const mode = agent.mode || gatewayMode;
  const fallbacks = agent.fallbacks ?? [];
  // An older backend does not report the provider state at all, and the card
  // still has to render for the parts that do not depend on it.
  const provider = agent.provider ?? {
    supported: false,
    applied: false,
    channel: "",
    mode: "",
    baseUrl: "",
    time: "",
    files: [],
    detail: "",
  };
  const noteKey = agentSetupNoteKey(agent.agentId);
  const healthOf = (id: string) => health.find(item => item.channel === id);
  const boundHealth = healthOf(agent.channel);

  // A channel can only take over for one that speaks the same wire format: the
  // proxy relays the request as it arrived.
  const candidates = channels.filter(
    channel =>
      channelIdOf(channel) !== agent.channel &&
      (bound === undefined || channelProtocol(channel.type) === channelProtocol(bound.type)),
  );

  const toggleFallback = (id: string) => {
    onRouting({
      fallbacks: fallbacks.includes(id)
        ? fallbacks.filter(item => item !== id)
        : [...fallbacks, id],
    });
  };

  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <CardTitle className="text-base">{i18next.t("agent:Channel")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        <Select
          value={agent.channel === "" ? noChannel : agent.channel}
          disabled={busy}
          onValueChange={value => onRouting({channel: value === noChannel ? "" : value, fallbacks: []})}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={noChannel}>
              <span className="text-muted-foreground">{i18next.t("agent:No channel")}</span>
            </SelectItem>
            {channels.map(channel => (
              <SelectItem key={channelIdOf(channel)} value={channelIdOf(channel)}>
                {channel.displayName || channel.name}
                {/* The type is the wire format, which has to match the one the agent speaks. */}
                <span className="ml-2 text-xs text-muted-foreground">{channel.type}</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {boundHealth && !boundHealth.healthy ? (
          <p className="text-sm text-warning">
            {`${i18next.t("agent:Bound channel cooling down")}: ${boundHealth.lastError}`}
          </p>
        ) : null}

        {agent.channel === "" ? (
          <p className="text-sm text-muted-foreground">{i18next.t("agent:Channel hint")}</p>
        ) : bound === undefined ? null : (
          <>
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">{i18next.t("agent:Fallback hint")}</p>
              {candidates.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {i18next.t("agent:No other channel speaks this API")}
                </p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {candidates.map(channel => (
                    <ChannelChip
                      key={channelIdOf(channel)}
                      label={channel.displayName || channel.name}
                      active={fallbacks.includes(channelIdOf(channel))}
                      disabled={busy}
                      health={healthOf(channelIdOf(channel))}
                      onClick={() => toggleFallback(channelIdOf(channel))}
                    />
                  ))}
                </div>
              )}
            </div>

            <label className="flex items-start gap-2 text-sm">
              <Switch
                className="mt-0.5"
                checked={mode === gatewayMode}
                disabled={busy}
                onCheckedChange={checked =>
                  onRouting({mode: checked ? gatewayMode : directMode})
                }
              />
              <span>
                {i18next.t("agent:Route through Gateway")}
                <span className="block text-muted-foreground">
                  {mode === gatewayMode
                    ? i18next.t("agent:Gateway mode hint")
                    : i18next.t("agent:Direct mode hint")}
                </span>
              </span>
            </label>

            {provider.supported ? (
              <div className="space-y-2 rounded-md border p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <FileCog className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">
                    {i18next.t("agent:Agent configuration")}
                  </span>
                  {provider.applied ? (
                    <Badge variant="success">{i18next.t("agent:Written by Gateway")}</Badge>
                  ) : (
                    <Badge variant="secondary">{i18next.t("agent:Not written")}</Badge>
                  )}
                </div>

                {provider.detail ? (
                  <p className="text-sm text-muted-foreground">{provider.detail}</p>
                ) : null}
                {(provider.files ?? []).map(path => (
                  <code key={path} className="block truncate text-xs">
                    {path}
                  </code>
                ))}

                <div className="flex flex-wrap gap-2">
                  <ConfirmDialog
                    title={i18next.t("agent:Write the configuration of {agent}?").replace("{agent}", agent.name)}
                    description={i18next.t("agent:Write confirm hint")}
                    confirmText={i18next.t("agent:Write configuration")}
                    onConfirm={() => onWrite(false)}
                  >
                    <Button disabled={busy}>
                      {provider.applied
                        ? i18next.t("agent:Rewrite configuration")
                        : i18next.t("agent:Write configuration")}
                    </Button>
                  </ConfirmDialog>

                  <Button variant="outline" disabled={busy} onClick={() => setPreview(true)}>
                    {i18next.t("agent:Preview")}
                  </Button>

                  {provider.applied || provider.channel !== "" ? (
                    <ConfirmDialog
                      title={i18next.t("agent:Restore the configuration of {agent}?").replace("{agent}", agent.name)}
                      description={i18next.t("agent:Restore confirm hint")}
                      confirmText={i18next.t("agent:Restore configuration")}
                      variant="destructive"
                      onConfirm={() => onWrite(true)}
                    >
                      <Button variant="outline" disabled={busy}>
                        <RotateCcw />
                        {i18next.t("agent:Restore configuration")}
                      </Button>
                    </ConfirmDialog>
                  ) : null}
                </div>
              </div>
            ) : (
              <>
                <p className="text-sm text-muted-foreground">{i18next.t("agent:Base URL hint")}</p>
                <EnvSnippet
                  protocol={channelProtocol(bound.type)}
                  baseUrl={agentProxyBaseUrl(agent.agentId)}
                  defaultShell={shellForPath(agent.path)}
                  includeToken={!usesClientAuth(bound)}
                />
                <p className="text-sm text-muted-foreground">
                  {usesClientAuth(bound) ? i18next.t("agent:Client auth token hint") : i18next.t("agent:Token hint")}
                </p>
                {noteKey === "" ? null : (
                  <p className="text-sm text-muted-foreground">{i18next.t(noteKey)}</p>
                )}
              </>
            )}
          </>
        )}

        <PreviewDialog agent={agent} open={preview} onOpenChange={setPreview} />
      </CardContent>
    </Card>
  );
}
