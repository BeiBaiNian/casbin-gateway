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
import {Link, useParams, useSearchParams} from "react-router-dom";
import {ArrowLeft, Bot, Image as ImageIcon, RefreshCw, Wrench} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {CodeBlock, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import type {Account, AgentMessage, AgentMessageBlock, AgentTranscript} from "@/types";

const ROLE_VARIANTS: Record<string, BadgeVariant> = {
  user: "info",
  assistant: "success",
  system: "warning",
  developer: "warning",
  tool: "muted",
};

function BlockView({block}: {block: AgentMessageBlock}) {
  switch (block.kind) {
  case "thinking":
    return (
      <div className="flex flex-wrap items-start gap-2">
        <Badge variant="muted">{i18next.t("agent:Thinking")}</Badge>
        {block.text ? (
          <pre className="text-muted-foreground scrollbar-thin min-w-0 flex-1 overflow-x-auto text-xs leading-relaxed whitespace-pre-wrap italic">
            {block.text}
          </pre>
        ) : (
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Not recorded")}</span>
        )}
      </div>
    );
  case "toolUse":
    return (
      <div className="grid gap-1">
        <Badge variant="warning">
          <Wrench />
          {block.tool || i18next.t("agent:Tool call")}
        </Badge>
        {block.text ? <CodeBlock maxHeight="16rem">{block.text}</CodeBlock> : null}
      </div>
    );
  case "toolResult":
    return (
      <div className="grid gap-1">
        <Badge variant={block.isError ? "danger" : "muted"}>
          {block.isError ? i18next.t("agent:Tool error") : i18next.t("agent:Tool result")}
        </Badge>
        {block.text ? <CodeBlock maxHeight="16rem">{block.text}</CodeBlock> : null}
      </div>
    );
  case "image":
    return (
      <Badge variant="muted">
        <ImageIcon />
        {i18next.t("agent:Image")}
      </Badge>
    );
  default:
    return (
      <pre className="scrollbar-thin overflow-x-auto text-xs leading-relaxed whitespace-pre-wrap">{block.text}</pre>
    );
  }
}

function MessageView({message, index}: {message: AgentMessage; index: number}) {
  return (
    <div className="rounded-lg border">
      <div className="bg-muted/40 flex flex-wrap items-center gap-2 border-b px-3 py-1.5">
        <Badge variant={ROLE_VARIANTS[message.role] ?? "secondary"}>{message.role}</Badge>
        <span className="text-muted-foreground text-xs">#{index + 1}</span>
        {message.time ? (
          <span className="text-muted-foreground text-xs">{new Date(message.time).toLocaleString()}</span>
        ) : null}
      </div>
      <div className="grid gap-3 px-3 py-2">
        {message.blocks.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Empty message")}</span>
        ) : (
          message.blocks.map((block, position) => <BlockView key={position} block={block} />)
        )}
      </div>
    </div>
  );
}

/**
 * One session, read out of the transcript the agent itself wrote. Monitoring
 * only ever sees what happened after it was turned on, so on the first day this
 * is the only way to open anything on the Sessions page.
 */
export default function AgentSessionPage({account}: {account: Account}) {
  const {sessionKey = ""} = useParams();
  const [searchParams] = useSearchParams();
  const agentId = searchParams.get("agent") ?? "";

  const [transcript, setTranscript] = React.useState<AgentTranscript | null>(null);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const isAdmin = Setting.isAdminUser(account);

  const load = React.useCallback(() => {
    if (!isAdmin || sessionKey === "") {
      return;
    }

    setLoading(true);
    AgentBackend.getAgentSession(agentId, sessionKey)
      .then(res => {
        if (res.status === "ok") {
          setTranscript(res.data ?? null);
          setError("");
        } else {
          setError(res.msg || i18next.t("agent:Failed to read this transcript"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [agentId, isAdmin, sessionKey]);

  React.useEffect(() => {
    load();
  }, [load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const session = transcript?.session;
  const messages = transcript?.messages ?? [];

  return (
    <PageContainer>
      <PageHeader
        title={session?.title || sessionKey}
        description={session?.cwd || session?.path}
        actions={
          <>
            <Button variant="outline" size="sm" asChild>
              <Link to="/agent-sessions">
                <ArrowLeft />
                {i18next.t("agent:Agent Sessions")}
              </Link>
            </Button>
            <Button variant="outline" size="sm" onClick={load} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      {session ? (
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="info">
            <AgentIcon agent={session.agent} fallback={<Bot className="size-3" />} size={12} />
            {session.agent}
          </Badge>
          <Badge variant="muted">
            {i18next.t("agent:{count} messages").replace("{count}", String(messages.length))}
          </Badge>
          <span className="text-muted-foreground text-xs">{new Date(session.lastTime).toLocaleString()}</span>
        </div>
      ) : null}

      {transcript?.truncated ? <MessageAlert title={i18next.t("agent:Transcript truncated")} /> : null}

      {transcript !== null && messages.length === 0 ? (
        <Card>
          <CardContent className="text-muted-foreground py-8 text-center text-sm">
            {i18next.t("agent:This transcript holds no conversation")}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {messages.map((message, index) => (
            <MessageView key={index} message={message} index={index} />
          ))}
        </div>
      )}
    </PageContainer>
  );
}
