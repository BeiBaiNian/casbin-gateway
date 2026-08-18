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
import {Link, useLocation, useNavigate} from "react-router-dom";
import {
  ChevronDown,
  FileSearch,
  LogOut,
  Menu as MenuIcon,
  MessageSquare,
  Settings,
  Bot,
} from "lucide-react";
import i18next from "i18next";
import {useTranslation} from "react-i18next";

import * as Setting from "@/Setting";
import {cn} from "@/lib/utils";
import type {Account} from "@/types";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {Button} from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {LanguageSelect} from "@/components/layout/LanguageSelect";

interface MenuEntry {
  key: string;
  label: string;
  icon?: React.ReactNode;
  adminOnly?: boolean;
}

function getMenuEntries(): MenuEntry[] {
  return [
    {key: "/dashboard", label: i18next.t("general:Dashboard")},
    {key: "/agents", label: i18next.t("agent:Agents"), icon: <Bot />, adminOnly: true},
    {
      key: "/agent-records",
      label: i18next.t("agent:Agent Records"),
      icon: <FileSearch />,
      adminOnly: true,
    },
    {
      key: "/agent-sessions",
      label: i18next.t("agent:Agent Sessions"),
      icon: <MessageSquare />,
      adminOnly: true,
    },
    {key: "/nodes", label: i18next.t("general:Nodes")},
    {key: "/sites", label: i18next.t("general:Sites")},
    {key: "/certs", label: i18next.t("general:Certs")},
    {key: "/records", label: i18next.t("general:Records")},
    {key: "/rules", label: i18next.t("general:Rules")},
    {key: "/channels", label: i18next.t("channel:Channels")},
  ];
}

export function Header({
  account,
  onSignout,
}: {
  account: Account | null | undefined;
  onSignout: () => void;
}) {
  // Subscribing to the language keeps the menu labels, which are read eagerly
  // through i18next.t, re-rendering when the language changes.
  useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [mobileOpen, setMobileOpen] = React.useState(false);

  const isAdmin = Setting.isAdminUser(account);
  const entries = getMenuEntries().filter(entry => !entry.adminOnly || isAdmin);
  const selected = entries.find(entry => location.pathname.startsWith(entry.key))?.key;

  const openProfile = () => {
    const profileUrl = Setting.getMyProfileUrl(account);
    if (profileUrl === "") {
      navigate("/account");
    } else {
      Setting.openLink(profileUrl);
    }
  };

  const renderNav = (orientation: "row" | "column") => (
    <nav
      className={cn(
        "gap-1",
        orientation === "row" ? "hidden items-center lg:flex" : "flex flex-col p-2 lg:hidden",
      )}
    >
      {entries.map(entry => (
        <Link
          key={entry.key}
          to={entry.key}
          onClick={() => setMobileOpen(false)}
          className={cn(
            "inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
            selected === entry.key
              ? "bg-primary/10 text-primary"
              : "text-muted-foreground hover:bg-accent hover:text-foreground",
          )}
        >
          {entry.icon}
          {entry.label}
        </Link>
      ))}
    </nav>
  );

  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
      <div className="flex h-14 items-center gap-3 px-4">
        <Link to="/" className="flex shrink-0 items-center">
          <img
            src={`${Setting.StaticBaseUrl}/img/logo_384x96.png`}
            alt="Casbin Gateway"
            className="h-6 w-auto"
          />
        </Link>

        {renderNav("row")}

        <div className="ml-auto flex items-center gap-1">
          <LanguageSelect />
          {account ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button className="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-accent">
                  <Avatar className="h-8 w-8">
                    {account.avatar ? <AvatarImage src={account.avatar} alt={account.name} /> : null}
                    <AvatarFallback style={{backgroundColor: Setting.getAvatarColor(account.name)}}>
                      <span className="text-white">
                        {Setting.getShortName(account.name).slice(0, 2).toUpperCase()}
                      </span>
                    </AvatarFallback>
                  </Avatar>
                  <span className="hidden text-sm font-medium md:inline">
                    {Setting.getShortName(account.displayName || account.name)}
                  </span>
                  <ChevronDown className="h-4 w-4 text-muted-foreground" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48">
                <DropdownMenuItem onClick={openProfile}>
                  <Settings />
                  {i18next.t("account:My Account")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={onSignout}>
                  <LogOut />
                  {i18next.t("account:Sign Out")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : account === null ? (
            <Button asChild size="sm">
              <a href={Setting.getSigninUrl() || "/signin"}>{i18next.t("account:Sign In")}</a>
            </Button>
          ) : null}

          {entries.length > 0 && (
            <Button
              variant="ghost"
              size="icon"
              className="lg:hidden"
              aria-label="Menu"
              onClick={() => setMobileOpen(open => !open)}
            >
              <MenuIcon className="h-5 w-5" />
            </Button>
          )}
        </div>
      </div>
      {mobileOpen ? <div className="border-t lg:hidden">{renderNav("column")}</div> : null}
    </header>
  );
}
