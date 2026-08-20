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

import {cn} from "@/lib/utils";

const DOT_COUNT = 3;

/** The bouncing-dot indicator this UI uses in place of a spinner. */
export function AiDots({
  size = "medium",
  className,
}: {
  size?: "small" | "medium" | "large";
  className?: string;
}) {
  const dotClass = {small: "size-1", medium: "size-2", large: "size-3"}[size];
  const gapClass = {small: "gap-1", medium: "gap-1.5", large: "gap-2.5"}[size];

  return (
    <span className={cn("inline-flex items-center", gapClass, className)}>
      {Array.from({length: DOT_COUNT}).map((_, index) => (
        <span
          key={index}
          className={cn("bg-foreground inline-block rounded-full", dotClass)}
          style={{animation: "dot-bounce 1.4s ease-in-out infinite", animationDelay: `${index * 0.16}s`}}
        />
      ))}
    </span>
  );
}

export function Loading({
  spinning = true,
  tip,
  type = "section",
  className,
}: {
  spinning?: boolean;
  tip?: React.ReactNode;
  type?: "page" | "section" | "small";
  className?: string;
}) {
  if (!spinning) {
    return null;
  }
  const isPage = type === "page";
  const isSmall = type === "small";

  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center",
        isPage && "h-[calc(100vh-160px)] w-full",
        type === "section" && "py-12",
        className,
      )}
    >
      <AiDots size={isSmall ? "small" : isPage ? "large" : "medium"} />
      {tip && !isSmall ? <div className="text-muted-foreground mt-3.5 text-xs tracking-wide">{tip}</div> : null}
    </div>
  );
}
