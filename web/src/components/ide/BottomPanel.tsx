"use client";

import React, { useRef, useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import { useIDEStore, type BottomPanelTab } from "@/stores/ide";
import { useTranslations } from "@/lib/i18n/client";
import { Button } from "@/components/ui/button";
import {
  ChevronDown,
  ChevronUp,
  X,
  Terminal,
  AlertCircle,
  MessageSquare,
  Activity,
} from "lucide-react";

interface BottomPanelProps {
  className?: string;
}

const TAB_ICONS: Record<BottomPanelTab, React.ReactNode> = {
  output: <Terminal className="w-3.5 h-3.5" />,
  problems: <AlertCircle className="w-3.5 h-3.5" />,
  channels: <MessageSquare className="w-3.5 h-3.5" />,
  activity: <Activity className="w-3.5 h-3.5" />,
};

const TAB_IDS: BottomPanelTab[] = ["output", "problems", "channels", "activity"];

export function BottomPanel({ className }: BottomPanelProps) {
  const t = useTranslations();
  const {
    bottomPanelOpen,
    bottomPanelHeight,
    bottomPanelTab,
    setBottomPanelOpen,
    setBottomPanelHeight,
    setBottomPanelTab,
    toggleBottomPanel,
  } = useIDEStore();

  const resizeRef = useRef<HTMLDivElement>(null);
  const [isResizing, setIsResizing] = useState(false);

  // Handle panel resize
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;
      const windowHeight = window.innerHeight;
      const newHeight = Math.min(
        Math.max(windowHeight - e.clientY, 100),
        windowHeight * 0.6
      );
      setBottomPanelHeight(newHeight);
    };

    const handleMouseUp = () => {
      setIsResizing(false);
    };

    if (isResizing) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
    }

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing, setBottomPanelHeight]);

  if (!bottomPanelOpen) {
    return (
      <div
        className={cn(
          "h-8 bg-background border-t border-border flex items-center px-2 gap-2",
          className
        )}
      >
        {TAB_IDS.map((tabId) => (
          <button
            key={tabId}
            className={cn(
              "flex items-center gap-1.5 px-2 py-1 text-xs rounded hover:bg-muted",
              bottomPanelTab === tabId
                ? "text-foreground"
                : "text-muted-foreground"
            )}
            onClick={() => {
              setBottomPanelTab(tabId);
              setBottomPanelOpen(true);
            }}
          >
            {TAB_ICONS[tabId]}
            <span>{t(`ide.bottomPanel.${tabId}`)}</span>
          </button>
        ))}
        <div className="flex-1" />
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 p-0"
          onClick={toggleBottomPanel}
        >
          <ChevronUp className="w-4 h-4" />
        </Button>
      </div>
    );
  }

  return (
    <div
      className={cn("bg-background border-t border-border flex flex-col", className)}
      style={{ height: bottomPanelHeight }}
    >
      {/* Resize handle */}
      <div
        ref={resizeRef}
        className={cn(
          "h-1 cursor-row-resize hover:bg-primary/50 transition-colors",
          isResizing && "bg-primary/50"
        )}
        onMouseDown={() => setIsResizing(true)}
      />

      {/* Tab bar */}
      <div className="h-8 flex items-center px-2 gap-2 border-b border-border">
        {TAB_IDS.map((tabId) => (
          <button
            key={tabId}
            className={cn(
              "flex items-center gap-1.5 px-2 py-1 text-xs rounded transition-colors",
              bottomPanelTab === tabId
                ? "text-foreground bg-muted"
                : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
            )}
            onClick={() => setBottomPanelTab(tabId)}
          >
            {TAB_ICONS[tabId]}
            <span>{t(`ide.bottomPanel.${tabId}`)}</span>
          </button>
        ))}
        <div className="flex-1" />
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 p-0"
          onClick={toggleBottomPanel}
        >
          <ChevronDown className="w-4 h-4" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 p-0"
          onClick={() => setBottomPanelOpen(false)}
        >
          <X className="w-4 h-4" />
        </Button>
      </div>

      {/* Content area */}
      <div className="flex-1 overflow-auto p-2">
        {bottomPanelTab === "output" && (
          <div className="text-xs font-mono text-muted-foreground">
            <p>{t("ide.bottomPanel.noOutput")}</p>
          </div>
        )}
        {bottomPanelTab === "problems" && (
          <div className="text-xs text-muted-foreground">
            <p>{t("ide.bottomPanel.noProblems")}</p>
          </div>
        )}
        {bottomPanelTab === "channels" && (
          <div className="text-xs text-muted-foreground">
            <p>{t("ide.bottomPanel.noChannels")}</p>
          </div>
        )}
        {bottomPanelTab === "activity" && (
          <div className="text-xs text-muted-foreground">
            <p>{t("ide.bottomPanel.noActivity")}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default BottomPanel;
