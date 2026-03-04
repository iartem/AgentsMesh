"use client";

import { useState, useEffect, useCallback } from "react";
import {
  CreditCard,
  Snowflake,
  Play,
  XCircle,
  RefreshCw,
  Settings,
  Plus,
  AlertTriangle,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  getOrganizationSubscription,
  getSubscriptionPlans,
  createSubscription,
  updateSubscriptionPlan,
  updateSubscriptionSeats,
  updateSubscriptionCycle,
  freezeSubscription,
  unfreezeSubscription,
  cancelSubscription,
  renewSubscription,
  setSubscriptionAutoRenew,
  setSubscriptionQuota,
} from "@/lib/api/admin";
import type { SubscriptionPlan } from "@/lib/api/admin";
import { formatDate } from "@/lib/utils";

const QUOTA_RESOURCES = ["users", "runners", "concurrent_pods", "repositories", "pod_minutes"];

function statusVariant(status: string) {
  switch (status) {
    case "active":
      return "success" as const;
    case "frozen":
      return "destructive" as const;
    case "trialing":
      return "default" as const;
    case "canceled":
    case "expired":
      return "secondary" as const;
    default:
      return "outline" as const;
  }
}

function getPlanLimit(plan: SubscriptionPlan, resource: string): number {
  switch (resource) {
    case "users": return plan.max_users;
    case "runners": return plan.max_runners;
    case "concurrent_pods": return plan.max_concurrent_pods;
    case "repositories": return plan.max_repositories;
    case "pod_minutes": return plan.included_pod_minutes;
    default: return 0;
  }
}

