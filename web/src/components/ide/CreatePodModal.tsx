"use client";

import React, { useEffect, useRef } from "react";
import { PodData } from "@/lib/api/client";
import { useTranslations } from "@/lib/i18n/client";
import { Button } from "@/components/ui/button";
import { PluginConfigForm } from "./PluginConfigForm/index";
import {
  usePodCreationData,
  usePluginOptions,
  useFocusTrap,
  useCreatePodForm,
} from "./hooks";

interface CreatePodModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (pod?: PodData) => void;
}

export function CreatePodModal({ open, onClose, onCreated }: CreatePodModalProps) {
  const t = useTranslations();

  // Load base data (runners, agents, repositories)
  const {
    runners,
    agentTypes,
    repositories,
    loading: loadingData,
  } = usePodCreationData(open);

  // Form state management
  const form = useCreatePodForm(agentTypes, repositories, onCreated);

  // Plugin options management
  console.log("[CreatePodModal] form.selectedRunner:", form.selectedRunner, "form.selectedAgentSlug:", form.selectedAgentSlug);

  const {
    plugins: pluginOptions,
    loading: loadingPlugins,
    config: pluginConfig,
    updateConfig: handlePluginConfigChange,
    resetConfig: resetPluginConfig,
  } = usePluginOptions(form.selectedRunner, form.selectedAgentSlug);

  // Focus trap for modal accessibility
  const modalRef = useFocusTrap<HTMLDivElement>(open, onClose);

  // Track previous open state to detect close transition
  const prevOpenRef = useRef(open);

  // Reset form when modal closes (transition from open to closed)
  useEffect(() => {
    if (prevOpenRef.current && !open) {
      // Modal just closed
      form.reset();
      resetPluginConfig();
    }
    prevOpenRef.current = open;
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps
  // Note: form.reset and resetPluginConfig are intentionally excluded from deps
  // because we only want this to run on open state change, not on every render

  // Handle form submission
  const handleCreate = async () => {
    await form.submit(pluginConfig);
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-pod-title"
    >
      <div
        ref={modalRef}
        className="bg-background border border-border rounded-lg w-full max-w-md p-4 md:p-6 max-h-[90vh] overflow-y-auto"
      >
        <h2 id="create-pod-title" className="text-lg md:text-xl font-semibold mb-4">
          {t("ide.createPod.title")}
        </h2>

        {loadingData ? (
          <div className="flex items-center justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Agent Type Select */}
            <div>
              <label htmlFor="agent-type-select" className="block text-sm font-medium mb-2">
                {t("ide.createPod.selectAgent")}
              </label>
              <select
                id="agent-type-select"
                className={`w-full px-3 py-2 border rounded-md bg-background ${
                  form.validationErrors.agent ? "border-destructive" : "border-border"
                }`}
                value={form.selectedAgent || ""}
                onChange={(e) => {
                  console.log("[CreatePodModal] Agent selected:", e.target.value, "agentTypes:", agentTypes);
                  form.setSelectedAgent(e.target.value ? Number(e.target.value) : null);
                }}
                aria-required="true"
                aria-invalid={!!form.validationErrors.agent}
                aria-describedby={form.validationErrors.agent ? "agent-error" : undefined}
              >
                <option value="">{t("ide.createPod.selectAgentPlaceholder")}</option>
                {agentTypes.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.name}
                  </option>
                ))}
              </select>
              {form.validationErrors.agent && (
                <p id="agent-error" className="text-xs text-destructive mt-1">
                  {form.validationErrors.agent}
                </p>
              )}
            </div>

            {/* Runner Select */}
            <div>
              <label htmlFor="runner-select" className="block text-sm font-medium mb-2">
                {t("ide.createPod.selectRunner")}
              </label>
              <select
                id="runner-select"
                className={`w-full px-3 py-2 border rounded-md bg-background ${
                  form.validationErrors.runner ? "border-destructive" : "border-border"
                }`}
                value={form.selectedRunner || ""}
                onChange={(e) => {
                  console.log("[CreatePodModal] Runner selected:", e.target.value, "runners:", runners);
                  form.setSelectedRunner(e.target.value ? Number(e.target.value) : null);
                }}
                aria-required="true"
                aria-invalid={!!form.validationErrors.runner}
                aria-describedby={
                  form.validationErrors.runner
                    ? "runner-error"
                    : runners.length === 0
                    ? "runner-help"
                    : undefined
                }
              >
                <option value="">{t("ide.createPod.selectRunnerPlaceholder")}</option>
                {runners.map((runner) => (
                  <option key={runner.id} value={runner.id}>
                    {runner.node_id} ({runner.current_pods}/{runner.max_concurrent_pods})
                  </option>
                ))}
              </select>
              {form.validationErrors.runner && (
                <p id="runner-error" className="text-xs text-destructive mt-1">
                  {form.validationErrors.runner}
                </p>
              )}
              {!form.validationErrors.runner && runners.length === 0 && (
                <p id="runner-help" className="text-xs text-muted-foreground mt-1">
                  {t("ide.createPod.selectRunnerPlaceholder")}
                </p>
              )}
            </div>

            {/* Repository Select */}
            <div>
              <label htmlFor="repository-select" className="block text-sm font-medium mb-2">
                {t("ide.createPod.selectRepository")}
              </label>
              <select
                id="repository-select"
                className="w-full px-3 py-2 border border-border rounded-md bg-background"
                value={form.selectedRepository || ""}
                onChange={(e) => form.setSelectedRepository(e.target.value ? Number(e.target.value) : null)}
              >
                <option value="">{t("ide.createPod.selectRepositoryPlaceholder")}</option>
                {repositories.map((repo) => (
                  <option key={repo.id} value={repo.id}>
                    {repo.full_path}
                  </option>
                ))}
              </select>
            </div>

            {/* Branch Input */}
            {form.selectedRepository && (
              <div>
                <label htmlFor="branch-input" className="block text-sm font-medium mb-2">
                  {t("ide.createPod.branch")}
                </label>
                <input
                  id="branch-input"
                  type="text"
                  className={`w-full px-3 py-2 border rounded-md bg-background ${
                    form.validationErrors.branch ? "border-destructive" : "border-border"
                  }`}
                  placeholder={t("ide.createPod.branchPlaceholder")}
                  value={form.selectedBranch}
                  onChange={(e) => form.setSelectedBranch(e.target.value)}
                  aria-invalid={!!form.validationErrors.branch}
                  aria-describedby={form.validationErrors.branch ? "branch-error" : undefined}
                />
                {form.validationErrors.branch && (
                  <p id="branch-error" className="text-xs text-destructive mt-1">
                    {form.validationErrors.branch}
                  </p>
                )}
              </div>
            )}

            {/* Initial Prompt */}
            <div>
              <label htmlFor="prompt-input" className="block text-sm font-medium mb-2">
                {t("ide.createPod.initialPrompt")}
              </label>
              <textarea
                id="prompt-input"
                className="w-full px-3 py-2 border border-border rounded-md bg-background resize-none"
                rows={3}
                placeholder={t("ide.createPod.initialPromptPlaceholder")}
                value={form.prompt}
                onChange={(e) => form.setPrompt(e.target.value)}
              />
            </div>

            {/* Plugin Configuration Section */}
            {loadingPlugins ? (
              <div className="flex items-center justify-center py-4">
                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary mr-2"></div>
                <span className="text-sm text-muted-foreground">Loading plugin options...</span>
              </div>
            ) : (
              pluginOptions.length > 0 && (
                <div>
                  <label className="block text-sm font-medium mb-2">Plugin Configuration</label>
                  <PluginConfigForm
                    plugins={pluginOptions}
                    values={pluginConfig}
                    onChange={handlePluginConfigChange}
                  />
                </div>
              )
            )}

            {/* Error Display */}
            {form.error && (
              <div
                role="alert"
                aria-live="assertive"
                className="bg-destructive/10 border border-destructive/30 rounded-md p-3"
              >
                <p className="text-sm text-destructive">{form.error}</p>
              </div>
            )}
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex flex-col-reverse sm:flex-row justify-end gap-3 mt-6">
          <Button variant="outline" onClick={onClose} className="w-full sm:w-auto">
            {t("ide.createPod.cancel")}
          </Button>
          <Button
            onClick={handleCreate}
            disabled={!form.selectedAgent || !form.selectedRunner || form.loading || loadingData}
            className="w-full sm:w-auto"
          >
            {form.loading ? t("ide.createPod.creating") : t("ide.createPod.create")}
          </Button>
        </div>
      </div>
    </div>
  );
}

export default CreatePodModal;
