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
import {ChevronDown, CircleCheck, CircleX, Pencil, Plug, Plus, RefreshCw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {ProviderIcon, ProviderIconField} from "@/components/ProviderIcon";
import {ProviderModelsField} from "@/components/ProviderModelsField";
import {ProviderSourcePicker, sourceTitle} from "@/components/ProviderSourcePicker";
import {ProviderTestField, useProviderTest} from "@/components/ProviderTestField";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column, type SortOrder} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {CodeText} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SearchSelect, SimpleSelect} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  authProvider,
  authClient,
  baseUrlPlaceholder,
  baseUrlPresets,
  customSource,
  usesClientAuth,
  type ProviderSource,
} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Account, Provider, ProviderHealth} from "@/types";

function newProvider(owner: string, label = "New Provider"): Provider {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `provider_${randomName}`,
    displayName: `${label} - ${randomName}`,
    type: "openai",
    status: "enabled",
    models: [],
    priority: 0,
    baseUrl: "",
    apiKey: "",
    authMode: authProvider,
    icon: "",
    notes: "",
  };
}

/** What a picked source leaves to fill in, which for a subscription is nothing. */
function providerFromSource(owner: string, source: ProviderSource): Provider {
  return {...newProvider(owner, sourceTitle(source)), ...source.provider};
}

/**
 * The fields the source already answered. They stay reachable — a preset is a
 * starting point, not a lock — but out of the way of the one field, if any,
 * that is actually left to fill in.
 */
function Advanced({defaultOpen, children}: {defaultOpen: boolean; children: React.ReactNode}) {
  const [open, setOpen] = React.useState(defaultOpen);

  return (
    <div className="grid gap-4">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-sm"
      >
        <ChevronDown className={cn("size-3.5 transition-transform", open && "rotate-180")} />
        {i18next.t("general:Advanced")}
      </button>
      {open ? children : null}
    </div>
  );
}

