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
import {useNavigate, useParams} from "react-router-dom";
import i18next from "i18next";

import * as NodeBackend from "@/backend/NodeBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import type {Node} from "@/types";

const upgradeModes = ["At Any Time", "No Upgrade", "Half A Hour"];

export default function NodeEditPage() {
  const {owner = "", nodeName = ""} = useParams();
  const navigate = useNavigate();
  const [node, setNode] = React.useState<Node | null>(null);

  React.useEffect(() => {
    NodeBackend.getNode(owner, nodeName).then(res => {
      if (res.status === "ok") {
        setNode(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [nodeName, owner]);

  const updateField = <K extends keyof Node>(key: K, value: Node[K]) => {
    setNode(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (node === null) {
      return;
    }

    NodeBackend.updateNode(node.owner, nodeName, Setting.deepCopy(node))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          navigate(`/nodes/${node.owner}/${node.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`));
  };

  if (node === null) {
    return <Loading type="page" />;
  }

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("node:Edit Node")}
        description={`${node.owner} / ${node.name}`}
        actions={
          <>
            <Button variant="outline" onClick={() => navigate("/nodes")}>
              {i18next.t("general:Cancel")}
            </Button>
            <Button onClick={save}>{i18next.t("general:Save")}</Button>
          </>
        }
      />

      <Section>
        <Field label={i18next.t("general:Name")} htmlFor="node-name">
          <Input id="node-name" value={node.name} onChange={event => updateField("name", event.target.value)} />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="node-display-name">
          <Input
            id="node-display-name"
            value={node.displayName}
            onChange={event => updateField("displayName", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("general:Tag")} htmlFor="node-tag">
          <Input id="node-tag" value={node.tag ?? ""} onChange={event => updateField("tag", event.target.value)} />
        </Field>
        <Field label={i18next.t("general:Client IP")} htmlFor="node-client-ip">
          <Input
            id="node-client-ip"
            value={node.clientIp}
            onChange={event => updateField("clientIp", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("general:Upgrade mode")}>
          <SimpleSelect
            value={node.upgradeMode}
            onChange={value => updateField("upgradeMode", value)}
            options={upgradeModes.map(mode => ({label: i18next.t(`general:${mode}`), value: mode}))}
          />
        </Field>
      </Section>
    </PageContainer>
  );
}
