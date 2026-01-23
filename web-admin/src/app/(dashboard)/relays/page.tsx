"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import {
  Radio,
  Trash2,
  Activity,
  Wifi,
  WifiOff,
  ArrowRightLeft,
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  listRelays,
  getRelayStats,
  forceUnregisterRelay,
  bulkMigrateSessions,
  RelayInfo,
} from "@/lib/api/admin";
import { formatRelativeTime } from "@/lib/utils";

export default function RelaysPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [selectedSource, setSelectedSource] = useState<string>("");
  const [selectedTarget, setSelectedTarget] = useState<string>("");

  const { data: relaysData, isLoading } = useQuery({
    queryKey: ["relays"],
    queryFn: listRelays,
    refetchInterval: 10000, // Auto-refresh every 10 seconds
  });

  const { data: stats } = useQuery({
    queryKey: ["relay-stats"],
    queryFn: getRelayStats,
    refetchInterval: 10000,
  });

  const unregisterMutation = useMutation({
    mutationFn: ({ id, migrate }: { id: string; migrate: boolean }) =>
      forceUnregisterRelay(id, migrate),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["relays"] });
      queryClient.invalidateQueries({ queryKey: ["relay-stats"] });
      toast.success(
        `Relay unregistered. ${data.affected_sessions} sessions affected.`
      );
    },
    onError: (err: { error?: string }) => {
      toast.error(err.error || "Failed to unregister relay");
    },
  });

  const bulkMigrateMutation = useMutation({
    mutationFn: ({
      source,
      target,
    }: {
      source: string;
      target: string;
    }) => bulkMigrateSessions(source, target),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["relays"] });
      queryClient.invalidateQueries({ queryKey: ["relay-stats"] });
      toast.success(
        `Migration completed: ${data.migrated}/${data.total} sessions migrated`
      );
      setSelectedSource("");
      setSelectedTarget("");
    },
    onError: (err: { error?: string }) => {
      toast.error(err.error || "Failed to migrate sessions");
    },
  });

  const handleUnregister = (relay: RelayInfo, migrate: boolean) => {
    const msg = migrate
      ? `Unregister relay "${relay.id}" and migrate all sessions to another relay?`
      : `Unregister relay "${relay.id}"? ${relay.connections} active connections will be affected.`;
    if (confirm(msg)) {
      unregisterMutation.mutate({ id: relay.id, migrate });
    }
  };

  const handleBulkMigrate = () => {
    if (!selectedSource || !selectedTarget) {
      toast.error("Please select source and target relays");
      return;
    }
    if (selectedSource === selectedTarget) {
      toast.error("Source and target cannot be the same");
      return;
    }
    if (confirm(`Migrate all sessions from "${selectedSource}" to "${selectedTarget}"?`)) {
      bulkMigrateMutation.mutate({
        source: selectedSource,
        target: selectedTarget,
      });
    }
  };

  const healthyRelays = relaysData?.data.filter((r) => r.healthy) || [];

  return (
    <div className="space-y-4">
      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Relays</CardTitle>
            <Radio className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{stats?.total_relays || 0}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Healthy Relays</CardTitle>
            <Wifi className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {stats?.healthy_relays || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Connections
            </CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.total_connections || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Active Sessions
            </CardTitle>
            <ArrowRightLeft className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.active_sessions || 0}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Bulk Migration */}
      {healthyRelays.length >= 2 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Bulk Session Migration</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4">
              <div className="flex-1">
                <Select
                  value={selectedSource}
                  onValueChange={setSelectedSource}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Source Relay" />
                  </SelectTrigger>
                  <SelectContent>
                    {relaysData?.data.map((relay) => (
                      <SelectItem key={relay.id} value={relay.id}>
                        {relay.id} ({relay.connections} connections)
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <ArrowRightLeft className="h-4 w-4 text-muted-foreground" />
              <div className="flex-1">
                <Select
                  value={selectedTarget}
                  onValueChange={setSelectedTarget}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Target Relay" />
                  </SelectTrigger>
                  <SelectContent>
                    {healthyRelays
                      .filter((r) => r.id !== selectedSource)
                      .map((relay) => (
                        <SelectItem key={relay.id} value={relay.id}>
                          {relay.id} ({relay.region})
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                onClick={handleBulkMigrate}
                disabled={
                  !selectedSource ||
                  !selectedTarget ||
                  bulkMigrateMutation.isPending
                }
              >
                {bulkMigrateMutation.isPending ? (
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <ArrowRightLeft className="mr-2 h-4 w-4" />
                )}
                Migrate
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Relays Table */}
      <Card>
        <CardHeader>
          <CardTitle>Relay Servers ({relaysData?.total || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div
                  key={i}
                  className="h-20 animate-pulse rounded-lg bg-muted"
                />
              ))}
            </div>
          ) : relaysData?.data.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              <Radio className="mx-auto mb-2 h-8 w-8" />
              <p>No relay servers registered</p>
              <p className="text-sm">
                Relay servers will appear here once they connect to the backend.
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              {relaysData?.data.map((relay) => (
                <RelayRow
                  key={relay.id}
                  relay={relay}
                  onClick={() => router.push(`/relays/${encodeURIComponent(relay.id)}`)}
                  onUnregister={(migrate) => handleUnregister(relay, migrate)}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function RelayRow({
  relay,
  onClick,
  onUnregister,
}: {
  relay: RelayInfo;
  onClick: () => void;
  onUnregister: (migrate: boolean) => void;
}) {
  const loadPercent =
    relay.capacity > 0
      ? Math.round((relay.connections / relay.capacity) * 100)
      : 0;

  return (
    <div
      className="flex items-center justify-between rounded-lg border border-border p-4 cursor-pointer hover:bg-accent/50 transition-colors"
      onClick={onClick}
    >
      <div className="flex items-center gap-4">
        <div
          className={`flex h-10 w-10 items-center justify-center rounded-lg ${
            relay.healthy ? "bg-green-100" : "bg-red-100"
          }`}
        >
          {relay.healthy ? (
            <Wifi className="h-5 w-5 text-green-600" />
          ) : (
            <WifiOff className="h-5 w-5 text-red-600" />
          )}
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{relay.id}</span>
            <Badge variant={relay.healthy ? "success" : "destructive"}>
              {relay.healthy ? "Healthy" : "Unhealthy"}
            </Badge>
            {relay.region && (
              <Badge variant="outline">{relay.region}</Badge>
            )}
          </div>
          <div className="flex items-center gap-3 text-sm text-muted-foreground">
            <span>{relay.url}</span>
          </div>
        </div>
      </div>
      <div className="flex items-center gap-6">
        <div className="text-right">
          <div className="flex items-center gap-2">
            <div className="w-24 h-2 bg-secondary rounded-full overflow-hidden">
              <div
                className={`h-full ${
                  loadPercent > 80
                    ? "bg-red-500"
                    : loadPercent > 50
                    ? "bg-yellow-500"
                    : "bg-green-500"
                }`}
                style={{ width: `${loadPercent}%` }}
              />
            </div>
            <span className="text-sm w-16 text-right">
              {relay.connections}/{relay.capacity}
            </span>
          </div>
          <div className="text-xs text-muted-foreground">
            CPU: {relay.cpu_usage?.toFixed(1) || 0}% | Mem:{" "}
            {relay.memory_usage?.toFixed(1) || 0}%
          </div>
        </div>
        <div className="text-right text-xs text-muted-foreground">
          {relay.last_heartbeat && (
            <p>Last seen {formatRelativeTime(relay.last_heartbeat)}</p>
          )}
        </div>
        <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onUnregister(true)}
            title="Unregister with migration"
          >
            <ArrowRightLeft className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onUnregister(false)}
            title="Force unregister"
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
