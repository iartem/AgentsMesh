// Re-export from shared pod hooks for backward compatibility
export { usePodCreationData, useFocusTrap } from "@/components/pod/hooks";
export type { PodCreationData, FormValidationErrors } from "@/components/pod/hooks";

// New config options (replaces usePluginOptions)
export { useConfigOptions } from "./useConfigOptions";
export type { ConfigOptionsState } from "./useConfigOptions";

export { useCreatePodForm, RUNNER_HOST_PROFILE_ID } from "./useCreatePodForm";
export type { CreatePodFormState } from "./useCreatePodForm";
