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

import * as MiscBackend from "@/backend/MiscBackend";

/** One request per page load: the token does not change while Gateway runs. */
let pending: Promise<string> | null = null;

function fetchRelayToken() {
  if (pending === null) {
    pending = MiscBackend.getRelayToken()
      .then(res => (res.status === "ok" ? (res.data?.relayToken ?? "") : ""))
      .catch(() => "");
  }
  return pending;
}

/** The token the snippets have to show, so what is copied actually works. */
export function useRelayToken() {
  const [token, setToken] = React.useState("");

  React.useEffect(() => {
    let live = true;
    fetchRelayToken().then(value => {
      if (live) {
        setToken(value);
      }
    });
    return () => {
      live = false;
    };
  }, []);

  return token;
}
