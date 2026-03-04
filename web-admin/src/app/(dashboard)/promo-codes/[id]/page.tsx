"use client";

import { use, useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Tag,
  Calendar,
  Users,
  Clock,
  Power,
  PowerOff,
  Trash2,
  Building2,
  User,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  getPromoCode,
  activatePromoCode,
  deactivatePromoCode,
  deletePromoCode,
  listPromoCodeRedemptions,
  PromoCodeType,
  PromoCode,
  PromoCodeRedemption,
} from "@/lib/api/admin";
import type { PaginatedResponse } from "@/lib/api/base";
import { formatDate } from "@/lib/utils";

const typeLabels: Record<PromoCodeType, string> = {
  media: "Media",
  partner: "Partner",
  campaign: "Campaign",
  internal: "Internal",
  referral: "Referral",
};

export default function PromoCodeDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const promoCodeId = parseInt(id, 10);
  const router = useRouter();

  const [promoCode, setPromoCode] = useState<PromoCode | null>(null);
  const [codeLoading, setCodeLoading] = useState(true);
  const [redemptionsData, setRedemptionsData] = useState<PaginatedResponse<PromoCodeRedemption> | null>(null);
  const [redemptionsLoading, setRedemptionsLoading] = useState(true);
  const [isDeleting, setIsDeleting] = useState(false);

  const fetchPromoCode = useCallback(async () => {
    try {
      const result = await getPromoCode(promoCodeId);
      setPromoCode(result);
    } catch {
      // Keep null on error
    } finally {
      setCodeLoading(false);
    }
  }, [promoCodeId]);

  const fetchRedemptions = useCallback(async () => {
    try {
      const result = await listPromoCodeRedemptions(promoCodeId, { page_size: 50 });
      setRedemptionsData(result);
    } catch {
      // Keep null on error
    } finally {
      setRedemptionsLoading(false);
    }
  }, [promoCodeId]);

  useEffect(() => {
    fetchPromoCode();
  }, [fetchPromoCode]);

  useEffect(() => {
    if (promoCode) {
      fetchRedemptions();
    }
  }, [promoCode, fetchRedemptions]);

  const handleActivate = async () => {
    try {
      await activatePromoCode(promoCodeId);
      toast.success("Promo code activated");
      await fetchPromoCode();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to activate promo code");
    }
  };

  const handleDeactivate = async () => {
    try {
      await deactivatePromoCode(promoCodeId);
      toast.success("Promo code deactivated");
      await fetchPromoCode();
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to deactivate promo code");
    }
  };

  const handleDelete = async () => {
    if (
      !promoCode ||
      !confirm(`Are you sure you want to delete "${promoCode.code}"? This action cannot be undone.`)
    ) return;
    setIsDeleting(true);
    try {
      await deletePromoCode(promoCodeId);
      toast.success("Promo code deleted");
      router.push("/promo-codes");
    } catch (err: unknown) {
      toast.error((err as { error?: string })?.error || "Failed to delete promo code");
    } finally {
      setIsDeleting(false);
    }
  };

  if (codeLoading) {
    return (
      <div className="space-y-6">
        <div className="h-8 w-48 animate-pulse rounded bg-muted" />
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i} className="animate-pulse">
              <CardHeader className="pb-2">
                <div className="h-4 w-24 rounded bg-muted" />
              </CardHeader>
              <CardContent>
                <div className="h-8 w-16 rounded bg-muted" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  if (!promoCode) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-4">
        <p className="text-muted-foreground">Promo code not found</p>
        <Link href="/promo-codes">
          <Button variant="outline">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Promo Codes
          </Button>
        </Link>
      </div>
    );
  }

  const redemptions = redemptionsData?.data || [];
  const isExpired = promoCode.expires_at && new Date(promoCode.expires_at) < new Date();
  const remainingUses =
    promoCode.max_uses === null
      ? "Unlimited"
      : `${promoCode.max_uses - promoCode.used_count}`;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <Link href="/promo-codes">
            <Button variant="ghost" size="icon">
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/20">
              <Tag className="h-6 w-6 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="font-mono text-2xl font-bold">{promoCode.code}</h1>
                {promoCode.is_active && !isExpired ? (
                  <Badge variant="success">Active</Badge>
                ) : isExpired ? (
                  <Badge variant="destructive">Expired</Badge>
                ) : (
                  <Badge variant="secondary">Inactive</Badge>
                )}
              </div>
              <p className="text-sm text-muted-foreground">{promoCode.name}</p>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {promoCode.is_active ? (
            <Button
              variant="outline"
              onClick={handleDeactivate}
            >
              <PowerOff className="mr-2 h-4 w-4" />
              Deactivate
            </Button>
          ) : (
            <Button
              variant="outline"
              onClick={handleActivate}
            >
              <Power className="mr-2 h-4 w-4" />
              Activate
            </Button>
          )}
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={isDeleting}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            Delete
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Type
            </CardTitle>
            <Tag className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-lg font-semibold capitalize">
              {typeLabels[promoCode.type]}
            </div>
            <p className="text-xs text-muted-foreground">
              Plan: {promoCode.plan_name}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Duration
            </CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{promoCode.duration_months}</div>
            <p className="text-xs text-muted-foreground">months</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Uses
            </CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{promoCode.used_count}</div>
            <p className="text-xs text-muted-foreground">
              {remainingUses} remaining
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Expires
            </CardTitle>
            <Calendar className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className={`text-sm font-medium ${isExpired ? "text-destructive" : ""}`}>
              {promoCode.expires_at ? formatDate(promoCode.expires_at) : "Never"}
            </div>
            <p className="text-xs text-muted-foreground">
              Created {formatDate(promoCode.created_at)}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Description */}
      {promoCode.description && (
        <Card>
          <CardHeader>
            <CardTitle>Description</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-muted-foreground">{promoCode.description}</p>
          </CardContent>
        </Card>
      )}

      {/* Usage Details */}
      <Card>
        <CardHeader>
          <CardTitle>Usage Limits</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <p className="text-sm text-muted-foreground">Max Total Uses</p>
              <p className="font-medium">
                {promoCode.max_uses === null ? "Unlimited" : promoCode.max_uses}
              </p>
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Max Uses per Organization</p>
              <p className="font-medium">{promoCode.max_uses_per_org}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Redemptions */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Users className="h-5 w-5" />
            Redemptions ({redemptions.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {redemptionsLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-14 animate-pulse rounded-lg bg-muted" />
              ))}
            </div>
          ) : redemptions.length === 0 ? (
            <p className="py-4 text-center text-muted-foreground">
              No redemptions yet
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Organization</TableHead>
                  <TableHead>Plan</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Redeemed At</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {redemptions.map((redemption) => (
                  <TableRow key={redemption.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {redemption.user?.avatar_url ? (
                          <img
                            src={redemption.user.avatar_url}
                            alt={redemption.user.username}
                            className="h-8 w-8 rounded-full"
                          />
                        ) : (
                          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20">
                            <User className="h-4 w-4 text-primary" />
                          </div>
                        )}
                        <div>
                          <p className="font-medium">
                            {redemption.user?.name || redemption.user?.username || "Unknown"}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {redemption.user?.email}
                          </p>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Building2 className="h-4 w-4 text-muted-foreground" />
                        {redemption.organization?.name || "Unknown"}
                      </div>
                    </TableCell>
                    <TableCell className="capitalize">{redemption.plan_name}</TableCell>
                    <TableCell>{redemption.duration_months} months</TableCell>
                    <TableCell>{formatDate(redemption.created_at)}</TableCell>
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
