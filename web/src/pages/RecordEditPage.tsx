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

import * as RecordBackend from "@/backend/RecordBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import type {Record as GatewayRecord} from "@/types";

export default function RecordEditPage() {
  const {owner = "", id = ""} = useParams();
  const navigate = useNavigate();
  const [record, setRecord] = React.useState<GatewayRecord | null>(null);

  React.useEffect(() => {
    RecordBackend.getRecord(owner, id).then(res => {
      if (res.status === "ok") {
        setRecord(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [id, owner]);

  const updateField = <K extends keyof GatewayRecord>(key: K, value: GatewayRecord[K]) => {
    setRecord(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (record === null) {
      return;
    }

    RecordBackend.updateRecord(record.owner, id, Setting.deepCopy(record))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          navigate(`/records/${record.owner}/${record.id}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`));
  };

  if (record === null) {
    return <Loading type="page" />;
  }

  const fields: {label: string; key: keyof GatewayRecord}[] = [
    {label: i18next.t("general:Owner"), key: "owner"},
    {label: i18next.t("general:CreatedTime"), key: "createdTime"},
    {label: i18next.t("general:Method"), key: "method"},
    {label: i18next.t("general:Host"), key: "host"},
    {label: i18next.t("general:Path"), key: "path"},
    {label: i18next.t("general:Client ip"), key: "clientIp"},
    {label: i18next.t("general:User-Agent"), key: "userAgent"},
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("general:Edit Record")}
        description={`${record.owner} / ${record.id}`}
        actions={
          <>
            <Button variant="outline" onClick={() => navigate("/records")}>
              {i18next.t("general:Cancel")}
            </Button>
            <Button onClick={save}>{i18next.t("general:Save")}</Button>
          </>
        }
      />

      <Section>
        {fields.map(field => (
          <Field key={String(field.key)} label={field.label} htmlFor={`record-${String(field.key)}`}>
            <Input
              id={`record-${String(field.key)}`}
              value={String(record[field.key] ?? "")}
              onChange={event => updateField(field.key, event.target.value as never)}
            />
          </Field>
        ))}
      </Section>
    </PageContainer>
  );
}
