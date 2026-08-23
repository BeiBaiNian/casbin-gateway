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
import {Link} from "react-router-dom";
import {Bot, Check, ChevronRight, CircleX, Container, Plug, Plus, RefreshCw, Table2} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {RunBadge, RunButton} from "@/components/AgentRunControl";
import {ProviderIcon} from "@/components/ProviderIcon";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {EmptyState} from "@/components/shared/empty-state";
import {Loading} from "@/components/shared/loading";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Switch} from "@/components/ui/switch";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  agentCanUse,
  agentDetailPath,
  agentKey,
  agentNeedsResponsesApi,
  agentProtocol,
  runtimeOf,
  useAgents,
} from "@/lib/agents";
import {
  providerIdOf,
  providerProtocol,
  servesResponsesApi,
  usesClientAuth,
} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Account, Agent, AgentRuntime, Provider, ProviderHealth} from "@/types";

/** One row of the rail: the tool, and the provider it is on right now. */
function AgentRow({
  agent,
  provider,
  status,
  active,
  onSelect,
}: {
  agent: Agent;
  provider: string;
  status?: AgentRuntime;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <SimpleTooltip title={agent.path} side="right">
      <button
        type="button"
        onClick={onSelect}
        className={cn(
          "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors",
          active ? "bg-accent text-accent-foreground" : "hover:bg-accent/60",
        )}
      >
        <AgentIcon
          agent={agent.agentId || agent.name}
          size={22}
          fallback={<Bot className="text-muted-foreground size-5" />}
        />
        <span className="min-w-0 flex-1">
          <span className="flex items-center gap-1.5">
            <span className="truncate text-sm font-medium">{agent.name}</span>
            {status?.running ? <span className="bg-success size-1.5 shrink-0 rounded-full" /> : null}
          </span>
          <span className="text-muted-foreground block truncate text-xs">{provider}</span>
        </span>
      </button>
    </SimpleTooltip>
  );
}

/** Why this agent cannot be put on this provider, empty when it can. */
function blockedReason(agent: Agent, provider: Provider) {
  if (agentCanUse(agent, provider)) {
    return "";
  }
  return agentNeedsResponsesApi(agent.agentId) && !servesResponsesApi(provider)
    ? i18next.t("agent:Gateway only hint")
    : i18next.t("agent:Provider speaks another API").replace("{protocol}", agentProtocol(agent));
}

