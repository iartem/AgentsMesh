"use client";

import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import {
  X,
  Loader2,
  AlertCircle,
  Square,
} from "lucide-react";

interface InitProgress {
  progress: number;
  phase: string;
  message: string;
}

interface TerminalLoadingStateProps {
  podStatus: string;
  initProgress?: InitProgress;
  isTerminating: boolean;
  onTerminate: () => void;
}

/**
 * Loading/Waiting state view for TerminalPane
 */
export function TerminalLoadingState({
  podStatus,
  initProgress,
  isTerminating,
  onTerminate,
}: TerminalLoadingStateProps) {
  return (
    <div className="flex-1 flex items-center justify-center bg-terminal-bg">
      <div className="text-center p-4 max-w-sm">
        <Loader2 className="w-12 h-12 text-primary animate-spin mx-auto mb-3" />
        <p className="text-terminal-text font-medium mb-1">
          {initProgress?.message || "Waiting for Pod to be ready..."}
        </p>
        {initProgress ? (
          <div className="mt-3 space-y-2">
            <Progress value={initProgress.progress} className="h-2" />
            <p className="text-xs text-terminal-text-muted">
              {initProgress.phase} - {initProgress.progress}%
            </p>
          </div>
        ) : (
          <p className="text-sm text-terminal-text-muted">
            Status: <span className="text-yellow-500 dark:text-yellow-400">{podStatus}</span>
          </p>
        )}
        {/* Show terminate button when status is unknown */}
        {podStatus === "unknown" && (
          <Button
            variant="outline"
            size="sm"
            className="mt-4 text-red-500 dark:text-red-400 border-red-500/50 hover:bg-red-500/10"
            onClick={onTerminate}
            disabled={isTerminating}
          >
            {isTerminating ? (
              <Loader2 className="w-4 h-4 mr-2 animate-spin" />
            ) : (
              <Square className="w-4 h-4 mr-2" />
            )}
            Terminate Pod
          </Button>
        )}
      </div>
    </div>
  );
}

interface TerminalErrorStateProps {
  error: string;
  onClose?: () => void;
}

/**
 * Error state view for TerminalPane
 */
export function TerminalErrorState({
  error,
  onClose,
}: TerminalErrorStateProps) {
  return (
    <div className="flex-1 flex items-center justify-center bg-terminal-bg">
      <div className="text-center p-4">
        <AlertCircle className="w-12 h-12 text-red-500 dark:text-red-400 mx-auto mb-3" />
        <p className="text-terminal-text font-medium mb-1">{error}</p>
        <p className="text-sm text-terminal-text-muted mb-4">
          The pod cannot be connected. Please check the pod status or create a new one.
        </p>
        {onClose && (
          <Button
            variant="outline"
            size="sm"
            className="text-red-500 dark:text-red-400 border-red-500/50 hover:bg-red-500/10"
            onClick={onClose}
          >
            <X className="w-4 h-4 mr-2" />
            Close Terminal
          </Button>
        )}
      </div>
    </div>
  );
}

// Named exports only - no default export needed
