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
import {Navigate, Route, Routes, useLocation} from "react-router-dom";
import {useTranslation} from "react-i18next";
import i18next from "i18next";

import * as AccountBackend from "@/backend/AccountBackend";
import * as MiscBackend from "@/backend/MiscBackend";
import * as Conf from "@/Conf";
import * as Setting from "@/Setting";
import {findGroupOf, selectedKeyOf} from "@/nav";
import {AppHeader} from "@/components/shared/app-header";
import {AppSidebar, persistOpenKeys, readSavedOpenKeys, useIsDesktop} from "@/components/shared/app-sidebar";
import {Loading} from "@/components/shared/loading";
import {TooltipProvider} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import AccountPage from "@/pages/AccountPage";
import AgentConfigsPage from "@/pages/AgentConfigsPage";
import AgentDashboardPage from "@/pages/AgentDashboardPage";
import AgentDetailPage from "@/pages/AgentDetailPage";
import AgentRecordsPage from "@/pages/AgentRecordsPage";
import AgentSessionPage from "@/pages/AgentSessionPage";
import AgentSessionsPage from "@/pages/AgentSessionsPage";
import AgentsPage from "@/pages/AgentsPage";
import AuthCallback from "@/pages/AuthCallback";
import CertEditPage from "@/pages/CertEditPage";
import CertListPage from "@/pages/CertListPage";
import ProviderEditPage from "@/pages/ProviderEditPage";
import ProviderListPage from "@/pages/ProviderListPage";
import LlmRecordsPage from "@/pages/LlmRecordsPage";
import NodeEditPage from "@/pages/NodeEditPage";
import NodeListPage from "@/pages/NodeListPage";
import RecordEditPage from "@/pages/RecordEditPage";
import RecordListPage from "@/pages/RecordListPage";
import RuleEditPage from "@/pages/RuleEditPage";
import RuleListPage from "@/pages/RuleListPage";
import SettingPage from "@/pages/SettingPage";
import SigninPage from "@/pages/SigninPage";
import SiteEditPage from "@/pages/SiteEditPage";
import SiteListPage from "@/pages/SiteListPage";
import type {Account, ThemeAlgorithm} from "@/types";

// The gateway analytics page is the only one that draws charts, and the
// charting runtime is by far the largest dependency here, so it is fetched only
// when that page is opened.
const DashboardPage = React.lazy(() => import("@/pages/DashboardPage"));

Setting.initCasdoorSdk(Conf.AuthConfig);

const collapsedKey = "siderCollapsed";

