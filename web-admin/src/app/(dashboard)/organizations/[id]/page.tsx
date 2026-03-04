"use client";

import { use, useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Users, Calendar, Server, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  getOrganization,
  getOrganizationMembers,
  deleteOrganization,
  listRunners,
  Organization,
  OrganizationMember,
  Runner,
} from "@/lib/api/admin";
import type { PaginatedResponse } from "@/lib/api/base";
import { formatDate } from "@/lib/utils";
import { SubscriptionSection } from "./_components/subscription-section";
import { MembersSection } from "./_components/members-section";
import { RunnersSection } from "./_components/runners-section";

export default function OrganizationDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const orgId = parseInt(id, 10);
  const router = useRouter();

  const [org, setOrg] = useState<Organization | null>(null);
  const [orgLoading, setOrgLoading] = useState(true);
  const [membersData, setMembersData] = useState<{ organization: Organization; members: OrganizationMember[] } | null>(null);
  const [membersLoading, setMembersLoading] = useState(true);
  const [runnersData, setRunnersData] = useState<PaginatedResponse<Runner> | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const fetchOrg = useCallback(async () => {
    try {
      const result = await getOrganization(orgId);
      setOrg(result);
    } catch {
      // Keep null on error
    } finally {
      setOrgLoading(false);
    }
  }, [orgId]);

  const fetchMembers = useCallback(async () => {
    try {
      const result = await getOrganizationMembers(orgId);
      setMembersData(result);
    } catch {
      // Keep null on error
    } finally {
      setMembersLoading(false);
    }
  }, [orgId]);

  const fetchRunners = useCallback(async () => {
    try {
      const result = await listRunners({ org_id: orgId, page_size: 100 });
      setRunnersData(result);
    } catch {
      // Keep null on error
    }
  }, [orgId]);

  useEffect(() => {
    fetchOrg();
    fetchMembers();
    fetchRunners();
  }, [fetchOrg, fetchMembers, fetchRunners]);

  const handleDelete = async () => {
    if (
      !org ||
      !confirm(
        `Are you sure you want to delete "${org.name}"? This action cannot be undone.`
      )
    ) {
      return;
    }
    setIsDeleting(true);
    try {
      await deleteOrganization(orgId);
      toast.success("Organization deleted successfully");
      router.push("/organizations");
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to delete organization";
      toast.error(message);
    } finally {
      setIsDeleting(false);
    }
  };

  if (orgLoading) {
    return (
      <div className="space-y-6">
        <div className="h-8 w-48 animate-pulse rounded bg-muted" />
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
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

  if (!org) {
    return (
      <div className="flex h-64 flex-col items-center justify-center gap-4">
        <p className="text-muted-foreground">Organization not found</p>
        <Link href="/organizations">
          <Button variant="outline">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Organizations
          </Button>
        </Link>
      </div>
    );
  }

  const members = membersData?.members || [];
  const runners = runnersData?.data || [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <Link href="/organizations">
            <Button variant="ghost" size="icon">
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div className="flex items-center gap-4">
            {org.logo_url ? (
              <img
                src={org.logo_url}
                alt={org.name}
                className="h-12 w-12 rounded-lg object-cover"
              />
            ) : (
              <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/20 text-lg font-medium text-primary">
                {org.name[0].toUpperCase()}
              </div>
            )}
            <div>
              <h1 className="text-2xl font-bold">{org.name}</h1>
              <p className="text-sm text-muted-foreground">{org.slug}</p>
            </div>
          </div>
        </div>
        <Button
          variant="destructive"
          onClick={handleDelete}
          disabled={isDeleting}
        >
          <Trash2 className="mr-2 h-4 w-4" />
          Delete Organization
        </Button>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Members
            </CardTitle>
            <Users className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{members.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Runners
            </CardTitle>
            <Server className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{runners.length}</div>
            <p className="text-xs text-muted-foreground">
              {runners.filter((r) => r.status === "online").length} online
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Created
            </CardTitle>
            <Calendar className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-sm font-medium">{formatDate(org.created_at)}</div>
          </CardContent>
        </Card>
      </div>

      {/* Subscription */}
      <SubscriptionSection orgId={orgId} />

      {/* Members */}
      <MembersSection members={members} isLoading={membersLoading} />

      {/* Runners */}
      <RunnersSection runners={runners} />
    </div>
  );
}
