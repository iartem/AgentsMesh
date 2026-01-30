import { create } from "zustand";
import {
  autopilotApi,
  AutopilotControllerData,
  AutopilotIterationData,
  CreateAutopilotControllerRequest,
  ApproveRequest,
} from "@/lib/api/autopilot";
import { getErrorMessage } from "@/lib/utils";
import { AutopilotThinkingData } from "@/lib/realtime/types";

// Re-export types for component use
export type AutopilotController = AutopilotControllerData;
export type AutopilotIteration = AutopilotIterationData;
export type AutopilotThinking = AutopilotThinkingData;

interface AutopilotState {
  // State
  autopilotControllers: AutopilotController[];
  currentAutopilotController: AutopilotController | null;
  iterations: Record<string, AutopilotIteration[]>; // keyed by autopilot_controller_key
  thinking: Record<string, AutopilotThinking | null>; // Latest thinking event per autopilot
  thinkingHistory: Record<string, AutopilotThinking[]>; // All thinking events per autopilot
  loading: boolean;
  error: string | null;

  // Actions
  fetchAutopilotControllers: () => Promise<void>;
  fetchAutopilotController: (key: string) => Promise<void>;
  createAutopilotController: (data: CreateAutopilotControllerRequest) => Promise<AutopilotController>;
  pauseAutopilotController: (key: string) => Promise<void>;
  resumeAutopilotController: (key: string) => Promise<void>;
  stopAutopilotController: (key: string) => Promise<void>;
  approveAutopilotController: (key: string, data?: ApproveRequest) => Promise<void>;
  takeoverAutopilotController: (key: string) => Promise<void>;
  handbackAutopilotController: (key: string) => Promise<void>;
  fetchIterations: (key: string) => Promise<void>;

  // Real-time updates (called from RealtimeProvider)
  updateAutopilotControllerStatus: (
    key: string,
    phase: string,
    currentIteration: number,
    maxIterations: number,
    circuitBreakerState: string,
    circuitBreakerReason?: string
  ) => void;
  addIteration: (key: string, iteration: AutopilotIteration) => void;
  updateThinking: (key: string, thinking: AutopilotThinking) => void;
  setCurrentAutopilotController: (controller: AutopilotController | null) => void;
  removeAutopilotController: (key: string) => void;

  // Error handling
  clearError: () => void;

  // Selectors
  getAutopilotControllerByPodKey: (podKey: string) => AutopilotController | undefined;
  getThinking: (key: string) => AutopilotThinking | null;
  getThinkingHistory: (key: string) => AutopilotThinking[];
}

