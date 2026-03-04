"use client";

import { useState, useEffect, useCallback } from "react";
import { Search, Power, PowerOff, Trash2, Server } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  listRunners,
  disableRunner,
  enableRunner,
  deleteRunner,
  Runner,
} from "@/lib/api/admin";
import type { PaginatedResponse } from "@/lib/api/base";
import { formatRelativeTime } from "@/lib/utils";

export default function RunnersPage() {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<PaginatedResponse<Runner> | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const fetchRunners = useCallback(async () => {
    setIsLoading(true);
    try {
      const result = await listRunners({ search, page, page_size: 20 });
      setData(result);
    } catch {
      // Keep previous data on error
    } finally {
      setIsLoading(false);
    }
  }, [search, page]);

  useEffect(() => {
    fetchRunners();
  }, [fetchRunners]);

  const handleDisable = async (runnerId: number) => {
    try {
      await disableRunner(runnerId);
      toast.success("Runner disabled successfully");
      await fetchRunners();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to disable runner";
      toast.error(message);
    }
  };

  const handleEnable = async (runnerId: number) => {
    try {
      await enableRunner(runnerId);
      toast.success("Runner enabled successfully");
      await fetchRunners();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to enable runner";
      toast.error(message);
    }
  };

  const handleDelete = async (runner: Runner) => {
    if (!confirm(`Are you sure you want to delete runner "${runner.node_id}"? This action cannot be undone.`)) {
      return;
    }
    try {
      await deleteRunner(runner.id);
      toast.success("Runner deleted successfully");
      await fetchRunners();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to delete runner";
      toast.error(message);
    }
  };

  return (
    <div className="space-y-4">
      {/* Search */}
      <div className="flex items-center gap-4">
        <div className="relative flex-1 sm:max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search runners..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            className="pl-9"
          />
        </div>
      </div>

      {/* Runners Table */}
      <Card>
        <CardHeader>
          <CardTitle>Runners ({data?.total || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-20 animate-pulse rounded-lg bg-muted" />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              {data?.data.map((runner) => (
                <RunnerRow
                  key={runner.id}
                  runner={runner}
                  onDisable={() => handleDisable(runner.id)}
                  onEnable={() => handleEnable(runner.id)}
                  onDelete={() => handleDelete(runner)}
                />
              ))}
              {data?.data.length === 0 && (
                <p className="py-8 text-center text-muted-foreground">
                  No runners found
                </p>
              )}
            </div>
          )}

          {/* Pagination */}
          {data && data.total_pages > 1 && (
            <div className="mt-4 flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                Page {data.page} of {data.total_pages}
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 1}
                  onClick={() => setPage(page - 1)}
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= data.total_pages}
                  onClick={() => setPage(page + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function RunnerRow({
  runner,
  onDisable,
  onEnable,
  onDelete,
}: {
  runner: Runner;
  onDisable: () => void;
  onEnable: () => void;
  onDelete: () => void;
}) {
  const isOnline = runner.status === "online";

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-4">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-secondary">
          <Server className="h-5 w-5 text-muted-foreground" />
        </div>
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{runner.node_id}</span>
            <Badge variant={isOnline ? "success" : "secondary"}>
              {runner.status}
            </Badge>
            {!runner.is_enabled && (
              <Badge variant="destructive">Disabled</Badge>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            {runner.organization && (
              <span>{runner.organization.name}</span>
            )}
            {runner.runner_version && (
              <span>v{runner.runner_version}</span>
            )}
            <span>
              {runner.current_pods}/{runner.max_concurrent_pods} pods
            </span>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-4">
        <div className="hidden text-right text-xs text-muted-foreground sm:block">
          {runner.last_heartbeat && (
            <p>Last seen {formatRelativeTime(runner.last_heartbeat)}</p>
          )}
          {runner.available_agents && runner.available_agents.length > 0 && (
            <p>{runner.available_agents.length} agents</p>
          )}
        </div>
        <div className="flex gap-1">
          {runner.is_enabled ? (
            <Button
              variant="ghost"
              size="icon"
              onClick={onDisable}
              title="Disable runner"
            >
              <PowerOff className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              onClick={onEnable}
              title="Enable runner"
            >
              <Power className="h-4 w-4" />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            onClick={onDelete}
            title="Delete runner"
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
