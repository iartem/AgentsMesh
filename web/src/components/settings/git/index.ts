/**
 * Git Settings Components
 * Dialogs and components for Git provider and credential management.
 */

// Dialogs
export { AddProviderDialog } from "./AddProviderDialog";
export { EditProviderDialog } from "./EditProviderDialog";
export { AddCredentialDialog } from "./AddCredentialDialog";

// Cards
export { GitProviderCard } from "./GitProviderCard";
export type { GitProviderCardProps } from "./GitProviderCard";
export { GitCredentialCard } from "./GitCredentialCard";
export type { GitCredentialCardProps } from "./GitCredentialCard";

// Sections
export { DefaultCredentialSection } from "./DefaultCredentialSection";
export type { SelectableCredential, DefaultCredentialSectionProps } from "./DefaultCredentialSection";

// Hooks
export { useGitSettings, getAllSelectableCredentials } from "./useGitSettings";
export type { GitSettingsData, UseGitSettingsResult } from "./useGitSettings";
