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
import copy from "copy-to-clipboard";
import FileSaver from "file-saver";
import i18next from "i18next";

import * as CertBackend from "@/backend/CertBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import type {Cert} from "@/types";

export default function CertEditPage() {
  const {owner = "", certName = ""} = useParams();
  const navigate = useNavigate();
  const [cert, setCert] = React.useState<Cert | null>(null);

  React.useEffect(() => {
    CertBackend.getCert(owner, certName).then(res => {
      if (res.status === "ok") {
        setCert(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [certName, owner]);

  const updateField = <K extends keyof Cert>(key: K, value: Cert[K]) => {
    setCert(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (cert === null) {
      return;
    }

    CertBackend.updateCert(cert.owner, certName, Setting.deepCopy(cert))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          navigate(`/certs/${cert.owner}/${cert.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`));
  };

  if (cert === null) {
    return <Loading type="page" />;
  }

  const download = (text: string, filename: string) => {
    FileSaver.saveAs(new Blob([text], {type: "text/plain;charset=utf-8"}), filename);
  };

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("cert:Edit Cert")}
        description={`${cert.owner} / ${cert.name}`}
        actions={
          <>
            <Button variant="outline" onClick={() => navigate("/certs")}>
              {i18next.t("general:Cancel")}
            </Button>
            <Button onClick={save}>{i18next.t("general:Save")}</Button>
          </>
        }
      />

      <Section title={i18next.t("cert:Cert")}>
        <Field label={i18next.t("general:Name")} htmlFor="cert-name">
          <Input id="cert-name" value={cert.name} onChange={event => updateField("name", event.target.value)} />
        </Field>
        <Field label={i18next.t("cert:Type")}>
          <SimpleSelect
            value={cert.type}
            onChange={value => updateField("type", value)}
            options={[{label: "SSL", value: "SSL"}]}
          />
        </Field>
        <Field label={i18next.t("cert:Crypto algorithm")}>
          <SimpleSelect
            value={cert.cryptoAlgorithm}
            onChange={value => updateField("cryptoAlgorithm", value)}
            options={[
              {label: "RSA", value: "RSA"},
              {label: "ECC", value: "ECC"},
            ]}
          />
        </Field>
        <Field label={i18next.t("cert:Expire time")}>
          <Input value={Setting.getFormattedDate(cert.expireTime) ?? ""} disabled />
        </Field>
        <Field label={i18next.t("cert:Domain expire")}>
          <Input value={Setting.getFormattedDate(cert.domainExpireTime) ?? ""} disabled />
        </Field>
        <Field label={i18next.t("cert:Provider")}>
          <SimpleSelect
            value={cert.provider}
            onChange={value => updateField("provider", value)}
            options={[
              {label: "GoDaddy", value: "GoDaddy"},
              {label: "Aliyun", value: "Aliyun"},
            ]}
          />
        </Field>
        <Field label={i18next.t("cert:Account")} htmlFor="cert-account">
          <Input
            id="cert-account"
            value={cert.account}
            onChange={event => updateField("account", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("cert:Access key")} htmlFor="cert-access-key">
          <Input
            id="cert-access-key"
            value={cert.accessKey}
            onChange={event => updateField("accessKey", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("cert:Access secret")} htmlFor="cert-access-secret">
          <Input
            id="cert-access-secret"
            value={cert.accessSecret}
            onChange={event => updateField("accessSecret", event.target.value)}
          />
        </Field>
      </Section>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Section
          title={i18next.t("cert:Certificate")}
          columns={1}
          description={
            <span className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  copy(cert.certificate);
                  Setting.showMessage("success", i18next.t("cert:Certificate copied to clipboard successfully"));
                }}
              >
                {i18next.t("cert:Copy certificate")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => download(cert.certificate, "token_jwt_key.pem")}
              >
                {i18next.t("cert:Download certificate")}
              </Button>
            </span>
          }
        >
          <Textarea
            className="scrollbar-thin h-[420px] font-mono text-xs"
            value={cert.certificate}
            onChange={event => updateField("certificate", event.target.value)}
          />
        </Section>

        <Section
          title={i18next.t("cert:Private key")}
          columns={1}
          description={
            <span className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  copy(cert.privateKey);
                  Setting.showMessage("success", i18next.t("cert:Private key copied to clipboard successfully"));
                }}
              >
                {i18next.t("cert:Copy private key")}
              </Button>
              <Button variant="outline" size="sm" onClick={() => download(cert.privateKey, "token_jwt_key.key")}>
                {i18next.t("cert:Download private key")}
              </Button>
            </span>
          }
        >
          <Textarea
            className="scrollbar-thin h-[420px] font-mono text-xs"
            value={cert.privateKey}
            onChange={event => updateField("privateKey", event.target.value)}
          />
        </Section>
      </div>
    </PageContainer>
  );
}
