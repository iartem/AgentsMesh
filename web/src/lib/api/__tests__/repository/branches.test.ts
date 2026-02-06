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
import { mockFetch, EXPECTED_API_URL, basePath, setupRepositoryTests } from "./testSetup";

describe("repositoryApi - Branch Operations", () => {
  beforeEach(() => {
    setupRepositoryTests();
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  describe("listBranches", () => {
    it("should list branches with access token", async () => {
      const mockBranches = ["main", "develop", "feature/test"];
      const accessToken = "git-access-token";

      mockFetch.mockResolvedValue({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({ branches: mockBranches })),
      });

      const result = await repositoryApi.listBranches(1, accessToken);

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}${basePath}/1/branches`,
        expect.objectContaining({
          method: "GET",
          headers: expect.objectContaining({
            "X-Git-Access-Token": accessToken,
          }),
        })
      );
      expect(result.branches).toEqual(mockBranches);
    });
  });

  describe("syncBranches", () => {
    it("should sync branches with access token", async () => {
      const mockBranches = ["main", "develop", "feature/new"];
      const accessToken = "git-access-token";

      mockFetch.mockResolvedValue({
        ok: true,
        text: () => Promise.resolve(JSON.stringify({ branches: mockBranches })),
      });

      const result = await repositoryApi.syncBranches(1, accessToken);

      expect(mockFetch).toHaveBeenCalledWith(
        `${EXPECTED_API_URL}${basePath}/1/sync-branches`,
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ access_token: accessToken }),
        })
      );
      expect(result.branches).toEqual(mockBranches);
    });
  });
});
