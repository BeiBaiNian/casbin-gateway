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
import {useNavigate, useParams} from "react-router-dom";
import {CircleCheck, CircleX, Save, Zap} from "lucide-react";
import i18next from "i18next";

import * as ChannelBackend from "@/backend/ChannelBackend";
import * as Setting from "@/Setting";
import {EnvSnippet} from "@/components/EnvSnippet";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {CodeText, ResultScreen} from "@/components/shared/misc";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SearchSelect, SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {TagsInput} from "@/components/ui/tags-input";
import {channelProtocol, gatewayBaseUrl, localShell} from "@/lib/channels";
import type {Channel, ChannelTestResult} from "@/types";

// Hard-coded presets for milestone 1.1 (per channel type).
const BASE_URL_PRESETS: Record<string, string[]> = {
  openai: ["https://api.openai.com/v1"],
  anthropic: ["https://api.anthropic.com"],
  custom: ["https://oneapi.example.com", "https://api.deepseek.com/v1", "https://api.moonshot.cn/v1"],
};

const MODEL_PRESETS: Record<string, string[]> = {
  openai: ["gpt-5.5", "gpt-5", "gpt-5-mini", "o3", "o4-mini"],
  anthropic: ["claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"],
  custom: ["deepseek-chat", "deepseek-reasoner", "moonshot-v1-8k", "qwen-max"],
};

// Mirrors object.BuildOpenAiUrl on the server.
function buildOpenAiUrl(baseUrl: string, endpoint: string) {
  let url: URL;
  try {
    url = new URL(baseUrl);
  } catch {
    return "";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "";
  }

  let path = url.pathname.replace(/\/+$/, "");
  if (path.endsWith(endpoint)) {
    path = path.slice(0, path.length - endpoint.length);
  }
  if (!path.endsWith("/v1")) {
    path += "/v1";
  }

  url.pathname = path + endpoint;
  return url.toString();
}

// Mirrors object.BuildAnthropicUrl: the base URL is bare and the endpoint
// carries the /v1 prefix, the opposite of the OpenAI convention.
function buildAnthropicUrl(baseUrl: string, endpoint: string) {
  let url: URL;
  try {
    url = new URL(baseUrl);
  } catch {
    return "";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "";
  }

  let path = url.pathname.replace(/\/+$/, "");
  if (path.endsWith(endpoint)) {
    path = path.slice(0, path.length - endpoint.length);
  }
  path = path.replace(/\/+$/, "").replace(/\/v1$/, "");

  url.pathname = path + endpoint;
  return url.toString();
}

/** The upstream URL requests to a channel of this type end up at. */
function buildUpstreamUrl(baseUrl: string, type: string) {
  return channelProtocol(type) === "anthropic"
    ? buildAnthropicUrl(baseUrl, "/v1/messages")
    : buildOpenAiUrl(baseUrl, "/chat/completions");
}

