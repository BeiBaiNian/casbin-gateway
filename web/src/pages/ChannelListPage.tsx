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
import {Link, useNavigate} from "react-router-dom";
import {CircleCheck, CircleX, Pencil, Plug, Plus, RefreshCw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as ChannelBackend from "@/backend/ChannelBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column, type SortOrder} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {CodeText} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {TagsInput} from "@/components/ui/tags-input";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {Account, Channel, ChannelHealth} from "@/types";

function newChannel(owner: string): Channel {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `channel_${randomName}`,
    displayName: `New Channel - ${randomName}`,
    type: "openai",
    status: "enabled",
    models: [],
    priority: 0,
    baseUrl: "",
    apiKey: "",
  };
}

export default function ChannelListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Channel[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [sort, setSort] = React.useState<{field: string; order: SortOrder}>({
    field: "",
    order: undefined,
  });
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Channel>(() => newChannel(account.name));
  const [nameError, setNameError] = React.useState("");
  const [health, setHealth] = React.useState<ChannelHealth[]>([]);

  const fetchChannels = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, nextSort = sort) => {
      setLoading(true);
      ChannelBackend.getChannels(
        account.name,
        nextPage,
        nextPageSize,
        nextSort.order ? nextSort.field : "",
        nextSort.order ?? "",
      )
        .then(res => {
          setLoading(false);
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setSort(nextSort);
          } else {
            Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${res.msg}`);
          }
        })
        .catch(error => {
          setLoading(false);
          Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${error}`);
        });
    },
    [account.name, page, pageSize, sort],
  );

  React.useEffect(() => {
    fetchChannels(1, 10, {field: "", order: undefined});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  // What the proxy has seen of each channel lives in memory and changes as
  // requests are relayed, so it is polled rather than read once.
  React.useEffect(() => {
    const load = () => {
      ChannelBackend.getChannelHealth()
        .then(res => setHealth(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setHealth([]));
    };

    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  const openAddDialog = () => {
    setForm(newChannel(account.name));
    setNameError("");
    setAddOpen(true);
  };

  const setFormField = <K extends keyof Channel>(key: K, value: Channel[K]) => {
    setForm(prev => ({...prev, [key]: value}));
  };

  const addChannel = () => {
    const name = form.name.trim();
    if (name === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    setAdding(true);
    ChannelBackend.addChannel({...form, name: name})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("channel:Channel added successfully"));
          setAddOpen(false);
          fetchChannels();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${error}`);
      });
  };

  const deleteChannel = (channel: Channel) => {
    ChannelBackend.deleteChannel(channel)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${res.msg}`);
          return;
        }
        Setting.showMessage("success", i18next.t("channel:Channel deleted successfully"));
        fetchChannels();
      })
      .catch(error =>
        Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${error}`),
      );
  };

  const columns: Column<Channel>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "160px",
      // The server sorts and paginates, so sorting locally would only reorder
      // the current page.
      sorter: true,
      render: (text: string, record) => (
        <SimpleTooltip title={text}>
          <Link
            to={`/channels/${record.owner}/${record.name}`}
            className="text-primary block truncate font-medium hover:underline"
          >
            {text}
          </Link>
        </SimpleTooltip>
      ),
    },
    {
      title: i18next.t("general:Display name"),
      key: "displayName",
      dataIndex: "displayName",
      width: "200px",
      sorter: true,
      render: (text: string) => (text ? <span className="block truncate">{text}</span> : "-"),
    },
    {
      title: i18next.t("channel:Type"),
      key: "type",
      dataIndex: "type",
      width: "110px",
      render: (text: string) => <Badge variant={text === "openai" ? "success" : "info"}>{text}</Badge>,
    },
    {
      title: i18next.t("channel:Base URL"),
      key: "baseUrl",
      dataIndex: "baseUrl",
      width: "220px",
      ellipsis: true,
      render: (text: string) =>
        text ? (
          <SimpleTooltip title={text}>
            <span className="inline-flex max-w-full">
              <CodeText>{text}</CodeText>
            </span>
          </SimpleTooltip>
        ) : (
          "-"
        ),
    },
    {
      title: i18next.t("channel:Models"),
      key: "models",
      dataIndex: "models",
      width: "220px",
      render: (models: string[]) =>
        !models || models.length === 0 ? (
          "-"
        ) : (
          <div className="flex flex-wrap gap-1">
            {models.map(model => (
              <Badge key={model} variant="muted">
                {model}
              </Badge>
            ))}
          </div>
        ),
    },
    {
      title: i18next.t("channel:Priority"),
      key: "priority",
      dataIndex: "priority",
      width: "100px",
      sorter: true,
    },
    {
      title: i18next.t("channel:Status"),
      key: "status",
      dataIndex: "status",
      width: "120px",
      render: (text: string) =>
        text === "enabled" ? (
          <Badge variant="success">
            <CircleCheck />
            {i18next.t("channel:Enabled")}
          </Badge>
        ) : (
          <Badge variant="muted">
            <CircleX />
            {i18next.t("channel:Disabled")}
          </Badge>
        ),
    },
    {
      title: i18next.t("channel:Health"),
      key: "health",
      width: "150px",
      render: (_text, record) => {
        const item = health.find(entry => entry.channel === `${record.owner}/${record.name}`);
        if (!item) {
          return <span className="text-muted-foreground">{i18next.t("channel:Not used yet")}</span>;
        }
        // A channel that is out of its cooldown but whose last attempts failed
        // is back in rotation without having proven anything yet.
        const badge = !item.healthy ? (
          <Badge variant="warning">
            <CircleX />
            {i18next.t("channel:Cooling down")}
          </Badge>
        ) : item.consecutive > 0 ? (
          <Badge variant="muted">{i18next.t("channel:Recovering")}</Badge>
        ) : (
          <Badge variant="success">
            <CircleCheck />
            {i18next.t("channel:Healthy")}
          </Badge>
        );
        return (
          <SimpleTooltip
            title={
              item.healthy
                ? `${item.successes} / ${item.successes + item.failures}`
                : `${item.lastError} · ${i18next.t("channel:Retried at")} ${item.retryTime}`
            }
          >
            {badge}
          </SimpleTooltip>
        );
      },
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "190px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={() => navigate(`/channels/${record.owner}/${record.name}`)}>
            <Pencil />
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            confirmText={i18next.t("general:Delete")}
            onConfirm={() => deleteChannel(record)}
          >
            <Button size="sm" variant="outline" className="text-destructive">
              <Trash2 />
              {i18next.t("general:Delete")}
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("channel:Channels")}
        actions={
          <Button onClick={openAddDialog}>
            <Plus />
            {i18next.t("channel:New Channel")}
          </Button>
        }
      />

      <DataTable
        columns={columns}
        dataSource={data}
        // An admin sees channels across owners, where names may collide.
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        onSort={(field, order) => fetchChannels(1, pageSize, {field: field, order: order})}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchChannels(nextPage, nextPageSize),
        }}
        title={i18next.t("channel:Channels")}
        description={`${total} ${i18next.t("channel:Channels")}`}
        emptyIcon={Plug}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchChannels()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("channel:New Channel")}
        submitting={adding}
        onSubmit={addChannel}
      >
        <Field label={i18next.t("general:Name")} htmlFor="channel-name" required error={nameError}>
          <Input
            id="channel-name"
            value={form.name}
            onChange={event => {
              setFormField("name", event.target.value);
              setNameError("");
            }}
          />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="channel-display-name">
          <Input
            id="channel-display-name"
            value={form.displayName}
            onChange={event => setFormField("displayName", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("channel:Type")}>
          <SimpleSelect
            value={form.type}
            onChange={value => setFormField("type", value)}
            options={[
              {label: "OpenAI", value: "openai"},
              {label: "Anthropic", value: "anthropic"},
              {label: "Custom", value: "custom"},
            ]}
          />
        </Field>
        <Field label={i18next.t("channel:Base URL")} htmlFor="channel-base-url">
          <Input
            id="channel-base-url"
            value={form.baseUrl}
            placeholder={form.type === "anthropic" ? "https://api.anthropic.com" : "https://api.openai.com/v1"}
            onChange={event => setFormField("baseUrl", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("channel:API Key")} htmlFor="channel-api-key">
          <PasswordInput
            id="channel-api-key"
            placeholder="sk-..."
            value={form.apiKey}
            onChange={event => setFormField("apiKey", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("channel:Models")}>
          <TagsInput
            value={form.models}
            placeholder={form.type === "anthropic" ? "claude-opus-5, claude-sonnet-5" : "gpt-5, gpt-5-mini"}
            onChange={value => setFormField("models", value)}
          />
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
