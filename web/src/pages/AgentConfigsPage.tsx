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
import {Check, Eye, Minus, Package, Plus, RefreshCw, Send, Trash2} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {AddMcpDialog} from "@/components/agent-config/add-mcp-dialog";
import {CopyDialog} from "@/components/agent-config/copy-dialog";
import {DetailDialog, type DetailTarget} from "@/components/agent-config/detail-dialog";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {EmptyState} from "@/components/shared/empty-state";
import {CodeText, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {
  blockedReason,
  counted,
  endpointOf,
  formatBytes,
  inventoryKey,
  itemsOf,
  locationOf,
  presenceOf,
  supports,
  useAgentConfigs,
} from "@/lib/agent-configs";
import type {Account, AgentConfigInventory, AgentConfigItem, AgentConfigKind} from "@/types";

/** The agent picker doubles as the overview: every row carries its own counts. */
function SourcePicker({
  inventories,
  selectedKey,
  onSelect,
}: {
  inventories: AgentConfigInventory[];
  selectedKey: string;
  onSelect: (inventory: AgentConfigInventory) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {inventories.map(inventory => {
        const active = inventoryKey(inventory) === selectedKey;
        return (
          <button
            key={inventoryKey(inventory)}
            type="button"
            onClick={() => onSelect(inventory)}
            className={cn(
              "flex items-center gap-2.5 rounded-md border px-3 py-2 text-left transition-colors",
              active ? "border-primary bg-primary/10" : "hover:bg-accent",
            )}
          >
            <AgentIcon agent={inventory.name} size={20} fallback={<Package className="size-5" />} />
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5 text-sm leading-tight font-medium">
                {inventory.name}
                {inventory.installed ? null : (
                  <SimpleTooltip title={i18next.t("agentConfig:Not installed detail")}>
                    <Badge variant="muted">{i18next.t("agentConfig:Not installed")}</Badge>
                  </SimpleTooltip>
                )}
              </span>
              <span className="text-muted-foreground text-xs leading-tight">
                {i18next.t("agentConfig:Skills")} {inventory.skills.length}
                {" · "}
                {i18next.t("agentConfig:MCP")} {inventory.mcpServers.length}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

export default function AgentConfigsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {inventories, loading, error, scanned, refresh} = useAgentConfigs();

  const [kind, setKind] = React.useState<AgentConfigKind>("skill");
  const [sourceKey, setSourceKey] = React.useState("");
  const [selected, setSelected] = React.useState<string[]>([]);
  const [detail, setDetail] = React.useState<DetailTarget | null>(null);
  const [copyOpen, setCopyOpen] = React.useState(false);
  const [addOpen, setAddOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState("");

  // The scan replaces every inventory, so the chosen source is re-resolved by
  // key rather than held as an object that would go stale on every refresh.
  const source = inventories.find(inventory => inventoryKey(inventory) === sourceKey) ?? inventories[0];

  // The first agent with something in it is a better landing page than the
  // first one alphabetically, but it is chosen once and then committed: left
  // derived, a copy that fills an empty agent would move the page off the
  // source the reader is working from.
  React.useEffect(() => {
    if (sourceKey === "" && inventories.length > 0) {
      const landing = inventories.find(inventory => itemsOf(inventory, kind).length > 0) ?? inventories[0];
      setSourceKey(inventoryKey(landing));
    }
  }, [inventories, kind, sourceKey]);

  React.useEffect(() => {
    setSelected([]);
  }, [sourceKey, kind]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const peers = source ? inventories.filter(inventory => inventoryKey(inventory) !== inventoryKey(source)) : [];
  const items = source ? itemsOf(source, kind) : [];
  const presence = presenceOf(inventories, kind);
  const selectable = items.filter(item => !item.managed);
  const allSelected = selectable.length > 0 && selected.length === selectable.length;

  const toggleAll = () => {
    setSelected(allSelected ? [] : selectable.map(item => item.name));
  };

  const toggleOne = (name: string) => {
    setSelected(previous =>
      previous.includes(name) ? previous.filter(item => item !== name) : [...previous, name],
    );
  };

  const deleteItem = (item: AgentConfigItem) => {
    setDeleting(item.name);
    return AgentConfigBackend.deleteAgentConfigItem(item.agentId, item.owner, item.kind, item.name)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("agentConfig:Deleted")}: ${item.name}`);
          refresh();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to delete"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setDeleting(""));
  };

  const columns: Column<AgentConfigItem>[] = [
    {
      key: "selected",
      width: "40px",
      title: (
        <input
          type="checkbox"
          className="accent-primary size-4 align-middle"
          aria-label={i18next.t("agentConfig:Select all")}
          checked={allSelected}
          disabled={selectable.length === 0}
          onChange={toggleAll}
        />
      ),
      render: (_value, record) =>
        record.managed ? null : (
          <input
            type="checkbox"
            className="accent-primary size-4 align-middle"
            aria-label={record.name}
            checked={selected.includes(record.name)}
            onChange={() => toggleOne(record.name)}
          />
        ),
    },
    {
      key: "name",
      dataIndex: "name",
      title: i18next.t("general:Name"),
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (_value, record) => (
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <span className="truncate font-medium">{record.name}</span>
            {record.managed ? (
              <SimpleTooltip title={i18next.t("agentConfig:Managed by Gateway detail")}>
                <Badge variant="info">{i18next.t("agentConfig:Managed by Gateway")}</Badge>
              </SimpleTooltip>
            ) : null}
          </div>
          {record.description ? (
            <p className="text-muted-foreground line-clamp-2 text-xs">{record.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      key: "summary",
      width: "220px",
      title: kind === "skill" ? i18next.t("agentConfig:Contents") : i18next.t("agentConfig:Endpoint"),
      render: (_value, record) =>
        kind === "skill" ? (
          <span className="text-muted-foreground text-xs">
            {counted(record.files ?? 0, "agentConfig:1 file", "agentConfig:{files} files", "{files}")}
            {record.bytes ? ` · ${formatBytes(record.bytes)}` : ""}
          </span>
        ) : (
          <div className="flex min-w-0 items-center gap-1.5">
            {record.transport ? <Badge variant="muted">{record.transport}</Badge> : null}
            <span className="text-muted-foreground truncate font-mono text-xs" title={endpointOf(record)}>
              {endpointOf(record)}
            </span>
          </div>
        ),
    },
    ...peers.map<Column<AgentConfigItem>>(peer => ({
      key: `peer-${inventoryKey(peer)}`,
      width: "110px",
      align: "center",
      title: (
        <SimpleTooltip title={peer.name}>
          <span className="inline-flex items-center gap-1.5">
            <AgentIcon agent={peer.name} size={14} />
            <span className="truncate">{peer.name}</span>
          </span>
        </SimpleTooltip>
      ),
      render: (_value, record) =>
        !supports(peer, kind) ? (
          <span className="text-muted-foreground/50 text-xs">—</span>
        ) : presence.get(record.name)?.has(inventoryKey(peer)) ? (
          <Check className="text-success mx-auto size-4" />
        ) : (
          <Minus className="text-muted-foreground/40 mx-auto size-4" />
        ),
    })),
    {
      key: "actions",
      width: "110px",
      align: "right",
      title: i18next.t("general:Action"),
      render: (_value, record) => (
        <div className="flex justify-end gap-1">
          <SimpleTooltip title={i18next.t("agentConfig:View")}>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={i18next.t("agentConfig:View")}
              onClick={() => setDetail({item: record, agentName: source?.name ?? record.agentId})}
            >
              <Eye className="size-4" />
            </Button>
          </SimpleTooltip>
          <ConfirmDialog
            title={i18next.t(
              kind === "skill" ? "agentConfig:Delete this skill?" : "agentConfig:Delete this MCP server?",
            )}
            description={
              <span className="flex flex-col gap-1">
                <span>{i18next.t("agentConfig:Delete description")}</span>
                <code className="text-foreground font-mono text-xs break-all">{record.path}</code>
              </span>
            }
            onConfirm={() => deleteItem(record)}
            disabled={record.managed || deleting === record.name}
          >
            <Button
              variant="ghost"
              size="icon-sm"
              className="text-destructive"
              aria-label={i18next.t("general:Delete")}
              disabled={record.managed || deleting === record.name}
            >
              <Trash2 className="size-4" />
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  const location = source ? locationOf(source, kind) : "";
  const blocked = source ? blockedReason(source, kind) : "";

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agentConfig:Skills & MCP")}
        description={i18next.t("agentConfig:Page description")}
        actions={
          <>
            {kind === "mcp" && inventories.length > 0 ? (
              <Button onClick={() => setAddOpen(true)}>
                <Plus className="size-4" />
                {i18next.t("agentConfig:Add MCP server")}
              </Button>
            ) : null}
            <Button variant="outline" onClick={() => refresh(true)} disabled={loading}>
              <RefreshCw className={cn("size-4", loading && "animate-spin")} />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert description={error} /> : null}

      {scanned && !error && inventories.length === 0 ? (
        <EmptyState
          icon={Package}
          title={i18next.t("agentConfig:No agents found")}
          description={i18next.t("agentConfig:No agents found detail")}
        />
      ) : null}

      {source ? (
        <>
          <SourcePicker
            inventories={inventories}
            selectedKey={inventoryKey(source)}
            onSelect={inventory => setSourceKey(inventoryKey(inventory))}
          />

          {source.errors?.length ? <MessageAlert description={source.errors.join("; ")} /> : null}

          <div className="flex flex-wrap items-center justify-between gap-3">
            <Tabs value={kind} onValueChange={value => setKind(value as AgentConfigKind)}>
              <TabsList>
                <TabsTrigger value="skill">
                  {i18next.t("agentConfig:Skills")}
                  <Badge variant="muted">{source.skills.length}</Badge>
                </TabsTrigger>
                <TabsTrigger value="mcp">
                  {i18next.t("agentConfig:MCP servers")}
                  <Badge variant="muted">{source.mcpServers.length}</Badge>
                </TabsTrigger>
              </TabsList>
            </Tabs>

            {location ? (
              <span className="flex items-center gap-2 text-xs">
                <span className="text-muted-foreground">{i18next.t("agentConfig:Read from")}</span>
                <CodeText copyable>{location}</CodeText>
              </span>
            ) : null}
          </div>

          {source.sharedWith?.length ? (
            <p className="text-muted-foreground text-xs">
              {i18next.t("agentConfig:Shared with").replace("{agents}", source.sharedWith.join(", "))}
            </p>
          ) : null}

          {blocked ? (
            <MessageAlert variant="info" description={blocked} />
          ) : (
            <DataTable<AgentConfigItem>
              columns={columns}
              dataSource={items}
              rowKey="name"
              loading={loading && !scanned}
              searchable
              searchPlaceholder={i18next.t("agentConfig:Search by name")}
              pageSize={20}
              emptyIcon={Package}
              emptyText={i18next.t(
                kind === "skill" ? "agentConfig:No skills yet" : "agentConfig:No MCP servers yet",
              )}
              toolbar={
                <Button disabled={selected.length === 0} onClick={() => setCopyOpen(true)}>
                  <Send className="size-4" />
                  {selected.length === 0
                    ? i18next.t("agentConfig:Copy to other agents")
                    : i18next
                      .t("agentConfig:Copy {count} to other agents")
                      .replace("{count}", String(selected.length))}
                </Button>
              }
            />
          )}

          <CopyDialog
            open={copyOpen}
            onOpenChange={setCopyOpen}
            kind={kind}
            source={source}
            inventories={inventories}
            names={selected}
            onDone={refresh}
          />
        </>
      ) : null}

      {source ? (
        <AddMcpDialog
          open={addOpen}
          onOpenChange={setAddOpen}
          inventories={inventories}
          source={source}
          onDone={refresh}
        />
      ) : null}

      <DetailDialog target={detail} onOpenChange={open => !open && setDetail(null)} />
    </PageContainer>
  );
}
