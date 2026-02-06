"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import { cn } from "@/lib/utils";
import { repositoryApi } from "@/lib/api/repository";
import { useEventSubscription } from "@/hooks/useRealtimeEvents";
import type { MREventData, PipelineEventData } from "@/lib/realtime";
import type { PodData } from "@/lib/api/pod";
import {
  GitPullRequest,
  GitMerge,
  XCircle,
  GitBranch,
  ArrowRight,
  ExternalLink,
  RefreshCw,
  Loader2,
  Terminal,
} from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * MR data structure
 */
interface MergeRequestInfo {
  id: number;
  mr_iid: number;
  title: string;
  state: "opened" | "merged" | "closed" | string;
  mr_url: string;
  source_branch: string;
  target_branch: string;
  pipeline_status?: string;
  pipeline_url?: string;
}

interface DeliveryTabContentProps {
  selectedPodKey: string | null;
  pod: PodData | null;
  t: (key: string, params?: Record<string, string | number>) => string;
}

export function DeliveryTabContent({
  selectedPodKey,
  pod,
  t,
}: DeliveryTabContentProps) {
  const [mergeRequests, setMergeRequests] = useState<MergeRequestInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Check if delivery can be displayed - requires repository and branch
  const canShowDelivery = useMemo(() => {
    return !!(pod?.repository?.id && pod?.branch_name);
  }, [pod?.repository?.id, pod?.branch_name]);

  // Get provider type from repository
  const providerType = useMemo(() => {
    return pod?.repository?.provider_type;
  }, [pod?.repository?.provider_type]);

  // Fetch MR data from repository API
  const fetchMRs = useCallback(async () => {
    if (!pod?.repository?.id || !pod?.branch_name) return;

    setLoading(true);
    setError(null);
    try {
      const response = await repositoryApi.listMergeRequests(
        pod.repository.id,
        pod.branch_name // Filter by current branch
      );
      setMergeRequests(response.merge_requests as MergeRequestInfo[]);
    } catch {
      setError(t("ide.bottomPanel.deliveryTab.loadError"));
    } finally {
      setLoading(false);
    }
  }, [pod?.repository?.id, pod?.branch_name, t]);

  // Initial load
  useEffect(() => {
    if (canShowDelivery) {
      fetchMRs();
    } else {
      setMergeRequests([]);
    }
  }, [canShowDelivery, fetchMRs]);

  // Subscribe to MR events - filter by repository_id
  const handleMREvent = useCallback(
    (event: { data: MREventData }) => {
      // Check if event is for this repository
      if (event.data.repository_id !== pod?.repository?.id) return;
      // Optionally filter by branch
      if (event.data.source_branch && event.data.source_branch !== pod?.branch_name) return;
      fetchMRs();
    },
    [pod?.repository?.id, pod?.branch_name, fetchMRs]
  );

  // Subscribe to Pipeline events
  const handlePipelineEvent = useCallback(
    (event: { data: PipelineEventData }) => {
      // Check if event is for this repository
      if (event.data.repository_id !== pod?.repository?.id) return;
      // Update the pipeline status of the corresponding MR
      setMergeRequests((prev) =>
        prev.map((mr) =>
          mr.id === event.data.mr_id
            ? {
                ...mr,
                pipeline_status: event.data.pipeline_status,
                pipeline_url: event.data.pipeline_url,
              }
            : mr
        )
      );
    },
    [pod?.repository?.id]
  );

  useEventSubscription("mr:created", handleMREvent);
  useEventSubscription("mr:updated", handleMREvent);
  useEventSubscription("mr:merged", handleMREvent);
  useEventSubscription("mr:closed", handleMREvent);
  useEventSubscription("pipeline:updated", handlePipelineEvent);

  // Empty state: No Pod selected
  if (!selectedPodKey) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
        <Terminal className="w-8 h-8 mb-2 opacity-50" />
        <span className="text-xs">{t("ide.bottomPanel.selectPodFirst")}</span>
      </div>
    );
  }

  // Empty state: Conditions not met (need repository and branch)
  if (!canShowDelivery) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
        <GitPullRequest className="w-8 h-8 mb-2 opacity-50" />
        <span className="text-xs">
          {t("ide.bottomPanel.deliveryTab.notAvailable")}
        </span>
        <span className="text-[10px] mt-1 opacity-70">
          {t("ide.bottomPanel.deliveryTab.requiresRepoBranch")}
        </span>
      </div>
    );
  }

  // Loading state
  if (loading && mergeRequests.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-4 h-4 animate-spin mr-2" />
        <span className="text-muted-foreground text-xs">
          {t("ide.bottomPanel.deliveryTab.loading")}
        </span>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
        <span className="text-xs text-destructive">{error}</span>
        <Button variant="ghost" size="sm" onClick={fetchMRs} className="mt-2">
          <RefreshCw className="w-3 h-3 mr-1" />
          {t("common.refresh")}
        </Button>
      </div>
    );
  }

  // Empty MR list
  if (mergeRequests.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
        <GitPullRequest className="w-8 h-8 mb-2 opacity-50" />
        <span className="text-xs">
          {t("ide.bottomPanel.deliveryTab.noMergeRequests")}
        </span>
      </div>
    );
  }

  // MR list
  return (
    <div className="space-y-2 h-full overflow-auto">
      {/* Header */}
      <div className="flex items-center justify-between sticky top-0 bg-background pb-1">
        <span className="text-xs text-muted-foreground">
          {t("ide.bottomPanel.deliveryTab.mrCount", {
            count: mergeRequests.length,
          })}
        </span>
        <Button
          variant="ghost"
          size="sm"
          onClick={fetchMRs}
          className="h-6 w-6 p-0"
        >
          <RefreshCw className={cn("w-3 h-3", loading && "animate-spin")} />
        </Button>
      </div>

      {/* MR card list */}
      <div className="space-y-1.5">
        {mergeRequests.map((mr) => (
          <MergeRequestCard
            key={mr.id}
            mr={mr}
            providerType={providerType}
            t={t}
          />
        ))}
      </div>
    </div>
  );
}

