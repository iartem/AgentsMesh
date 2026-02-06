import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

// Mock must be at module level for Vitest hoisting
vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({
      token: "test-token",
      currentOrg: { slug: "test-org" },
    }),
  },
}));

import { repositoryApi } from "../../repository";
import { mockFetch, setupRepositoryTests } from "./testSetup";

describe("repositoryApi - Error Handling", () => {
  beforeEach(() => {
    setupRepositoryTests();
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  it("should handle 404 error for non-existent repository", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      statusText: "Not Found",
      json: () => Promise.resolve({ error: "Repository not found" }),
    });

    await expect(repositoryApi.get(999)).rejects.toThrow();
  });

  it("should handle 403 error for forbidden access", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 403,
      statusText: "Forbidden",
      json: () => Promise.resolve({ error: "Access denied" }),
    });

    await expect(repositoryApi.delete(1)).rejects.toThrow();
  });

  it("should handle 500 server error", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.resolve({ error: "Server error" }),
    });

    await expect(repositoryApi.list()).rejects.toThrow();
  });
});
