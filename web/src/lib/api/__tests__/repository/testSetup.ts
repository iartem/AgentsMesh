import { vi } from "vitest";

// Get the expected API URL
export const EXPECTED_API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:10000";

// Mock global fetch
export const mockFetch = vi.fn();

// Test constants
export const testOrg = "test-org";
export const testToken = "test-token";
export const basePath = `/api/v1/orgs/${testOrg}/repositories`;

/**
 * Setup function to be called in beforeEach of each test file.
 * NOTE: Each test file must also include its own vi.mock() call at the top level
 * because Vitest hoists mocks per-file, not across imports.
 *
 * Add this to the TOP of each test file (before any other imports):
 * ```
 * vi.mock("@/stores/auth", () => ({
 *   useAuthStore: {
 *     getState: () => ({
 *       token: "test-token",
 *       currentOrg: { slug: "test-org" },
 *     }),
 *   },
 * }));
 * ```
 */
export function setupRepositoryTests() {
  // Setup mocks before each test
  vi.clearAllMocks();

  // Reset the fetch mock
  global.fetch = mockFetch;
}
