"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAutopilotStore, AutopilotController, AutopilotThinking, AutopilotIteration } from "@/stores/autopilot";
import {
  Brain,
  CheckCircle,
  AlertTriangle,
  XCircle,
  ArrowRight,
  Clock,
  Eye,
  Send,
  MessageSquare,
  ListChecks,
  History,
  X,
  Play,
  Loader2,
  FileText,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

interface AutopilotFloatingPanelProps {
  autopilotController: AutopilotController;
  className?: string;
  onClose?: () => void;
}

// Normalized decision types (lowercase only)
type NormalizedDecisionType = "continue" | "completed" | "need_help" | "give_up";

// Decision type configuration
const decisionConfig: Record<
  NormalizedDecisionType,
  { label: string; bgColor: string; textColor: string; icon: React.ReactNode }
> = {
  continue: {
    label: "Continue",
    bgColor: "bg-blue-500",
    textColor: "text-blue-500",
    icon: <ArrowRight className="h-3 w-3" />,
  },
  completed: {
    label: "Completed",
    bgColor: "bg-green-500",
    textColor: "text-green-500",
    icon: <CheckCircle className="h-3 w-3" />,
  },
  need_help: {
    label: "Need Help",
    bgColor: "bg-orange-500",
    textColor: "text-orange-500",
    icon: <AlertTriangle className="h-3 w-3" />,
  },
  give_up: {
    label: "Give Up",
    bgColor: "bg-red-500",
    textColor: "text-red-500",
    icon: <XCircle className="h-3 w-3" />,
  },
};

// Action type configuration
const actionConfig: Record<string, { label: string; icon: React.ReactNode }> = {
  observe: { label: "Observing", icon: <Eye className="h-3 w-3" /> },
  send_input: { label: "Sending Input", icon: <Send className="h-3 w-3" /> },
  wait: { label: "Waiting", icon: <Clock className="h-3 w-3" /> },
  none: { label: "No Action", icon: <MessageSquare className="h-3 w-3" /> },
};

// Iteration phase display configuration
const iterationPhaseConfig: Record<
  string,
  { label: string; color: string; icon: React.ReactNode }
> = {
  initial_prompt: {
    label: "Initial",
    color: "bg-blue-500",
    icon: <Send className="h-3 w-3" />,
  },
  started: {
    label: "Started",
    color: "bg-blue-400",
    icon: <Play className="h-3 w-3" />,
  },
  control_running: {
    label: "Running",
    color: "bg-yellow-500",
    icon: <Loader2 className="h-3 w-3 animate-spin" />,
  },
  action_sent: {
    label: "Sent",
    color: "bg-green-500",
    icon: <Send className="h-3 w-3" />,
  },
  completed: {
    label: "Done",
    color: "bg-green-600",
    icon: <CheckCircle className="h-3 w-3" />,
  },
  error: {
    label: "Error",
    color: "bg-red-500",
    icon: <XCircle className="h-3 w-3" />,
  },
};

// Map backend decision types to frontend keys
// Backend uses: CONTINUE, TASK_COMPLETED, NEED_HUMAN_HELP, GIVE_UP
// Frontend expects: continue, completed, need_help, give_up
function normalizeDecisionType(backendType: string): NormalizedDecisionType {
  const mapping: Record<string, NormalizedDecisionType> = {
    "CONTINUE": "continue",
    "TASK_COMPLETED": "completed",
    "NEED_HUMAN_HELP": "need_help",
    "GIVE_UP": "give_up",
    // Also support lowercase (in case backend is updated)
    "continue": "continue",
    "completed": "completed",
    "need_help": "need_help",
    "give_up": "give_up",
  };
  return mapping[backendType] || "continue";
}

// Thinking Tab Content
function ThinkingTab({ thinking }: { thinking: AutopilotThinking | null }) {
  if (!thinking) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <Brain className="h-8 w-8 mb-2 opacity-50" />
        <span className="text-sm">Waiting for Control Agent...</span>
      </div>
    );
  }

  const normalizedDecisionType = normalizeDecisionType(thinking.decision_type);
  const decisionInfo = decisionConfig[normalizedDecisionType];
  const actionInfo = thinking.action ? actionConfig[thinking.action.type] : null;

  return (
    <div className="space-y-3 p-3">
      {/* Decision Type Badge */}
      <div className="flex items-center gap-2">
        <Badge
          variant="outline"
          className={cn("flex items-center gap-1", decisionInfo.bgColor, "text-white")}
        >
          {decisionInfo.icon}
          <span>{decisionInfo.label}</span>
        </Badge>
        <span className="text-xs text-muted-foreground">Iteration #{thinking.iteration}</span>
      </div>

      {/* Reasoning */}
      <div>
        <div className="text-xs text-muted-foreground mb-1">Reasoning</div>
        <p className="text-sm leading-relaxed">{thinking.reasoning}</p>
      </div>

      {/* Confidence */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-muted-foreground">Confidence:</span>
        <div className="flex-1 max-w-[120px]">
          <Progress value={thinking.confidence * 100} className="h-1.5" />
        </div>
        <span className="text-xs font-medium">{Math.round(thinking.confidence * 100)}%</span>
      </div>

      {/* Action */}
      {actionInfo && thinking.action && (
        <div className="rounded-md bg-muted/50 p-2">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            {actionInfo.icon}
            <span className="text-xs font-medium">{actionInfo.label}</span>
          </div>
          {thinking.action.content && (
            <p className="text-xs font-mono bg-background/50 p-1.5 rounded break-all">
              {thinking.action.content}
            </p>
          )}
          {thinking.action.reason && (
            <p className="text-xs text-muted-foreground mt-1">{thinking.action.reason}</p>
          )}
        </div>
      )}

      {/* Help Request */}
      {thinking.help_request && (
        <div className="rounded-md bg-orange-500/10 border border-orange-500/30 p-2">
          <div className="flex items-center gap-2 text-orange-500 mb-1">
            <AlertTriangle className="h-3 w-3" />
            <span className="font-medium text-xs">Help Needed</span>
          </div>
          <p className="text-xs mb-1">{thinking.help_request.reason}</p>
          {thinking.help_request.context && (
            <p className="text-xs text-muted-foreground">
              Context: {thinking.help_request.context}
            </p>
          )}
          {thinking.help_request.terminal_excerpt && (
            <pre className="text-xs font-mono bg-zinc-900 text-zinc-100 p-2 rounded mt-2 overflow-x-auto whitespace-pre-wrap break-all max-h-24">
              {thinking.help_request.terminal_excerpt}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

// Progress Tab Content
function ProgressTab({ thinking }: { thinking: AutopilotThinking | null }) {
  const progress = thinking?.progress;

  if (!progress) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <ListChecks className="h-8 w-8 mb-2 opacity-50" />
        <span className="text-sm">No progress data available</span>
      </div>
    );
  }

  return (
    <div className="space-y-4 p-3">
      {/* Progress Summary & Percent */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium">{progress.summary || "Task Progress"}</span>
          {progress.percent > 0 && (
            <span className="text-sm font-medium">{progress.percent}%</span>
          )}
        </div>
        {progress.percent > 0 && <Progress value={progress.percent} className="h-2" />}
      </div>

      {/* Completed Steps */}
      {progress.completed_steps && progress.completed_steps.length > 0 && (
        <div>
          <div className="text-xs text-muted-foreground mb-2 flex items-center gap-1.5">
            <CheckCircle className="h-3 w-3 text-green-500" />
            <span>Completed ({progress.completed_steps.length})</span>
          </div>
          <ul className="space-y-1">
            {progress.completed_steps.map((step, i) => (
              <li key={i} className="flex items-start gap-2 text-sm">
                <CheckCircle className="h-4 w-4 text-green-500 flex-shrink-0 mt-0.5" />
                <span>{step}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Remaining Steps */}
      {progress.remaining_steps && progress.remaining_steps.length > 0 && (
        <div>
          <div className="text-xs text-muted-foreground mb-2 flex items-center gap-1.5">
            <Clock className="h-3 w-3 text-muted-foreground" />
            <span>Remaining ({progress.remaining_steps.length})</span>
          </div>
          <ul className="space-y-1">
            {progress.remaining_steps.map((step, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-muted-foreground">
                <div className="h-4 w-4 rounded-full border border-muted-foreground flex-shrink-0 mt-0.5" />
                <span>{step}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

// History Tab Content - Iteration Item
function IterationItem({ iteration }: { iteration: AutopilotIteration }) {
  const [expanded, setExpanded] = React.useState(false);
  const phaseInfo = iterationPhaseConfig[iteration.phase] || {
    label: iteration.phase,
    color: "bg-gray-500",
    icon: <FileText className="h-3 w-3" />,
  };

  const hasDetails =
    iteration.summary ||
    (iteration.files_changed && iteration.files_changed.length > 0);

  return (
    <div className="border-b border-border/50 last:border-b-0 py-2">
      <div
        className={cn(
          "flex items-center gap-2",
          hasDetails && "cursor-pointer hover:bg-muted/50 -mx-2 px-2 rounded"
        )}
        onClick={() => hasDetails && setExpanded(!expanded)}
      >
        {hasDetails ? (
          expanded ? (
            <ChevronDown className="h-3 w-3 text-muted-foreground flex-shrink-0" />
          ) : (
            <ChevronRight className="h-3 w-3 text-muted-foreground flex-shrink-0" />
          )
        ) : (
          <div className="w-3 flex-shrink-0" />
        )}

        <Badge
          variant="outline"
          className={cn(
            "flex items-center gap-1 text-[10px] h-5",
            phaseInfo.color,
            "text-white"
          )}
        >
          {phaseInfo.icon}
          <span>{phaseInfo.label}</span>
        </Badge>

        <span className="text-xs text-muted-foreground">#{iteration.iteration}</span>

        {iteration.duration_ms && (
          <span className="text-[10px] text-muted-foreground ml-auto">
            {(iteration.duration_ms / 1000).toFixed(1)}s
          </span>
        )}
      </div>

      {expanded && hasDetails && (
        <div className="mt-2 ml-5 pl-3 border-l-2 border-muted">
          {iteration.summary && (
            <p className="text-xs text-muted-foreground mb-2">{iteration.summary}</p>
          )}

          {iteration.files_changed && iteration.files_changed.length > 0 && (
            <div className="text-[10px]">
              <span className="text-muted-foreground">Files: </span>
              <span className="font-mono">{iteration.files_changed.join(", ")}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// History Tab Content
function HistoryTab({ autopilotControllerKey }: { autopilotControllerKey: string }) {
  const { iterations, fetchIterations } = useAutopilotStore();
  const controllerIterations = iterations[autopilotControllerKey] || [];

  React.useEffect(() => {
    if (autopilotControllerKey) {
      fetchIterations(autopilotControllerKey);
    }
  }, [autopilotControllerKey, fetchIterations]);

  if (controllerIterations.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <History className="h-8 w-8 mb-2 opacity-50" />
        <span className="text-sm">No iterations yet</span>
      </div>
    );
  }

  // Show iterations in reverse order (most recent first)
  const displayIterations = [...controllerIterations].reverse();

  return (
    <div className="max-h-48 overflow-y-auto px-3">
      {displayIterations.map((iteration) => (
        <IterationItem key={iteration.id || iteration.iteration} iteration={iteration} />
      ))}
    </div>
  );
}

export function AutopilotFloatingPanel({
  autopilotController,
  className,
  onClose,
}: AutopilotFloatingPanelProps) {
  const [activeTab, setActiveTab] = React.useState<"thinking" | "progress" | "history">("thinking");
  const { getThinking } = useAutopilotStore();

  const thinking = getThinking(autopilotController.autopilot_controller_key);

  // Auto switch to thinking tab when help is needed
  React.useEffect(() => {
    if (thinking?.decision_type === "need_help") {
      setActiveTab("thinking");
    }
  }, [thinking?.decision_type]);

  return (
    <div
      className={cn(
        "border-b bg-card shadow-sm",
        className
      )}
    >
      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as typeof activeTab)}
        className="w-full"
      >
        <div className="flex items-center justify-between px-2 border-b">
          <TabsList className="h-8 bg-transparent p-0 gap-0">
            <TabsTrigger
              value="thinking"
              className="h-8 px-3 text-xs data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none"
            >
              <Brain className="h-3 w-3 mr-1.5" />
              Thinking
            </TabsTrigger>
            <TabsTrigger
              value="progress"
              className="h-8 px-3 text-xs data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none"
            >
              <ListChecks className="h-3 w-3 mr-1.5" />
              Progress
            </TabsTrigger>
            <TabsTrigger
              value="history"
              className="h-8 px-3 text-xs data-[state=active]:bg-transparent data-[state=active]:shadow-none data-[state=active]:border-b-2 data-[state=active]:border-primary rounded-none"
            >
              <History className="h-3 w-3 mr-1.5" />
              History
            </TabsTrigger>
          </TabsList>

          {onClose && (
            <Button
              size="sm"
              variant="ghost"
              className="h-6 w-6 p-0"
              onClick={onClose}
            >
              <X className="h-3.5 w-3.5 text-muted-foreground" />
            </Button>
          )}
        </div>

        <TabsContent value="thinking" className="mt-0">
          <ThinkingTab thinking={thinking} />
        </TabsContent>

        <TabsContent value="progress" className="mt-0">
          <ProgressTab thinking={thinking} />
        </TabsContent>

        <TabsContent value="history" className="mt-0 py-2">
          <HistoryTab autopilotControllerKey={autopilotController.autopilot_controller_key} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default AutopilotFloatingPanel;
