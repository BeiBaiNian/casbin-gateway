// Copyright 2023 The casbin Authors. All Rights Reserved.
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

import * as Setting from "../Setting";

function request(url, method = "GET", body = null) {
  const options = {
    method: method,
    credentials: "include",
  };
  if (body !== null) {
    options.body = JSON.stringify(Setting.deepCopy(body));
  }
  return fetch(`${Setting.ServerUrl}${url}`, options).then(res => res.json());
}

export function getChannels(owner, page = "", pageSize = "", sortField = "", sortOrder = "") {
  return request(`/api/get-channels?owner=${encodeURIComponent(owner)}&p=${page}&pageSize=${pageSize}&sortField=${sortField}&sortOrder=${sortOrder}`);
}

export function getChannel(owner, name) {
  return request(`/api/get-channel?id=${encodeURIComponent(owner)}/${encodeURIComponent(name)}`);
}

export function addChannel(channel) {
  return request("/api/add-channel", "POST", channel);
}

export function updateChannel(owner, name, channel) {
  return request(`/api/update-channel?id=${encodeURIComponent(owner)}/${encodeURIComponent(name)}`, "POST", channel);
}

export function deleteChannel(channel) {
  return request("/api/delete-channel", "POST", channel);
}

export function testChannel(owner, name) {
  return request("/api/test-channel", "POST", {
    owner: owner,
    name: name,
  });
}