/** One provider the selected agent can be put on, with the button that does it. */
function ProviderChoice({
  provider,
  health,
  active,
  blocked,
  busy,
  waiting,
  onEnable,
}: {
  provider: Provider;
  health?: ProviderHealth;
  active: boolean;
  /** Why this agent cannot reach it as routed now, empty when it can. */
  blocked: string;
  busy: boolean;
  /** Another card of the same agent is being enabled. */
  waiting: boolean;
  onEnable: () => void;
}) {
  const protocol = providerProtocol(provider.type);

  return (
    <Card className={cn("gap-0 py-0", active && "border-primary ring-primary/20 ring-1")}>
      <CardContent className="flex h-full flex-col gap-3 p-4">
        <div className="flex items-start gap-2">
          <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={provider.name} size={22} />
          <div className="min-w-0 flex-1">
            <p className="truncate font-medium">{provider.displayName || provider.name}</p>
            <SimpleTooltip title={provider.baseUrl}>
              <p className="text-muted-foreground truncate font-mono text-xs">{provider.baseUrl || "-"}</p>
            </SimpleTooltip>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant={protocol === "anthropic" ? "info" : "success"}>{protocol}</Badge>
          {usesClientAuth(provider) ? (
            <Badge variant="muted">{i18next.t("provider:Caller's own login")}</Badge>
          ) : null}
          {provider.status === "enabled" ? null : (
            <Badge variant="muted">
              <CircleX />
              {i18next.t("provider:Disabled")}
            </Badge>
          )}
          {health && !health.healthy ? (
            <SimpleTooltip title={health.lastError}>
              <span>
                <Badge variant="warning">{i18next.t("agent:Cooling down")}</Badge>
              </span>
            </SimpleTooltip>
          ) : null}
        </div>

        <div className="mt-auto flex items-center gap-2 border-t pt-3">
          {active ? (
            <Badge variant="success">
              <Check />
              {i18next.t("agent:In use")}
            </Badge>
          ) : blocked === "" ? (
            <Button size="sm" loading={busy} disabled={waiting} onClick={onEnable}>
              {i18next.t("agent:Enable")}
            </Button>
          ) : (
            <SimpleTooltip title={blocked}>
              <span>
                <Button size="sm" variant="outline" disabled>
                  {i18next.t("agent:Enable")}
                </Button>
              </span>
            </SimpleTooltip>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

/**
 * The home screen: the agents installed on this machine on the left, and the
 * providers the selected one can be put on to the right of it. One click on a
 * card is the whole switch — the routing is stored and, where Gateway writes
 * the agent's configuration file, that file is rewritten with it.
 */
export default function HomePage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {
    agents,
    loading,
    error,
    busyKey,
    scanned,
    inContainer,
    runtime,
    runBusyKey,
    scan,
    loadRuntime,
    toggleRunning,
    togglePatch,
    activateProvider,
  } = useAgents(isAdmin);
  const [providers, setProviders] = React.useState<Provider[]>([]);
  const [health, setHealth] = React.useState<ProviderHealth[]>([]);
  // False until the first listing lands, so "no providers" never flashes.
  const [loaded, setLoaded] = React.useState(false);
  const [selected, setSelected] = React.useState("");
  // Which card was clicked, so only that one spins while the switch is made.
  const [enabling, setEnabling] = React.useState("");

  const loadProviders = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name)
      .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setProviders([]))
      .then(() => setLoaded(true));
  }, [isAdmin, account.name]);

  React.useEffect(() => {
    loadProviders();
  }, [loadProviders]);

  // What the proxy has seen of each provider changes as requests are relayed,
  // so it is polled rather than read once.
  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    const load = () => {
      ProviderBackend.getProviderHealth()
        .then(res => setHealth(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setHealth([]));
    };

    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, [isAdmin]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  // The rail keeps its selection across a rescan; the first agent stands in
  // until something is picked, so the right side is never blank.
  const agent = agents.find(candidate => agentKey(candidate) === selected) ?? agents[0];
  const status = agent ? runtimeOf(runtime, agent) : undefined;
  const busy = agent !== undefined && busyKey === agentKey(agent);
  const nameOf = (id: string) => {
    const provider = providers.find(candidate => providerIdOf(candidate) === id);
    return provider ? provider.displayName || provider.name : id;
  };
  const patchAction = agent ? i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`) : "";

  const refresh = () => {
    scan(true);
    loadRuntime(true);
    loadProviders();
  };

  const rescan = (
    <Button variant="outline" onClick={refresh} loading={loading}>
      <RefreshCw />
      {i18next.t("agent:Scan")}
    </Button>
  );

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agents on this machine")}
        description={account.hostname}
        actions={
          <>
            <Button asChild variant="ghost">
              <Link to="/agents">
                <Table2 />
                {i18next.t("agent:Advanced view")}
              </Link>
            </Button>
            {rescan}
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      {!scanned ? (
        <Loading tip={i18next.t("agent:Scan")} />
      ) : agents.length === 0 ? (
        <Card className="py-0">
          <EmptyState
            icon={inContainer ? Container : Bot}
            title={i18next.t(inContainer ? "agent:Running in a container" : "agent:No supported agents found")}
            description={i18next.t(
              inContainer
                ? "agent:Running in a container detail"
                : "agent:Install an AI agent on this machine, then scan again",
            )}
            action={rescan}
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[15rem_minmax(0,1fr)]">
          <Card className="h-max gap-0 py-2">
            <CardContent className="space-y-0.5 px-2">
              {agents.map(item => (
                <AgentRow
                  key={agentKey(item)}
                  agent={item}
                  provider={item.provider === "" ? i18next.t("agent:No provider") : nameOf(item.provider)}
                  status={runtimeOf(runtime, item)}
                  active={agent !== undefined && agentKey(item) === agentKey(agent)}
                  onSelect={() => setSelected(agentKey(item))}
                />
              ))}
            </CardContent>
          </Card>

          <div className="min-w-0 space-y-3">
            {agent === undefined ? null : (
              <>
                <div className="flex flex-wrap items-center gap-2 border-b pb-3">
                  <AgentIcon
                    agent={agent.agentId || agent.name}
                    size={24}
                    fallback={<Bot className="text-muted-foreground size-6" />}
                  />
                  <span className="font-semibold">{agent.name}</span>
                  <Badge variant="muted">{agent.version || i18next.t("agent:Unknown")}</Badge>
                  <RunBadge status={status} />

                  <div className="ml-auto flex flex-wrap items-center gap-3">
                    <label className="text-muted-foreground flex items-center gap-1.5 text-xs">
                      {i18next.t("agent:Monitored")}
                      {agent.supported ? (
                        <ConfirmDialog
                          title={`${patchAction} ${agent.name}?`}
                          description={[agent.notice, agent.followup].filter(Boolean).join(" ") || undefined}
                          confirmText={patchAction}
                          variant={agent.patched ? "destructive" : "default"}
                          onConfirm={() => togglePatch(agent)}
                        >
                          {/* The dialog owns the click, so the switch only ever
                              mirrors what the last scan reported. */}
                          <Switch
                            checked={agent.patched}
                            disabled={busy}
                            aria-label={patchAction}
                            onCheckedChange={() => undefined}
                          />
                        </ConfirmDialog>
                      ) : (
                        <SimpleTooltip title={agent.detail || i18next.t("agent:Not supported")}>
                          <span>
                            <Switch checked={false} disabled aria-label={i18next.t("agent:Patch")} />
                          </span>
                        </SimpleTooltip>
                      )}
                    </label>
                    <RunButton
                      agent={agent}
                      status={status}
                      busy={runBusyKey === agentKey(agent)}
                      onToggle={toggleRunning}
                    />
                    <Link
                      to={agentDetailPath(agent, agents)}
                      className="text-primary inline-flex items-center text-sm hover:underline"
                    >
                      {i18next.t("agent:Details")}
                      <ChevronRight className="size-4" />
                    </Link>
                  </div>
                </div>

                {loaded && providers.length === 0 ? (
                  <Card className="py-0">
                    <EmptyState
                      icon={Plug}
                      title={i18next.t("provider:No providers yet")}
                      description={i18next.t("provider:No providers yet detail")}
                      action={
                        <Button asChild>
                          <Link to="/providers">
                            <Plus />
                            {i18next.t("provider:New Provider")}
                          </Link>
                        </Button>
                      }
                    />
                  </Card>
                ) : (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                    {providers.map(provider => (
                      <ProviderChoice
                        key={providerIdOf(provider)}
                        provider={provider}
                        health={health.find(item => item.provider === providerIdOf(provider))}
                        active={agent.provider === providerIdOf(provider)}
                        blocked={blockedReason(agent, provider)}
                        busy={busy && enabling === providerIdOf(provider)}
                        waiting={busy && enabling !== providerIdOf(provider)}
                        onEnable={() => {
                          setEnabling(providerIdOf(provider));
                          activateProvider(agent, providerIdOf(provider));
                        }}
                      />
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </PageContainer>
  );
}
