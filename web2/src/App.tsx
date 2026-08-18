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

import * as AccountBackend from "@/backend/AccountBackend";
import * as Conf from "@/Conf";
import * as Setting from "@/Setting";
import {Footer} from "@/components/layout/Footer";
import {Header} from "@/components/layout/Header";
import {PageSpinner} from "@/components/ui/spinner";
import {TooltipProvider} from "@/components/ui/tooltip";
import AccountPage from "@/pages/AccountPage";
import AgentRecordsPage from "@/pages/AgentRecordsPage";
import AgentSessionsPage from "@/pages/AgentSessionsPage";
import AgentsPage from "@/pages/AgentsPage";
import AuthCallback from "@/pages/AuthCallback";
import CertEditPage from "@/pages/CertEditPage";
import CertListPage from "@/pages/CertListPage";
import ChannelEditPage from "@/pages/ChannelEditPage";
import ChannelListPage from "@/pages/ChannelListPage";
import HomePage from "@/pages/HomePage";
import NodeEditPage from "@/pages/NodeEditPage";
import NodeListPage from "@/pages/NodeListPage";
import RecordEditPage from "@/pages/RecordEditPage";
import RecordListPage from "@/pages/RecordListPage";
import RuleEditPage from "@/pages/RuleEditPage";
import RuleListPage from "@/pages/RuleListPage";
import SigninPage from "@/pages/SigninPage";
import SiteEditPage from "@/pages/SiteEditPage";
import SiteListPage from "@/pages/SiteListPage";
import type {Account} from "@/types";

// The dashboard is the only page that draws charts, and echarts is by far the
// largest dependency here, so it is fetched only when that page is opened.
const DashboardPage = React.lazy(() => import("@/pages/DashboardPage"));

Setting.initCasdoorSdk(Conf.AuthConfig);

export default function App() {
  // undefined while the account request is in flight, null when signed out.
  const [account, setAccount] = React.useState<Account | null | undefined>(undefined);
  const location = useLocation();

  const getAccount = React.useCallback(() => {
    AccountBackend.getAccount().then(res => {
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
    });
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

  const signout = () => {
    AccountBackend.signout().then(res => {
      if (res.status === "ok") {
        setAccount(null);
        Setting.showMessage("success", "Successfully signed out, redirected to homepage");
        Setting.goToLink("/");
      } else {
        Setting.showMessage("error", `Signout failed: ${res.msg}`);
      }
    });
  };

  /** Wraps a page that only makes sense for a signed-in user. */
  const requireSignin = (render: (user: Account) => React.ReactNode) => {
    if (account === undefined) {
      return <PageSpinner />;
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

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex min-h-screen flex-col">
        <Header account={account} onSignout={signout} />
        <main className="flex-1">
          <React.Suspense fallback={<PageSpinner />}>
            <Routes>
              <Route path="/callback" element={<AuthCallback />} />
              <Route path="/home" element={<HomePage />} />
              <Route path="/" element={<Navigate to="/sites" replace />} />
              <Route path="/signin" element={redirectHomeIfSignedIn(<SigninPage />)} />
              <Route
                path="/account"
                element={requireSignin(user => <AccountPage account={user} />)}
              />
              <Route path="/agents" element={requireSignin(user => <AgentsPage account={user} />)} />
              <Route
                path="/agent-records"
                element={requireSignin(user => <AgentRecordsPage account={user} />)}
              />
              <Route
                path="/agent-sessions"
                element={requireSignin(user => <AgentSessionsPage account={user} />)}
              />
              <Route path="/nodes" element={requireSignin(user => <NodeListPage account={user} />)} />
              <Route path="/nodes/:owner/:nodeName" element={requireSignin(() => <NodeEditPage />)} />
              <Route path="/sites" element={requireSignin(user => <SiteListPage account={user} />)} />
              <Route
                path="/sites/:owner/:siteName"
                element={requireSignin(user => <SiteEditPage account={user} />)}
              />
              <Route path="/certs" element={requireSignin(user => <CertListPage account={user} />)} />
              <Route path="/certs/:owner/:certName" element={requireSignin(() => <CertEditPage />)} />
              <Route
                path="/records"
                element={requireSignin(user => <RecordListPage account={user} />)}
              />
              <Route path="/records/:owner/:id" element={requireSignin(() => <RecordEditPage />)} />
              <Route path="/rules" element={requireSignin(user => <RuleListPage account={user} />)} />
              <Route path="/rules/:owner/:ruleName" element={requireSignin(() => <RuleEditPage />)} />
              <Route
                path="/channels"
                element={requireSignin(user => <ChannelListPage account={user} />)}
              />
              <Route
                path="/channels/:owner/:channelName"
                element={requireSignin(() => <ChannelEditPage />)}
              />
              <Route path="/dashboard" element={requireSignin(() => <DashboardPage />)} />
            </Routes>
          </React.Suspense>
        </main>
        <Footer />
      </div>
    </TooltipProvider>
  );
}
