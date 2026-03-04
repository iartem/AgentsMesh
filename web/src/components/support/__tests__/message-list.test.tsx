import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@/test/test-utils";
import { MessageList } from "../message-list";
import type { SupportTicketMessage } from "@/lib/api/support-ticket";

// Mock getSupportTicketAttachmentUrl
vi.mock("@/lib/api/support-ticket", () => ({
  getSupportTicketAttachmentUrl: vi.fn().mockResolvedValue({
    url: "https://example.com/download/file.png",
  }),
}));

// Mock window.open
const mockWindowOpen = vi.fn();
Object.defineProperty(window, "open", {
  writable: true,
  value: mockWindowOpen,
});

const baseMessage: SupportTicketMessage = {
  id: 1,
  ticket_id: 10,
  user_id: 100,
  content: "Hello, I need help with my account.",
  is_admin_reply: false,
  created_at: new Date().toISOString(),
  user: { id: 100, name: "John Doe", email: "john@example.com" },
};

const adminMessage: SupportTicketMessage = {
  id: 2,
  ticket_id: 10,
  user_id: 200,
  content: "Sure, let me look into that.",
  is_admin_reply: true,
  created_at: new Date().toISOString(),
  user: { id: 200, name: "Admin User", email: "admin@example.com" },
};

const messageWithAttachments: SupportTicketMessage = {
  id: 3,
  ticket_id: 10,
  user_id: 100,
  content: "Here are the screenshots",
  is_admin_reply: false,
  created_at: new Date().toISOString(),
  user: { id: 100, name: "John Doe", email: "john@example.com" },
  attachments: [
    {
      id: 101,
      ticket_id: 10,
      message_id: 3,
      uploader_id: 100,
      original_name: "screenshot.png",
      mime_type: "image/png",
      size: 204800,
      created_at: new Date().toISOString(),
    },
    {
      id: 102,
      ticket_id: 10,
      message_id: 3,
      uploader_id: 100,
      original_name: "log.txt",
      mime_type: "text/plain",
      size: 512,
      created_at: new Date().toISOString(),
    },
  ],
};

describe("MessageList", () => {
  it("renders empty state when no messages", () => {
    render(<MessageList messages={[]} />);
    // The component uses t("support.detail.noComments") which may resolve
    // to "No comments yet" (from tickets.detail) or the key path itself.
    // We check for text content presence in a flexible way.
    const emptyDiv = document.querySelector(".py-12.text-center");
    expect(emptyDiv).toBeInTheDocument();
    expect(emptyDiv?.textContent).toBeTruthy();
  });

  it("renders messages", () => {
    render(<MessageList messages={[baseMessage, adminMessage]} />);
    expect(
      screen.getByText("Hello, I need help with my account.")
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sure, let me look into that.")
    ).toBeInTheDocument();
  });
});

describe("MessageBubble (via MessageList)", () => {
  it("renders user message correctly", () => {
    render(<MessageList messages={[baseMessage]} />);
    expect(
      screen.getByText("Hello, I need help with my account.")
    ).toBeInTheDocument();
    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("renders admin message with Admin badge", () => {
    render(<MessageList messages={[adminMessage]} />);
    expect(
      screen.getByText("Sure, let me look into that.")
    ).toBeInTheDocument();
    expect(screen.getByText("Admin")).toBeInTheDocument();
  });

  it("shows attachments with download buttons", () => {
    render(<MessageList messages={[messageWithAttachments]} />);
    expect(screen.getByText("screenshot.png")).toBeInTheDocument();
    expect(screen.getByText("log.txt")).toBeInTheDocument();
    // Verify file sizes are shown
    expect(screen.getByText("(200.0 KB)")).toBeInTheDocument();
    expect(screen.getByText("(512 B)")).toBeInTheDocument();
  });

  it("calls download handler when attachment button is clicked", async () => {
    render(<MessageList messages={[messageWithAttachments]} />);
    const downloadBtn = screen.getByText("screenshot.png").closest("button");
    expect(downloadBtn).toBeInTheDocument();
    fireEvent.click(downloadBtn!);

    // getSupportTicketAttachmentUrl is mocked to resolve, then window.open is called
    const { getSupportTicketAttachmentUrl } = await import(
      "@/lib/api/support-ticket"
    );
    expect(getSupportTicketAttachmentUrl).toHaveBeenCalledWith(101);
  });

  it("shows user name when available", () => {
    render(<MessageList messages={[baseMessage]} />);
    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("shows user email when name is not available", () => {
    const msgWithEmailOnly: SupportTicketMessage = {
      ...baseMessage,
      user: { id: 100, name: "", email: "john@example.com" },
    };
    render(<MessageList messages={[msgWithEmailOnly]} />);
    expect(screen.getByText("john@example.com")).toBeInTheDocument();
  });
});
