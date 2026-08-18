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
import {Bot, CircleX, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/DataTable";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {Spinner} from "@/components/ui/spinner";
import {Tooltip} from "@/components/ui/tooltip";
import type {Account, Agent} from "@/types";

const rowKey = (record: Agent) => `${record.owner}:${record.path}`;

/** The monitor keys the two Codex front ends under one agent id. */
function monitorAgentId(agentId: string) {
  return agentId === "codex_vscode" || agentId === "codex-vscode" ? "codex-cli" : agentId;
}

export default function AgentsPage({account}: {account: Account}) {
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [busyKey, setBusyKey] = React.useState("");
  const isAdmin = Setting.isAdminUser(account);

  const scan = React.useCallback(
    (forceRefresh = false) => {
      if (!isAdmin) {
        return;
      }

      setLoading(true);
      setError("");
      AgentBackend.getAgents(forceRefresh)
        .then(res => {
          if (res.status === "ok") {
            setAgents(res.data ?? []);
          } else {
            setError(res.msg || i18next.t("agent:Failed to scan agents"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => setLoading(false));
    },
    [isAdmin],
  );

  React.useEffect(() => {
    scan();
  }, [scan]);

  const togglePatch = (record: Agent) => {
    const target = {agentId: record.agentId, path: record.path, owner: record.owner};
    const patched = record.patched;

    setBusyKey(rowKey(record));
    (patched ? AgentBackend.unpatchAgent(target) : AgentBackend.patchAgent(target))
      .then(res => {
        if (res.status === "ok") {
          const done = patched
            ? i18next.t("agent:Monitoring disabled")
            : i18next.t("agent:Monitoring enabled");
          const followup = res.data?.followup || record.followup;
          Setting.showMessage(
            "success",
            followup ? `${done}: ${record.name}. ${followup}` : `${done}: ${record.name}`,
          );
          scan();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to update agent patch"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusyKey(""));
  };

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<Agent>[] = [
    {
      title: i18next.t("agent:Agent"),
      key: "name",
      dataIndex: "name",
      render: (value: string, record) => (
        <span className="flex items-center gap-2">
          <AgentIcon
            agent={record.agentId || value}
            fallback={<Bot className="h-[18px] w-[18px] text-muted-foreground" />}
          />
          {value}
        </span>
      ),
    },
    {
      title: i18next.t("agent:Version"),
      key: "version",
      dataIndex: "version",
      render: (value: string) => value || i18next.t("agent:Unknown"),
    },
    {
      title: i18next.t("agent:Install Method"),
      key: "installMethod",
      dataIndex: "installMethod",
      render: (value: string) => <Badge variant="secondary">{value || "-"}</Badge>,
    },
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
    },
    {
      title: i18next.t("general:Path"),
      key: "path",
      dataIndex: "path",
      render: (value: string) => <code className="text-xs">{value}</code>,
    },
    {
      title: i18next.t("agent:Patch Status"),
      key: "patched",
      render: (_value, record) => {
        const badge = !record.supported ? (
          <Badge variant="secondary">{i18next.t("agent:Not supported")}</Badge>
        ) : record.patched ? (
          <Badge variant="success">{i18next.t("agent:Patched")}</Badge>
        ) : (
          <Badge variant="secondary">{i18next.t("agent:Not patched")}</Badge>
        );
        return (
          <Tooltip title={record.detail}>
            <span>{badge}</span>
          </Tooltip>
        );
      },
    },
    {
      title: i18next.t("agent:Records"),
      key: "records",
      render: (_value, record) =>
        record.patched ? (
          <Link
            to={`/agent-records?agent=${encodeURIComponent(monitorAgentId(record.agentId))}`}
            className="text-primary hover:underline"
          >
            {i18next.t("agent:View Records")}
          </Link>
        ) : null,
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, record) => {
        if (!record.supported) {
          return (
            <Button size="sm" variant="outline" disabled>
              {i18next.t("agent:Patch")}
            </Button>
          );
        }

        const action = i18next.t(`agent:${record.patched ? "Unpatch" : "Patch"}`);
        const note = [record.notice, record.followup].filter(Boolean).join(" ");
        return (
          <ConfirmButton
            title={`${action} ${record.name}?`}
            description={note || undefined}
            okText={action}
            destructive={record.patched}
            onConfirm={() => togglePatch(record)}
          >
            <Button
              size="sm"
              variant={record.patched ? "outline" : "default"}
              disabled={busyKey === rowKey(record)}
            >
              {busyKey === rowKey(record) ? <Spinner /> : null}
              {action}
            </Button>
          </ConfirmButton>
        );
      },
    },
  ];

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("agent:Agents")}>
        <Button variant="outline" onClick={() => scan(true)} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("agent:Scan")}
        </Button>
      </PageHeader>

      {error && (
        <Alert variant="destructive" className="mb-4">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <DataTable
        columns={columns}
        data={agents}
        rowKey={rowKey}
        loading={loading}
        pageSize={0}
        emptyText={i18next.t("agent:No supported agents found")}
      />
    </div>
  );
}
