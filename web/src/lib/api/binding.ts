import { request, orgPath } from "./base";

// Binding types
export interface PodBinding {
  id: number;
  organization_id: number;
  initiator_pod: string;
  target_pod: string;
  granted_scopes: string[];
  pending_scopes: string[];
  status: "pending" | "active" | "rejected" | "inactive" | "expired";
  requested_at?: string;
  responded_at?: string;
  expires_at?: string;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
}

// Binding API
export const bindingApi = {
  // Request a new binding
  requestBinding: (targetPod: string, scopes: string[], policy?: string, podKey?: string) =>
    request<{ binding: PodBinding }>(orgPath("/bindings"), {
      method: "POST",
      body: { target_pod: targetPod, scopes, policy },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Accept a pending binding
  acceptBinding: (bindingId: number, podKey?: string) =>
    request<{ binding: PodBinding }>(orgPath("/bindings/accept"), {
      method: "POST",
      body: { binding_id: bindingId },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Reject a pending binding
  rejectBinding: (bindingId: number, reason?: string, podKey?: string) =>
    request<{ binding: PodBinding }>(orgPath("/bindings/reject"), {
      method: "POST",
      body: { binding_id: bindingId, reason },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Request additional scopes
  requestScopes: (bindingId: number, scopes: string[], podKey?: string) =>
    request<{ binding: PodBinding }>(`${orgPath("/bindings")}/${bindingId}/scopes`, {
      method: "POST",
      body: { scopes },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Approve pending scopes
  approveScopes: (bindingId: number, scopes: string[], podKey?: string) =>
    request<{ binding: PodBinding }>(`${orgPath("/bindings")}/${bindingId}/scopes/approve`, {
      method: "POST",
      body: { scopes },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Unbind from a pod
  unbind: (targetPod: string, podKey?: string) =>
    request<void>(orgPath("/bindings/unbind"), {
      method: "POST",
      body: { target_pod: targetPod },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // List all bindings
  listBindings: (status?: string, podKey?: string) => {
    const params = status ? `?status=${status}` : "";
    return request<{ bindings: PodBinding[]; total: number }>(`${orgPath("/bindings")}${params}`, {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    });
  },

  // Get pending binding requests
  getPendingBindings: (podKey?: string) =>
    request<{ pending: PodBinding[]; count: number }>(orgPath("/bindings/pending"), {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Get bound pods
  getBoundPods: (podKey?: string) =>
    request<{ pods: string[]; count: number }>(orgPath("/bindings/pods"), {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Check binding status with a specific pod
  checkBinding: (targetPod: string, podKey?: string) =>
    request<{ is_bound: boolean; binding?: PodBinding }>(`${orgPath("/bindings/check")}/${targetPod}`, {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),
};
