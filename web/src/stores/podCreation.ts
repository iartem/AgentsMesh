import { create } from "zustand";
import { persist } from "zustand/middleware";

/**
 * Pod creation preferences - remembers last user choices
 */
interface PodCreationPreferences {
  lastAgentTypeId: number | null;
  lastRepositoryId: number | null;
  lastCredentialProfileId: number | null;
  lastBranchName: string | null;

  setLastChoices: (
    choices: Partial<
      Pick<
        PodCreationPreferences,
        "lastAgentTypeId" | "lastRepositoryId" | "lastCredentialProfileId" | "lastBranchName"
      >
    >
  ) => void;
  clearLastChoices: () => void;

  // Hydration state for SSR
  _hasHydrated: boolean;
  setHasHydrated: (state: boolean) => void;
}

export const usePodCreationStore = create<PodCreationPreferences>()(
  persist(
    (set) => ({
      lastAgentTypeId: null,
      lastRepositoryId: null,
      lastCredentialProfileId: null,
      lastBranchName: null,

      setLastChoices: (choices) => set((state) => ({ ...state, ...choices })),
      clearLastChoices: () =>
        set({
          lastAgentTypeId: null,
          lastRepositoryId: null,
          lastCredentialProfileId: null,
          lastBranchName: null,
        }),

      // Hydration
      _hasHydrated: false,
      setHasHydrated: (state) => set({ _hasHydrated: state }),
    }),
    {
      name: "agentsmesh-pod-creation",
      partialize: (state) => ({
        lastAgentTypeId: state.lastAgentTypeId,
        lastRepositoryId: state.lastRepositoryId,
        lastCredentialProfileId: state.lastCredentialProfileId,
        lastBranchName: state.lastBranchName,
      }),
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true);
      },
    }
  )
);
