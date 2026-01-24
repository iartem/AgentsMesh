"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogBody,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  runnerApi,
  podApi,
  type RunnerData,
  type RunnerPodData,
  type SandboxStatus,
} from "@/lib/api";
import { useTranslations } from "@/lib/i18n/client";
import {
  Server,
  ArrowLeft,
  RefreshCw,
  Trash2,
  Power,
  PowerOff,
  CheckCircle,
  XCircle,
  Clock,
  AlertCircle,
  HardDrive,
  Cpu,
  Activity,
  Terminal,
  GitBranch,
  FolderOpen,
  RotateCcw,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { formatDistanceToNow, format } from "date-fns";

export default function RunnerDetailPage() {
  const t = useTranslations();
  const params = useParams();
  const router = useRouter();
  const runnerId = Number(params.id);

  const [runner, setRunner] = useState<RunnerData | null>(null);
  const [pods, setPods] = useState<RunnerPodData[]>([]);
  const [sandboxStatuses, setSandboxStatuses] = useState<Map<string, SandboxStatus>>(new Map());
  const [loading, setLoading] = useState(true);
  const [loadingPods, setLoadingPods] = useState(false);
  const [loadingSandbox, setLoadingSandbox] = useState(false);
  const [activeTab, setActiveTab] = useState<"overview" | "pods">("overview");
  const [podFilter, setPodFilter] = useState<string>("");
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  // Resume confirmation dialog state
  const [resumeDialogOpen, setResumeDialogOpen] = useState(false);
  const [resumingPod, setResumingPod] = useState<RunnerPodData | null>(null);
  const [resumeLoading, setResumeLoading] = useState(false);

  const loadRunner = useCallback(async () => {
    try {
      const res = await runnerApi.get(runnerId);
      setRunner(res.runner);
    } catch (error) {
      console.error("Failed to load runner:", error);
    } finally {
      setLoading(false);
    }
  }, [runnerId]);

  const loadPods = useCallback(async () => {
    setLoadingPods(true);
    try {
      const res = await runnerApi.listPods(runnerId, {
        status: podFilter || undefined,
        limit,
        offset,
      });
      setPods(res.pods || []);
      setTotal(res.total);
    } catch (error) {
      console.error("Failed to load pods:", error);
    } finally {
      setLoadingPods(false);
    }
  }, [runnerId, podFilter, offset]);

  useEffect(() => {
    loadRunner();
  }, [loadRunner]);

  useEffect(() => {
    if (activeTab === "pods") {
      loadPods();
    }
  }, [activeTab, loadPods]);

  const handleRefreshSandboxStatus = async () => {
    if (!runner || runner.status !== "online") return;

    // Get all non-running pod keys that might have sandboxes
    // Include: terminated, completed, orphaned, error, etc.
    const inactivePodKeys = pods
      .filter(p => p.status !== "running" && p.status !== "initializing")
      .map(p => p.pod_key);

    if (inactivePodKeys.length === 0) return;

    setLoadingSandbox(true);
    try {
      const res = await runnerApi.querySandboxes(runnerId, inactivePodKeys);
      const newStatuses = new Map<string, SandboxStatus>();
      for (const status of res.sandboxes || []) {
        newStatuses.set(status.pod_key, status);
      }
      setSandboxStatuses(newStatuses);
    } catch (error) {
      console.error("Failed to query sandbox status:", error);
    } finally {
      setLoadingSandbox(false);
    }
  };

  // Open resume confirmation dialog
  const openResumeDialog = (pod: RunnerPodData) => {
    setResumingPod(pod);
    setResumeDialogOpen(true);
  };

  // Execute resume after confirmation
  const handleConfirmResume = async () => {
    if (!runner || !resumingPod) return;

    setResumeLoading(true);
    try {
      const res = await podApi.create({
        runner_id: runner.id,
        source_pod_key: resumingPod.pod_key,
        resume_agent_session: true,
        cols: 120,
        rows: 30,
      });

      setResumeDialogOpen(false);
      setResumingPod(null);
      // Navigate to workspace with the new pod key as query param
      // The workspace page will auto-open this pod
      router.push(`/${params.org}/workspace?pod=${res.pod.pod_key}`);
    } catch (error) {
      console.error("Failed to resume pod:", error);
      // Keep dialog open on error so user can retry or cancel
    } finally {
      setResumeLoading(false);
    }
  };

  const handleToggleEnabled = async () => {
    if (!runner) return;
    try {
      await runnerApi.update(runner.id, { is_enabled: !runner.is_enabled });
      loadRunner();
    } catch (error) {
      console.error("Failed to update runner:", error);
    }
  };

  const handleDelete = async () => {
    if (!runner) return;
    if (!confirm(t("runners.detail.confirmDelete", { nodeId: runner.node_id }))) {
      return;
    }
    try {
      await runnerApi.delete(runner.id);
      router.push("../runners");
    } catch (error) {
      console.error("Failed to delete runner:", error);
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "online":
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case "offline":
        return <PowerOff className="w-4 h-4 text-gray-400" />;
      case "busy":
        return <Activity className="w-4 h-4 text-yellow-500" />;
      case "maintenance":
        return <AlertCircle className="w-4 h-4 text-orange-500" />;
      default:
        return <Clock className="w-4 h-4 text-gray-400" />;
    }
  };

  const getPodStatusBadge = (status: string) => {
    const statusColors: Record<string, string> = {
      running: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
      initializing: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
      terminated: "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400",
      error: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
      paused: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400",
    };
    return statusColors[status] || "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400";
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="w-6 h-6 animate-spin text-gray-400" />
      </div>
    );
  }

  if (!runner) {
    return (
      <div className="p-6">
        <p className="text-gray-500 dark:text-gray-400">
          {t("runners.detail.notFound")}
        </p>
        <Link href="../runners">
          <Button variant="outline" className="mt-4">
            <ArrowLeft className="w-4 h-4 mr-2" />
            {t("common.back")}
          </Button>
        </Link>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Link href="../runners">
            <Button variant="ghost" size="icon">
              <ArrowLeft className="w-5 h-5" />
            </Button>
          </Link>
          <div className="flex items-center space-x-3">
            <Server className="w-8 h-8 text-gray-400" />
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
                {runner.node_id}
              </h1>
              <div className="flex items-center space-x-2 text-sm text-gray-500">
                {getStatusIcon(runner.status)}
                <span className="capitalize">{runner.status}</span>
                {!runner.is_enabled && (
                  <span className="text-red-500">({t("runners.detail.disabled")})</span>
                )}
              </div>
            </div>
          </div>
        </div>
        <div className="flex items-center space-x-2">
          <Button variant="outline" onClick={loadRunner}>
            <RefreshCw className="w-4 h-4 mr-2" />
            {t("common.refresh")}
          </Button>
          <Button
            variant={runner.is_enabled ? "outline" : "default"}
            onClick={handleToggleEnabled}
          >
            {runner.is_enabled ? (
              <>
                <PowerOff className="w-4 h-4 mr-2" />
                {t("runners.detail.disable")}
              </>
            ) : (
              <>
                <Power className="w-4 h-4 mr-2" />
                {t("runners.detail.enable")}
              </>
            )}
          </Button>
          <Button variant="destructive" onClick={handleDelete}>
            <Trash2 className="w-4 h-4 mr-2" />
            {t("common.delete")}
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700">
        <nav className="flex space-x-8">
          <button
            onClick={() => setActiveTab("overview")}
            className={cn(
              "py-4 px-1 border-b-2 font-medium text-sm transition-colors",
              activeTab === "overview"
                ? "border-blue-500 text-blue-600 dark:text-blue-400"
                : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
            )}
          >
            {t("runners.detail.tabs.overview")}
          </button>
          <button
            onClick={() => setActiveTab("pods")}
            className={cn(
              "py-4 px-1 border-b-2 font-medium text-sm transition-colors",
              activeTab === "pods"
                ? "border-blue-500 text-blue-600 dark:text-blue-400"
                : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
            )}
          >
            {t("runners.detail.tabs.pods")}
          </button>
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === "overview" && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Basic Info */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
              {t("runners.detail.basicInfo")}
            </h3>
            <dl className="space-y-4">
              <div>
                <dt className="text-sm text-gray-500 dark:text-gray-400">
                  {t("runners.detail.nodeId")}
                </dt>
                <dd className="text-sm font-medium text-gray-900 dark:text-white">
                  {runner.node_id}
                </dd>
              </div>
              {runner.description && (
                <div>
                  <dt className="text-sm text-gray-500 dark:text-gray-400">
                    {t("runners.detail.description")}
                  </dt>
                  <dd className="text-sm text-gray-900 dark:text-white">
                    {runner.description}
                  </dd>
                </div>
              )}
              <div>
                <dt className="text-sm text-gray-500 dark:text-gray-400">
                  {t("runners.detail.version")}
                </dt>
                <dd className="text-sm text-gray-900 dark:text-white">
                  {runner.runner_version || "-"}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-gray-500 dark:text-gray-400">
                  {t("runners.detail.lastHeartbeat")}
                </dt>
                <dd className="text-sm text-gray-900 dark:text-white">
                  {runner.last_heartbeat
                    ? formatDistanceToNow(new Date(runner.last_heartbeat), { addSuffix: true })
                    : "-"}
                </dd>
              </div>
              <div>
                <dt className="text-sm text-gray-500 dark:text-gray-400">
                  {t("runners.detail.createdAt")}
                </dt>
                <dd className="text-sm text-gray-900 dark:text-white">
                  {format(new Date(runner.created_at), "PPpp")}
                </dd>
              </div>
            </dl>
          </div>

          {/* Capacity */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
              {t("runners.detail.capacity")}
            </h3>
            <dl className="space-y-4">
              <div>
                <dt className="text-sm text-gray-500 dark:text-gray-400">
                  {t("runners.detail.currentPods")}
                </dt>
                <dd className="text-sm font-medium text-gray-900 dark:text-white">
                  {runner.current_pods} / {runner.max_concurrent_pods}
                </dd>
              </div>
              {runner.host_info && (
                <>
                  <div>
                    <dt className="text-sm text-gray-500 dark:text-gray-400 flex items-center">
                      <Cpu className="w-4 h-4 mr-1" />
                      {t("runners.detail.cpu")}
                    </dt>
                    <dd className="text-sm text-gray-900 dark:text-white">
                      {runner.host_info.cpu_cores} cores ({runner.host_info.arch})
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm text-gray-500 dark:text-gray-400 flex items-center">
                      <HardDrive className="w-4 h-4 mr-1" />
                      {t("runners.detail.memory")}
                    </dt>
                    <dd className="text-sm text-gray-900 dark:text-white">
                      {runner.host_info.memory
                        ? `${(runner.host_info.memory / 1024 / 1024 / 1024).toFixed(1)} GB`
                        : "-"}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-sm text-gray-500 dark:text-gray-400">
                      {t("runners.detail.os")}
                    </dt>
                    <dd className="text-sm text-gray-900 dark:text-white">
                      {runner.host_info.os || "-"}
                    </dd>
                  </div>
                </>
              )}
            </dl>
          </div>

          {/* Available Agents */}
          {runner.available_agents && runner.available_agents.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6 md:col-span-2">
              <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
                {t("runners.detail.availableAgents")}
              </h3>
              <div className="flex flex-wrap gap-2">
                {runner.available_agents.map((agent) => (
                  <span
                    key={agent}
                    className="inline-flex items-center px-3 py-1 rounded-full text-sm bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                  >
                    <Terminal className="w-4 h-4 mr-1" />
                    {agent}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {activeTab === "pods" && (
        <div className="space-y-4">
          {/* Filters and Actions */}
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <select
                value={podFilter}
                onChange={(e) => {
                  setPodFilter(e.target.value);
                  setOffset(0);
                }}
                className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-sm"
              >
                <option value="">{t("runners.detail.allStatus")}</option>
                <option value="running">{t("pods.status.running")}</option>
                <option value="terminated">{t("pods.status.terminated")}</option>
                <option value="error">{t("pods.status.error")}</option>
              </select>
            </div>
            <div className="flex items-center space-x-2">
              <Button
                variant="outline"
                onClick={handleRefreshSandboxStatus}
                disabled={loadingSandbox || runner.status !== "online"}
              >
                {loadingSandbox ? (
                  <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                ) : (
                  <FolderOpen className="w-4 h-4 mr-2" />
                )}
                {t("runners.detail.refreshSandbox")}
              </Button>
              <Button variant="outline" onClick={loadPods} disabled={loadingPods}>
                {loadingPods ? (
                  <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                ) : (
                  <RefreshCw className="w-4 h-4 mr-2" />
                )}
                {t("common.refresh")}
              </Button>
            </div>
          </div>

          {/* Pods Table */}
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-gray-900">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.podKey")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.status")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.sandbox")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.branch")}
                  </th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.createdAt")}
                  </th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {t("runners.detail.actions")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {pods.map((pod) => {
                  const sandboxStatus = sandboxStatuses.get(pod.pod_key);
                  const isInactive = pod.status !== "running" && pod.status !== "initializing";
                  const canResume = isInactive && sandboxStatus?.can_resume;

                  return (
                    <tr key={pod.pod_key} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                      <td className="px-4 py-3">
                        <span className="text-sm font-medium text-gray-900 dark:text-white">
                          {pod.pod_key}
                        </span>
                        {pod.source_pod_key && (
                          <span className="ml-2 text-xs text-gray-400">
                            (resumed from {pod.source_pod_key.slice(0, 8)}...)
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={cn(
                            "inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium",
                            getPodStatusBadge(pod.status)
                          )}
                        >
                          {pod.status}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        {pod.status === "running" ? (
                          <span className="flex items-center text-green-600 dark:text-green-400 text-sm">
                            <CheckCircle className="w-4 h-4 mr-1" />
                            {t("runners.detail.active")}
                          </span>
                        ) : isInactive ? (
                          sandboxStatus === undefined ? (
                            <span className="text-gray-400 text-sm">-</span>
                          ) : sandboxStatus.exists ? (
                            <span className="flex items-center text-green-600 dark:text-green-400 text-sm">
                              <CheckCircle className="w-4 h-4 mr-1" />
                              {sandboxStatus.can_resume ? t("runners.detail.canResume") : t("runners.detail.exists")}
                            </span>
                          ) : (
                            <span className="flex items-center text-gray-400 text-sm">
                              <XCircle className="w-4 h-4 mr-1" />
                              {t("runners.detail.notExists")}
                            </span>
                          )
                        ) : (
                          <span className="text-gray-400 text-sm">-</span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                        {pod.branch_name ? (
                          <span className="flex items-center">
                            <GitBranch className="w-4 h-4 mr-1" />
                            {pod.branch_name}
                          </span>
                        ) : (
                          "-"
                        )}
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                        {formatDistanceToNow(new Date(pod.created_at), { addSuffix: true })}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end space-x-2">
                          {canResume && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => openResumeDialog(pod)}
                              title={t("runners.detail.resumeTooltip")}
                            >
                              <RotateCcw className="w-4 h-4 mr-1" />
                              {t("runners.detail.resume")}
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
                {pods.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                      {t("runners.detail.noPods")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {total > limit && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {t("runners.detail.showing", {
                  from: offset + 1,
                  to: Math.min(offset + limit, total),
                  total,
                })}
              </p>
              <div className="flex items-center space-x-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - limit))}
                >
                  {t("common.previous")}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={offset + limit >= total}
                  onClick={() => setOffset(offset + limit)}
                >
                  {t("common.next")}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Resume Confirmation Dialog */}
      <Dialog open={resumeDialogOpen} onOpenChange={setResumeDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("runners.detail.resumeDialogTitle")}</DialogTitle>
            <DialogDescription>
              {t("runners.detail.resumeDialogDescription", {
                podKey: resumingPod?.pod_key || "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              {t("runners.detail.resumeDialogInfo")}
            </p>
          </DialogBody>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setResumeDialogOpen(false);
                setResumingPod(null);
              }}
              disabled={resumeLoading}
            >
              {t("common.cancel")}
            </Button>
            <Button
              onClick={handleConfirmResume}
              disabled={resumeLoading}
            >
              {resumeLoading ? (
                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
              ) : (
                <RotateCcw className="w-4 h-4 mr-2" />
              )}
              {t("runners.detail.confirmResumeBtn")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
