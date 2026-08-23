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
import {Check, Copy, HelpCircle} from "lucide-react";
import i18next from "i18next";

import {cn} from "@/lib/utils";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";

/** Label followed by a question mark that explains it. */
export function LabelWithTip({
  text,
  tooltip,
  className,
}: {
  text: React.ReactNode;
  tooltip?: React.ReactNode;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-center gap-1", className)}>
      <span>{text}</span>
      <SimpleTooltip title={tooltip}>
        <HelpCircle className="text-muted-foreground size-3.5 cursor-help" />
      </SimpleTooltip>
    </span>
  );
}

function copyBySelection(text: string) {
  const area = document.createElement("textarea");
  area.value = text;
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.select();
  const copied = document.execCommand("copy");
  area.remove();
  return copied;
}

/** Copies text to the clipboard and confirms it in place for a moment. */
export function CopyButton({
  value,
  className,
  size = "icon-xs",
  label,
}: {
  value?: React.ReactNode;
  className?: string;
  size?: "icon-xs" | "icon-sm" | "icon";
  label?: string;
}) {
  const [copied, setCopied] = React.useState(false);
  const buttonLabel = label ?? i18next.t("general:Copy");

  React.useEffect(() => {
    if (!copied) {
      return undefined;
    }
    const timer = setTimeout(() => setCopied(false), 1600);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleCopy = async() => {
    const text = String(value ?? "");
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      return;
    } catch {
      // The clipboard API is missing on a plain-HTTP deployment and refuses an
      // unfocused document, so the older selection copy has to carry those.
    }
    setCopied(copyBySelection(text));
  };

  return (
    <SimpleTooltip title={copied ? i18next.t("general:Copied") : buttonLabel}>
      <Button
        type="button"
        variant="ghost"
        size={size}
        className={className}
        onClick={handleCopy}
        aria-label={buttonLabel}
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </Button>
    </SimpleTooltip>
  );
}

/** Monospace value with a copy affordance — tokens, hostnames, agent ids. */
export function CodeText({
  children,
  className,
  copyable = false,
}: {
  children?: React.ReactNode;
  className?: string;
  copyable?: boolean;
}) {
  return (
    <span className={cn("inline-flex min-w-0 max-w-full items-center gap-1", className)}>
      <code className="bg-muted min-w-0 truncate rounded px-1.5 py-0.5 font-mono text-xs">{children}</code>
      {copyable ? <CopyButton value={children} /> : null}
    </span>
  );
}

/** Scrollable pre block for payloads, prompts and command output. */
export function CodeBlock({
  children,
  className,
  maxHeight = "24rem",
  copyable = false,
}: {
  children?: React.ReactNode;
  className?: string;
  maxHeight?: string;
  copyable?: boolean;
}) {
  return (
    <div className={cn("bg-muted/60 relative overflow-hidden rounded-lg border", className)}>
      {copyable ? <CopyButton value={children} className="absolute top-1.5 right-1.5 z-10" /> : null}
      <pre className="scrollbar-thin overflow-auto p-3 font-mono text-xs leading-relaxed" style={{maxHeight}}>
        {children}
      </pre>
    </div>
  );
}

export interface DescriptionItem {
  key?: string;
  label: React.ReactNode;
  value?: React.ReactNode;
}

/** Key/value rows, for the detail panes above a table. */
export function DescriptionList({
  items,
  columns = 1,
  className,
}: {
  // A row is often written as `condition && {...}`, so anything falsy is a row
  // the caller decided not to render.
  items: (DescriptionItem | false | null | undefined | "" | 0)[];
  columns?: 1 | 2 | 3;
  className?: string;
}) {
  return (
    <dl
      className={cn(
        "grid gap-x-6 gap-y-3 text-sm",
        columns === 2 && "sm:grid-cols-2",
        columns === 3 && "sm:grid-cols-3",
        className,
      )}
    >
      {(items ?? []).filter(Boolean).map((item, index) => {
        const entry = item as DescriptionItem;
        return (
          <div key={entry.key ?? index} className="grid gap-0.5">
            <dt className="text-muted-foreground text-xs">{entry.label}</dt>
            <dd className="break-words">{entry.value ?? "—"}</dd>
          </div>
        );
      })}
    </dl>
  );
}

/** Full-page status screen — 404s and permission errors. */
export function ResultScreen({
  status = "404",
  title,
  subTitle,
  extra,
  className,
}: {
  status?: React.ReactNode;
  title: React.ReactNode;
  subTitle?: React.ReactNode;
  extra?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-3 px-6 py-24 text-center", className)}>
      <p className="text-muted-foreground/60 text-6xl font-semibold tracking-tight">{status}</p>
      <h2 className="text-lg font-semibold">{title}</h2>
      {subTitle ? <p className="text-muted-foreground max-w-md text-sm">{subTitle}</p> : null}
      {extra ? <div className="mt-2">{extra}</div> : null}
    </div>
  );
}

/** The 403 every admin-only page falls back to. */
export function UnauthorizedResult() {
  return (
    <ResultScreen
      status="403"
      title={i18next.t("general:Unauthorized")}
      subTitle={i18next.t(
        "general:Sorry, you do not have permission to access this page or logged in status invalid.",
      )}
      extra={
        <Button asChild>
          <a href="/">{i18next.t("general:Back Home")}</a>
        </Button>
      }
    />
  );
}

/** Horizontal group with consistent spacing. */
export function Space({
  children,
  className,
  size = "default",
  direction = "horizontal",
  wrap = false,
  align,
}: {
  children: React.ReactNode;
  className?: string;
  size?: "small" | "default" | "large";
  direction?: "horizontal" | "vertical";
  wrap?: boolean;
  align?: "start" | "end";
}) {
  const gap = {small: "gap-1.5", default: "gap-2", large: "gap-4"}[size] ?? "gap-2";
  return (
    <div
      className={cn(
        "flex",
        direction === "vertical" ? "flex-col items-stretch" : "items-center",
        wrap && "flex-wrap",
        align === "start" && "items-start",
        align === "end" && "items-end",
        gap,
        className,
      )}
    >
      {children}
    </div>
  );
}
