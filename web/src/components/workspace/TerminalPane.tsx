"use client";

import React, { useCallback, useState } from "react";
import "@xterm/xterm/css/xterm.css";
import { cn } from "@/lib/utils";
import { useWorkspaceStore } from "@/stores/workspace";
import { usePodStore } from "@/stores/pod";
import { useAutopilotStore } from "@/stores/autopilot";
import { usePodStatus, useTerminal, useTouchScroll } from "@/hooks";
import {
  CircuitBreakerAlert,
  TakeoverBanner,
  CreateAutopilotControllerModal,
  AutopilotStatusBar,
} from "@/components/autopilot";
import { useIDEStore } from "@/stores/ide";
import { TerminalPaneHeader } from "./TerminalPaneHeader";
import { TerminalLoadingState, TerminalErrorState } from "./TerminalStateViews";
import { RelayStatusOverlay } from "./RelayStatusOverlay";

interface TerminalPaneProps {
  paneId: string;
  podKey: string;
  title: string;
  isActive: boolean;
  onClose?: () => void;
  onMaximize?: () => void;
  onPopout?: () => void;
  showHeader?: boolean;
  className?: string;
}

export function TerminalPane({
  paneId,
  podKey,
  title,
  isActive,
  onClose,
  onMaximize,
  onPopout,
  showHeader = true,
  className,
}: TerminalPaneProps) {
  const [isMaximized, setIsMaximized] = useState(false);
  const [isTerminating, setIsTerminating] = useState(false);
  const [showAutopilotModal, setShowAutopilotModal] = useState(false);
  const { terminalFontSize, setActivePane } = useWorkspaceStore();
  const { setBottomPanelOpen, setBottomPanelTab } = useIDEStore();
  const initProgress = usePodStore((state) => state.initProgress[podKey]);
  const terminatePod = usePodStore((state) => state.terminatePod);

  // AutopilotController state - find if there's an active AutopilotController for this Pod
  const autopilotController = useAutopilotStore((state) => state.getAutopilotControllerByPodKey(podKey));
  const getThinking = useAutopilotStore((state) => state.getThinking);

  // Get thinking state for this autopilot
  const thinking = autopilotController
    ? getThinking(autopilotController.autopilot_controller_key)
    : null;

  // Auto-open BottomPanel Autopilot tab when help is needed
  React.useEffect(() => {
    if (
      thinking?.decision_type === "need_help" ||
      thinking?.decision_type === "NEED_HUMAN_HELP" ||
      autopilotController?.phase === "waiting_approval"
    ) {
      setBottomPanelTab("autopilot");
      setBottomPanelOpen(true);
    }
  }, [thinking?.decision_type, autopilotController?.phase, setBottomPanelTab, setBottomPanelOpen]);

  // Pod status tracking
  const { podStatus, isPodReady, podError } = usePodStatus(podKey);

  // Terminal initialization and management
  const {
    terminalRef,
    xtermRef,
    fitAddonRef,
    connectionStatus,
    isRunnerDisconnected,
    syncSize,
  } = useTerminal(podKey, terminalFontSize, isPodReady, isActive);

  // Mobile touch scrolling support
  useTouchScroll(terminalRef, xtermRef, isPodReady);

  const handleFocus = useCallback(() => {
    setActivePane(paneId);
  }, [paneId, setActivePane]);

  const handleMaximize = useCallback(() => {
    setIsMaximized(!isMaximized);
    onMaximize?.();
    // Fit terminal after layout change
    setTimeout(() => {
      fitAddonRef.current?.fit();
    }, 100);
  }, [isMaximized, onMaximize, fitAddonRef]);

  const handleTerminate = useCallback(async () => {
    setIsTerminating(true);
    try {
      await terminatePod(podKey);
      onClose?.();
    } catch (error) {
      console.error("Failed to terminate pod:", error);
    } finally {
      setIsTerminating(false);
    }
  }, [podKey, terminatePod, onClose]);

  return (
    <div
      className={cn(
        "flex flex-col h-full bg-terminal-bg rounded-lg overflow-hidden border",
        isActive ? "border-primary" : "border-border",
        isMaximized && "fixed inset-4 z-50",
        className
      )}
      onClick={handleFocus}
    >
      {/* Header */}
      {showHeader && (
        <TerminalPaneHeader
          title={title}
          podKey={podKey}
          connectionStatus={connectionStatus}
          isMaximized={isMaximized}
          isPodReady={isPodReady}
          hasAutopilot={!!autopilotController}
          onSyncSize={syncSize}
          onStartAutopilot={() => setShowAutopilotModal(true)}
          onPopout={onPopout}
          onMaximize={handleMaximize}
          onClose={onClose}
        />
      )}

      {/* Terminal or Loading/Error State */}
      {!isPodReady ? (
        podError ? (
          <TerminalErrorState error={podError} onClose={onClose} />
        ) : (
          <TerminalLoadingState
            podStatus={podStatus}
            initProgress={initProgress}
            isTerminating={isTerminating}
            onTerminate={handleTerminate}
            onClose={onClose}
          />
        )
      ) : (
        <div className="flex flex-col flex-1 min-h-0">
          {/* AutopilotController Components - show when terminal is ready and AutopilotController exists */}
          {autopilotController && (
            <>
              {/* Takeover Banner - only show when user has taken over */}
              <TakeoverBanner autopilotController={autopilotController} className="rounded-none" />

              {/* Circuit Breaker Alert - show when circuit breaker is open */}
              <CircuitBreakerAlert autopilotController={autopilotController} className="mx-2 mt-2 rounded-md" />

              {/* Simplified AutopilotStatusBar - click to open BottomPanel */}
              <AutopilotStatusBar
                autopilotController={autopilotController}
                onTogglePanel={() => {
                  setBottomPanelTab("autopilot");
                  setBottomPanelOpen(true);
                }}
              />
            </>
          )}
          <div className="relative flex-1 min-h-0">
            {/* Relay connection status overlay - always visible, floating at top */}
            <RelayStatusOverlay
              connectionStatus={connectionStatus}
              isRunnerDisconnected={isRunnerDisconnected}
            />
            <div
              ref={terminalRef}
              className="h-full overflow-auto"
              style={{
                touchAction: "pan-y pinch-zoom", // Enable touch scrolling and zoom
              }}
            />
          </div>
        </div>
      )}

      {/* Create AutopilotController Modal */}
      <CreateAutopilotControllerModal
        open={showAutopilotModal}
        onClose={() => setShowAutopilotModal(false)}
        podKey={podKey}
        podTitle={title}
      />
    </div>
  );
}

export default TerminalPane;
