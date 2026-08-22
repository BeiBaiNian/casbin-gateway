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
import {Link2} from "lucide-react";
import i18next from "i18next";

import * as CertBackend from "@/backend/CertBackend";
import * as MiscBackend from "@/backend/MiscBackend";
import * as NodeBackend from "@/backend/NodeBackend";
import * as RuleBackend from "@/backend/RuleBackend";
import * as SiteBackend from "@/backend/SiteBackend";
import * as Setting from "@/Setting";
import {NodeTable} from "@/components/NodeTable";
import {SiteRuleTable} from "@/components/rules/SiteRuleTable";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {SearchSelect, SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import type {Account, Application, Cert, Node, Rule, Site} from "@/types";

const sslModes = ["HTTP", "HTTPS and HTTP", "HTTPS Only", "Static Folder"];
const statuses = ["Active", "Inactive"];

/** A switch reads as a field only when its label sits beside it. */
function ToggleField({
  label,
  checked,
  onCheckedChange,
}: {
  label: React.ReactNode;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <Field>
      <div className="flex h-9 items-center gap-3">
        <Switch checked={checked} onCheckedChange={onCheckedChange} />
        <span className="text-sm font-medium">{label}</span>
      </div>
    </Field>
  );
}

export default function SiteEditPage({account}: {account: Account}) {
  const {owner = "", siteName = ""} = useParams();
  const navigate = useNavigate();

  const [site, setSite] = React.useState<Site | null>(null);
  const [certs, setCerts] = React.useState<Cert[]>([]);
  const [rules, setRules] = React.useState<Rule[]>([]);
  const [applications, setApplications] = React.useState<Application[]>([]);
  const [providers, setProviders] = React.useState<string[]>([]);
  const [nodes, setNodes] = React.useState<Node[]>([]);

  const getSite = React.useCallback(() => {
    SiteBackend.getSite(owner, siteName).then(res => {
      if (res.status === "ok") {
        setSite(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [owner, siteName]);

  React.useEffect(() => {
    getSite();
    CertBackend.getCerts(account.name).then(res => {
      if (res.status === "ok") {
        setCerts(res.data ?? []);
      }
    });
    RuleBackend.getRules(account.name).then(res => {
      if (res.status === "ok") {
        setRules(res.data ?? []);
      }
    });
    MiscBackend.getApplications(account.name).then(res => {
      if (res.status === "ok") {
        setApplications(res.data ?? []);
      }
    });
    MiscBackend.getCasdoorProviders().then(res => {
      if (res.status === "ok") {
        // Only the notification providers can be alert targets.
        setProviders(
          (res.data ?? [])
            .filter(provider => provider.category === "SMS" || provider.category === "Email")
            .map(provider => `${provider.category}/${provider.name}`),
        );
      }
    });
    NodeBackend.getNodes(account.name).then(res => {
      if (res.status === "ok") {
        setNodes(res.data ?? []);
      }
    });
  }, [account.name, getSite]);

  const updateField = <K extends keyof Site>(key: K, value: Site[K]) => {
    setSite(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (site === null) {
      return;
    }

    SiteBackend.updateSite(site.owner, siteName, Setting.deepCopy(site))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          navigate(`/sites/${site.owner}/${site.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`));
  };

  if (site === null) {
    return <Loading type="page" />;
  }

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("site:Edit Site")}
        description={`${site.owner} / ${site.name}`}
        actions={
          <>
            <Button variant="outline" onClick={() => navigate("/sites")}>
              {i18next.t("general:Cancel")}
            </Button>
            <Button onClick={save}>{i18next.t("general:Save")}</Button>
          </>
        }
      />

      <Section title={i18next.t("site:Site")}>
        <Field label={i18next.t("general:Name")} htmlFor="site-name">
          <Input id="site-name" value={site.name} onChange={event => updateField("name", event.target.value)} />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="site-display-name">
          <Input
            id="site-display-name"
            value={site.displayName}
            onChange={event => updateField("displayName", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("general:Tag")} htmlFor="site-tag">
          <Input id="site-tag" value={site.tag ?? ""} onChange={event => updateField("tag", event.target.value)} />
        </Field>
        <Field label={i18next.t("site:Domain")} htmlFor="site-domain">
          <Input
            id="site-domain"
            value={site.domain}
            onChange={event => updateField("domain", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("site:Other domains")} className="lg:col-span-2">
          <TagsInput value={site.otherDomains} onChange={value => updateField("otherDomains", value)} />
        </Field>
        <Field label={i18next.t("site:Status")}>
          <SimpleSelect value={site.status} onChange={value => updateField("status", value)} options={statuses} />
        </Field>
        <ToggleField
          label={i18next.t("site:Need redirect")}
          checked={site.needRedirect}
          onCheckedChange={checked => updateField("needRedirect", checked)}
        />
        <ToggleField
          label={i18next.t("site:Disable verbose")}
          checked={site.disableVerbose}
          onCheckedChange={checked => updateField("disableVerbose", checked)}
        />
      </Section>

      <Section title={i18next.t("site:Host")}>
        <Field label={i18next.t("site:Host")} htmlFor="site-host">
          <div className="relative">
            <Link2 className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
            <Input
              id="site-host"
              className="pl-9"
              value={site.host}
              onChange={event => updateField("host", event.target.value)}
            />
          </div>
        </Field>
        <Field label={i18next.t("site:Port")}>
          <NumberInput min={0} max={65535} value={site.port} onChange={value => updateField("port", value)} />
        </Field>
        <Field label={i18next.t("site:Hosts")}>
          <TagsInput value={site.hosts} onChange={value => updateField("hosts", value)} />
        </Field>
        <Field label={i18next.t("site:Public IP")}>
          <Input value={site.publicIp} disabled />
        </Field>
        <Field label={i18next.t("site:Node")}>
          <Input value={site.node} disabled />
        </Field>
        <Field label={i18next.t("site:Challenges")}>
          <TagsInput value={site.challenges} onChange={value => updateField("challenges", value)} />
        </Field>
      </Section>

      <Section title={i18next.t("site:Mode")}>
        <Field label={i18next.t("site:Mode")}>
          <SimpleSelect value={site.sslMode} onChange={value => updateField("sslMode", value)} options={sslModes} />
        </Field>
        {/* The certificate is issued and attached by the server. */}
        <Field label={i18next.t("site:SSL cert")}>
          <SearchSelect
            disabled
            value={site.sslCert}
            options={certs.map(cert => cert.name)}
            onChange={value => updateField("sslCert", value)}
          />
        </Field>
        <Field label={i18next.t("site:Casdoor app")}>
          <SearchSelect
            value={site.casdoorApplication}
            options={applications.map(application => application.name)}
            onChange={value => updateField("casdoorApplication", value)}
          />
        </Field>
      </Section>

      <Section title={i18next.t("site:Alert")}>
        <ToggleField
          label={i18next.t("site:Enable alert")}
          checked={site.enableAlert}
          onCheckedChange={checked => updateField("enableAlert", checked)}
        />
        {site.enableAlert ? (
          <>
            <Field label={i18next.t("site:Alert interval")}>
              <NumberInput
                min={1}
                value={site.alertInterval}
                addonAfter={i18next.t("usage:seconds")}
                onChange={value => updateField("alertInterval", value)}
              />
            </Field>
            <Field label={i18next.t("site:Alert try times")}>
              <NumberInput
                min={1}
                value={site.alertTryTimes}
                onChange={value => updateField("alertTryTimes", value)}
              />
            </Field>
            <Field label={i18next.t("site:Alert providers")} className="lg:col-span-3">
              <TagsInput
                value={site.alertProviders}
                suggestions={providers}
                onChange={value => updateField("alertProviders", value)}
              />
            </Field>
          </>
        ) : null}
      </Section>

      <Section title={i18next.t("general:Rules")} columns={1}>
        <SiteRuleTable
          title={i18next.t("general:Rules")}
          account={account}
          sources={rules}
          rules={site.rules}
          onUpdateRules={value => updateField("rules", value)}
        />
      </Section>

      <Section title={i18next.t("site:Nodes")} columns={1}>
        <NodeTable
          title={i18next.t("site:Nodes")}
          table={site.nodes}
          siteName={site.name}
          account={account}
          nodes={nodes}
          onUpdateTable={value => updateField("nodes", value)}
        />
      </Section>
    </PageContainer>
  );
}
