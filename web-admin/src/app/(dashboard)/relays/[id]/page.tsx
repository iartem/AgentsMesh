"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter, useParams } from "next/navigation";
import {
  ArrowLeft,
  Radio,
  Wifi,
  WifiOff,
  ArrowRightLeft,
  Trash2,
  RefreshCw,
  Clock,
  Activity,
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  getRelay,
  listRelays,
  forceUnregisterRelay,
  migrateSession,
  ActiveSession,
  RelayDetailResponse,
  RelayListResponse,
} from "@/lib/api/admin";
import { formatRelativeTime } from "@/lib/utils";

export default function RelayDetailPage() {
  const router = useRouter();
  const params = useParams();
  const relayId = decodeURIComponent(params.id as string);
  const [migratingPod, setMigratingPod] = useState<string | null>(null);
  const [targetRelay, setTargetRelay] = useState<string>("");
  const [isUnregistering, setIsUnregistering] = useState(false);
  const [isMigratingSession, setIsMigratingSession] = useState(false);

  const [data, setData] = useState<RelayDetailResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [relaysData, setRelaysData] = useState<RelayListResponse | null>(null);

  const fetchRelay = useCallback(async () => {
    try {
      const result = await getRelay(relayId);
      setData(result);
      setError(null);
    } catch (err) {
      setError(err);
    } finally {
      setIsLoading(false);
    }
  }, [relayId]);

  const fetchRelays = useCallback(async () => {
    try {
      const result = await listRelays();
      setRelaysData(result);
    } catch {
      // Non-critical
    }
  }, []);

  useEffect(() => {
    fetchRelay();
    fetchRelays();
    const interval = setInterval(fetchRelay, 5000);
    return () => clearInterval(interval);
  }, [fetchRelay, fetchRelays]);

  const handleUnregister = async (migrate: boolean) => {
    const msg = migrate
      ? `Unregister relay "${relayId}" and migrate all sessions?`
      : `Unregister relay "${relayId}"? ${data?.session_count || 0} sessions will be affected.`;
    if (!confirm(msg)) return;
    setIsUnregistering(true);
    try {
      await forceUnregisterRelay(relayId, migrate);
      toast.success("Relay unregistered");
      router.push("/relays");
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to unregister relay");
    } finally {
      setIsUnregistering(false);
    }
  };

  const handleMigrate = async (session: ActiveSession) => {
    if (migratingPod === session.pod_key) {
      if (!targetRelay) {
        toast.error("Please select a target relay");
        return;
      }
      setIsMigratingSession(true);
      try {
        const result = await migrateSession(session.pod_key, targetRelay);
        toast.success(`Session migrated from ${result.from_relay} to ${result.to_relay}`);
        setMigratingPod(null);
        setTargetRelay("");
        await fetchRelay();
      } catch (err: unknown) {
        toast.error((err as { error?: string })?.error || "Failed to migrate session");
      } finally {
        setIsMigratingSession(false);
      }
    } else {
      setMigratingPod(session.pod_key);
      setTargetRelay("");
    }
  };

  const healthyRelays =
    relaysData?.data.filter((r) => r.healthy && r.id !== relayId) || [];

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 animate-pulse rounded bg-muted" />
        <div className="h-32 animate-pulse rounded-lg bg-muted" />
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => router.push("/relays")}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Relays
        </Button>
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground">
            Relay not found or has been unregistered.
          </CardContent>
        </Card>
      </div>
    );
  }

  const { relay, sessions } = data;
  const loadPercent =
    relay.capacity > 0
      ? Math.round((relay.connections / relay.capacity) * 100)
      : 0;

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" onClick={() => router.push("/relays")}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
          <div className="flex items-center gap-2">
            <Radio className="h-5 w-5" />
            <h1 className="text-xl font-semibold">{relay.id}</h1>
            <Badge variant={relay.healthy ? "success" : "destructive"}>
              {relay.healthy ? "Healthy" : "Unhealthy"}
            </Badge>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            onClick={() => handleUnregister(true)}
            disabled={isUnregistering || healthyRelays.length === 0}
          >
            <ArrowRightLeft className="mr-2 h-4 w-4" />
            Unregister & Migrate
          </Button>
          <Button
            variant="destructive"
            onClick={() => handleUnregister(false)}
            disabled={isUnregistering}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            Force Unregister
          </Button>
        </div>
      </div>

      {/* Relay Info */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Connection Info</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Status</span>
              <div className="flex items-center gap-2">
                {relay.healthy ? (
                  <Wifi className="h-4 w-4 text-green-500" />
                ) : (
                  <WifiOff className="h-4 w-4 text-red-500" />
                )}
                <span>{relay.healthy ? "Online" : "Offline"}</span>
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">URL</span>
              <span className="text-sm font-mono">{relay.url}</span>
            </div>
            {relay.internal_url && (
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Internal URL</span>
                <span className="text-sm font-mono">{relay.internal_url}</span>
              </div>
            )}
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Region</span>
              <Badge variant="outline">{relay.region || "default"}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Last Heartbeat</span>
              <span className="text-sm">
                {relay.last_heartbeat
                  ? formatRelativeTime(relay.last_heartbeat)
                  : "Never"}
              </span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Load & Resources</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-muted-foreground">Connections</span>
                <span className="text-sm">
                  {relay.connections} / {relay.capacity}
                </span>
              </div>
              <div className="w-full h-2 bg-secondary rounded-full overflow-hidden">
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
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">CPU Usage</span>
              <span className="text-sm">{relay.cpu_usage?.toFixed(1) || 0}%</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Memory Usage</span>
              <span className="text-sm">{relay.memory_usage?.toFixed(1) || 0}%</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Active Sessions</span>
              <span className="text-sm font-medium">{data.session_count}</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Sessions Table */}
      <Card>
        <CardHeader>
          <CardTitle>Active Sessions ({sessions?.length || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {!sessions || sessions.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">
              <Activity className="mx-auto mb-2 h-8 w-8" />
              <p>No active sessions on this relay</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Pod Key</TableHead>
                  <TableHead>Session ID</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map((session) => (
                  <TableRow key={session.pod_key}>
                    <TableCell className="font-mono text-sm">
                      {session.pod_key}
                    </TableCell>
                    <TableCell className="font-mono text-sm">
                      {session.session_id}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      <div className="flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        {formatRelativeTime(session.created_at)}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatRelativeTime(session.expire_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      {migratingPod === session.pod_key ? (
                        <div className="flex items-center justify-end gap-2">
                          <Select
                            value={targetRelay}
                            onValueChange={setTargetRelay}
                          >
                            <SelectTrigger className="w-40">
                              <SelectValue placeholder="Target relay" />
                            </SelectTrigger>
                            <SelectContent>
                              {healthyRelays.map((r) => (
                                <SelectItem key={r.id} value={r.id}>
                                  {r.id}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <Button
                            size="sm"
                            onClick={() => handleMigrate(session)}
                            disabled={isMigratingSession || !targetRelay}
                          >
                            {isMigratingSession ? (
                              <RefreshCw className="h-4 w-4 animate-spin" />
                            ) : (
                              "Confirm"
                            )}
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => {
                              setMigratingPod(null);
                              setTargetRelay("");
                            }}
                          >
                            Cancel
                          </Button>
                        </div>
                      ) : (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleMigrate(session)}
                          disabled={healthyRelays.length === 0}
                          title={
                            healthyRelays.length === 0
                              ? "No other healthy relays available"
                              : "Migrate to another relay"
                          }
                        >
                          <ArrowRightLeft className="h-4 w-4" />
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