export default function App() {
  // Pages call i18next.t() directly, so re-render the whole tree on a language change.
  useTranslation();
  // undefined while the account request is in flight, null when signed out.
  const [account, setAccount] = React.useState<Account | null | undefined>(undefined);
  // The reverse-proxy pages are hidden until the proxy is on, so this decides
  // whether the sidebar shows them at all.
  const [gatewayEnabled, setGatewayEnabled] = React.useState(false);
  const [themeAlgorithm, setThemeAlgorithm] = React.useState<ThemeAlgorithm>(() => {
    const stored = Setting.readThemeAlgorithm();
    // Applied before the first paint so a dark-mode reload never flashes the
    // light palette.
    Setting.applyThemeAlgorithm(stored);
    return stored;
  });
  const [collapsed, setCollapsed] = React.useState(() => localStorage.getItem(collapsedKey) === "true");
  // Below md the rail is a drawer, so the same header button opens it instead
  // of narrowing a column that is not on screen.
  const [mobileNavOpen, setMobileNavOpen] = React.useState(false);
  const isDesktop = useIsDesktop();
  const location = useLocation();

  const selectedKey = selectedKeyOf(location.pathname);
  const wasCollapsedRef = React.useRef(false);
  const [openKeys, setOpenKeys] = React.useState<string[]>(() => {
    if (localStorage.getItem(collapsedKey) === "true") {
      return [];
    }
    const saved = new Set(readSavedOpenKeys());
    const group = findGroupOf(selectedKey);
    if (group) {
      saved.add(group.key);
    }
    return [...saved];
  });

  // Navigating into a collapsed group opens it, and expanding the rail restores
  // whatever the reader last had open rather than the defaults.
  React.useEffect(() => {
    if (collapsed) {
      wasCollapsedRef.current = true;
      setOpenKeys([]);
      return;
    }
    const justExpanded = wasCollapsedRef.current;
    wasCollapsedRef.current = false;
    const group = findGroupOf(selectedKey);

    setOpenKeys(previous => {
      if (justExpanded) {
        const restored = new Set(readSavedOpenKeys());
        if (group) {
          restored.add(group.key);
        }
        return [...restored];
      }
      if (group && !previous.includes(group.key)) {
        return [...previous, group.key];
      }
      return previous;
    });
  }, [selectedKey, collapsed]);

  React.useEffect(() => {
    if (!collapsed) {
      persistOpenKeys(openKeys);
    }
  }, [openKeys, collapsed]);

  const toggleSidebar = () => {
    if (!isDesktop) {
      setMobileNavOpen(value => !value);
      return;
    }
    setCollapsed(value => {
      localStorage.setItem(collapsedKey, String(!value));
      return !value;
    });
  };

  const changeTheme = (next: ThemeAlgorithm) => {
    setThemeAlgorithm(next);
    Setting.saveThemeAlgorithm(next);
  };

  const getAccount = React.useCallback(() => {
    AccountBackend.getAccount()
      .then(res => {
        const user = res.data;
        if (user !== null && user !== undefined) {
          user.hostname = res.data2;
          const language = localStorage.getItem("language");
          if (language && language !== Setting.getLanguage()) {
            Setting.setLanguage(language);
          }
          setAccount(user);
        } else {
          setAccount(null);
        }
      })
      // An unreachable backend is not a signed-in session: without this the
      // app waits on account forever behind the full-page spinner.
      .catch(() => setAccount(null));
  }, []);

  React.useEffect(() => {
    // The auth config lives in app.conf, so ask the backend for it before doing
    // anything that depends on whether Casdoor is configured at all.
    AccountBackend.getSigninOptions()
      .then(res => {
        if (res.status === "ok") {
          Conf.setAuthConfig(res.data.authConfig);
          Setting.initCasdoorSdk(Conf.AuthConfig);
        }
        getAccount();
      })
      .catch(() => getAccount());
  }, [getAccount]);

  React.useEffect(() => {
    if (!account) {
      return;
    }
    MiscBackend.getGatewayStatus()
      .then(res => setGatewayEnabled(res.status === "ok" && (res.data?.gatewayEnabled ?? false)))
      .catch(() => setGatewayEnabled(false));
  }, [account]);

  const signout = () => {
    AccountBackend.signout().then(res => {
      if (res.status === "ok") {
        setAccount(null);
        Setting.showMessage("success", i18next.t("general:Signed out successfully"));
        Setting.goToLink("/");
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to sign out")}: ${res.msg}`);
      }
    });
  };

  /** Wraps a page that only makes sense for a signed-in user. */
  const requireSignin = (render: (user: Account) => React.ReactNode) => {
    if (account === undefined) {
      return <Loading type="page" />;
    }
    if (account === null) {
      sessionStorage.setItem("from", location.pathname);
      return <Navigate to="/signin" replace />;
    }
    return <>{render(account)}</>;
  };

  const redirectHomeIfSignedIn = (element: React.ReactNode) => {
    if (account !== null && account !== undefined) {
      return <Navigate to="/" replace />;
    }
    return <>{element}</>;
  };

  // The sign-in screen is its own full-page layout: no rail, no header.
  if (location.pathname === "/signin" || location.pathname === "/callback") {
    return (
      <TooltipProvider>
        <Routes>
          <Route path="/callback" element={<AuthCallback />} />
          <Route path="/signin" element={redirectHomeIfSignedIn(<SigninPage />)} />
        </Routes>
      </TooltipProvider>
    );
  }

  return (
    <TooltipProvider>
      <div className="bg-muted/30 min-h-screen">
        <AppSidebar
          collapsed={collapsed}
          selectedKey={selectedKey}
          openKeys={openKeys}
          onOpenKeysChange={setOpenKeys}
          isAdmin={Setting.isAdminUser(account)}
          gatewayEnabled={gatewayEnabled}
          mobileOpen={mobileNavOpen}
          onMobileOpenChange={setMobileNavOpen}
        />

        <div
          className={cn(
            "flex min-h-screen min-w-0 flex-col transition-[margin] duration-200",
            collapsed ? "md:ml-16" : "md:ml-64",
          )}
        >
          <AppHeader
            collapsed={collapsed}
            onToggleSidebar={toggleSidebar}
            uri={location.pathname}
            account={account}
            isAdmin={Setting.isAdminUser(account)}
            themeAlgorithm={themeAlgorithm}
            onThemeChange={changeTheme}
            onSignout={signout}
          />

          {/* min-w-0 keeps a wide table from stretching the whole layout. */}
          <main className="flex min-w-0 flex-1 flex-col">
            <React.Suspense fallback={<Loading type="page" />}>
              <Routes>
                <Route
                  path="/"
                  element={requireSignin(user =>
                    Setting.isAdminUser(user) ? (
                      <AgentDashboardPage account={user} />
                    ) : (
                      // The dashboard is about the agents on this host, which
                      // only an admin may see, so everyone else lands on the
                      // first page they can actually use.
                      <Navigate to="/sites" replace />
                    ),
                  )}
                />
                <Route path="/account" element={requireSignin(user => <AccountPage account={user} />)} />
                <Route path="/agents" element={requireSignin(user => <AgentsPage account={user} />)} />
                <Route
                  path="/agents/:agentId"
                  element={requireSignin(user => <AgentDetailPage account={user} />)}
                />
                <Route
                  path="/agent-configs"
                  element={requireSignin(user => <AgentConfigsPage account={user} />)}
                />
                <Route
                  path="/agent-records"
                  element={requireSignin(user => <AgentRecordsPage account={user} />)}
                />
                <Route
                  path="/agent-sessions"
                  element={requireSignin(user => <AgentSessionsPage account={user} />)}
                />
                <Route
                  path="/agent-sessions/:sessionKey"
                  element={requireSignin(user => <AgentSessionPage account={user} />)}
                />
                <Route path="/nodes" element={requireSignin(user => <NodeListPage account={user} />)} />
                <Route path="/nodes/:owner/:nodeName" element={requireSignin(() => <NodeEditPage />)} />
                <Route path="/settings" element={requireSignin(user => <SettingPage account={user} />)} />
                <Route path="/sites" element={requireSignin(user => <SiteListPage account={user} />)} />
                <Route
                  path="/sites/:owner/:siteName"
                  element={requireSignin(user => <SiteEditPage account={user} />)}
                />
                <Route path="/certs" element={requireSignin(user => <CertListPage account={user} />)} />
                <Route path="/certs/:owner/:certName" element={requireSignin(() => <CertEditPage />)} />
                <Route path="/records" element={requireSignin(user => <RecordListPage account={user} />)} />
                <Route path="/records/:owner/:id" element={requireSignin(() => <RecordEditPage />)} />
                <Route path="/rules" element={requireSignin(user => <RuleListPage account={user} />)} />
                <Route path="/rules/:owner/:ruleName" element={requireSignin(() => <RuleEditPage />)} />
                <Route path="/providers" element={requireSignin(user => <ProviderListPage account={user} />)} />
                <Route
                  path="/providers/:owner/:providerName"
                  element={requireSignin(() => <ProviderEditPage />)}
                />
                <Route
                  path="/llm-records"
                  element={requireSignin(user => <LlmRecordsPage account={user} />)}
                />
                <Route path="/proxy-analytics" element={requireSignin(() => <DashboardPage />)} />
              </Routes>
            </React.Suspense>
          </main>

          <footer className="flex items-center justify-center border-t py-5">
            <a target="_blank" rel="noreferrer" href="https://github.com/apache/casbin-gateway">
              <img
                className="h-[30px] w-auto"
                alt="Casbin"
                src={`${Setting.StaticBaseUrl}/img/casbin_logo_1024x256.png`}
              />
            </a>
          </footer>
        </div>
      </div>
    </TooltipProvider>
  );
}
