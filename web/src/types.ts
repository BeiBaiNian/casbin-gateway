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

/** The theme the shell renders in; "dark" toggles the .dark class on <html>. */
export type ThemeAlgorithm = ("default" | "dark")[];

/**
 * The envelope every /api handler returns (controllers.Response): "ok" or
 * "error" in `status`, the payload in `data`, and a second value such as a row
 * count or the hostname in `data2`.
 */
export interface ApiResponse<T = any, T2 = any> {
  status: "ok" | "error";
  msg: string;
  data: T;
  data2: T2;
}

export interface Account {
  owner: string;
  name: string;
  displayName: string;
  avatar: string;
  isAdmin: boolean;
  /** Filled in from `data2` of /api/get-account, not part of the user row. */
  hostname?: string;
}

export interface SigninOptions {
  casdoorAvailable: boolean;
  signinAvailable: boolean;
  autoSignin: boolean;
  authConfig: {
    serverUrl: string;
    clientId: string;
    appName: string;
    organizationName: string;
  };
}

export interface SiteNode {
  name: string;
  version: string;
  diff: string;
  pid?: number;
  status: string;
  message: string;
  provider?: string;
}

export interface Site {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  tag: string;
  domain: string;
  otherDomains: string[];
  needRedirect: boolean;
  disableVerbose: boolean;
  rules: string[];
  enableAlert: boolean;
  alertInterval: number;
  alertTryTimes: number;
  alertProviders: string[];
  challenges: string[];
  host: string;
  port: number;
  hosts: string[];
  sslMode: string;
  sslCert: string;
  publicIp: string;
  node: string;
  isSelf: boolean;
  nodes: SiteNode[];
  casdoorApplication: string;
  status?: string;
}

export interface Node {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  tag: string;
  clientIp: string;
  upgradeMode: string;
}

export interface Cert {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  type: string;
  cryptoAlgorithm: string;
  expireTime: string;
  domainExpireTime: string;
  provider: string;
  account: string;
  accessKey: string;
  accessSecret: string;
  certificate: string;
  privateKey: string;
}

export interface RuleExpression {
  name: string;
  operator: string;
  value: string;
}

export interface Rule {
  owner: string;
  name: string;
  createdTime: string;
  updatedTime: string;
  type: string;
  expressions: RuleExpression[];
  action: string;
  statusCode: number;
  reason: string;
  isVerbose: boolean;
}

export interface Record {
  id: number;
  owner: string;
  name: string;
  createdTime: string;
  method: string;
  host: string;
  path: string;
  clientIp: string;
  userAgent: string;
}

export interface Channel {
  owner: string;
  name: string;
  displayName: string;
  type: string;
  status: string;
  models: string[];
  priority: number;
  baseUrl: string;
  apiKey: string;
}

export interface ChannelTestResult {
  success: boolean;
  statusCode?: number;
  message: string;
}

/** One configuration file a provider switch writes, with what it will contain. */
export interface AgentProviderFile {
  path: string;
  format: string;
  preview: string;
}

/** The state of the agent's own configuration file, written by the orchestrator. */
export interface AgentProvider {
  supported: boolean;
  applied: boolean;
  channel: string;
  mode: string;
  baseUrl: string;
  time: string;
  files: string[];
  detail: string;
}

export interface Agent {
  agentId: string;
  name: string;
  version: string;
  installMethod: string;
  owner: string;
  path: string;
  supported: boolean;
  patched: boolean;
  detail?: string;
  notice?: string;
  followup?: string;
  /** The "owner/name" id of the channel this agent's requests are sent to. */
  channel: string;
  /** The channels tried, in order, when the bound one cannot answer. */
  fallbacks: string[];
  /** "gateway" routes through the local proxy, "direct" writes the upstream. */
  mode: string;
  provider: AgentProvider;
}

/** What the proxy has seen of one channel since Gateway started. */
export interface ChannelHealth {
  channel: string;
  healthy: boolean;
  successes: number;
  failures: number;
  consecutive: number;
  lastError: string;
  lastFailure: string;
  /** When a suspended channel is tried again, empty while it is healthy. */
  retryTime: string;
}

