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

import {
  Bot,
  ChartColumn,
  FileSearch,
  Globe,
  LayoutDashboard,
  ListFilter,
  MessageSquare,
  Plug,
  ScrollText,
  Server,
  Settings,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";

export interface NavLeaf {
  key: string;
  label: string;
  path: string;
  icon?: LucideIcon;
  adminOnly?: boolean;
}

export interface NavGroup {
  key: string;
  label: string;
  icon?: LucideIcon;
  path?: string;
  adminOnly?: boolean;
  children?: NavLeaf[];
}

/**
 * One description of the navigation, read by both the sidebar and the
 * breadcrumb. Keeping them on the same tree is what stops the two from
 * disagreeing about which section a page belongs to.
 *
 * `label` is an i18next key, resolved at render time so a language switch does
 * not need the tree rebuilt. The reverse-proxy pages are a full feature set, but
 * someone running Gateway to manage the agents on their own machine never needs
 * them, so they live in one group instead of competing with the agent pages.
 */
export const navGroups: NavGroup[] = [
  {key: "/", label: "general:Dashboard", icon: LayoutDashboard, path: "/"},
  {key: "/agents", label: "agent:Agents", icon: Bot, path: "/agents", adminOnly: true},
  {
    key: "/agent-sessions",
    label: "agent:Agent Sessions",
    icon: MessageSquare,
    path: "/agent-sessions",
    adminOnly: true,
  },
  {key: "/agent-records", label: "agent:Agent Records", icon: FileSearch, path: "/agent-records", adminOnly: true},
  {key: "/channels", label: "channel:Channels", icon: Plug, path: "/channels"},
  {
    key: "/advanced",
    label: "general:Advanced",
    icon: Settings,
    children: [
      {key: "/sites", label: "general:Sites", path: "/sites", icon: Globe},
      {key: "/certs", label: "general:Certs", path: "/certs", icon: ShieldCheck},
      {key: "/rules", label: "general:Rules", path: "/rules", icon: ListFilter},
      {key: "/nodes", label: "general:Nodes", path: "/nodes", icon: Server},
      {key: "/records", label: "general:Records", path: "/records", icon: ScrollText},
      {key: "/dashboard", label: "general:Gateway Analytics", path: "/dashboard", icon: ChartColumn},
    ],
  },
];

/** All leaf entries, flattened, for lookups by first path segment. */
export const navLeaves: NavLeaf[] = navGroups.flatMap(group =>
  group.children ? group.children : [{...group, path: group.path ?? group.key} as NavLeaf],
);

export function findLeaf(segmentKey: string) {
  return navLeaves.find(leaf => leaf.key === segmentKey);
}

export function findGroupOf(segmentKey: string) {
  return navGroups.find(group => group.children?.some(child => child.key === segmentKey));
}

/** The key the sidebar treats as selected: the first path segment, or "/" at home. */
export function selectedKeyOf(pathname: string) {
  const firstSegment = pathname.split("/").filter(Boolean)[0];
  return firstSegment === undefined ? "/" : `/${firstSegment}`;
}