export function SubscriptionSection({ orgId }: { orgId: number }) {
  const [renewMonths, setRenewMonths] = useState("1");
  const [newSeatCount, setNewSeatCount] = useState("");
  const [quotaResource, setQuotaResource] = useState("users");
  const [quotaLimit, setQuotaLimit] = useState("");

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [sub, setSub] = useState<any>(null);
  const [subLoading, setSubLoading] = useState(true);
  const [plans, setPlans] = useState<SubscriptionPlan[]>([]);

  // Loading states for various actions
  const [changePlanPending, setChangePlanPending] = useState(false);
  const [changeSeatsPending, setChangeSeatsPending] = useState(false);
  const [changeCyclePending, setChangeCyclePending] = useState(false);
  const [freezePending, setFreezePending] = useState(false);
  const [unfreezePending, setUnfreezePending] = useState(false);
  const [cancelPending, setCancelPending] = useState(false);
  const [renewPending, setRenewPending] = useState(false);
  const [autoRenewPending, setAutoRenewPending] = useState(false);
  const [quotaPending, setQuotaPending] = useState(false);

  const fetchSubscription = useCallback(async () => {
    try {
      const result = await getOrganizationSubscription(orgId);
      setSub(result);
    } catch (err: unknown) {
      // Return null for 404 (no subscription) instead of throwing
      if (err && typeof err === "object" && "status" in err && (err as { status: number }).status === 404) {
        setSub(null);
      }
    } finally {
      setSubLoading(false);
    }
  }, [orgId]);

  const fetchPlans = useCallback(async () => {
    try {
      const result = await getSubscriptionPlans(orgId);
      setPlans(result?.data || []);
    } catch {
      // Keep empty plans on error
    }
  }, [orgId]);

  useEffect(() => {
    fetchSubscription();
    fetchPlans();
  }, [fetchSubscription, fetchPlans]);

  const refreshData = async () => {
    await fetchSubscription();
  };

  const handleChangePlan = async (planName: string) => {
    if (!confirm(`Change plan to "${planName}"? This takes effect immediately.`)) return;
    setChangePlanPending(true);
    try {
      await updateSubscriptionPlan(orgId, planName);
      toast.success("Plan updated");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to update plan");
    } finally {
      setChangePlanPending(false);
    }
  };

  const handleChangeSeats = async (count: number) => {
    setChangeSeatsPending(true);
    try {
      await updateSubscriptionSeats(orgId, count);
      setNewSeatCount("");
      toast.success("Seats updated");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to update seats");
    } finally {
      setChangeSeatsPending(false);
    }
  };

  const handleChangeCycle = async () => {
    const newCycle = sub.billing_cycle === "monthly" ? "yearly" : "monthly";
    setChangeCyclePending(true);
    try {
      await updateSubscriptionCycle(orgId, newCycle);
      toast.success("Billing cycle updated");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to update cycle");
    } finally {
      setChangeCyclePending(false);
    }
  };

  const handleFreeze = async () => {
    if (!confirm("Freeze this subscription? Users will lose access to restricted resources.")) return;
    setFreezePending(true);
    try {
      await freezeSubscription(orgId);
      toast.success("Subscription frozen");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to freeze");
    } finally {
      setFreezePending(false);
    }
  };

  const handleUnfreeze = async () => {
    setUnfreezePending(true);
    try {
      await unfreezeSubscription(orgId);
      toast.success("Subscription unfrozen");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to unfreeze");
    } finally {
      setUnfreezePending(false);
    }
  };

  const handleCancel = async () => {
    if (!confirm("Cancel this subscription? This will not call external payment APIs.")) return;
    setCancelPending(true);
    try {
      await cancelSubscription(orgId);
      toast.success("Subscription canceled");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to cancel");
    } finally {
      setCancelPending(false);
    }
  };

  const handleRenew = async (months: number) => {
    if (!confirm(`Renew subscription for ${months} month(s)?`)) return;
    setRenewPending(true);
    try {
      await renewSubscription(orgId, months);
      toast.success("Subscription renewed");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to renew");
    } finally {
      setRenewPending(false);
    }
  };

  const handleToggleAutoRenew = async () => {
    setAutoRenewPending(true);
    try {
      await setSubscriptionAutoRenew(orgId, !sub.auto_renew);
      toast.success("Auto-renew updated");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to update auto-renew");
    } finally {
      setAutoRenewPending(false);
    }
  };

  const handleSetQuota = async (resource: string, limit: number) => {
    setQuotaPending(true);
    try {
      await setSubscriptionQuota(orgId, resource, limit);
      setQuotaLimit("");
      toast.success("Quota updated");
      await refreshData();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to set quota");
    } finally {
      setQuotaPending(false);
    }
  };

  if (subLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CreditCard className="h-5 w-5" />
            Subscription
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="h-10 animate-pulse rounded bg-muted" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  if (!sub) {
    return (
      <NoSubscriptionPanel
        plans={plans}
        orgId={orgId}
        onCreated={refreshData}
      />
    );
  }

  const seatUsage = sub.seat_usage;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <CreditCard className="h-5 w-5" />
          Subscription
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* Plan & Status + Billing Details */}
        <div className="grid gap-4 md:grid-cols-2">
          <PlanStatusPanel
            sub={sub}
            plans={plans}
            onChangePlan={(v) => {
              if (v !== sub.plan?.name) handleChangePlan(v);
            }}
          />
          <BillingDetailsPanel
            sub={sub}
            onToggleCycle={handleChangeCycle}
            onToggleAutoRenew={handleToggleAutoRenew}
            cyclePending={changeCyclePending}
            autoRenewPending={autoRenewPending}
          />
        </div>

        {/* Seats + Actions */}
        <div className="grid gap-4 md:grid-cols-2">
          <SeatsPanel
            seatUsage={seatUsage}
            newSeatCount={newSeatCount}
            onNewSeatCountChange={setNewSeatCount}
            onSetSeats={(count) => handleChangeSeats(count)}
            seatsPending={changeSeatsPending}
          />
          <ActionsPanel
            status={sub.status}
            renewMonths={renewMonths}
            onRenewMonthsChange={setRenewMonths}
            onFreeze={handleFreeze}
            onUnfreeze={handleUnfreeze}
            onCancel={handleCancel}
            onRenew={handleRenew}
            freezePending={freezePending}
            unfreezePending={unfreezePending}
            cancelPending={cancelPending}
            renewPending={renewPending}
          />
        </div>

        {/* Custom Quotas */}
        <CustomQuotasPanel
          plan={sub.plan}
          customQuotas={sub.custom_quotas}
          quotaResource={quotaResource}
          quotaLimit={quotaLimit}
          onQuotaResourceChange={setQuotaResource}
          onQuotaLimitChange={setQuotaLimit}
          onSetQuota={(resource, limit) => handleSetQuota(resource, limit)}
          quotaPending={quotaPending}
        />

        {/* Payment Provider Info */}
        {(sub.has_stripe || sub.has_alipay || sub.has_wechat || sub.has_lemonsqueezy) && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span>Payment Integrations:</span>
            {sub.has_stripe && <Badge variant="outline" className="text-xs">Stripe</Badge>}
            {sub.has_alipay && <Badge variant="outline" className="text-xs">Alipay</Badge>}
            {sub.has_wechat && <Badge variant="outline" className="text-xs">WeChat</Badge>}
            {sub.has_lemonsqueezy && <Badge variant="outline" className="text-xs">LemonSqueezy</Badge>}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ─── Sub-panels ──────────────────────────────────────────────────────────────

function PlanStatusPanel({
  sub,
  plans,
  onChangePlan,
}: {
  sub: { plan?: SubscriptionPlan; status: string; payment_provider?: string; downgrade_to_plan?: string };
  plans: SubscriptionPlan[];
  onChangePlan: (planName: string) => void;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <h3 className="text-sm font-semibold text-muted-foreground">Plan & Status</h3>
      <div className="flex items-center justify-between">
        <span className="text-sm">Plan</span>
        <div className="flex items-center gap-2">
          <span className="font-medium capitalize">{sub.plan?.display_name || sub.plan?.name || "-"}</span>
          <Select
            value={sub.plan?.name || ""}
            onValueChange={(v) => { if (v !== sub.plan?.name) onChangePlan(v); }}
          >
            <SelectTrigger className="h-7 w-auto min-w-[100px] text-xs">
              <SelectValue placeholder="Change..." />
            </SelectTrigger>
            <SelectContent>
              {plans.map((p) => (
                <SelectItem key={p.name} value={p.name}>
                  {p.display_name || p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-sm">Status</span>
        <Badge variant={statusVariant(sub.status)}>{sub.status}</Badge>
      </div>
      {sub.payment_provider && (
        <div className="flex items-center justify-between">
          <span className="text-sm">Provider</span>
          <span className="text-sm text-muted-foreground">{sub.payment_provider}</span>
        </div>
      )}
      {sub.downgrade_to_plan && (
        <div className="flex items-center justify-between">
          <span className="text-sm">Pending Downgrade</span>
          <Badge variant="warning">{sub.downgrade_to_plan}</Badge>
        </div>
      )}
    </div>
  );
}

function BillingDetailsPanel({
  sub,
  onToggleCycle,
  onToggleAutoRenew,
  cyclePending,
  autoRenewPending,
}: {
  sub: { billing_cycle: string; auto_renew: boolean; current_period_start: string; current_period_end: string; next_billing_cycle?: string };
  onToggleCycle: () => void;
  onToggleAutoRenew: () => void;
  cyclePending: boolean;
  autoRenewPending: boolean;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <h3 className="text-sm font-semibold text-muted-foreground">Billing Details</h3>
      <div className="flex items-center justify-between">
        <span className="text-sm">Cycle</span>
        <div className="flex items-center gap-2">
          <span className="text-sm capitalize">{sub.billing_cycle}</span>
          <Button variant="outline" size="sm" className="h-7 text-xs" disabled={cyclePending} onClick={onToggleCycle}>
            → {sub.billing_cycle === "monthly" ? "Yearly" : "Monthly"}
          </Button>
        </div>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-sm">Auto-Renew</span>
        <Button
          variant={sub.auto_renew ? "default" : "outline"}
          size="sm"
          className="h-7 text-xs"
          disabled={autoRenewPending}
          onClick={onToggleAutoRenew}
        >
          {sub.auto_renew ? "On" : "Off"}
        </Button>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-sm">Period</span>
        <span className="text-xs text-muted-foreground">
          {formatDate(sub.current_period_start)} — {formatDate(sub.current_period_end)}
        </span>
      </div>
      {sub.next_billing_cycle && (
        <div className="flex items-center justify-between">
          <span className="text-sm">Next Cycle</span>
          <span className="text-sm capitalize text-muted-foreground">{sub.next_billing_cycle}</span>
        </div>
      )}
    </div>
  );
}

function SeatsPanel({
  seatUsage,
  newSeatCount,
  onNewSeatCountChange,
  onSetSeats,
  seatsPending,
}: {
  seatUsage?: { total_seats: number; used_seats: number; available_seats: number };
  newSeatCount: string;
  onNewSeatCountChange: (v: string) => void;
  onSetSeats: (count: number) => void;
  seatsPending: boolean;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <h3 className="text-sm font-semibold text-muted-foreground">Seats</h3>
      {seatUsage ? (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-sm">
            <span>{seatUsage.used_seats}/{seatUsage.total_seats} used</span>
            <span className="text-muted-foreground">{seatUsage.available_seats} available</span>
          </div>
          <div className="h-2 rounded-full bg-muted">
            <div
              className="h-2 rounded-full bg-primary transition-all"
              style={{ width: `${seatUsage.total_seats > 0 ? Math.min((seatUsage.used_seats / seatUsage.total_seats) * 100, 100) : 0}%` }}
            />
          </div>
          <div className="flex items-center gap-2">
            <Input
              type="number"
              min={1}
              placeholder="New count"
              value={newSeatCount}
              onChange={(e) => onNewSeatCountChange(e.target.value)}
              className="h-8 w-24 text-sm"
            />
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs"
              disabled={!newSeatCount || seatsPending}
              onClick={() => {
                const count = parseInt(newSeatCount);
                if (count > 0) onSetSeats(count);
              }}
            >
              Set Seats
            </Button>
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">No seat data</p>
      )}
    </div>
  );
}

function ActionsPanel({
  status,
  renewMonths,
  onRenewMonthsChange,
  onFreeze,
  onUnfreeze,
  onCancel,
  onRenew,
  freezePending,
  unfreezePending,
  cancelPending,
  renewPending,
}: {
  status: string;
  renewMonths: string;
  onRenewMonthsChange: (v: string) => void;
  onFreeze: () => void;
  onUnfreeze: () => void;
  onCancel: () => void;
  onRenew: (months: number) => void;
  freezePending: boolean;
  unfreezePending: boolean;
  cancelPending: boolean;
  renewPending: boolean;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <h3 className="text-sm font-semibold text-muted-foreground">Actions</h3>
      <div className="flex flex-wrap gap-2">
        {status !== "frozen" ? (
          <Button variant="outline" size="sm" disabled={freezePending} onClick={onFreeze}>
            <Snowflake className="mr-1.5 h-3.5 w-3.5" />
            Freeze
          </Button>
        ) : (
          <Button variant="outline" size="sm" disabled={unfreezePending} onClick={onUnfreeze}>
            <Play className="mr-1.5 h-3.5 w-3.5" />
            Unfreeze
          </Button>
        )}
        {status !== "canceled" && (
          <Button variant="destructive" size="sm" disabled={cancelPending} onClick={onCancel}>
            <XCircle className="mr-1.5 h-3.5 w-3.5" />
            Cancel
          </Button>
        )}
      </div>
      <div className="flex items-center gap-2 pt-1">
        <Input
          type="number"
          min={1}
          max={120}
          value={renewMonths}
          onChange={(e) => onRenewMonthsChange(e.target.value)}
          className="h-8 w-20 text-sm"
        />
        <span className="text-xs text-muted-foreground">months</span>
        <Button
          variant="outline"
          size="sm"
          className="h-8"
          disabled={renewPending}
          onClick={() => {
            const months = parseInt(renewMonths);
            if (months > 0 && months <= 120) onRenew(months);
          }}
        >
          <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
          Renew
        </Button>
      </div>
    </div>
  );
}

function CustomQuotasPanel({
  plan,
  customQuotas,
  quotaResource,
  quotaLimit,
  onQuotaResourceChange,
  onQuotaLimitChange,
  onSetQuota,
  quotaPending,
}: {
  plan?: SubscriptionPlan;
  customQuotas: Record<string, number> | null;
  quotaResource: string;
  quotaLimit: string;
  onQuotaResourceChange: (v: string) => void;
  onQuotaLimitChange: (v: string) => void;
  onSetQuota: (resource: string, limit: number) => void;
  quotaPending: boolean;
}) {
  return (
    <div className="space-y-3 rounded-lg border border-border p-4">
      <h3 className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
        <Settings className="h-4 w-4" />
        Custom Quotas
      </h3>

      {plan && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-muted-foreground">
                <th className="pb-2 pr-4 font-medium">Resource</th>
                <th className="pb-2 pr-4 font-medium">Plan Limit</th>
                <th className="pb-2 pr-4 font-medium">Custom Override</th>
              </tr>
            </thead>
            <tbody>
              {QUOTA_RESOURCES.map((res) => {
                const planLimit = getPlanLimit(plan, res);
                const customVal = customQuotas?.[res];
                return (
                  <tr key={res} className="border-b border-border/50">
                    <td className="py-2 pr-4 font-mono text-xs">{res}</td>
                    <td className="py-2 pr-4 text-muted-foreground">
                      {planLimit === -1 ? "∞" : planLimit}
                    </td>
                    <td className="py-2 pr-4">
                      {customVal != null ? (
                        <Badge variant="outline" className="font-mono">
                          {Number(customVal) === -1 ? "∞" : String(customVal)}
                        </Badge>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex items-center gap-2 pt-1">
        <Select value={quotaResource} onValueChange={onQuotaResourceChange}>
          <SelectTrigger className="h-8 w-40 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {QUOTA_RESOURCES.map((r) => (
              <SelectItem key={r} value={r}>
                {r}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          type="number"
          placeholder="Limit (-1=∞)"
          value={quotaLimit}
          onChange={(e) => onQuotaLimitChange(e.target.value)}
          className="h-8 w-28 text-sm"
        />
        <Button
          variant="outline"
          size="sm"
          className="h-8 text-xs"
          disabled={quotaLimit === "" || quotaPending}
          onClick={() => {
            const limit = parseInt(quotaLimit);
            if (!isNaN(limit)) onSetQuota(quotaResource, limit);
          }}
        >
          Set Override
        </Button>
      </div>
    </div>
  );
}

// ─── No Subscription Panel ──────────────────────────────────────────────────

function NoSubscriptionPanel({
  plans,
  orgId,
  onCreated,
}: {
  plans: SubscriptionPlan[];
  orgId: number;
  onCreated: () => void;
}) {
  const [selectedPlan, setSelectedPlan] = useState(plans[0]?.name || "based");
  const [months, setMonths] = useState("1");
  const [isCreating, setIsCreating] = useState(false);

  const handleCreate = async () => {
    const m = parseInt(months);
    if (m <= 0 || m > 120) return;
    if (!confirm(`Create an active "${selectedPlan}" subscription for ${m} month(s)?`)) return;
    setIsCreating(true);
    try {
      await createSubscription(orgId, selectedPlan, m);
      toast.success("Subscription created");
      onCreated();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to create subscription");
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <CreditCard className="h-5 w-5" />
          Subscription
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div className="flex items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-900 dark:bg-amber-950">
            <AlertTriangle className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
            <div>
              <p className="text-sm font-medium text-amber-800 dark:text-amber-200">
                No subscription record found
              </p>
              <p className="text-xs text-amber-600 dark:text-amber-400">
                This organization is missing a subscription record. Use the form below to create one.
              </p>
            </div>
          </div>

          <div className="space-y-3 rounded-lg border border-border p-4">
            <h3 className="text-sm font-semibold text-muted-foreground">Create Subscription</h3>
            <div className="flex items-center justify-between">
              <span className="text-sm">Plan</span>
              <Select value={selectedPlan} onValueChange={setSelectedPlan}>
                <SelectTrigger className="h-8 w-40 text-sm">
                  <SelectValue placeholder="Select plan" />
                </SelectTrigger>
                <SelectContent>
                  {plans.map((p) => (
                    <SelectItem key={p.name} value={p.name}>
                      {p.display_name || p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm">Duration</span>
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={1}
                  max={120}
                  value={months}
                  onChange={(e) => setMonths(e.target.value)}
                  className="h-8 w-20 text-sm"
                />
                <span className="text-xs text-muted-foreground">months</span>
              </div>
            </div>
            <Button
              className="w-full"
              disabled={isCreating || !selectedPlan}
              onClick={handleCreate}
            >
              <Plus className="mr-2 h-4 w-4" />
              Create Subscription
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
