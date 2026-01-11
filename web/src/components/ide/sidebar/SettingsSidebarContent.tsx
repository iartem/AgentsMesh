"use client";

import React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth";
import { useTranslations } from "@/lib/i18n/client";
import {
  Settings,
  Users,
  Bot,
  Server,
  GitBranch,
  Bell,
  CreditCard,
} from "lucide-react";

interface SettingsSidebarContentProps {
  className?: string;
}

export function SettingsSidebarContent({ className }: SettingsSidebarContentProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { currentOrg } = useAuthStore();
  const t = useTranslations();

  // Settings tabs configuration with translation keys
  const settingsTabs = [
    { id: "general", labelKey: "ide.sidebar.settings.tabs.general", icon: Settings, descKey: "ide.sidebar.settings.tabs.generalDesc" },
    { id: "members", labelKey: "ide.sidebar.settings.tabs.members", icon: Users, descKey: "ide.sidebar.settings.tabs.membersDesc" },
    { id: "agents", labelKey: "ide.sidebar.settings.tabs.agents", icon: Bot, descKey: "ide.sidebar.settings.tabs.agentsDesc" },
    { id: "runners", labelKey: "ide.sidebar.settings.tabs.runners", icon: Server, descKey: "ide.sidebar.settings.tabs.runnersDesc" },
    { id: "git-providers", labelKey: "ide.sidebar.settings.tabs.gitProviders", icon: GitBranch, descKey: "ide.sidebar.settings.tabs.gitProvidersDesc" },
    { id: "notifications", labelKey: "ide.sidebar.settings.tabs.notifications", icon: Bell, descKey: "ide.sidebar.settings.tabs.notificationsDesc" },
    { id: "billing", labelKey: "ide.sidebar.settings.tabs.billing", icon: CreditCard, descKey: "ide.sidebar.settings.tabs.billingDesc" },
  ];

  // Get current tab from URL params
  const currentTab = searchParams.get("tab") || "general";

  // Handle tab click
  const handleTabClick = (tabId: string) => {
    router.push(`/${currentOrg?.slug}/settings?tab=${tabId}`);
  };

  return (
    <div className={cn("flex flex-col h-full", className)}>
      {/* Header */}
      <div className="px-3 py-3 border-b border-border">
        <h3 className="text-sm font-semibold">{t("ide.sidebar.settings.title")}</h3>
        <p className="text-xs text-muted-foreground mt-0.5">
          {t("ide.sidebar.settings.description")}
        </p>
      </div>

      {/* Settings navigation */}
      <div className="flex-1 overflow-y-auto py-2">
        {settingsTabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = currentTab === tab.id;

          return (
            <button
              key={tab.id}
              className={cn(
                "w-full flex items-start gap-3 px-3 py-2 text-left transition-colors",
                isActive
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
              )}
              onClick={() => handleTabClick(tab.id)}
            >
              <Icon className={cn(
                "w-4 h-4 mt-0.5 flex-shrink-0",
                isActive && "text-primary"
              )} />
              <div className="flex-1 min-w-0">
                <p className={cn(
                  "text-sm truncate",
                  isActive && "font-medium"
                )}>
                  {t(tab.labelKey)}
                </p>
                <p className="text-xs text-muted-foreground truncate">
                  {t(tab.descKey)}
                </p>
              </div>
            </button>
          );
        })}
      </div>

      {/* Organization info */}
      {currentOrg && (
        <div className="border-t border-border px-3 py-3">
          <div className="text-xs text-muted-foreground mb-1">{t("ide.sidebar.settings.currentOrg")}</div>
          <div className="text-sm font-medium truncate">{currentOrg.name}</div>
          <div className="text-xs text-muted-foreground truncate">/{currentOrg.slug}</div>
        </div>
      )}
    </div>
  );
}

export default SettingsSidebarContent;
