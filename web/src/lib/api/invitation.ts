import { request, orgPath } from "./base";

// Invitation types
export interface Invitation {
  id: number;
  organization_id: number;
  email: string;
  role: "admin" | "member";
  expires_at: string;
  accepted_at?: string;
  created_at: string;
}

export interface InvitationInfo {
  id: number;
  email: string;
  role: string;
  organization_id: number;
  organization_name: string;
  organization_slug: string;
  inviter_name: string;
  expires_at: string;
  is_expired: boolean;
}

export interface PendingInvitation {
  id: number;
  organization_id: number;
  organization_name: string;
  organization_slug: string;
  role: string;
  expires_at: string;
  token: string;
}

// Invitation API
export const invitationApi = {
  // Organization-scoped routes (require org context via X-Organization-Slug header)
  list: () =>
    request<{ invitations: Invitation[] }>(orgPath("/invitations")),

  create: (email: string, role: "admin" | "member") =>
    request<{ message: string; invitation: Invitation }>(orgPath("/invitations"), {
      method: "POST",
      body: { email, role },
    }),

  revoke: (id: number) =>
    request<{ message: string }>(`${orgPath("/invitations")}/${id}`, {
      method: "DELETE",
    }),

  resend: (id: number) =>
    request<{ message: string }>(`${orgPath("/invitations")}/${id}/resend`, {
      method: "POST",
    }),

  // Public/auth routes
  getByToken: (token: string) =>
    request<{ invitation: InvitationInfo }>(`/api/v1/invitations/${token}`),

  accept: (token: string) =>
    request<{ message: string; organization: { id: number; name: string; slug: string } }>(
      `/api/v1/invitations/${token}/accept`,
      { method: "POST" }
    ),

  listPending: () =>
    request<{ invitations: PendingInvitation[] }>("/api/v1/invitations/pending"),
};
