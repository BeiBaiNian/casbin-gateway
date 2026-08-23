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
import i18next from "i18next";

import * as SettingBackend from "@/backend/SettingBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {UnauthorizedResult} from "@/components/shared/misc";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {Textarea} from "@/components/ui/textarea";
import type {Account, Setting as SettingType} from "@/types";

type KeysOfType<T> = {[K in keyof SettingType]: SettingType[K] extends T ? K : never}[keyof SettingType];
type StringKey = KeysOfType<string>;
type NumberKey = KeysOfType<number>;
type BooleanKey = KeysOfType<boolean>;

export default function SettingPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [setting, setSetting] = React.useState<SettingType | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }

    SettingBackend.getSetting().then(res => {
      if (res.status === "ok") {
        setSetting(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [isAdmin]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  if (setting === null) {
    return <Loading type="page" />;
  }

  const updateField = <K extends keyof SettingType>(key: K, value: SettingType[K]) => {
    setSetting(current => (current === null ? current : {...current, [key]: value}));
  };

  // The backend applies what it stores, so a port it cannot bind or a value a
  // subsystem refuses comes back as an error while the setting stays saved.
  const save = () => {
    setSaving(true);
    SettingBackend.updateSetting(Setting.deepCopy(setting))
      .then(res => {
        setSaving(false);
        if (res.status === "ok") {
          setSetting(res.data);
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(error => {
        setSaving(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`);
      });
  };

  const textField = (key: StringKey, label: string, hint?: string) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <Input
        id={`setting-${key}`}
        value={setting[key]}
        onChange={event => updateField(key, event.target.value)}
      />
    </Field>
  );

  const secretField = (key: StringKey, label: string, hint?: string) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <PasswordInput
        id={`setting-${key}`}
        value={setting[key]}
        onChange={event => updateField(key, event.target.value)}
      />
    </Field>
  );

  const numberField = (key: NumberKey, label: string, hint?: string, min = 0, max?: number) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <NumberInput
        id={`setting-${key}`}
        value={setting[key]}
        onChange={value => updateField(key, value)}
        min={min}
        max={max}
      />
    </Field>
  );

  const switchField = (key: BooleanKey, label: string, hint?: string) => (
    <Field label={label} hint={hint}>
      <div className="flex h-9 items-center">
        <Switch checked={setting[key]} onCheckedChange={value => updateField(key, value)} />
      </div>
    </Field>
  );

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("setting:Settings")}
        description={i18next.t("setting:Page description")}
        actions={
          <Button onClick={save} loading={saving}>
            {i18next.t("general:Save")}
          </Button>
        }
      />

      <Section columns={2} title={i18next.t("setting:Reverse proxy")} description={i18next.t("setting:Reverse proxy description")}>
        {switchField("gatewayEnabled", i18next.t("setting:Enable the reverse proxy"))}
        {numberField("gatewayHttpPort", i18next.t("setting:Gateway HTTP port"), undefined, 1, 65535)}
        {numberField("gatewayHttpsPort", i18next.t("setting:Gateway HTTPS port"), undefined, 1, 65535)}
      </Section>

      <Section columns={2} title={i18next.t("setting:LLM records")} description={i18next.t("setting:LLM records description")}>
        <Field label={i18next.t("setting:Record mode")}>
          <SimpleSelect
            value={setting.llmRecordMode}
            onChange={value => updateField("llmRecordMode", value as SettingType["llmRecordMode"])}
            options={[
              {label: i18next.t("llm:Recording off"), value: "off"},
              {label: i18next.t("llm:Record metadata"), value: "metadata"},
              {label: i18next.t("llm:Record metadata and bodies"), value: "full"},
            ]}
          />
        </Field>
        {numberField("llmRecordRetentionDays", i18next.t("setting:Retention days"), undefined, 1)}
        {numberField("llmRecordMaxRecords", i18next.t("setting:Max records"), undefined, 1)}
        {numberField("llmRecordQueueCapacity", i18next.t("setting:Queue capacity"), i18next.t("setting:Queue capacity hint"), 1)}
        {numberField("llmRecordMaxPayloadBytes", i18next.t("setting:Max payload bytes"), undefined, 65536, 33554432)}
        {textField("llmPricingFile", i18next.t("setting:Pricing file"), i18next.t("setting:Pricing file hint"))}
      </Section>

      <Section columns={2} title={i18next.t("setting:Agents")}>
        {textField("agentPatchStateDir", i18next.t("setting:Agent state dir"), i18next.t("setting:Agent state dir hint"))}
        {numberField("agentRecordCapacity", i18next.t("setting:Agent record capacity"), i18next.t("setting:Agent record capacity hint"), 1)}
        {numberField("agentMonitorPollSeconds", i18next.t("setting:Agent poll seconds"), undefined, 1)}
      </Section>

      <Section columns={2} collapsible title={i18next.t("setting:Sign-in")} description={i18next.t("setting:Sign-in description")}>
        {textField("casdoorEndpoint", i18next.t("setting:Casdoor endpoint"))}
        {textField("clientId", i18next.t("setting:Client ID"))}
        {secretField("clientSecret", i18next.t("setting:Client secret"))}
        {textField("casdoorOrganization", i18next.t("setting:Casdoor organization"))}
        {textField("casdoorApplication", i18next.t("setting:Casdoor application"))}
      </Section>

      <Section columns={2} title={i18next.t("setting:Security")}>
        {secretField("apiKeyEncryptionKey", i18next.t("setting:API key encryption key"), i18next.t("setting:API key encryption key hint"))}
        {textField("relayToken", i18next.t("setting:Relay token"), i18next.t("setting:Relay token hint"))}
      </Section>

      <Section columns={2} collapsible title={i18next.t("setting:Network and certificates")}>
        {textField("httpProxy", i18next.t("setting:Outbound SOCKS5 proxy"), i18next.t("setting:Outbound SOCKS5 proxy hint"))}
        {textField("acmeEmail", i18next.t("setting:ACME email"), i18next.t("setting:ACME email hint"))}
        <Field label={i18next.t("setting:ACME private key")} htmlFor="setting-acmePrivateKey" className="md:col-span-2">
          <Textarea
            id="setting-acmePrivateKey"
            rows={4}
            value={setting.acmePrivateKey}
            onChange={event => updateField("acmePrivateKey", event.target.value)}
          />
        </Field>
      </Section>

      <Section
        columns={2}
        collapsible
        title={i18next.t("setting:Application deployment")}
        description={i18next.t("setting:Application deployment description")}
      >
        {textField("appDir", i18next.t("setting:App dir"))}
        {textField("language", i18next.t("setting:Deployed app language"))}
        {textField("appMap", i18next.t("setting:App map"), i18next.t("setting:App map hint"))}
        {textField("clientIdPrefix", i18next.t("setting:Client ID prefix"))}
        {textField("clientSecretPrefix", i18next.t("setting:Client secret prefix"))}
        {textField("dbRegionId", i18next.t("setting:RDS region"))}
        {textField("dbAccessKeyId", i18next.t("setting:RDS access key ID"))}
        {secretField("dbAccessKeySecret", i18next.t("setting:RDS access key secret"))}
        {textField("dbInstanceId", i18next.t("setting:RDS instance"))}
        {textField("dbHost", i18next.t("setting:Database host"))}
        {textField("dbUser", i18next.t("setting:Database user"))}
        {secretField("dbPass", i18next.t("setting:Database password"))}
      </Section>
    </PageContainer>
  );
}
