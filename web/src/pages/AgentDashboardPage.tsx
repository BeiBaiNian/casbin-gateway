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
import {
  Bot,
  Check,
  ChevronRight,
  Container,
  FileSearch,
  MessageSquare,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {RunBadge, RunButton} from "@/components/AgentRunControl";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {EmptyState} from "@/components/shared/empty-state";
import {Loading} from "@/components/shared/loading";
import {CodeText, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {StatCard} from "@/components/shared/stat-card";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader} from "@/components/ui/card";
import {Switch} from "@/components/ui/switch";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  activityOf,
  agentDetailPath,
  agentKey,
  runtimeOf,
  useAgents,
  useAgentSessions,
} from "@/lib/agents";
import {cn} from "@/lib/utils";
import type {Account, Agent, AgentRuntime} from "@/types";

/** One labelled line inside an agent card. */
function CardRow({label, children}: {label: string; children: React.ReactNode}) {
  return (
    <div className="flex items-baseline gap-2">
      <span className="text-muted-foreground shrink-0 text-xs">{label}</span>
      <span className="min-w-0 flex-1 truncate text-right">{children}</span>
    </div>
  );
}

function AgentCard({
  agent,
  detailTo,
  busy,
  activity,
  status,
  runBusy,
  onToggle,
  onRun,
}: {
  agent: Agent;
  detailTo: string;
  busy: boolean;
  activity?: {sessionCount: number; recordCount: number; lastTime: string};
  status?: AgentRuntime;
  runBusy: boolean;
  onToggle: () => void;
  onRun: (agent: Agent, running: boolean) => void;
}) {
  const action = i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`);
  const note = [agent.notice, agent.followup].filter(Boolean).join(" ");

  const toggle = (
    <Switch
      checked={agent.patched}
      disabled={!agent.supported || busy}
      aria-label={action}
      // The confirmation dialog owns the click, so the switch never flips on
      // its own - it only ever mirrors what the last scan reported.
      onCheckedChange={() => undefined}
    />
  );

  return (
    <Card className="hover:border-ring/50 flex flex-col gap-3 py-4 transition-colors">
      <CardHeader className="flex flex-row items-start gap-3 px-4">
        <AgentIcon
          agent={agent.agentId || agent.name}
          size={40}
          fallback={<Bot className="text-muted-foreground size-10" />}
        />
        <div className="min-w-0 flex-1">
          <Link to={detailTo} className="hover:text-primary block truncate font-semibold hover:underline">
            {agent.name}
          </Link>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <RunBadge status={status} />
            <Badge variant="muted">{agent.version || i18next.t("agent:Unknown")}</Badge>
            {agent.installMethod ? <Badge variant="muted">{agent.installMethod}</Badge> : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {busy ? <Loading type="small" className="py-0" /> : null}
          {agent.supported ? (
            // The dialog trigger clones its child to attach the click, so the
            // switch has to be that child - a wrapper that is not a DOM element
            // would drop the handler and the switch would do nothing.
            <ConfirmDialog
              title={`${action} ${agent.name}?`}
              description={note || undefined}
              confirmText={action}
              variant={agent.patched ? "destructive" : "default"}
              onConfirm={onToggle}
            >
              {toggle}
            </ConfirmDialog>
          ) : (
            <SimpleTooltip title={agent.detail || i18next.t("agent:Not supported")}>
              <span>{toggle}</span>
            </SimpleTooltip>
          )}
        </div>
      </CardHeader>

      <CardContent className="flex flex-1 flex-col gap-2 px-4 text-sm">
        <CardRow label={i18next.t("general:Path")}>
          <SimpleTooltip title={agent.path}>
            <span className="inline-flex max-w-full">
              <CodeText copyable>{agent.path}</CodeText>
            </span>
          </SimpleTooltip>
        </CardRow>
        <CardRow label={i18next.t("general:Owner")}>
          <span className="truncate text-xs">{agent.owner}</span>
        </CardRow>

        <div className="mt-auto flex flex-col gap-2 border-t pt-3">
          {/* The patcher's own wording is the most precise thing there is about
              a given state, so it hangs off the status line as its tooltip. */}
          <SimpleTooltip title={agent.detail}>
            <span className="text-muted-foreground min-w-0 truncate text-xs">
              {!agent.supported
                ? i18next.t("agent:Not supported")
                : !agent.patched
                  ? i18next.t("agent:Turn on monitoring to collect activity")
                  : activity
                    ? `${i18next
                      .t("agent:{count} sessions")
                      .replace("{count}", String(activity.sessionCount))} · ${new Date(
                      activity.lastTime,
                    ).toLocaleString()}`
                    : i18next.t("agent:Monitoring, no activity yet")}
            </span>
          </SimpleTooltip>
          <div className="flex items-center justify-between gap-2">
            <RunButton agent={agent} status={status} busy={runBusy} onToggle={onRun} />
            <Link
              to={detailTo}
              className="text-primary inline-flex shrink-0 items-center text-xs hover:underline"
            >
              {i18next.t("agent:Details")}
              <ChevronRight className="size-3.5" />
            </Link>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

/** The two steps between a fresh install and an agent running on your own key. */
function SetupGuide({hasProvider, agents}: {hasProvider: boolean; agents: Agent[]}) {
  const steps = [
    {
      done: hasProvider,
      title: i18next.t("agent:Add a provider"),
      hint: i18next.t("agent:Add a provider hint"),
      to: "/providers",
    },
    {
      done: agents.some(agent => agent.provider !== ""),
      title: i18next.t("agent:Bind the provider"),
      hint: i18next.t("agent:Bind the provider hint"),
      // With nothing scanned there is no agent to open, only the list saying so.
      to: agents.length > 0 ? agentDetailPath(agents[0], agents) : "/agents",
    },
  ];

  return (
    <Card className="gap-3 py-4">
      <CardHeader className="flex flex-col gap-1 px-4">
        <span className="font-semibold">{i18next.t("agent:Use your own API")}</span>
        <span className="text-muted-foreground text-sm">{i18next.t("agent:Use your own API hint")}</span>
      </CardHeader>
      <CardContent className="flex flex-col gap-3 px-4">
        {steps.map((step, index) => (
          <div key={step.title} className="flex items-center gap-3">
            <span
              className={cn(
                "flex size-6 shrink-0 items-center justify-center rounded-full text-xs",
                step.done ? "bg-success/15 text-success" : "bg-muted text-muted-foreground",
              )}
            >
              {step.done ? <Check className="size-3.5" /> : index + 1}
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium">{step.title}</p>
              <p className="text-muted-foreground text-xs">{step.hint}</p>
            </div>
            <Button asChild variant="outline" size="sm">
              <Link to={step.to}>{i18next.t("general:Open")}</Link>
            </Button>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

export default function AgentDashboardPage({account}: {account: Account}) {
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
  } = useAgents(isAdmin);
  const {activity, recordCount} = useAgentSessions(isAdmin, "", 5000);
  // null until the provider count is known, so the guide never flashes.
  const [hasProvider, setHasProvider] = React.useState<boolean | null>(null);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name, 1, 1)
      .then(res => setHasProvider(res.status === "ok" && (res.data2 ?? 0) > 0))
      .catch(() => setHasProvider(false));
  }, [isAdmin, account.name]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const patchedCount = agents.filter(agent => agent.patched).length;
  const sessionCount = Object.values(activity).reduce((total, entry) => total + entry.sessionCount, 0);
  const setupDone = hasProvider === true && agents.some(agent => agent.provider !== "");

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agents on this machine")}
        description={account.hostname}
        actions={
          <Button
            variant="outline"
            onClick={() => {
              scan(true);
              loadRuntime(true);
            }}
            loading={loading}
          >
            <RefreshCw />
            {i18next.t("agent:Scan")}
          </Button>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label={i18next.t("agent:Installed")} value={agents.length} icon={Bot} />
        <StatCard label={i18next.t("agent:Monitored")} value={patchedCount} icon={ShieldCheck} tone="success" />
        <StatCard label={i18next.t("agent:Agent Sessions")} value={sessionCount} icon={MessageSquare} />
        <StatCard label={i18next.t("agent:Records")} value={recordCount} icon={FileSearch} />
      </div>

      {scanned && hasProvider !== null && !setupDone ? (
        <SetupGuide hasProvider={hasProvider} agents={agents} />
      ) : null}

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
            action={
              <Button
                variant="outline"
                onClick={() => {
                  scan(true);
                  loadRuntime(true);
                }}
                loading={loading}
              >
                <RefreshCw />
                {i18next.t("agent:Scan")}
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {agents.map(agent => (
            <AgentCard
              key={agentKey(agent)}
              agent={agent}
              detailTo={agentDetailPath(agent, agents)}
              busy={busyKey === agentKey(agent)}
              activity={activityOf(activity, agent)}
              status={runtimeOf(runtime, agent)}
              runBusy={runBusyKey === agentKey(agent)}
              onToggle={() => togglePatch(agent)}
              onRun={toggleRunning}
            />
          ))}
        </div>
      )}
    </PageContainer>
  );
}
