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

import * as Setting from "@/Setting";

/** Mirrors object.ChannelProtocol on the server. */
export function channelProtocol(type: string) {
  return type === "anthropic" ? "anthropic" : "openai";
}

/** Mirrors object.ChannelAuthClient on the server. */
export const authClient = "client";

/** Mirrors object.UsesClientAuth: whose credentials reach the upstream. */
export function usesClientAuth(channel: {authMode?: string} | undefined) {
  return channel?.authMode === authClient;
}

/** The env vars a client of one wire format reads its endpoint and key from. */
const protocolEnv: Record<string, {baseUrl: string; token: string}> = {
  anthropic: {baseUrl: "ANTHROPIC_BASE_URL", token: "ANTHROPIC_AUTH_TOKEN"},
  openai: {baseUrl: "OPENAI_BASE_URL", token: "OPENAI_API_KEY"},
};

/**
 * A stand-in for the key a client refuses to start without. Gateway authenticates
 * upstream with the channel's own key, so this value is never used for anything.
 */
const placeholderToken = "casbin-gateway";

export const shells = ["bash", "PowerShell"] as const;
export type Shell = (typeof shells)[number];

/**
 * The lines that point a client of one wire format at a base URL. A client-auth
 * channel forwards the client's own credentials, so the token is left out
 * there: setting it would replace the sign-in the client already has.
 */
export function envSnippet(protocol: string, baseUrl: string, shell: Shell, includeToken = true) {
  const env = protocolEnv[protocol] ?? protocolEnv.openai;
  const variables: [string, string][] = [[env.baseUrl, baseUrl]];
  if (includeToken) {
    variables.push([env.token, placeholderToken]);
  }

  return variables
    .map(([name, value]) =>
      shell === "PowerShell" ? `$env:${name} = "${value}"` : `export ${name}="${value}"`,
    )
    .join("\n");
}

/**
 * The base URL for requests routed by model name, i.e. not through an agent.
 * An OpenAI client appends /chat/completions to it and an Anthropic one appends
 * /v1/messages, which is why the /v1 belongs to only one of them.
 */
export function gatewayBaseUrl(protocol: string) {
  const origin = Setting.ServerUrl || window.location.origin;
  return protocol === "anthropic" ? origin : `${origin}/v1`;
}

/** The shell a host is driven from, guessed from an absolute path on it. */
export function shellForPath(path: string): Shell {
  return /^[a-zA-Z]:[\\/]/.test(path) ? "PowerShell" : "bash";
}

/** The shell the browser's own machine is driven from. */
export function localShell(): Shell {
  return navigator.userAgent.includes("Windows") ? "PowerShell" : "bash";
}

/** A vendor the channel forms can fill themselves in from. */
export interface ChannelPreset {
  label: string;
  type: string;
  baseUrl: string;
  models: string[];
}

export const channelPresets: ChannelPreset[] = [
  {
    label: "OpenAI",
    type: "openai",
    baseUrl: "https://api.openai.com/v1",
    models: ["gpt-5.5", "gpt-5", "gpt-5-mini", "o3", "o4-mini"],
  },
  {
    label: "Anthropic",
    type: "anthropic",
    baseUrl: "https://api.anthropic.com",
    models: ["claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"],
  },
  {
    label: "DeepSeek",
    type: "custom",
    baseUrl: "https://api.deepseek.com/v1",
    models: ["deepseek-chat", "deepseek-reasoner"],
  },
  {
    label: "Moonshot",
    type: "custom",
    baseUrl: "https://api.moonshot.cn/v1",
    models: ["moonshot-v1-8k", "moonshot-v1-32k"],
  },
  {
    label: "Qwen",
    type: "custom",
    baseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    models: ["qwen-max", "qwen-plus"],
  },
];

/**
 * What a channel starts from when it is meant to forward the caller's own
 * login. Anthropic is the vendor of the clients that mode exists for; Codex
 * signs in against an API Gateway does not relay.
 */
export function clientAuthDefaults() {
  const preset = channelPresets.find(item => item.type === "anthropic");
  return {
    type: "anthropic",
    baseUrl: preset?.baseUrl ?? "",
    models: [] as string[],
    apiKey: "",
    authMode: authClient,
  };
}

/** The base URLs and models offered for a channel type, from the vendors of it. */
export function baseUrlPresets(type: string) {
  return channelPresets.filter(preset => preset.type === type).map(preset => preset.baseUrl);
}

export function modelPresets(type: string) {
  return channelPresets.filter(preset => preset.type === type).flatMap(preset => preset.models);
}

export function baseUrlPlaceholder(type: string) {
  return channelProtocol(type) === "anthropic" ? "https://api.anthropic.com" : "https://api.openai.com/v1";
}

export function modelsPlaceholder(type: string) {
  return channelProtocol(type) === "anthropic" ? "claude-opus-5, claude-sonnet-5" : "gpt-5, gpt-5-mini";
}
