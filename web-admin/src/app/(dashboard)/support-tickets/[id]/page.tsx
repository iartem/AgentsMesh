"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  ArrowLeft,
  Send,
  Paperclip,
  Download,
  UserCircle,
  Shield,
  X,
} from "lucide-react";
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
  getSupportTicketDetail,
  replySupportTicket,
  updateSupportTicketStatus,
  assignSupportTicket,
  getSupportTicketAttachmentUrl,
  SupportTicketMessage,
  SupportTicketDetail,
} from "@/lib/api/admin";
import { useAuthStore } from "@/stores/auth";
import { formatRelativeTime } from "@/lib/utils";
import {
  categoryLabels,
  priorityLabels,
} from "@/lib/support-constants";

export default function SupportTicketDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { user } = useAuthStore();
  const ticketId = Number(params.id);

  const [data, setData] = useState<SupportTicketDetail | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [replyContent, setReplyContent] = useState("");
  const [replyFiles, setReplyFiles] = useState<File[]>([]);
  const [isSendingReply, setIsSendingReply] = useState(false);
  const [isUpdatingStatus, setIsUpdatingStatus] = useState(false);
  const [isAssigning, setIsAssigning] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const fetchDetail = useCallback(async () => {
    try {
      const result = await getSupportTicketDetail(ticketId);
      setData(result);
    } catch {
      // Keep previous data on error
    } finally {
      setIsLoading(false);
    }
  }, [ticketId]);

  useEffect(() => {
    if (!ticketId) return;
    fetchDetail();
    const interval = setInterval(() => {
      if (!document.hidden) {
        fetchDetail();
      }
    }, 15000);
    return () => clearInterval(interval);
  }, [ticketId, fetchDetail]);

  const ticket = data?.ticket;
  const messages = data?.messages || [];

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  const handleReply = async () => {
    if (!replyContent.trim()) return;
    setIsSendingReply(true);
    try {
      await replySupportTicket(ticketId, replyContent, replyFiles.length > 0 ? replyFiles : undefined);
      setReplyContent("");
      setReplyFiles([]);
      toast.success("Reply sent successfully");
      await fetchDetail();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to send reply";
      toast.error(message);
    } finally {
      setIsSendingReply(false);
    }
  };

  const handleStatusChange = async (status: string) => {
    setIsUpdatingStatus(true);
    try {
      await updateSupportTicketStatus(ticketId, status);
      toast.success("Status updated successfully");
      await fetchDetail();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to update status";
      toast.error(message);
    } finally {
      setIsUpdatingStatus(false);
    }
  };

  const handleAssign = async () => {
    setIsAssigning(true);
    try {
      await assignSupportTicket(ticketId, user?.id || 0);
      toast.success("Ticket assigned successfully");
      await fetchDetail();
    } catch (err: unknown) {
      const message = (err as { error?: string })?.error || "Failed to assign ticket";
      toast.error(message);
    } finally {
      setIsAssigning(false);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      setReplyFiles((prev) => [...prev, ...Array.from(e.target.files!)]);
    }
  };

  const removeFile = (index: number) => {
    setReplyFiles((prev) => prev.filter((_, i) => i !== index));
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 animate-pulse rounded bg-muted" />
        <div className="h-32 animate-pulse rounded bg-muted" />
        <div className="h-64 animate-pulse rounded bg-muted" />
      </div>
    );
  }

  if (!ticket) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">Ticket not found</p>
        <Button variant="outline" onClick={() => router.push("/support-tickets")} className="mt-4">
          Back to list
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => router.push("/support-tickets")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-xl font-bold">{ticket.title}</h1>
          <div className="mt-1 flex items-center gap-2 text-sm text-muted-foreground">
            <span>#{ticket.id}</span>
            <span>·</span>
            <span>{ticket.user?.email || `User #${ticket.user_id}`}</span>
            <span>·</span>
            <span>{formatRelativeTime(ticket.created_at)}</span>
          </div>
        </div>
      </div>

      {/* Ticket Meta + Actions */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Status:</span>
              <Select
                value={ticket.status}
                onValueChange={handleStatusChange}
                disabled={isUpdatingStatus}
              >
                <SelectTrigger className="w-36 h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">Open</SelectItem>
                  <SelectItem value="in_progress">In Progress</SelectItem>
                  <SelectItem value="resolved">Resolved</SelectItem>
                  <SelectItem value="closed">Closed</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Category:</span>
              <Badge variant="outline">{categoryLabels[ticket.category] || ticket.category}</Badge>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Priority:</span>
              <Badge variant={ticket.priority === "high" ? "destructive" : ticket.priority === "medium" ? "warning" : "secondary"}>
                {priorityLabels[ticket.priority] || ticket.priority}
              </Badge>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Assigned:</span>
              {ticket.assigned_admin ? (
                <span className="text-sm">{ticket.assigned_admin.name || ticket.assigned_admin.email}</span>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleAssign}
                  disabled={isAssigning}
                >
                  Assign to me
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Messages */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Conversation</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4 max-h-[500px] overflow-y-auto pr-2">
            {messages.length === 0 ? (
              <p className="py-8 text-center text-muted-foreground">No messages yet</p>
            ) : (
              messages.map((msg) => (
                <MessageBubble key={msg.id} message={msg} />
              ))
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Reply Form */}
          <div className="mt-6 border-t pt-4">
            <div className="space-y-3">
              <textarea
                value={replyContent}
                onChange={(e) => setReplyContent(e.target.value)}
                placeholder="Type your reply..."
                className="w-full min-h-[100px] rounded-lg border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-y"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    handleReply();
                  }
                }}
              />

              {/* File previews */}
              {replyFiles.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {replyFiles.map((file, i) => (
                    <div key={i} className="flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs">
                      <Paperclip className="h-3 w-3" />
                      <span className="max-w-[150px] truncate">{file.name}</span>
                      <button onClick={() => removeFile(i)} className="ml-1 text-muted-foreground hover:text-foreground">
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}

              <div className="flex items-center justify-between">
                <div>
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    accept="image/*,.pdf,.txt,.log"
                    onChange={handleFileSelect}
                    className="hidden"
                  />
                  <Button variant="ghost" size="sm" onClick={() => fileInputRef.current?.click()}>
                    <Paperclip className="mr-1 h-4 w-4" />
                    Attach
                  </Button>
                </div>
                <Button
                  onClick={handleReply}
                  disabled={!replyContent.trim() || isSendingReply}
                  size="sm"
                >
                  <Send className="mr-1 h-4 w-4" />
                  {isSendingReply ? "Sending..." : "Send Reply"}
                </Button>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function MessageBubble({ message }: { message: SupportTicketMessage }) {
  const isAdmin = message.is_admin_reply;

  const handleDownload = async (attachmentId: number, fileName: string) => {
    try {
      const { url } = await getSupportTicketAttachmentUrl(attachmentId);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName;
      a.target = "_blank";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    } catch {
      console.error("Failed to download attachment");
    }
  };

  return (
    <div className={`flex gap-3 ${isAdmin ? "flex-row-reverse" : ""}`}>
      <div className="flex-shrink-0 mt-1">
        {isAdmin ? (
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20">
            <Shield className="h-4 w-4 text-primary" />
          </div>
        ) : (
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted">
            <UserCircle className="h-4 w-4 text-muted-foreground" />
          </div>
        )}
      </div>
      <div className={`flex-1 max-w-[80%] ${isAdmin ? "text-right" : ""}`}>
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium">
            {message.user?.name || message.user?.email || (isAdmin ? "Admin" : "User")}
          </span>
          {isAdmin && <Badge variant="outline" className="text-[10px] px-1 py-0">Admin</Badge>}
          <span className="text-xs text-muted-foreground">{formatRelativeTime(message.created_at)}</span>
        </div>
        <div className={`rounded-lg px-3 py-2 text-sm ${isAdmin ? "bg-primary/10 ml-auto" : "bg-muted"}`}>
          <p className="whitespace-pre-wrap text-left">{message.content}</p>
        </div>

        {/* Attachments */}
        {message.attachments && message.attachments.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-2">
            {message.attachments.map((att) => (
              <button
                key={att.id}
                onClick={() => handleDownload(att.id, att.original_name)}
                className="flex items-center gap-1 rounded border px-2 py-1 text-xs hover:bg-muted transition-colors"
              >
                <Download className="h-3 w-3" />
                <span className="max-w-[120px] truncate">{att.original_name}</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
