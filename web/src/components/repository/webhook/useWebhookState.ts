"use client";

import { useState, useCallback } from "react";
import { repositoryApi, WebhookStatus, WebhookSecretResponse } from "@/lib/api";
import { WebhookState, WebhookSettingsState, WebhookSettingsActions } from "./types";

export interface UseWebhookStateResult extends WebhookSettingsState, WebhookSettingsActions {}

export function useWebhookState(repositoryId: number, onUpdate?: () => void): UseWebhookStateResult {
  const [state, setState] = useState<WebhookState>("loading");
  const [status, setStatus] = useState<WebhookStatus | null>(null);
  const [secretData, setSecretData] = useState<WebhookSecretResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const loadStatus = useCallback(async () => {
    setState("loading");
    setError(null);
    try {
      const res = await repositoryApi.getWebhookStatus(repositoryId);
      setStatus(res.webhook_status);

      if (res.webhook_status.registered && res.webhook_status.is_active) {
        setState("registered");
      } else if (res.webhook_status.needs_manual_setup) {
        setState("needs_manual_setup");
        // Load secret for manual setup
        try {
          const secretRes = await repositoryApi.getWebhookSecret(repositoryId);
          setSecretData(secretRes);
        } catch {
          // Secret might not be available if already configured
        }
      } else {
        setState("not_registered");
      }
    } catch (err) {
      console.error("Failed to load webhook status:", err);
      setError("Failed to load webhook status");
      setState("error");
    }
  }, [repositoryId]);

  const handleRegister = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await repositoryApi.registerWebhook(repositoryId);
      if (res.result.registered) {
        setState("registered");
      } else if (res.result.needs_manual_setup) {
        setState("needs_manual_setup");
        setSecretData({
          webhook_url: res.result.manual_webhook_url || "",
          webhook_secret: res.result.manual_webhook_secret || "",
          events: ["merge_request", "pipeline"],
        });
      }
      onUpdate?.();
      await loadStatus();
    } catch (err) {
      console.error("Failed to register webhook:", err);
      setError("Failed to register webhook");
    } finally {
      setLoading(false);
    }
  }, [repositoryId, onUpdate, loadStatus]);

  const handleDelete = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      await repositoryApi.deleteWebhook(repositoryId);
      setState("not_registered");
      setStatus(null);
      setSecretData(null);
      onUpdate?.();
    } catch (err) {
      console.error("Failed to delete webhook:", err);
      setError("Failed to delete webhook");
    } finally {
      setLoading(false);
    }
  }, [repositoryId, onUpdate]);

  const handleMarkConfigured = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      await repositoryApi.markWebhookConfigured(repositoryId);
      setState("registered");
      onUpdate?.();
      await loadStatus();
    } catch (err) {
      console.error("Failed to mark webhook as configured:", err);
      setError("Failed to mark webhook as configured");
    } finally {
      setLoading(false);
    }
  }, [repositoryId, onUpdate, loadStatus]);

  return {
    state,
    status,
    secretData,
    error,
    loading,
    handleRegister,
    handleDelete,
    handleMarkConfigured,
    loadStatus,
  };
}