export default function ProviderListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Provider[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [loaded, setLoaded] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [sort, setSort] = React.useState<{field: string; order: SortOrder}>({
    field: "",
    order: undefined,
  });
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Provider>(() => newProvider(account.name));
  // null while the dialog is still asking where the credentials come from.
  const [source, setSource] = React.useState<ProviderSource | null>(null);
  const [nameError, setNameError] = React.useState("");
  const [health, setHealth] = React.useState<ProviderHealth[]>([]);
  const test = useProviderTest(form);

  const fetchProviders = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, nextSort = sort) => {
      setLoading(true);
      ProviderBackend.getProviders(
        account.name,
        nextPage,
        nextPageSize,
        nextSort.order ? nextSort.field : "",
        nextSort.order ?? "",
      )
        .then(res => {
          setLoading(false);
          setLoaded(true);
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setSort(nextSort);
          } else {
            Setting.showMessage("error", `${i18next.t("provider:Failed to get providers")}: ${res.msg}`);
          }
        })
        .catch(error => {
          setLoading(false);
          setLoaded(true);
          Setting.showMessage("error", `${i18next.t("provider:Failed to get providers")}: ${error}`);
        });
    },
    [account.name, page, pageSize, sort],
  );

  React.useEffect(() => {
    fetchProviders(1, 10, {field: "", order: undefined});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  // What the proxy has seen of each provider lives in memory and changes as
  // requests are relayed, so it is polled rather than read once.
  React.useEffect(() => {
    const load = () => {
      ProviderBackend.getProviderHealth()
        .then(res => setHealth(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setHealth([]));
    };

    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  const openAddDialog = (start?: ProviderSource) => {
    setForm(start ? providerFromSource(account.name, start) : newProvider(account.name));
    setSource(start ?? null);
    setNameError("");
    setAddOpen(true);
  };

  const setFormField = <K extends keyof Provider>(key: K, value: Provider[K]) => {
    setForm(prev => ({...prev, [key]: value}));
  };

  const pickSource = (picked: ProviderSource) => {
    setForm(providerFromSource(account.name, picked));
    setNameError("");
    setSource(picked);
  };

  // The upstream is probed before the provider is stored, so a key that was
  // pasted wrong is caught here rather than by the first agent that uses it.
  const submitProvider = () => {
    if (form.name.trim() === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    test.guard(addProvider);
  };

  const addProvider = () => {
    const name = form.name.trim();
    setAdding(true);
    ProviderBackend.addProvider({...form, name: name})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("provider:Provider added successfully"));
          setAddOpen(false);
          fetchProviders();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("provider:Failed to add")}: ${error}`);
      });
  };

  const deleteProvider = (provider: Provider) => {
    ProviderBackend.deleteProvider(provider)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to delete")}: ${res.msg}`);
          return;
        }
        Setting.showMessage("success", i18next.t("provider:Provider deleted successfully"));
        fetchProviders();
      })
      .catch(error =>
        Setting.showMessage("error", `${i18next.t("provider:Failed to delete")}: ${error}`),
      );
  };

  const columns: Column<Provider>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "160px",
      // The server sorts and paginates, so sorting locally would only reorder
      // the current page.
      sorter: true,
      render: (text: string, record) => (
        <div className="flex min-w-0 items-center gap-2">
          <ProviderIcon icon={record.icon} baseUrl={record.baseUrl} alt={text} size={18} />
          <SimpleTooltip title={text}>
            <Link
              to={`/providers/${record.owner}/${record.name}`}
              className="text-primary block truncate font-medium hover:underline"
            >
              {text}
            </Link>
          </SimpleTooltip>
        </div>
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
      title: i18next.t("provider:Notes"),
      key: "notes",
      dataIndex: "notes",
      width: "180px",
      ellipsis: true,
      render: (text: string) =>
        text ? (
          <SimpleTooltip title={text}>
            <span className="text-muted-foreground block truncate">{text}</span>
          </SimpleTooltip>
        ) : (
          "-"
        ),
    },
    {
      title: i18next.t("provider:Type"),
      key: "type",
      dataIndex: "type",
      width: "110px",
      render: (text: string) => <Badge variant={text === "openai" ? "success" : "info"}>{text}</Badge>,
    },
    {
      title: i18next.t("provider:Base URL"),
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
      title: i18next.t("provider:Models"),
      key: "models",
      dataIndex: "models",
      width: "220px",
      render: (models: string[], record) =>
        !models || models.length === 0 ? (
          usesClientAuth(record) ? <Badge variant="muted">{i18next.t("provider:Any model")}</Badge> : "-"
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
      title: i18next.t("provider:Priority"),
      key: "priority",
      dataIndex: "priority",
      width: "100px",
      sorter: true,
    },
    {
      title: i18next.t("provider:Status"),
      key: "status",
      dataIndex: "status",
      width: "120px",
      render: (text: string) =>
        text === "enabled" ? (
          <Badge variant="success">
            <CircleCheck />
            {i18next.t("provider:Enabled")}
          </Badge>
        ) : (
          <Badge variant="muted">
            <CircleX />
            {i18next.t("provider:Disabled")}
          </Badge>
        ),
    },
    {
      title: i18next.t("provider:Health"),
      key: "health",
      width: "150px",
      render: (_text, record) => {
        const item = health.find(entry => entry.provider === `${record.owner}/${record.name}`);
        if (!item) {
          return <span className="text-muted-foreground">{i18next.t("provider:Not used yet")}</span>;
        }
        // A provider that is out of its cooldown but whose last attempts failed
        // is back in rotation without having proven anything yet.
        const badge = !item.healthy ? (
          <Badge variant="warning">
            <CircleX />
            {i18next.t("provider:Cooling down")}
          </Badge>
        ) : item.consecutive > 0 ? (
          <Badge variant="muted">{i18next.t("provider:Recovering")}</Badge>
        ) : (
          <Badge variant="success">
            <CircleCheck />
            {i18next.t("provider:Healthy")}
          </Badge>
        );
        return (
          <SimpleTooltip
            title={
              item.healthy
                ? `${item.successes} / ${item.successes + item.failures}`
                : `${item.lastError} · ${i18next.t("provider:Retried at")} ${item.retryTime}`
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
          <Button size="sm" variant="outline" onClick={() => navigate(`/providers/${record.owner}/${record.name}`)}>
            <Pencil />
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            confirmText={i18next.t("general:Delete")}
            onConfirm={() => deleteProvider(record)}
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
        title={i18next.t("provider:Providers")}
        description={i18next.t("provider:Page description")}
        actions={
          <Button onClick={() => openAddDialog()}>
            <Plus />
            {i18next.t("provider:New Provider")}
          </Button>
        }
      />

      {loaded && data.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Plug className="size-4" />
              {i18next.t("provider:No providers yet")}
            </CardTitle>
            <CardDescription>{i18next.t("provider:No providers yet detail")}</CardDescription>
          </CardHeader>
          <CardContent>
            <ProviderSourcePicker onPick={openAddDialog} />
          </CardContent>
        </Card>
      ) : (
        <DataTable
          columns={columns}
          dataSource={data}
          // An admin sees providers across owners, where names may collide.
          rowKey={record => `${record.owner}/${record.name}`}
          loading={loading}
          onSort={(field, order) => fetchProviders(1, pageSize, {field: field, order: order})}
          serverPagination={{
            page: page,
            pageSize: pageSize,
            total: total,
            onChange: (nextPage, nextPageSize) => fetchProviders(nextPage, nextPageSize),
          }}
          title={i18next.t("provider:Providers")}
          description={`${total} ${i18next.t("provider:Providers")}`}
          emptyIcon={Plug}
          toolbar={
            <Button variant="outline" size="sm" onClick={() => fetchProviders()} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          }
        />
      )}

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("provider:New Provider")}
        description={source === null ? i18next.t("provider:Source hint") : undefined}
        size={source === null ? "lg" : "default"}
        submitting={adding || test.testing}
        submitText={i18next.t("provider:Add provider")}
        onSubmit={submitProvider}
        // Nothing is filled in yet while the source is still being picked, so
        // there is nothing to submit.
        footer={
          source === null ? (
            <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
              {i18next.t("general:Cancel")}
            </Button>
          ) : undefined
        }
      >
        {source === null ? (
          <ProviderSourcePicker onPick={pickSource} />
        ) : (
          <>
            <Field label={i18next.t("provider:Source")}>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="info">{sourceTitle(source)}</Badge>
                {usesClientAuth(form) ? (
                  <Badge variant="muted">{i18next.t("provider:Caller's own login")}</Badge>
                ) : null}
                <Button type="button" size="xs" variant="ghost" onClick={() => setSource(null)}>
                  {i18next.t("provider:Change source")}
                </Button>
              </div>
            </Field>
            <Field label={i18next.t("general:Name")} htmlFor="provider-name" required error={nameError}>
              <Input
                id="provider-name"
                value={form.name}
                onChange={event => {
                  setFormField("name", event.target.value);
                  setNameError("");
                }}
              />
            </Field>
            <Field label={i18next.t("general:Display name")} htmlFor="provider-display-name">
              <Input
                id="provider-display-name"
                value={form.displayName}
                onChange={event => setFormField("displayName", event.target.value)}
              />
            </Field>
            {usesClientAuth(form) ? null : (
              <Field
                label={i18next.t("provider:API Key")}
                htmlFor="provider-api-key"
                hint={i18next.t("provider:API Key ownership hint")}
              >
                <PasswordInput
                  id="provider-api-key"
                  placeholder="sk-..."
                  value={form.apiKey}
                  onChange={event => setFormField("apiKey", event.target.value)}
                />
              </Field>
            )}
            <ProviderModelsField
              provider={form}
              hint={usesClientAuth(form) ? i18next.t("provider:Any model hint") : i18next.t("provider:Models hint")}
              onChange={value => setFormField("models", value)}
            />
            <ProviderTestField test={test} submitText={i18next.t("provider:Add provider")} />
            <Field
              label={i18next.t("provider:Notes")}
              htmlFor="provider-notes"
              hint={i18next.t("provider:Notes hint")}
            >
              <Textarea
                id="provider-notes"
                rows={2}
                value={form.notes}
                onChange={event => setFormField("notes", event.target.value)}
              />
            </Field>
            <Advanced key={source.key} defaultOpen={source.key === customSource}>
              <Field label={i18next.t("provider:Type")}>
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
              <Field label={i18next.t("provider:Base URL")} htmlFor="provider-base-url">
                <SearchSelect
                  allowCustomValue
                  id="provider-base-url"
                  value={form.baseUrl}
                  placeholder={baseUrlPlaceholder(form.type)}
                  options={baseUrlPresets(form.type)}
                  onChange={value => setFormField("baseUrl", value)}
                />
              </Field>
              <Field
                label={i18next.t("provider:Authentication")}
                hint={usesClientAuth(form) ? i18next.t("provider:Client auth hint") : undefined}
              >
                <SimpleSelect
                  value={form.authMode}
                  // The stored key is meaningless once the caller's own is forwarded.
                  onChange={value => setForm(prev => ({...prev, authMode: value, apiKey: ""}))}
                  options={[
                    {label: i18next.t("provider:Stored API key"), value: authProvider},
                    {label: i18next.t("provider:Caller's own login"), value: authClient},
                  ]}
                />
              </Field>
              <ProviderIconField provider={form} onChange={value => setFormField("icon", value)} />
            </Advanced>
          </>
        )}
      </FormDialog>
    </PageContainer>
  );
}