export const useAutopilotStore = create<AutopilotState>((set, get) => ({
  autopilotControllers: [],
  currentAutopilotController: null,
  iterations: {},
  thinking: {},
  thinkingHistory: {},
  loading: false,
  error: null,

  fetchAutopilotControllers: async () => {
    set({ loading: true, error: null });
    try {
      const controllers = await autopilotApi.list();
      set({ autopilotControllers: controllers || [], loading: false });
    } catch (error: unknown) {
      set({
        error: getErrorMessage(error, "Failed to fetch AutopilotControllers"),
        loading: false,
      });
    }
  },

  fetchAutopilotController: async (key) => {
    set({ loading: true, error: null });
    try {
      const controller = await autopilotApi.get(key);
      set((state) => {
        const existingIndex = state.autopilotControllers.findIndex(
          (c) => c.autopilot_controller_key === key
        );
        const updatedControllers =
          existingIndex >= 0
            ? state.autopilotControllers.map((c) =>
                c.autopilot_controller_key === key ? controller : c
              )
            : [...state.autopilotControllers, controller];
        return {
          autopilotControllers: updatedControllers,
          currentAutopilotController: controller,
          loading: false,
        };
      });
    } catch (error: unknown) {
      set({
        error: getErrorMessage(error, "Failed to fetch AutopilotController"),
        loading: false,
      });
    }
  },

  createAutopilotController: async (data) => {
    set({ loading: true, error: null });
    try {
      const controller = await autopilotApi.create(data);
      set((state) => ({
        autopilotControllers: [...state.autopilotControllers, controller],
        currentAutopilotController: controller,
        loading: false,
      }));
      return controller;
    } catch (error: unknown) {
      set({
        error: getErrorMessage(error, "Failed to create AutopilotController"),
        loading: false,
      });
      throw error;
    }
  },

  pauseAutopilotController: async (key) => {
    try {
      await autopilotApi.pause(key);
      // Optimistic update
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key ? { ...c, phase: "paused" as const } : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? { ...state.currentAutopilotController, phase: "paused" as const }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to pause AutopilotController") });
    }
  },

  resumeAutopilotController: async (key) => {
    try {
      await autopilotApi.resume(key);
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key ? { ...c, phase: "running" as const } : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? { ...state.currentAutopilotController, phase: "running" as const }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to resume AutopilotController") });
    }
  },

  stopAutopilotController: async (key) => {
    try {
      await autopilotApi.stop(key);
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key ? { ...c, phase: "stopped" as const } : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? { ...state.currentAutopilotController, phase: "stopped" as const }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to stop AutopilotController") });
    }
  },

  approveAutopilotController: async (key, data) => {
    try {
      await autopilotApi.approve(key, data);
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key
            ? {
                ...c,
                phase: data?.continue_execution === false ? ("stopped" as const) : ("running" as const),
                max_iterations: data?.additional_iterations
                  ? c.max_iterations + data.additional_iterations
                  : c.max_iterations,
              }
            : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? {
                ...state.currentAutopilotController,
                phase: data?.continue_execution === false ? ("stopped" as const) : ("running" as const),
                max_iterations: data?.additional_iterations
                  ? state.currentAutopilotController.max_iterations + data.additional_iterations
                  : state.currentAutopilotController.max_iterations,
              }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to approve AutopilotController") });
    }
  },

  takeoverAutopilotController: async (key) => {
    try {
      await autopilotApi.takeover(key);
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key
            ? { ...c, phase: "user_takeover" as const, user_takeover: true }
            : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? { ...state.currentAutopilotController, phase: "user_takeover" as const, user_takeover: true }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to takeover AutopilotController") });
    }
  },

  handbackAutopilotController: async (key) => {
    try {
      await autopilotApi.handback(key);
      set((state) => ({
        autopilotControllers: state.autopilotControllers.map((c) =>
          c.autopilot_controller_key === key
            ? { ...c, phase: "running" as const, user_takeover: false }
            : c
        ),
        currentAutopilotController:
          state.currentAutopilotController?.autopilot_controller_key === key
            ? { ...state.currentAutopilotController, phase: "running" as const, user_takeover: false }
            : state.currentAutopilotController,
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to handback AutopilotController") });
    }
  },

  fetchIterations: async (key) => {
    try {
      const iterations = await autopilotApi.getIterations(key);
      set((state) => ({
        iterations: {
          ...state.iterations,
          [key]: iterations || [],
        },
      }));
    } catch (error: unknown) {
      set({ error: getErrorMessage(error, "Failed to fetch iterations") });
    }
  },

  // Real-time update handlers
  updateAutopilotControllerStatus: (
    key,
    phase,
    currentIteration,
    maxIterations,
    circuitBreakerState,
    circuitBreakerReason
  ) => {
    set((state) => ({
      autopilotControllers: state.autopilotControllers.map((c) =>
        c.autopilot_controller_key === key
          ? {
              ...c,
              phase: phase as AutopilotController["phase"],
              current_iteration: currentIteration,
              max_iterations: maxIterations,
              circuit_breaker: {
                state: circuitBreakerState as AutopilotController["circuit_breaker"]["state"],
                reason: circuitBreakerReason,
              },
            }
          : c
      ),
      currentAutopilotController:
        state.currentAutopilotController?.autopilot_controller_key === key
          ? {
              ...state.currentAutopilotController,
              phase: phase as AutopilotController["phase"],
              current_iteration: currentIteration,
              max_iterations: maxIterations,
              circuit_breaker: {
                state: circuitBreakerState as AutopilotController["circuit_breaker"]["state"],
                reason: circuitBreakerReason,
              },
            }
          : state.currentAutopilotController,
    }));
  },

  addIteration: (key, iteration) => {
    set((state) => ({
      iterations: {
        ...state.iterations,
        [key]: [...(state.iterations[key] || []), iteration],
      },
    }));
  },

  updateThinking: (key, thinking) => {
    set((state) => ({
      thinking: {
        ...state.thinking,
        [key]: thinking,
      },
      thinkingHistory: {
        ...state.thinkingHistory,
        [key]: [...(state.thinkingHistory[key] || []), thinking],
      },
    }));
  },

  setCurrentAutopilotController: (controller) => {
    set({ currentAutopilotController: controller });
  },

  removeAutopilotController: (key) => {
    set((state) => ({
      autopilotControllers: state.autopilotControllers.filter((c) => c.autopilot_controller_key !== key),
      currentAutopilotController:
        state.currentAutopilotController?.autopilot_controller_key === key
          ? null
          : state.currentAutopilotController,
    }));
  },

  clearError: () => {
    set({ error: null });
  },

  // Selector: get AutopilotController by pod key
  getAutopilotControllerByPodKey: (podKey: string) => {
    const state = get();
    return state.autopilotControllers.find(
      (c) => c.pod_key === podKey &&
        ["initializing", "running", "paused", "user_takeover", "waiting_approval"].includes(c.phase)
    );
  },

  // Selector: get latest thinking for an autopilot
  getThinking: (key: string) => {
    const state = get();
    return state.thinking[key] || null;
  },

  // Selector: get thinking history for an autopilot
  getThinkingHistory: (key: string) => {
    const state = get();
    return state.thinkingHistory[key] || [];
  },
}));
