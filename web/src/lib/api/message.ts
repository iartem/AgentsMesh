import { request, orgPath } from "./base";

// Agent Message types
export interface AgentMessage {
  id: number;
  sender_pod: string;
  receiver_pod: string;
  message_type: string;
  content: Record<string, unknown>;
  status: "pending" | "delivered" | "read" | "failed" | "dead_letter";
  correlation_id?: string;
  parent_message_id?: number;
  delivery_attempts: number;
  max_retries: number;
  delivered_at?: string;
  read_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DeadLetterEntry {
  id: number;
  original_message_id: number;
  original_message?: AgentMessage;
  reason: string;
  final_attempt: number;
  moved_at: string;
  replayed_at?: string;
  replay_result?: string;
}

// Message API
export const messageApi = {
  // Send a message to another pod
  sendMessage: (data: {
    receiver_pod: string;
    message_type: string;
    content: Record<string, unknown>;
    correlation_id?: string;
    reply_to_id?: number;
  }, podKey?: string) =>
    request<{ message: AgentMessage }>(orgPath("/messages"), {
      method: "POST",
      body: data,
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Get messages for the current pod
  getMessages: (params?: {
    unread_only?: boolean;
    message_types?: string[];
    limit?: number;
    offset?: number;
  }, podKey?: string) => {
    const searchParams = new URLSearchParams();
    if (params?.unread_only) searchParams.append("unread_only", "true");
    if (params?.message_types) {
      params.message_types.forEach(t => searchParams.append("message_types", t));
    }
    if (params?.limit) searchParams.append("limit", String(params.limit));
    if (params?.offset) searchParams.append("offset", String(params.offset));
    const query = searchParams.toString() ? `?${searchParams.toString()}` : "";
    return request<{ messages: AgentMessage[]; total: number; unread_count: number }>(
      `${orgPath("/messages")}${query}`,
      { headers: podKey ? { "X-Pod-Key": podKey } : undefined }
    );
  },

  // Get count of unread messages
  getUnreadCount: (podKey?: string) =>
    request<{ count: number }>(orgPath("/messages/unread-count"), {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Get a specific message by ID
  getMessage: (id: number, podKey?: string) =>
    request<{ message: AgentMessage }>(`${orgPath("/messages")}/${id}`, {
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Mark messages as read
  markRead: (messageIds: number[], podKey?: string) =>
    request<{ marked_count: number }>(orgPath("/messages/mark-read"), {
      method: "POST",
      body: { message_ids: messageIds },
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Mark all messages as read
  markAllRead: (podKey?: string) =>
    request<{ marked_count: number }>(orgPath("/messages/mark-all-read"), {
      method: "POST",
      headers: podKey ? { "X-Pod-Key": podKey } : undefined,
    }),

  // Get conversation by correlation ID
  getConversation: (correlationId: string, limit?: number, podKey?: string) => {
    const params = limit ? `?limit=${limit}` : "";
    return request<{ messages: AgentMessage[]; total: number }>(
      `${orgPath("/messages/conversation")}/${correlationId}${params}`,
      { headers: podKey ? { "X-Pod-Key": podKey } : undefined }
    );
  },

  // Get sent messages
  getSentMessages: (params?: { limit?: number; offset?: number }, podKey?: string) => {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.append("limit", String(params.limit));
    if (params?.offset) searchParams.append("offset", String(params.offset));
    const query = searchParams.toString() ? `?${searchParams.toString()}` : "";
    return request<{ messages: AgentMessage[]; total: number }>(
      `${orgPath("/messages/sent")}${query}`,
      { headers: podKey ? { "X-Pod-Key": podKey } : undefined }
    );
  },

  // Get dead letter queue entries
  getDeadLetters: (params?: { limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.append("limit", String(params.limit));
    if (params?.offset) searchParams.append("offset", String(params.offset));
    const query = searchParams.toString() ? `?${searchParams.toString()}` : "";
    return request<{ entries: DeadLetterEntry[]; total: number }>(`${orgPath("/messages/dlq")}${query}`);
  },

  // Replay a dead letter message
  replayDeadLetter: (entryId: number) =>
    request<{ message: string; replayed_message: AgentMessage }>(
      `${orgPath("/messages/dlq")}/${entryId}/replay`,
      { method: "POST" }
    ),
};
