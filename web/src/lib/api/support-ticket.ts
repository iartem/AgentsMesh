import { request, ApiError, handleTokenRefresh } from "./base";
import { useAuthStore } from "@/stores/auth";
import { getApiBaseUrl } from "@/lib/env";

const API_BASE_URL = getApiBaseUrl();

// Types

export interface SupportTicket {
  id: number;
  user_id: number;
  title: string;
  category: string;
  status: string;
  priority: string;
  assigned_admin_id?: number;
  created_at: string;
  updated_at: string;
  resolved_at?: string;
}

export interface SupportTicketMessage {
  id: number;
  ticket_id: number;
  user_id: number;
  content: string;
  is_admin_reply: boolean;
  created_at: string;
  user?: { id: number; name: string; email: string; avatar_url?: string };
  attachments?: SupportTicketAttachment[];
}

export interface SupportTicketAttachment {
  id: number;
  ticket_id: number;
  message_id?: number;
  uploader_id: number;
  original_name: string;
  mime_type: string;
  size: number;
  created_at: string;
}

export interface SupportTicketDetail {
  ticket: SupportTicket;
  messages: SupportTicketMessage[];
}

export interface SupportTicketListResponse {
  data: SupportTicket[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface SupportTicketListParams {
  status?: string;
  page?: number;
  page_size?: number;
}

// Helper for multipart form upload with auth
async function requestFormData<T>(endpoint: string, formData: FormData): Promise<T> {
  const { token } = useAuthStore.getState();
  const headers: Record<string, string> = {};
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    method: "POST",
    headers,
    body: formData,
  });

  // Handle 401 Unauthorized - try to refresh token
  if (response.status === 401) {
    const refreshed = await handleTokenRefresh();
    if (refreshed) {
      const { token: newToken } = useAuthStore.getState();
      const retryHeaders: Record<string, string> = {};
      if (newToken) {
        retryHeaders["Authorization"] = `Bearer ${newToken}`;
      }
      const retryResponse = await fetch(`${API_BASE_URL}${endpoint}`, {
        method: "POST",
        headers: retryHeaders,
        body: formData,
      });
      if (!retryResponse.ok) {
        const data = await retryResponse.json().catch(() => null);
        throw new ApiError(retryResponse.status, retryResponse.statusText, data);
      }
      const text = await retryResponse.text();
      if (!text) return {} as T;
      return JSON.parse(text);
    } else {
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
      throw new ApiError(401, "Unauthorized", { message: "Session expired" });
    }
  }

  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new ApiError(response.status, response.statusText, data);
  }

  const text = await response.text();
  if (!text) return {} as T;
  return JSON.parse(text);
}

// API functions

export async function createSupportTicket(data: {
  title: string;
  category: string;
  content: string;
  priority?: string;
  files?: File[];
}): Promise<SupportTicket> {
  const formData = new FormData();
  formData.append("title", data.title);
  formData.append("category", data.category);
  formData.append("content", data.content);
  if (data.priority) formData.append("priority", data.priority);
  if (data.files) {
    data.files.forEach((file) => formData.append("files[]", file));
  }
  return requestFormData<SupportTicket>("/api/v1/support-tickets", formData);
}

export async function listSupportTickets(
  params?: SupportTicketListParams
): Promise<SupportTicketListResponse> {
  const searchParams = new URLSearchParams();
  if (params?.status) searchParams.set("status", params.status);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.page_size) searchParams.set("page_size", String(params.page_size));
  const qs = searchParams.toString();
  return request<SupportTicketListResponse>(
    `/api/v1/support-tickets${qs ? `?${qs}` : ""}`
  );
}

export async function getSupportTicketDetail(
  id: number
): Promise<SupportTicketDetail> {
  return request<SupportTicketDetail>(`/api/v1/support-tickets/${id}`);
}

export async function addSupportTicketMessage(
  ticketId: number,
  content: string,
  files?: File[]
): Promise<SupportTicketMessage> {
  const formData = new FormData();
  formData.append("content", content);
  if (files) {
    files.forEach((file) => formData.append("files[]", file));
  }
  return requestFormData<SupportTicketMessage>(
    `/api/v1/support-tickets/${ticketId}/messages`,
    formData
  );
}

export async function getSupportTicketAttachmentUrl(
  attachmentId: number
): Promise<{ url: string }> {
  return request<{ url: string }>(
    `/api/v1/support-tickets/attachments/${attachmentId}/url`
  );
}
