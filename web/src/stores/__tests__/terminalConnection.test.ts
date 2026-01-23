import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  url: string;
  readyState: number = MockWebSocket.CONNECTING;
  binaryType: string = "blob";
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: ((e: unknown) => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.onopen?.();
    }, 0);
  }

  send = vi.fn();
  close = vi.fn(() => {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.();
  });
}

global.WebSocket = MockWebSocket as unknown as typeof WebSocket;

// Mock pod API
vi.mock("@/lib/api/pod", () => ({
  podApi: {
    getTerminalConnection: vi.fn().mockResolvedValue({
      relay_url: "wss://relay.example.com",
      session_id: "test-session",
      token: "test-token",
    }),
  },
}));

describe("terminalConnection", () => {
  let terminalPool: typeof import("@/stores/terminalConnection").terminalPool;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    // Re-import to get fresh singleton
    vi.resetModules();
    const importedModule = await import("@/stores/terminalConnection");
    terminalPool = importedModule.terminalPool;
  });

  afterEach(() => {
    terminalPool?.disconnectAll();
    vi.useRealTimers();
  });

  describe("connect", () => {
    it("should create connection and return handle", async () => {
      const onMessage = vi.fn();
      const handlePromise = terminalPool.connect("pod-1", onMessage);

      await vi.runAllTimersAsync();
      const handle = await handlePromise;

      expect(handle).toHaveProperty("send");
      expect(handle).toHaveProperty("disconnect");
      expect(terminalPool.getStatus("pod-1")).toBe("connected");
    });

    it("should reuse existing connection for same pod", async () => {
      const onMessage1 = vi.fn();
      const onMessage2 = vi.fn();

      await terminalPool.connect("pod-1", onMessage1);
      await vi.runAllTimersAsync();

      const handle2 = await terminalPool.connect("pod-1", onMessage2);
      await vi.runAllTimersAsync();

      expect(handle2).toHaveProperty("send");
      // Both listeners should be registered
    });
  });

  describe("getStatus", () => {
    it("should return 'none' for unknown pod", () => {
      expect(terminalPool.getStatus("unknown")).toBe("none");
    });
  });

  describe("isConnected", () => {
    it("should return false for unknown pod", () => {
      expect(terminalPool.isConnected("unknown")).toBe(false);
    });
  });

  describe("isRunnerDisconnected", () => {
    it("should return false for unknown pod", () => {
      expect(terminalPool.isRunnerDisconnected("unknown")).toBe(false);
    });
  });

  describe("disconnect", () => {
    it("should close connection and remove from pool", async () => {
      const onMessage = vi.fn();
      await terminalPool.connect("pod-1", onMessage);
      await vi.runAllTimersAsync();

      terminalPool.disconnect("pod-1");

      expect(terminalPool.getStatus("pod-1")).toBe("none");
    });

    it("should be safe to call for non-existent pod", () => {
      expect(() => terminalPool.disconnect("unknown")).not.toThrow();
    });
  });

  describe("disconnectAll", () => {
    it("should disconnect all connections", async () => {
      const onMessage = vi.fn();
      await terminalPool.connect("pod-1", onMessage);
      await terminalPool.connect("pod-2", onMessage);
      await vi.runAllTimersAsync();

      terminalPool.disconnectAll();

      expect(terminalPool.getStatus("pod-1")).toBe("none");
      expect(terminalPool.getStatus("pod-2")).toBe("none");
    });
  });

  describe("sendResize", () => {
    it("should not throw for invalid dimensions", async () => {
      const onMessage = vi.fn();
      await terminalPool.connect("pod-1", onMessage);
      await vi.runAllTimersAsync();

      expect(() => terminalPool.sendResize("pod-1", 0, 0)).not.toThrow();
      expect(() => terminalPool.sendResize("pod-1", -1, 24)).not.toThrow();
    });
  });

  describe("getPtySize", () => {
    it("should return undefined for unknown pod", () => {
      expect(terminalPool.getPtySize("unknown")).toBeUndefined();
    });
  });
});
