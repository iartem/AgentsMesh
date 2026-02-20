"use client";

import { Button } from "@/components/ui/button";
import { Runner, getRunnerStatusInfo } from "@/stores/runner";
import { isVersionOutdated } from "@/lib/utils/version";
import type { TranslationFn } from "../GeneralSettings";

interface RunnerCardProps {
  runner: Runner;
  onEdit: (runner: Runner) => void;
  onDelete: () => void;
  formatLastSeen: (dateString?: string) => string;
  t: TranslationFn;
  latestRunnerVersion?: string;
}

/**
 * Individual runner card in the runners list
 */
export function RunnerCard({
  runner,
  onEdit,
  onDelete,
  formatLastSeen,
  t,
  latestRunnerVersion,
}: RunnerCardProps) {
  const statusInfo = getRunnerStatusInfo(runner.status as "online" | "offline" | "maintenance" | "busy");

  return (
    <div
      className={`p-4 border rounded-lg ${
        runner.is_enabled ? "border-border" : "border-border bg-muted/50"
      }`}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <span className="font-medium">{runner.node_id}</span>
            <span
              className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${statusInfo?.color}`}
            >
              <span className={`w-1.5 h-1.5 rounded-full ${statusInfo?.dotColor}`} />
              {statusInfo?.label}
            </span>
            {!runner.is_enabled && (
              <span className="text-xs bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400 px-2 py-0.5 rounded">
                {t("settings.runnersSection.disabled")}
              </span>
            )}
          </div>
          {runner.description && (
            <p className="text-sm text-muted-foreground mt-1">
              {runner.description}
            </p>
          )}
          <div className="flex items-center gap-4 text-sm text-muted-foreground mt-2">
            <span>
              {t("settings.runnersSection.pods")} {runner.current_pods} / {runner.max_concurrent_pods}
            </span>
            {runner.runner_version && (
              <span className="flex items-center gap-1">
                v{runner.runner_version}
                {isVersionOutdated(runner.runner_version, latestRunnerVersion) && (
                  <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                    {t("settings.runnersSection.upgradeAvailable")}
                  </span>
                )}
              </span>
            )}
            <span>{t("settings.runnersSection.lastSeen")} {formatLastSeen(runner.last_heartbeat)}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => onEdit(runner)}>
            {t("settings.runnersSection.edit")}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive"
            onClick={onDelete}
          >
            {t("settings.runnersSection.delete")}
          </Button>
        </div>
      </div>
    </div>
  );
}

export default RunnerCard;
