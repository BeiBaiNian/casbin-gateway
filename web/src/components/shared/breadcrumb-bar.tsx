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

import {Link} from "react-router-dom";
import i18next from "i18next";

import {findLeaf} from "@/nav";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

/**
 * Derives the trail from the URL against the shared nav tree, so a page never
 * has to declare its own breadcrumb. A path whose first segment is not in the
 * tree renders nothing rather than an invented label.
 */
export function BreadcrumbBar({uri}: {uri: string}) {
  const segments = (uri || "").split("/").filter(Boolean);
  if (segments.length === 0) {
    return null;
  }

  const rootLeaf = findLeaf(`/${segments[0]}`);
  if (!rootLeaf) {
    return null;
  }
  const rootLabel = i18next.t(rootLeaf.label);

  const lastSegment = segments[segments.length - 1];
  const lastLeaf = segments.length > 1 ? findLeaf(`/${lastSegment}`) : null;
  const lastLabel = lastLeaf ? i18next.t(lastLeaf.label) : decodeURIComponent(lastSegment);

  return (
    <Breadcrumb>
      <BreadcrumbList className="text-xs sm:gap-1.5">
        <BreadcrumbItem>
          <BreadcrumbLink asChild>
            <Link to="/">{i18next.t("general:Home")}</Link>
          </BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        {segments.length === 1 ? (
          <BreadcrumbItem>
            <BreadcrumbPage>{rootLabel}</BreadcrumbPage>
          </BreadcrumbItem>
        ) : (
          <>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link to={`/${segments[0]}`}>{rootLabel}</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage className="max-w-[240px] truncate">{lastLabel}</BreadcrumbPage>
            </BreadcrumbItem>
          </>
        )}
      </BreadcrumbList>
    </Breadcrumb>
  );
}
