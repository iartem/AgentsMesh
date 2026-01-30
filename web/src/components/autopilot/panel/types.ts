import * as React from "react";

// Normalized decision types (lowercase only)
export type NormalizedDecisionType = "continue" | "completed" | "need_help" | "give_up";

// Map backend decision types to frontend keys
// Backend uses: CONTINUE, TASK_COMPLETED, NEED_HUMAN_HELP, GIVE_UP
// Frontend expects: continue, completed, need_help, give_up
export function normalizeDecisionType(backendType: string): NormalizedDecisionType {
  const mapping: Record<string, NormalizedDecisionType> = {
    CONTINUE: "continue",
    TASK_COMPLETED: "completed",
    NEED_HUMAN_HELP: "need_help",
    GIVE_UP: "give_up",
    continue: "continue",
    completed: "completed",
    need_help: "need_help",
    give_up: "give_up",
  };
  return mapping[backendType] || "continue";
}

// Decision type display configuration
export interface DecisionTypeConfig {
  label: string;
  bgColor: string;
  textColor: string;
  icon: React.ReactNode;
}

// Action type display configuration
export interface ActionTypeConfig {
  label: string;
  icon: React.ReactNode;
}

// Iteration phase display configuration
export interface IterationPhaseConfig {
  label: string;
  color: string;
  icon: React.ReactNode;
}
