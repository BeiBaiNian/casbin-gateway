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
import {CircleX, Info, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as LlmRequestAuditBackend from "@/backend/LlmRequestAuditBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Switch} from "@/components/ui/switch";
import type {Account, LlmRequestAudit} from "@/types";

function formatJSON(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export default function LlmRequestAuditsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [records, setRecords] = React.useState<LlmRequestAudit[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(50);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [autoRefresh, setAutoRefresh] = React.useState(true);

  const load = React.useCallback((foreground = true) => {
    if (!isAdmin) {
      return;
    }
    if (foreground) {
      setLoading(true);
    }
    LlmRequestAuditBackend.getLlmRequestAudits(page, pageSize)
      .then(res => {
        if (res.status === "ok") {
          setRecords(res.data ?? []);
          setTotal(res.data2 ?? 0);
          setError("");
        } else {
          setError(res.msg || i18next.t("llm:Failed to get request audits"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .finally(() => foreground && setLoading(false));
  }, [isAdmin, page, pageSize]);

  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }
    load(true);
    if (!autoRefresh) {
      return undefined;
    }
    const interval = setInterval(() => load(false), 3000);
    return () => clearInterval(interval);
  }, [autoRefresh, isAdmin, load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<LlmRequestAudit>[] = [
    {title: i18next.t("agent:Time"), key: "createdTime", dataIndex: "createdTime", width: "180px", render: value => new Date(value).toLocaleString()},
    {title: i18next.t("agent:Model"), key: "model", dataIndex: "model", width: "180px", render: value => <code className="text-xs">{value}</code>},
    {title: i18next.t("llm:Channel"), key: "channel", dataIndex: "channel", width: "180px", render: value => <code className="text-xs">{value}</code>},
    {title: i18next.t("general:Client IP"), key: "clientIp", dataIndex: "clientIp", width: "145px"},
    {title: i18next.t("llm:Mode"), key: "stream", dataIndex: "stream", width: "110px", render: value => <Badge variant="secondary">{value ? "SSE" : "JSON"}</Badge>},
    {title: i18next.t("llm:Stored"), key: "payload", width: "130px", render: (_value, record) => record.truncated ? <Badge variant="warning">{i18next.t("llm:Truncated")}</Badge> : <Badge variant="success">{i18next.t("llm:Complete")}</Badge>},
  ];

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("llm:Request Audits")}>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
          {i18next.t("agent:Auto refresh")}
        </label>
        <Button variant="outline" onClick={() => load(true)} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("general:Refresh")}
        </Button>
      </PageHeader>
      <Alert variant="info" className="mb-4">
        <Info />
        <AlertDescription>{i18next.t("llm:Request audit notice")}</AlertDescription>
      </Alert>
      {error && <Alert variant="destructive" className="mb-4"><CircleX /><AlertDescription>{error}</AlertDescription></Alert>}
      <DataTable
        columns={columns}
        data={records}
        rowKey={record => String(record.id)}
        loading={loading}
        emptyText={i18next.t("llm:No request audits yet")}
        expandedRowRender={record => (
          <pre className="max-h-[32rem] overflow-auto whitespace-pre-wrap break-all p-4 font-mono text-xs">{formatJSON(record.payload)}</pre>
        )}
        serverPagination={{page, pageSize, total, onChange: (nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize); }}}
      />
    </div>
  );
}
