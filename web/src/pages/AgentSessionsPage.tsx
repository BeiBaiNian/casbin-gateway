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
import {Bot, MessageSquare, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/shared/data-table";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import type {Account, AgentSession} from "@/types";

export default function AgentSessionsPage({account}: {account: Account}) {
  const [sessions, setSessions] = React.useState<AgentSession[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const isAdmin = Setting.isAdminUser(account);

  const load = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }

    setLoading(true);
    AgentBackend.getAgentSessions()
      .then(res => {
        if (res.status === "ok") {
          setSessions(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("agent:Failed to get agent sessions"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [isAdmin]);

  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, [isAdmin, load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<AgentSession>[] = [
    {
      title: i18next.t("agent:Session"),
      key: "session",
      render: (_value, session) => (
        <div className="flex min-w-0 flex-col">
          <Link
            to={`/agent-records?agent=${encodeURIComponent(session.agent)}&session=${encodeURIComponent(session.sessionKey)}`}
            className="text-primary truncate font-medium hover:underline"
          >
            {session.title || session.sessionKey}
          </Link>
          <span className="text-muted-foreground truncate text-xs">{session.sessionKey}</span>
        </div>
      ),
    },
    {
      title: i18next.t("agent:Agent"),
      key: "agent",
      dataIndex: "agent",
      width: "180px",
      render: (value: string) => (
        <Badge variant="info">
          <AgentIcon agent={value} fallback={<Bot className="size-3" />} size={12} />
          {value}
        </Badge>
      ),
    },
    {
      title: i18next.t("agent:Records"),
      key: "recordCount",
      dataIndex: "recordCount",
      width: "120px",
    },
    {
      title: i18next.t("agent:First activity"),
      key: "firstTime",
      dataIndex: "firstTime",
      width: "200px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Last activity"),
      key: "lastTime",
      dataIndex: "lastTime",
      width: "200px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
  ];

  return (
    <PageContainer>
      <PageHeader title={i18next.t("agent:Agent Sessions")} />

      {error ? <MessageAlert title={error} /> : null}

      <DataTable
        title={i18next.t("agent:Agent Sessions")}
        description={`${sessions.length} ${i18next.t("agent:Agent Sessions")}`}
        columns={columns}
        dataSource={sessions}
        rowKey={session => `${session.agent}:${session.sessionKey}`}
        loading={loading}
        pageSize={20}
        searchable
        emptyIcon={MessageSquare}
        emptyText={i18next.t("agent:No agent sessions yet - patch an agent to start collecting them")}
        toolbar={
          <Button variant="outline" size="sm" onClick={load} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />
    </PageContainer>
  );
}
