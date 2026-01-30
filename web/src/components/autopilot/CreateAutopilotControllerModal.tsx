"use client";

import * as React from "react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { useAutopilotStore } from "@/stores/autopilot";
import { Bot, Play, X, Loader2, Settings2 } from "lucide-react";

interface CreateAutopilotControllerModalProps {
  open: boolean;
  onClose: () => void;
  podKey: string;
  podTitle?: string;
}

export function CreateAutopilotControllerModal({
  open,
  onClose,
  podKey,
  podTitle,
}: CreateAutopilotControllerModalProps) {
  const { createAutopilotController, loading, error, clearError } = useAutopilotStore();

  // Form state
  const [initialPrompt, setInitialPrompt] = useState("");
  const [maxIterations, setMaxIterations] = useState(10);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [iterationTimeout, setIterationTimeout] = useState(300);
  const [noProgressThreshold, setNoProgressThreshold] = useState(3);
  const [sameErrorThreshold, setSameErrorThreshold] = useState(5);
  const [approvalTimeoutMin, setApprovalTimeoutMin] = useState(30);

  // Reset form when modal opens/closes
  React.useEffect(() => {
    if (!open) {
      setInitialPrompt("");
      setMaxIterations(10);
      setShowAdvanced(false);
      setIterationTimeout(300);
      setNoProgressThreshold(3);
      setSameErrorThreshold(5);
      setApprovalTimeoutMin(30);
      clearError();
    }
  }, [open, clearError]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      await createAutopilotController({
        pod_key: podKey,
        initial_prompt: initialPrompt || undefined,
        max_iterations: maxIterations,
        iteration_timeout_sec: iterationTimeout,
        no_progress_threshold: noProgressThreshold,
        same_error_threshold: sameErrorThreshold,
        approval_timeout_min: approvalTimeoutMin,
      });
      onClose();
    } catch {
      // Error is handled by store
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-autopilot-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="bg-background border border-border rounded-lg w-full max-w-md p-4 md:p-6 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Bot className="h-5 w-5 text-primary" />
            <h2 id="create-autopilot-title" className="text-lg font-semibold">
              Start Autopilot Mode
            </h2>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Description */}
        <p className="text-sm text-muted-foreground mb-4">
          Autopilot will automatically drive the Pod to complete tasks.
          {podTitle && (
            <span className="block mt-1">
              Target: <code className="text-xs bg-muted px-1 py-0.5 rounded">{podTitle}</code>
            </span>
          )}
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Initial Prompt */}
          <div>
            <label htmlFor="initial-prompt" className="block text-sm font-medium mb-2">
              Task Description
            </label>
            <textarea
              id="initial-prompt"
              className="w-full px-3 py-2 border border-border rounded-md bg-background resize-none"
              rows={3}
              placeholder="Describe the task for Autopilot to complete..."
              value={initialPrompt}
              onChange={(e) => setInitialPrompt(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Optional. If not provided, Autopilot will continue from current state.
            </p>
          </div>

          {/* Max Iterations */}
          <div>
            <label htmlFor="max-iterations" className="block text-sm font-medium mb-2">
              Max Iterations
            </label>
            <input
              id="max-iterations"
              type="number"
              min={1}
              max={100}
              className="w-full px-3 py-2 border border-border rounded-md bg-background"
              value={maxIterations}
              onChange={(e) => setMaxIterations(Number(e.target.value))}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Maximum number of decision cycles before stopping.
            </p>
          </div>

          {/* Advanced Settings Toggle */}
          <button
            type="button"
            className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            <Settings2 className="h-4 w-4" />
            {showAdvanced ? "Hide" : "Show"} Advanced Settings
          </button>

          {/* Advanced Settings */}
          {showAdvanced && (
            <div className="space-y-4 p-3 bg-muted/50 rounded-md">
              <div>
                <label htmlFor="iteration-timeout" className="block text-sm font-medium mb-1">
                  Iteration Timeout (seconds)
                </label>
                <input
                  id="iteration-timeout"
                  type="number"
                  min={60}
                  max={1800}
                  className="w-full px-3 py-2 border border-border rounded-md bg-background"
                  value={iterationTimeout}
                  onChange={(e) => setIterationTimeout(Number(e.target.value))}
                />
              </div>

              <div>
                <label htmlFor="no-progress-threshold" className="block text-sm font-medium mb-1">
                  No Progress Threshold
                </label>
                <input
                  id="no-progress-threshold"
                  type="number"
                  min={1}
                  max={10}
                  className="w-full px-3 py-2 border border-border rounded-md bg-background"
                  value={noProgressThreshold}
                  onChange={(e) => setNoProgressThreshold(Number(e.target.value))}
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Circuit breaker triggers after this many iterations without file changes.
                </p>
              </div>

              <div>
                <label htmlFor="same-error-threshold" className="block text-sm font-medium mb-1">
                  Same Error Threshold
                </label>
                <input
                  id="same-error-threshold"
                  type="number"
                  min={1}
                  max={10}
                  className="w-full px-3 py-2 border border-border rounded-md bg-background"
                  value={sameErrorThreshold}
                  onChange={(e) => setSameErrorThreshold(Number(e.target.value))}
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Circuit breaker triggers after this many identical errors.
                </p>
              </div>

              <div>
                <label htmlFor="approval-timeout" className="block text-sm font-medium mb-1">
                  Approval Timeout (minutes)
                </label>
                <input
                  id="approval-timeout"
                  type="number"
                  min={5}
                  max={120}
                  className="w-full px-3 py-2 border border-border rounded-md bg-background"
                  value={approvalTimeoutMin}
                  onChange={(e) => setApprovalTimeoutMin(Number(e.target.value))}
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Auto-stop if circuit breaker is not approved within this time.
                </p>
              </div>
            </div>
          )}

          {/* Error Display */}
          {error && (
            <div
              role="alert"
              aria-live="assertive"
              className="bg-destructive/10 border border-destructive/30 rounded-md p-3"
            >
              <p className="text-sm text-destructive">{error}</p>
            </div>
          )}

          {/* Action Buttons */}
          <div className="flex justify-end gap-3 pt-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Starting...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4 mr-2" />
                  Start Autopilot
                </>
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default CreateAutopilotControllerModal;