export interface AgentRecord {
  id: string;
  createdTime: string;
  agent: string;
  agentPath?: string;
  user?: string;
  sessionKey?: string;
  title?: string;
  promptId?: string;
  toolUseId?: string;
  eventType: string;
  action?: string;
  outcome?: string;
  toolName?: string;
  mcpServer?: string;
  mcpTool?: string;
  model?: string;
  durationMs?: number;
  clientIp?: string;
  detail?: string;
  object?: unknown;
}

export interface AgentSession {
  agent: string;
  sessionKey: string;
  title?: string;
  recordCount: number;
  firstTime: string;
  lastTime: string;
}

export interface LlmRecord {
  id: number;
  createdTime: string;
  protocol: string;
  endpoint: string;
  model: string;
  channel: string;
  agent: string;
  clientIp: string;
  stream: boolean;
  status: number;
  durationMs: number;
  attempts: number;
  error: string;
  /** Input billed as fresh: the cached part is counted separately. */
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  totalTokens: number;
  /** US dollars, meaningful only where `priced` is true. */
  cost: number;
  priced: boolean;
  systemBytes: number;
  messageCount: number;
  toolCount: number;
  summary: string;
  /** Only returned by getLlmRecord, the list endpoint leaves it out. */
  payload: string;
  redactions: number;
  truncated: boolean;
  bytes: number;
}

export interface LlmRecordStatus {
  mode: "off" | "metadata" | "full";
  retentionDays: number;
  maxRecords: number;
  dropped: number;
  count: number;
}

/** US dollars per million tokens, as the record was costed. */
export interface LlmPrice {
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
}

export interface LlmModelStat {
  model: string;
  requests: number;
  tokens: number;
  cost: number;
}

export interface LlmChannelStat {
  channel: string;
  requests: number;
  failed: number;
  tokens: number;
  cost: number;
}

export interface LlmRecordStats {
  requests: number;
  failed: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
  /** Records whose model has no price entry. */
  unpriced: number;
  models: LlmModelStat[];
  channels: LlmChannelStat[];
}

export interface MetricPoint {
  data: string;
  count: number;
}

export interface Provider {
  name: string;
  category: string;
}

export interface Application {
  name: string;
}

export interface GatewayStatus {
  gatewayEnabled: boolean;
}

/** The two kinds of configuration the Skills & MCP page manages. */
export type AgentConfigKind = "skill" | "mcp";

export interface AgentConfigItem {
  agentId: string;
  owner: string;
  kind: AgentConfigKind;
  name: string;
  description?: string;
  /** The skill's own folder, or the config file the MCP server is an entry of. */
  path: string;
  transport?: string;
  command?: string;
  url?: string;
  files?: number;
  bytes?: number;
  /** Written by Gateway's own agent monitoring, and not the operator's to move. */
  managed?: boolean;
}

/** One installation's skills and MCP servers, as they exist in its own files. */
export interface AgentConfigInventory {
  agentId: string;
  owner: string;
  name: string;
  path?: string;
  /** The account directory the locations below were resolved under. */
  home?: string;
  /** False when the configuration exists but no installation was detected. */
  installed: boolean;
  /** Other agents reading the same files, e.g. Cursor and its CLI. */
  sharedWith?: string[];
  skillsDir?: string;
  mcpFile?: string;
  skillsSupported: boolean;
  mcpSupported: boolean;
  mcpWritable: boolean;
  mcpReadOnly?: string;
  skills: AgentConfigItem[];
  mcpServers: AgentConfigItem[];
  errors?: string[];
}

export interface AgentConfigDetail {
  item: AgentConfigItem;
  content: string;
  files?: string[];
}

export type AgentConfigAction = "create" | "overwrite" | "skip" | "failed";

/** What a copy would do, or did, to one item at one target agent. */
export interface AgentConfigPlanItem {
  agentId: string;
  name: string;
  action: AgentConfigAction;
  reason?: string;
  path?: string;
}