/**
 * MR Card component
 */
function MergeRequestCard({
  mr,
  providerType,
  t,
}: {
  mr: MergeRequestInfo;
  providerType?: string;
  t: (key: string) => string;
}) {
  return (
    <a
      href={mr.mr_url}
      target="_blank"
      rel="noopener noreferrer"
      className="block px-3 py-2 rounded bg-muted/50 hover:bg-muted transition-colors"
    >
      <div className="flex items-center gap-2">
        {/* MR status icon */}
        <MRStateIcon state={mr.state} />

        {/* MR title */}
        <span
          className="text-xs font-medium flex-1 truncate"
          title={mr.title}
        >
          {mr.title || t("ide.bottomPanel.deliveryTab.untitled")}
        </span>

        {/* MR IID */}
        <span className="text-xs text-muted-foreground">
          {providerType === "github" ? `#${mr.mr_iid}` : `!${mr.mr_iid}`}
        </span>

        {/* External link icon */}
        <ExternalLink className="w-3 h-3 text-muted-foreground flex-shrink-0" />
      </div>

      {/* Branch info */}
      <div className="flex items-center gap-1 mt-1.5 text-[10px] text-muted-foreground">
        <GitBranch className="w-3 h-3 flex-shrink-0" />
        <span className="font-mono truncate">{mr.source_branch}</span>
        <ArrowRight className="w-3 h-3 flex-shrink-0" />
        <span className="font-mono truncate">{mr.target_branch}</span>
      </div>

      {/* Pipeline status */}
      {mr.pipeline_status && (
        <div className="flex items-center gap-1 mt-1.5">
          <PipelineStatusBadge status={mr.pipeline_status} url={mr.pipeline_url} />
        </div>
      )}
    </a>
  );
}

/**
 * MR state icon
 */
function MRStateIcon({ state }: { state: string }) {
  switch (state) {
    case "opened":
      return (
        <GitPullRequest className="w-3.5 h-3.5 text-green-500 flex-shrink-0" />
      );
    case "merged":
      return (
        <GitMerge className="w-3.5 h-3.5 text-purple-500 flex-shrink-0" />
      );
    case "closed":
      return <XCircle className="w-3.5 h-3.5 text-red-500 flex-shrink-0" />;
    default:
      return (
        <GitPullRequest className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
      );
  }
}

/**
 * Pipeline status badge
 */
function PipelineStatusBadge({
  status,
  url,
}: {
  status: string;
  url?: string;
}) {
  const getStatusStyle = () => {
    switch (status) {
      case "success":
        return "bg-green-500/10 text-green-500";
      case "failed":
        return "bg-red-500/10 text-red-500";
      case "running":
        return "bg-blue-500/10 text-blue-500";
      case "pending":
        return "bg-yellow-500/10 text-yellow-500";
      case "canceled":
        return "bg-gray-500/10 text-gray-500";
      default:
        return "bg-muted text-muted-foreground";
    }
  };

  const content = (
    <span
      className={cn(
        "px-1.5 py-0.5 rounded text-[10px] flex items-center gap-1",
        getStatusStyle()
      )}
    >
      {status === "running" && (
        <Loader2 className="w-2.5 h-2.5 animate-spin" />
      )}
      <span>Pipeline: {status}</span>
    </span>
  );

  return url ? (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
      className="hover:opacity-80"
    >
      {content}
    </a>
  ) : (
    content
  );
}

export default DeliveryTabContent;