export default function ChannelEditPage() {
  const {owner = "", channelName = ""} = useParams();
  const navigate = useNavigate();
  // undefined while loading, null when the channel could not be loaded.
  const [channel, setChannel] = React.useState<Channel | null | undefined>(undefined);
  const [testing, setTesting] = React.useState(false);
  const [result, setResult] = React.useState<ChannelTestResult | null>(null);

  React.useEffect(() => {
    ChannelBackend.getChannel(owner, channelName)
      .then(res => {
        if (res.status === "ok") {
          setChannel(res.data);
        } else {
          setChannel(null);
          Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${res.msg}`);
        }
      })
      .catch(error => {
        setChannel(null);
        Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${error}`);
      });
  }, [channelName, owner]);

  const setField = <K extends keyof Channel>(key: K, value: Channel[K]) => {
    setChannel(current => (current ? {...current, [key]: value} : current));
  };

  const save = () => {
    if (!channel) {
      return Promise.resolve(false);
    }

    return ChannelBackend.updateChannel(owner, channelName, channel)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${res.msg}`);
          return false;
        }
        Setting.showMessage("success", i18next.t("channel:Channel saved"));
        return true;
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${error}`);
        return false;
      });
  };

  // The test probes the stored channel, so the edits have to be saved first.
  const test = () => {
    setTesting(true);
    setResult(null);
    save()
      .then(saved => (saved ? ChannelBackend.testChannel(owner, channelName) : null))
      .then(res => {
        setTesting(false);
        if (res === null) {
          return;
        }
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${res.msg}`);
          return;
        }
        setResult(res.data);
      })
      .catch(error => {
        setTesting(false);
        Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${error}`);
      });
  };

  if (channel === undefined) {
    return <Loading type="page" />;
  }

  if (channel === null) {
    return (
      <ResultScreen
        status="404"
        title={i18next.t("channel:Channel not found")}
        extra={<Button onClick={() => navigate("/channels")}>{i18next.t("channel:Channels")}</Button>}
      />
    );
  }

  const upstreamUrl = buildUpstreamUrl(channel.baseUrl, channel.type);

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("channel:Edit Channel")}
        description={`${channel.owner} / ${channel.name}`}
        actions={
          <>
            <Button variant="outline" onClick={test} loading={testing}>
              <Zap />
              {i18next.t("channel:Test Connectivity")}
            </Button>
            <Button onClick={save}>
              <Save />
              {i18next.t("general:Save")}
            </Button>
          </>
        }
      />

      {result ? (
        <MessageAlert
          variant={result.success ? "success" : "destructive"}
          title={
            result.success ? i18next.t("channel:Connection Successful") : i18next.t("channel:Connection Failed")
          }
          description={result.statusCode ? `HTTP ${result.statusCode} - ${result.message}` : result.message}
        />
      ) : null}

      <Section title={i18next.t("channel:Channel")}>
        <Field label={i18next.t("general:Display name")} htmlFor="channel-display-name">
          <Input
            id="channel-display-name"
            value={channel.displayName}
            onChange={event => setField("displayName", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("channel:Type")}>
          <SimpleSelect
            value={channel.type}
            onChange={value => setField("type", value)}
            options={[
              {label: "OpenAI", value: "openai"},
              {label: "Anthropic", value: "anthropic"},
              {label: "Custom", value: "custom"},
            ]}
          />
        </Field>
        <Field
          label={i18next.t("channel:Base URL")}
          hint={
            upstreamUrl === "" ? undefined : (
              <>
                {i18next.t("channel:Base URL hint")}: <CodeText>{upstreamUrl}</CodeText>
              </>
            )
          }
        >
          <SearchSelect
            allowCustomValue
            value={channel.baseUrl}
            placeholder={channel.type === "anthropic" ? "https://api.anthropic.com" : "https://api.openai.com/v1"}
            options={(BASE_URL_PRESETS[channel.type] ?? []).map(url => ({label: url, value: url}))}
            onChange={value => setField("baseUrl", value)}
          />
        </Field>
        <Field
          label={i18next.t("channel:API Key")}
          htmlFor="channel-api-key"
          hint={i18next.t("channel:API Key hint")}
        >
          <PasswordInput
            id="channel-api-key"
            placeholder="sk-..."
            value={channel.apiKey}
            onChange={event => setField("apiKey", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("channel:Models")}>
          <TagsInput
            value={channel.models}
            placeholder={channel.type === "anthropic" ? "claude-opus-5, claude-sonnet-5" : "gpt-5, gpt-5-mini"}
            suggestions={MODEL_PRESETS[channel.type] ?? []}
            onChange={value => setField("models", value)}
          />
        </Field>
        <Field label={i18next.t("channel:Priority")} hint={i18next.t("channel:Priority hint")}>
          <NumberInput min={0} value={channel.priority} onChange={value => setField("priority", value)} />
        </Field>
        <Field label={i18next.t("channel:Status")}>
          <SimpleSelect
            value={channel.status}
            onChange={value => setField("status", value)}
            options={[
              {
                label: (
                  <span className="flex items-center gap-2">
                    <CircleCheck className="text-success size-4" />
                    {i18next.t("channel:Enabled")}
                  </span>
                ),
                value: "enabled",
              },
              {
                label: (
                  <span className="flex items-center gap-2">
                    <CircleX className="text-destructive size-4" />
                    {i18next.t("channel:Disabled")}
                  </span>
                ),
                value: "disabled",
              },
            ]}
          />
        </Field>
      </Section>

      <Section title={i18next.t("channel:Usage")} description={i18next.t("channel:Usage hint")} columns={1}>
        <EnvSnippet
          protocol={channelProtocol(channel.type)}
          baseUrl={gatewayBaseUrl(channelProtocol(channel.type))}
          defaultShell={localShell()}
        />
        <p className="text-muted-foreground text-sm">{i18next.t("channel:Model routing hint")}</p>
      </Section>
    </PageContainer>
  );
}
