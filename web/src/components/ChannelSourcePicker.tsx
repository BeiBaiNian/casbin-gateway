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

import {KeyRound, LogIn, Settings2} from "lucide-react";
import i18next from "i18next";

import {channelSources, customSource, subscriptionSource, type ChannelSource} from "@/lib/channels";

/**
 * The title and the line under it. A vendor card is named after the vendor and
 * says where its requests go; the two cards that are not a vendor say what they
 * are instead, because their base URL would not.
 */
export function sourceTitle(source: ChannelSource) {
  if (source.key === subscriptionSource) {
    return i18next.t("channel:Claude subscription");
  }
  if (source.key === customSource) {
    return i18next.t("channel:Custom vendor");
  }
  return source.label;
}

function sourceDetail(source: ChannelSource) {
  if (source.key === subscriptionSource) {
    return i18next.t("channel:Claude subscription detail");
  }
  if (source.key === customSource) {
    return i18next.t("channel:Custom vendor detail");
  }
  return source.channel.baseUrl ?? "";
}

function sourceIcon(source: ChannelSource) {
  if (source.key === subscriptionSource) {
    return LogIn;
  }
  if (source.key === customSource) {
    return Settings2;
  }
  return KeyRound;
}

/**
 * The first step of creating a channel: where its credentials come from. Picking
 * a card fills in the type, base URL, models and authentication mode, so a
 * subscription needs nothing typed and a vendor needs only its key.
 */
export function ChannelSourcePicker({onPick}: {onPick: (source: ChannelSource) => void}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {channelSources.map(source => {
        const Icon = sourceIcon(source);
        return (
          <button
            key={source.key}
            type="button"
            onClick={() => onPick(source)}
            className="hover:border-primary hover:bg-accent/40 flex flex-col items-start gap-1 rounded-lg border p-4 text-left transition-colors"
          >
            <span className="flex items-center gap-2 text-sm font-medium">
              <Icon className="size-4 shrink-0" />
              {sourceTitle(source)}
            </span>
            <span className="text-muted-foreground break-all text-xs">{sourceDetail(source)}</span>
          </button>
        );
      })}
    </div>
  );
}
