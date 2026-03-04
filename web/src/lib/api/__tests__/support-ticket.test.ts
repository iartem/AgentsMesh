import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getApiBaseUrl } from "@/lib/env";

const EXPECTED_API_URL = getApiBaseUrl();

// Mock useAuthStore
const mockGetState = vi.fn();
vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => mockGetState(),
  },
}));

// Mock handleTokenRefresh from base
const mockHandleTokenRefresh = vi.fn();
vi.mock("@/lib/api/base", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/base")>();
  return {
    ...actual,
    handleTokenRefresh: (...args: unknown[]) => mockHandleTokenRefresh(...args),
  };
});

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Import after mocks are set up
import {
  listSupportTickets,
  getSupportTicketDetail,
  addSupportTicketMessage,
  createSupportTicket,
  getSupportTicketAttachmentUrl,
} from "../support-ticket";

describe("support-ticket API", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetState.mockReturnValue({
      token: "test-token",
      currentOrg: { slug: "test-org" },
    });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  describe("listSupportTickets", () => {
    it("should build correct URL with query params", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ data: [], total: 0, page: 1, page_size: 20, total_pages: 0 })
          ),
      });

      await listSupportTickets({ status: "open", page: 2, page_size: 10 });

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets?status=open&page=2&page_size=10`,
        expect.objectContaining({ method: "GET" })
      );
    });

    it("should build URL without query params when none provided", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ data: [], total: 0, page: 1, page_size: 20, total_pages: 0 })
          ),
      });

      await listSupportTickets();

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets`,
        expect.objectContaining({ method: "GET" })
      );
    });

    it("should include auth header", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ data: [], total: 0, page: 1, page_size: 20, total_pages: 0 })
          ),
      });

      await listSupportTickets();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        })
      );
    });
  });

  describe("getSupportTicketDetail", () => {
    it("should call correct URL with ticket ID", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ ticket: { id: 42 }, messages: [] })
          ),
      });

      await getSupportTicketDetail(42);

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets/42`,
        expect.objectContaining({ method: "GET" })
      );
    });
  });

  describe("addSupportTicketMessage", () => {
    it("should send FormData with content", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(JSON.stringify({ id: 1, content: "hello" })),
      });

      await addSupportTicketMessage(5, "hello");

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets/5/messages`,
        expect.objectContaining({
          method: "POST",
        })
      );

      // Verify FormData body
      const callArgs = mockFetch.mock.calls[0];
      const body = callArgs[1].body;
      expect(body).toBeInstanceOf(FormData);
      expect(body.get("content")).toBe("hello");
    });

    it("should include files in FormData", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(JSON.stringify({ id: 1, content: "hello" })),
      });

      const file1 = new File(["data1"], "file1.txt", { type: "text/plain" });
      const file2 = new File(["data2"], "file2.png", { type: "image/png" });

      await addSupportTicketMessage(5, "hello", [file1, file2]);

      const callArgs = mockFetch.mock.calls[0];
      const body = callArgs[1].body as FormData;
      const fileEntries = body.getAll("files[]");
      expect(fileEntries).toHaveLength(2);
      expect((fileEntries[0] as File).name).toBe("file1.txt");
      expect((fileEntries[1] as File).name).toBe("file2.png");
    });
  });

  describe("createSupportTicket", () => {
    it("should send FormData with all fields", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ id: 10, title: "Bug", category: "bug" })
          ),
      });

      const file = new File(["screenshot"], "screenshot.png", { type: "image/png" });

      await createSupportTicket({
        title: "Bug report",
        category: "bug",
        content: "Something broke",
        priority: "high",
        files: [file],
      });

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets`,
        expect.objectContaining({
          method: "POST",
        })
      );

      const callArgs = mockFetch.mock.calls[0];
      const body = callArgs[1].body as FormData;
      expect(body.get("title")).toBe("Bug report");
      expect(body.get("category")).toBe("bug");
      expect(body.get("content")).toBe("Something broke");
      expect(body.get("priority")).toBe("high");
      expect((body.getAll("files[]")[0] as File).name).toBe("screenshot.png");
    });

    it("should not include priority or files when not provided", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(
            JSON.stringify({ id: 10, title: "Question" })
          ),
      });

      await createSupportTicket({
        title: "Question",
        category: "usage_question",
        content: "How do I?",
      });

      const callArgs = mockFetch.mock.calls[0];
      const body = callArgs[1].body as FormData;
      expect(body.get("title")).toBe("Question");
      expect(body.get("category")).toBe("usage_question");
      expect(body.get("content")).toBe("How do I?");
      expect(body.get("priority")).toBeNull();
      expect(body.getAll("files[]")).toHaveLength(0);
    });
  });

  describe("getSupportTicketAttachmentUrl", () => {
    it("should call correct URL with attachment ID", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        text: () =>
          Promise.resolve(JSON.stringify({ url: "https://example.com/file.png" })),
      });

      const result = await getSupportTicketAttachmentUrl(99);

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}/api/v1/support-tickets/attachments/99/url`,
        expect.objectContaining({ method: "GET" })
      );
      expect(result).toEqual({ url: "https://example.com/file.png" });
    });
  });

  describe("requestFormData 401 handling", () => {
    it("should retry with refreshed token on 401", async () => {
      // First call returns 401
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "token expired" }),
        text: () => Promise.resolve(JSON.stringify({ error: "token expired" })),
      });

      // After refresh, return success
      mockHandleTokenRefresh.mockResolvedValue(true);
      mockGetState
        .mockReturnValueOnce({ token: "test-token" }) // initial call
        .mockReturnValueOnce({ token: "new-token" }); // after refresh

      mockFetch.mockResolvedValueOnce({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({ id: 1, content: "hello" })),
      });

      await addSupportTicketMessage(5, "hello");

      // Should have called fetch twice: initial + retry
      expect(mockFetch).toHaveBeenCalledTimes(2);
      expect(mockHandleTokenRefresh).toHaveBeenCalledTimes(1);

      // The retry call should have the new token
      const retryArgs = mockFetch.mock.calls[1];
      expect(retryArgs[1].headers["Authorization"]).toBe("Bearer new-token");
    });

    it("should redirect to login when refresh fails on 401", async () => {
      // Mock window.location
      const originalLocation = window.location;
      Object.defineProperty(window, "location", {
        writable: true,
        value: { href: "" },
      });

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: () => Promise.resolve({ error: "token expired" }),
        text: () => Promise.resolve(JSON.stringify({ error: "token expired" })),
      });

      mockHandleTokenRefresh.mockResolvedValue(false);

      await expect(addSupportTicketMessage(5, "hello")).rejects.toThrow();
      expect(window.location.href).toBe("/login");

      // Restore
      Object.defineProperty(window, "location", {
        writable: true,
        value: originalLocation,
      });
    });
  });
});
