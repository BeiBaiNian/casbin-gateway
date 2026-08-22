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
import {Plug} from "lucide-react";
import i18next from "i18next";

import {Field} from "@/components/shared/form-dialog";
import {Input} from "@/components/ui/input";
import {cn} from "@/lib/utils";
import type {Provider} from "@/types";

const imageUrl = /\.(png|jpe?g|svg|gif|webp|ico|avif)$/i;

/**
 * The site a vendor serves its icon from, which is the registrable domain
 * rather than the host the API answers on: https://api.deepseek.com/v1 is
 * deepseek.com, and api.deepseek.com has no favicon of its own.
 */
export function providerSite(value: string) {
  const trimmed = value.trim();
  if (trimmed === "") {
    return "";
  }

  let host = "";
  try {
    host = new URL(/^[a-z]+:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`).hostname;
  } catch {
    return "";
  }

  // An address has no domain to shorten, and shortening it would leave a
  // meaningless fragment of it behind.
  if (/^[\d.]+$/.test(host) || !host.includes(".")) {
    return host;
  }

  const labels = host.split(".").filter(Boolean);
  if (labels.length < 3) {
    return labels.join(".");
  }
  // Two labels, unless the second-to-last is a suffix that carries no name of
  // its own, as in example.com.cn.
  const suffixes = ["com", "net", "org", "gov", "edu", "ac", "co"];
  return labels.slice(suffixes.includes(labels[labels.length - 2]) ? -3 : -2).join(".");
}

/** The image a provider is shown with, or "" when there is nothing to show. */
export function providerIconUrl(icon: string | undefined, baseUrl: string | undefined, size = 64) {
  const value = (icon ?? "").trim();
  if (value !== "" && /^https?:\/\//i.test(value)) {
    try {
      if (imageUrl.test(new URL(value).pathname)) {
        return value;
      }
    } catch {
      // Not a URL after all, so it is treated as a site below.
    }
  }

  const site = providerSite(value === "" ? (baseUrl ?? "") : value);
  return site === "" ? "" : `https://www.google.com/s2/favicons?domain=${site}&sz=${size}`;
}

/**
 * The vendor's own mark, so a list of providers is scanned by logo rather than
 * read. It is taken from the base URL unless the provider names an icon of its
 * own, which is what a self-hosted or aggregated endpoint needs.
 */
export function ProviderIcon({
  icon,
  baseUrl,
  alt,
  size = 20,
  className,
  fallback,
}: {
  icon?: string;
  baseUrl?: string;
  alt?: string;
  size?: number;
  className?: string;
  fallback?: React.ReactNode;
}) {
  const [broken, setBroken] = React.useState(false);
  const src = providerIconUrl(icon, baseUrl, 64);

  React.useEffect(() => {
    setBroken(false);
  }, [src]);

  if (src === "" || broken) {
    return (
      <>
        {fallback ?? (
          <Plug className={cn("text-muted-foreground shrink-0", className)} style={{width: size, height: size}} />
        )}
      </>
    );
  }

  return (
    <img
      src={src}
      alt={alt ?? ""}
      width={size}
      height={size}
      onError={() => setBroken(true)}
      className={cn("block shrink-0 rounded", className)}
    />
  );
}

/** The Icon field of both provider forms, with what it resolves to beside it. */
export function ProviderIconField({
  provider,
  onChange,
}: {
  provider: Provider;
  onChange: (icon: string) => void;
}) {
  return (
    <Field label={i18next.t("provider:Icon")} htmlFor="provider-icon" hint={i18next.t("provider:Icon hint")}>
      <div className="flex items-center gap-2">
        <span className="bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-md border">
          <ProviderIcon
            icon={provider.icon}
            baseUrl={provider.baseUrl}
            alt={provider.displayName || provider.name}
          />
        </span>
        <Input
          id="provider-icon"
          placeholder={providerSite(provider.baseUrl ?? "") || "openai.com"}
          value={provider.icon ?? ""}
          onChange={event => onChange(event.target.value)}
        />
      </div>
    </Field>
  );
}
