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

/** The lines that point a client of one wire format at a base URL. */
export function envSnippet(protocol: string, baseUrl: string, shell: Shell) {
  const env = protocolEnv[protocol] ?? protocolEnv.openai;
  const variables: [string, string][] = [
    [env.baseUrl, baseUrl],
    [env.token, placeholderToken],
  ];

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
