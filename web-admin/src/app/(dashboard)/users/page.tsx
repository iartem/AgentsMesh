"use client";

import { useState, useEffect, useCallback } from "react";
import { Search, Shield, ShieldOff, UserX, UserCheck, MailCheck, MailX } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  listUsers,
  disableUser,
  enableUser,
  grantAdmin,
  revokeAdmin,
  verifyUserEmail,
  unverifyUserEmail,
  User,
} from "@/lib/api/admin";
import type { PaginatedResponse } from "@/lib/api/base";
import { formatDate, formatRelativeTime } from "@/lib/utils";

export default function UsersPage() {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<PaginatedResponse<User> | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const fetchUsers = useCallback(async () => {
    setIsLoading(true);
    try {
      const result = await listUsers({ search, page, page_size: 20 });
      setData(result);
    } catch {
      // Keep previous data on error
    } finally {
      setIsLoading(false);
    }
  }, [search, page]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleDisable = async (userId: number) => {
    try {
      await disableUser(userId);
      toast.success("User disabled successfully");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to disable user";
      toast.error(message);
    }
  };

  const handleEnable = async (userId: number) => {
    try {
      await enableUser(userId);
      toast.success("User enabled successfully");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to enable user";
      toast.error(message);
    }
  };

  const handleGrantAdmin = async (userId: number) => {
    try {
      await grantAdmin(userId);
      toast.success("Admin privileges granted");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to grant admin privileges";
      toast.error(message);
    }
  };

  const handleRevokeAdmin = async (userId: number) => {
    try {
      await revokeAdmin(userId);
      toast.success("Admin privileges revoked");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to revoke admin privileges";
      toast.error(message);
    }
  };

  const handleVerifyEmail = async (userId: number) => {
    try {
      await verifyUserEmail(userId);
      toast.success("Email verified successfully");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to verify email";
      toast.error(message);
    }
  };

  const handleUnverifyEmail = async (userId: number) => {
    try {
      await unverifyUserEmail(userId);
      toast.success("Email unverified successfully");
      await fetchUsers();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to unverify email";
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
            placeholder="Search users..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            className="pl-9"
          />
        </div>
      </div>

      {/* Users Table */}
      <Card>
        <CardHeader>
          <CardTitle>Users ({data?.total || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-16 animate-pulse rounded-lg bg-muted" />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              {data?.data.map((user) => (
                <UserRow
                  key={user.id}
                  user={user}
                  onDisable={() => handleDisable(user.id)}
                  onEnable={() => handleEnable(user.id)}
                  onGrantAdmin={() => handleGrantAdmin(user.id)}
                  onRevokeAdmin={() => handleRevokeAdmin(user.id)}
                  onVerifyEmail={() => handleVerifyEmail(user.id)}
                  onUnverifyEmail={() => handleUnverifyEmail(user.id)}
                />
              ))}
              {data?.data.length === 0 && (
                <p className="py-8 text-center text-muted-foreground">
                  No users found
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

function UserRow({
  user,
  onDisable,
  onEnable,
  onGrantAdmin,
  onRevokeAdmin,
  onVerifyEmail,
  onUnverifyEmail,
}: {
  user: User;
  onDisable: () => void;
  onEnable: () => void;
  onGrantAdmin: () => void;
  onRevokeAdmin: () => void;
  onVerifyEmail: () => void;
  onUnverifyEmail: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-4">
        {user.avatar_url ? (
          <img
            src={user.avatar_url}
            alt={user.username}
            className="h-10 w-10 rounded-full"
          />
        ) : (
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/20 text-sm font-medium text-primary">
            {user.username[0].toUpperCase()}
          </div>
        )}
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{user.name || user.username}</span>
            {user.is_system_admin && (
              <Badge variant="default" className="text-xs">
                <Shield className="mr-1 h-3 w-3" />
                Admin
              </Badge>
            )}
            {!user.is_active && (
              <Badge variant="destructive" className="text-xs">
                Disabled
              </Badge>
            )}
            {!user.is_email_verified && (
              <Badge variant="outline" className="text-xs">
                Unverified
              </Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground">{user.email}</p>
        </div>
      </div>
      <div className="flex items-center gap-4">
        <div className="hidden text-right text-xs text-muted-foreground sm:block">
          <p>Joined {formatDate(user.created_at)}</p>
          {user.last_login_at && (
            <p>Last login {formatRelativeTime(user.last_login_at)}</p>
          )}
        </div>
        <div className="flex gap-1">
          {user.is_active ? (
            <Button
              variant="ghost"
              size="icon"
              onClick={onDisable}
              title="Disable user"
            >
              <UserX className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              onClick={onEnable}
              title="Enable user"
            >
              <UserCheck className="h-4 w-4" />
            </Button>
          )}
          {user.is_email_verified ? (
            <Button
              variant="ghost"
              size="icon"
              onClick={onUnverifyEmail}
              title="Unverify email"
            >
              <MailX className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              onClick={onVerifyEmail}
              title="Verify email"
            >
              <MailCheck className="h-4 w-4" />
            </Button>
          )}
          {user.is_system_admin ? (
            <Button
              variant="ghost"
              size="icon"
              onClick={onRevokeAdmin}
              title="Revoke admin"
            >
              <ShieldOff className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="icon"
              onClick={onGrantAdmin}
              title="Grant admin"
            >
              <Shield className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
