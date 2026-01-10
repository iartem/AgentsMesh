import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { sshKeyApi, SSHKeyData } from "../ssh-key";

// Mock useAuthStore first
const mockGetState = vi.fn();
vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => mockGetState(),
  },
}));

// Mock the request function from base module
vi.mock("../base", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../base")>();
  return {
    ...actual,
    request: vi.fn(),
  };
});

// Import the mocked request
import { request } from "../base";
const mockRequest = vi.mocked(request);

describe("sshKeyApi", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Setup org for orgPath to work correctly
    mockGetState.mockReturnValue({
      token: "test-token",
      currentOrg: { slug: "test-org" },
    });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  describe("list", () => {
    const mockSSHKeys: SSHKeyData[] = [
      {
        id: 1,
        organization_id: 1,
        name: "my-key",
        public_key: "ssh-rsa AAAA... my-key",
        fingerprint: "SHA256:abc123",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      {
        id: 2,
        organization_id: 1,
        name: "another-key",
        public_key: "ssh-rsa BBBB... another-key",
        fingerprint: "SHA256:def456",
        created_at: "2024-01-02T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
      },
    ];

    it("should list all SSH keys", async () => {
      mockRequest.mockResolvedValue({ ssh_keys: mockSSHKeys });

      const result = await sshKeyApi.list();

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys");
      expect(result.ssh_keys).toEqual(mockSSHKeys);
      expect(result.ssh_keys).toHaveLength(2);
    });

    it("should return empty array when no SSH keys", async () => {
      mockRequest.mockResolvedValue({ ssh_keys: [] });

      const result = await sshKeyApi.list();

      expect(result.ssh_keys).toEqual([]);
    });
  });

  describe("get", () => {
    const mockSSHKey: SSHKeyData = {
      id: 1,
      organization_id: 1,
      name: "my-key",
      public_key: "ssh-rsa AAAA... my-key",
      fingerprint: "SHA256:abc123",
      created_at: "2024-01-01T00:00:00Z",
      updated_at: "2024-01-01T00:00:00Z",
    };

    it("should get an SSH key by ID", async () => {
      mockRequest.mockResolvedValue({ ssh_key: mockSSHKey });

      const result = await sshKeyApi.get(1);

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys/1");
      expect(result.ssh_key).toEqual(mockSSHKey);
    });
  });

  describe("create", () => {
    it("should create a new SSH key (generate key pair)", async () => {
      const newKey: SSHKeyData = {
        id: 3,
        organization_id: 1,
        name: "generated-key",
        public_key: "ssh-rsa CCCC... generated-key",
        fingerprint: "SHA256:ghi789",
        created_at: "2024-01-03T00:00:00Z",
        updated_at: "2024-01-03T00:00:00Z",
      };
      mockRequest.mockResolvedValue({ ssh_key: newKey });

      const result = await sshKeyApi.create({ name: "generated-key" });

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys", {
        method: "POST",
        body: { name: "generated-key" },
      });
      expect(result.ssh_key).toEqual(newKey);
    });

    it("should create a new SSH key with provided private key", async () => {
      const newKey: SSHKeyData = {
        id: 4,
        organization_id: 1,
        name: "imported-key",
        public_key: "ssh-rsa DDDD... imported-key",
        fingerprint: "SHA256:jkl012",
        created_at: "2024-01-04T00:00:00Z",
        updated_at: "2024-01-04T00:00:00Z",
      };
      mockRequest.mockResolvedValue({ ssh_key: newKey });

      const privateKey = "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----";
      const result = await sshKeyApi.create({
        name: "imported-key",
        private_key: privateKey,
      });

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys", {
        method: "POST",
        body: { name: "imported-key", private_key: privateKey },
      });
      expect(result.ssh_key).toEqual(newKey);
    });
  });

  describe("update", () => {
    it("should update an SSH key name", async () => {
      const updatedKey: SSHKeyData = {
        id: 1,
        organization_id: 1,
        name: "renamed-key",
        public_key: "ssh-rsa AAAA... my-key",
        fingerprint: "SHA256:abc123",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-05T00:00:00Z",
      };
      mockRequest.mockResolvedValue({ ssh_key: updatedKey });

      const result = await sshKeyApi.update(1, "renamed-key");

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys/1", {
        method: "PUT",
        body: { name: "renamed-key" },
      });
      expect(result.ssh_key.name).toBe("renamed-key");
    });
  });

  describe("delete", () => {
    it("should delete an SSH key", async () => {
      mockRequest.mockResolvedValue({ message: "SSH key deleted" });

      const result = await sshKeyApi.delete(1);

      expect(mockRequest).toHaveBeenCalledWith("/api/v1/orgs/test-org/ssh-keys/1", {
        method: "DELETE",
      });
      expect(result.message).toBe("SSH key deleted");
    });
  });
});
