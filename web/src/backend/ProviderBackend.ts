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

import {itemId, query, request} from "@/backend/request";
import type {Provider, ProviderHealth, ProviderTestResult} from "@/types";

export function getProviders(
  owner: string,
  page: string | number = "",
  pageSize: string | number = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Provider[], number>(
    `/api/get-providers${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getProvider(owner: string, name: string) {
  return request<Provider>(`/api/get-provider${query({id: itemId(owner, name)})}`);
}

export function addProvider(provider: Provider) {
  return request("/api/add-provider", "POST", provider);
}

export function updateProvider(owner: string, name: string, provider: Provider) {
  return request(`/api/update-provider${query({id: itemId(owner, name)})}`, "POST", provider);
}

export function deleteProvider(provider: Provider) {
  return request("/api/delete-provider", "POST", provider);
}

/** The models the provider's upstream reports. The whole provider is posted, not
 * an id: the new-provider form has nothing saved to look up yet. */
export function getProviderModels(provider: Provider) {
  return request<string[]>("/api/get-provider-models", "POST", provider);
}

/** Probes the provider's upstream. The whole provider is posted rather than an
 * id, so a form can be checked before it is saved. */
export function testProvider(provider: Provider) {
  return request<ProviderTestResult>("/api/test-provider", "POST", provider);
}

/** What the proxy has seen of each provider, which is what says why a request
 * went to a fallback rather than to the bound provider. */
export function getProviderHealth() {
  return request<ProviderHealth[]>("/api/get-provider-health");
}
