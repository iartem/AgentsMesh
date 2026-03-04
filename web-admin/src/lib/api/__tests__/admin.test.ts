import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock the apiClient
const mockGet = vi.fn();
const mockPost = vi.fn();
const mockPut = vi.fn();
const mockPatch = vi.fn();
const mockDelete = vi.fn();
const mockPostFormData = vi.fn();

vi.mock("@/lib/api/base", () => ({
  apiClient: {
    get: (...args: unknown[]) => mockGet(...args),
    post: (...args: unknown[]) => mockPost(...args),
    put: (...args: unknown[]) => mockPut(...args),
    patch: (...args: unknown[]) => mockPatch(...args),
    delete: (...args: unknown[]) => mockDelete(...args),
    postFormData: (...args: unknown[]) => mockPostFormData(...args),
  },
}));

import {
  getDashboardStats,
  listUsers,
  getUser,
  updateUser,
  disableUser,
  enableUser,
  grantAdmin,
  revokeAdmin,
  listOrganizations,
  getOrganization,
  getOrganizationMembers,
  deleteOrganization,
  listRunners,
  getRunner,
  disableRunner,
  enableRunner,
  deleteRunner,
  listAuditLogs,
  listPromoCodes,
  getPromoCode,
  createPromoCode,
  updatePromoCode,
  activatePromoCode,
  deactivatePromoCode,
  deletePromoCode,
  listPromoCodeRedemptions,
  listRelays,
  getRelayStats,
  getRelay,
  forceUnregisterRelay,
  listSessions,
  migrateSession,
  bulkMigrateSessions,
  listSkillRegistries,
  createSkillRegistry,
  syncSkillRegistry,
  deleteSkillRegistry,
  getOrganizationSubscription,
  getSubscriptionPlans,
  createSubscription,
  updateSubscriptionPlan,
  updateSubscriptionSeats,
  updateSubscriptionCycle,
  freezeSubscription,
  unfreezeSubscription,
  cancelSubscription,
  renewSubscription,
  setSubscriptionAutoRenew,
  setSubscriptionQuota,
  listSupportTickets,
  getSupportTicketStats,
  getSupportTicketDetail,
  getSupportTicketMessages,
  replySupportTicket,
  updateSupportTicketStatus,
  assignSupportTicket,
  getSupportTicketAttachmentUrl,
  login,
  getCurrentAdmin,
} from "../admin";

describe("Admin API functions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // --- Dashboard ---
  describe("Dashboard", () => {
    it("getDashboardStats calls GET /dashboard/stats", async () => {
      mockGet.mockResolvedValue({ total_users: 100 });
      const result = await getDashboardStats();
      expect(mockGet).toHaveBeenCalledWith("/dashboard/stats");
      expect(result.total_users).toBe(100);
    });
  });

  // --- Users ---
  describe("Users", () => {
    it("listUsers calls GET /users with params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listUsers({ search: "admin", page: 2, page_size: 10 });
      expect(mockGet).toHaveBeenCalledWith(
        "/users",
        expect.objectContaining({ search: "admin", page: 2, page_size: 10 })
      );
    });

    it("getUser calls GET /users/:id", async () => {
      mockGet.mockResolvedValue({ id: 5, email: "a@b.com" });
      const result = await getUser(5);
      expect(mockGet).toHaveBeenCalledWith("/users/5");
      expect(result.id).toBe(5);
    });

    it("disableUser calls POST /users/:id/disable", async () => {
      mockPost.mockResolvedValue({ id: 1, is_active: false });
      await disableUser(1);
      expect(mockPost).toHaveBeenCalledWith("/users/1/disable");
    });

    it("enableUser calls POST /users/:id/enable", async () => {
      mockPost.mockResolvedValue({ id: 1, is_active: true });
      await enableUser(1);
      expect(mockPost).toHaveBeenCalledWith("/users/1/enable");
    });

    it("grantAdmin calls POST /users/:id/grant-admin", async () => {
      mockPost.mockResolvedValue({ id: 1, is_system_admin: true });
      await grantAdmin(1);
      expect(mockPost).toHaveBeenCalledWith("/users/1/grant-admin");
    });

    it("revokeAdmin calls POST /users/:id/revoke-admin", async () => {
      mockPost.mockResolvedValue({ id: 1, is_system_admin: false });
      await revokeAdmin(1);
      expect(mockPost).toHaveBeenCalledWith("/users/1/revoke-admin");
    });

    it("updateUser calls PUT /users/:id", async () => {
      mockPut.mockResolvedValue({ id: 1, name: "Updated" });
      await updateUser(1, { name: "Updated" });
      expect(mockPut).toHaveBeenCalledWith("/users/1", { name: "Updated" });
    });
  });

  // --- Organizations ---
  describe("Organizations", () => {
    it("listOrganizations calls GET /organizations", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listOrganizations({ search: "test" });
      expect(mockGet).toHaveBeenCalledWith(
        "/organizations",
        expect.objectContaining({ search: "test" })
      );
    });

    it("getOrganization calls GET /organizations/:id", async () => {
      mockGet.mockResolvedValue({ id: 1, name: "Org" });
      await getOrganization(1);
      expect(mockGet).toHaveBeenCalledWith("/organizations/1");
    });

    it("deleteOrganization calls DELETE /organizations/:id", async () => {
      mockDelete.mockResolvedValue({ message: "deleted" });
      await deleteOrganization(1);
      expect(mockDelete).toHaveBeenCalledWith("/organizations/1");
    });

    it("getOrganizationMembers calls GET /organizations/:id/members", async () => {
      mockGet.mockResolvedValue({ organization: {}, members: [] });
      await getOrganizationMembers(1);
      expect(mockGet).toHaveBeenCalledWith("/organizations/1/members");
    });
  });

  // --- Runners ---
  describe("Runners", () => {
    it("listRunners calls GET /runners with params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listRunners({ org_id: 5 });
      expect(mockGet).toHaveBeenCalledWith(
        "/runners",
        expect.objectContaining({ org_id: 5 })
      );
    });

    it("disableRunner calls POST /runners/:id/disable", async () => {
      mockPost.mockResolvedValue({ id: 1 });
      await disableRunner(1);
      expect(mockPost).toHaveBeenCalledWith("/runners/1/disable");
    });

    it("enableRunner calls POST /runners/:id/enable", async () => {
      mockPost.mockResolvedValue({ id: 1 });
      await enableRunner(1);
      expect(mockPost).toHaveBeenCalledWith("/runners/1/enable");
    });

    it("deleteRunner calls DELETE /runners/:id", async () => {
      mockDelete.mockResolvedValue({ message: "ok" });
      await deleteRunner(1);
      expect(mockDelete).toHaveBeenCalledWith("/runners/1");
    });

    it("getRunner calls GET /runners/:id", async () => {
      mockGet.mockResolvedValue({ id: 3, node_id: "node-3" });
      await getRunner(3);
      expect(mockGet).toHaveBeenCalledWith("/runners/3");
    });
  });

  // --- Audit Logs ---
  describe("Audit Logs", () => {
    it("listAuditLogs calls GET /audit-logs with params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listAuditLogs({ target_type: "user", page: 1 });
      expect(mockGet).toHaveBeenCalledWith(
        "/audit-logs",
        expect.objectContaining({ target_type: "user", page: 1 })
      );
    });
  });

  // --- Promo Codes ---
  describe("Promo Codes", () => {
    it("listPromoCodes converts boolean is_active to string", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listPromoCodes({ is_active: true, type: "media" });
      expect(mockGet).toHaveBeenCalledWith(
        "/promo-codes",
        expect.objectContaining({ is_active: "true", type: "media" })
      );
    });

    it("listPromoCodes omits undefined params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listPromoCodes({ page: 1 });
      const params = mockGet.mock.calls[0][1];
      expect(params.search).toBeUndefined();
      expect(params.type).toBeUndefined();
    });

    it("getPromoCode calls GET /promo-codes/:id", async () => {
      mockGet.mockResolvedValue({ id: 10 });
      await getPromoCode(10);
      expect(mockGet).toHaveBeenCalledWith("/promo-codes/10");
    });

    it("createPromoCode calls POST /promo-codes", async () => {
      const data = {
        code: "TEST50",
        name: "Test",
        type: "campaign" as const,
        plan_name: "pro",
        duration_months: 3,
      };
      mockPost.mockResolvedValue({ id: 1, ...data });
      await createPromoCode(data);
      expect(mockPost).toHaveBeenCalledWith("/promo-codes", data);
    });

    it("activatePromoCode calls POST /promo-codes/:id/activate", async () => {
      mockPost.mockResolvedValue({ message: "ok" });
      await activatePromoCode(5);
      expect(mockPost).toHaveBeenCalledWith("/promo-codes/5/activate");
    });

    it("deactivatePromoCode calls POST /promo-codes/:id/deactivate", async () => {
      mockPost.mockResolvedValue({ message: "ok" });
      await deactivatePromoCode(5);
      expect(mockPost).toHaveBeenCalledWith("/promo-codes/5/deactivate");
    });

    it("deletePromoCode calls DELETE /promo-codes/:id", async () => {
      mockDelete.mockResolvedValue({ message: "ok" });
      await deletePromoCode(5);
      expect(mockDelete).toHaveBeenCalledWith("/promo-codes/5");
    });

    it("updatePromoCode calls PUT /promo-codes/:id", async () => {
      mockPut.mockResolvedValue({ id: 5, name: "Updated" });
      await updatePromoCode(5, { name: "Updated" });
      expect(mockPut).toHaveBeenCalledWith("/promo-codes/5", { name: "Updated" });
    });

    it("listPromoCodeRedemptions calls GET /promo-codes/:id/redemptions", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listPromoCodeRedemptions(5, { page: 1, page_size: 10 });
      expect(mockGet).toHaveBeenCalledWith(
        "/promo-codes/5/redemptions",
        expect.objectContaining({ page: 1, page_size: 10 })
      );
    });

    it("listPromoCodes converts is_active false to string", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listPromoCodes({ is_active: false });
      const params = mockGet.mock.calls[0][1];
      expect(params.is_active).toBe("false");
    });

    it("listPromoCodes with all params set", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listPromoCodes({
        search: "TEST",
        type: "campaign",
        plan_name: "pro",
        is_active: true,
        page: 2,
        page_size: 25,
      });
      expect(mockGet).toHaveBeenCalledWith(
        "/promo-codes",
        expect.objectContaining({
          search: "TEST",
          type: "campaign",
          plan_name: "pro",
          is_active: "true",
          page: 2,
          page_size: 25,
        })
      );
    });
  });

  // --- Relays ---
  describe("Relays", () => {
    it("listRelays calls GET /relays", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listRelays();
      expect(mockGet).toHaveBeenCalledWith("/relays");
    });

    it("getRelayStats calls GET /relays/stats", async () => {
      mockGet.mockResolvedValue({ total_relays: 3 });
      await getRelayStats();
      expect(mockGet).toHaveBeenCalledWith("/relays/stats");
    });

    it("forceUnregisterRelay encodes relay ID", async () => {
      mockDelete.mockResolvedValue({ status: "ok" });
      await forceUnregisterRelay("relay/special", true);
      expect(mockDelete).toHaveBeenCalledWith(
        "/relays/relay%2Fspecial",
        { migrate_sessions: true }
      );
    });

    it("migrateSession calls POST /relays/sessions/migrate", async () => {
      mockPost.mockResolvedValue({ status: "ok" });
      await migrateSession("pod-1", "relay-2");
      expect(mockPost).toHaveBeenCalledWith("/relays/sessions/migrate", {
        pod_key: "pod-1",
        target_relay: "relay-2",
      });
    });

    it("bulkMigrateSessions calls POST /relays/sessions/bulk-migrate", async () => {
      mockPost.mockResolvedValue({ status: "ok", total: 5, migrated: 5 });
      await bulkMigrateSessions("relay-1", "relay-2");
      expect(mockPost).toHaveBeenCalledWith(
        "/relays/sessions/bulk-migrate",
        { source_relay: "relay-1", target_relay: "relay-2" }
      );
    });

    it("getRelay encodes relay ID and calls GET", async () => {
      mockGet.mockResolvedValue({ relay: {}, sessions: [] });
      await getRelay("relay/with-slash");
      expect(mockGet).toHaveBeenCalledWith("/relays/relay%2Fwith-slash");
    });

    it("listSessions calls GET /relays/sessions with optional relay_id", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listSessions("relay-1");
      expect(mockGet).toHaveBeenCalledWith(
        "/relays/sessions",
        { relay_id: "relay-1" }
      );
    });

    it("listSessions calls GET /relays/sessions without params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listSessions();
      expect(mockGet).toHaveBeenCalledWith("/relays/sessions", undefined);
    });

    it("forceUnregisterRelay defaults migrateSessions to false", async () => {
      mockDelete.mockResolvedValue({ status: "ok" });
      await forceUnregisterRelay("relay-1");
      expect(mockDelete).toHaveBeenCalledWith(
        "/relays/relay-1",
        { migrate_sessions: false }
      );
    });
  });

  // --- Skill Registries ---
  describe("Skill Registries", () => {
    it("listSkillRegistries calls GET /skill-registries", async () => {
      mockGet.mockResolvedValue({ items: [], total: 0 });
      await listSkillRegistries();
      expect(mockGet).toHaveBeenCalledWith("/skill-registries");
    });

    it("createSkillRegistry calls POST /skill-registries", async () => {
      const data = { repository_url: "https://github.com/org/repo" };
      mockPost.mockResolvedValue({ id: 1 });
      await createSkillRegistry(data);
      expect(mockPost).toHaveBeenCalledWith("/skill-registries", data);
    });

    it("syncSkillRegistry calls POST /skill-registries/:id/sync", async () => {
      mockPost.mockResolvedValue({ message: "ok" });
      await syncSkillRegistry(3);
      expect(mockPost).toHaveBeenCalledWith("/skill-registries/3/sync");
    });

    it("deleteSkillRegistry calls DELETE /skill-registries/:id", async () => {
      mockDelete.mockResolvedValue(undefined);
      await deleteSkillRegistry(3);
      expect(mockDelete).toHaveBeenCalledWith("/skill-registries/3");
    });
  });

  // --- Support Tickets ---
  describe("Support Tickets", () => {
    it("listSupportTickets calls GET /support-tickets with params", async () => {
      mockGet.mockResolvedValue({ data: [], total: 0 });
      await listSupportTickets({ status: "open", page: 1 });
      expect(mockGet).toHaveBeenCalledWith(
        "/support-tickets",
        expect.objectContaining({ status: "open", page: 1 })
      );
    });

    it("getSupportTicketStats calls GET /support-tickets/stats", async () => {
      mockGet.mockResolvedValue({ total: 10, open: 3 });
      await getSupportTicketStats();
      expect(mockGet).toHaveBeenCalledWith("/support-tickets/stats");
    });

    it("getSupportTicketDetail calls GET /support-tickets/:id", async () => {
      mockGet.mockResolvedValue({ ticket: {}, messages: [] });
      await getSupportTicketDetail(7);
      expect(mockGet).toHaveBeenCalledWith("/support-tickets/7");
    });

    it("replySupportTicket sends FormData via postFormData", async () => {
      mockPostFormData.mockResolvedValue({ id: 1 });
      await replySupportTicket(7, "Hello");

      expect(mockPostFormData).toHaveBeenCalledWith(
        "/support-tickets/7/reply",
        expect.any(FormData)
      );
      const formData = mockPostFormData.mock.calls[0][1] as FormData;
      expect(formData.get("content")).toBe("Hello");
    });

    it("replySupportTicket includes files in FormData", async () => {
      mockPostFormData.mockResolvedValue({ id: 1 });
      const file = new File(["data"], "test.txt", { type: "text/plain" });

      await replySupportTicket(7, "With attachment", [file]);

      const formData = mockPostFormData.mock.calls[0][1] as FormData;
      const files = formData.getAll("files[]");
      expect(files).toHaveLength(1);
      expect((files[0] as File).name).toBe("test.txt");
    });

    it("updateSupportTicketStatus calls PATCH", async () => {
      mockPatch.mockResolvedValue({ message: "ok" });
      await updateSupportTicketStatus(7, "resolved");
      expect(mockPatch).toHaveBeenCalledWith(
        "/support-tickets/7/status",
        { status: "resolved" }
      );
    });

    it("assignSupportTicket calls POST", async () => {
      mockPost.mockResolvedValue({ message: "ok" });
      await assignSupportTicket(7, 42);
      expect(mockPost).toHaveBeenCalledWith(
        "/support-tickets/7/assign",
        { admin_id: 42 }
      );
    });

    it("getSupportTicketMessages calls GET /support-tickets/:id/messages", async () => {
      mockGet.mockResolvedValue({ messages: [] });
      await getSupportTicketMessages(7);
      expect(mockGet).toHaveBeenCalledWith("/support-tickets/7/messages");
    });

    it("getSupportTicketAttachmentUrl calls GET /support-tickets/attachments/:id/url", async () => {
      mockGet.mockResolvedValue({ url: "https://s3.example.com/file.png" });
      const result = await getSupportTicketAttachmentUrl(99);
      expect(mockGet).toHaveBeenCalledWith("/support-tickets/attachments/99/url");
      expect(result.url).toBe("https://s3.example.com/file.png");
    });
  });

  // --- Auth ---
  describe("Auth", () => {
    it("login calls POST /auth/login", async () => {
      mockPost.mockResolvedValue({ token: "t", user: {} });
      await login({ email: "admin@test.com", password: "pass" });
      expect(mockPost).toHaveBeenCalledWith("/auth/login", {
        email: "admin@test.com",
        password: "pass",
      });
    });

    it("getCurrentAdmin calls GET /me", async () => {
      mockGet.mockResolvedValue({ id: 1 });
      await getCurrentAdmin();
      expect(mockGet).toHaveBeenCalledWith("/me");
    });
  });

  // --- Subscriptions ---
  describe("Subscriptions", () => {
    it("getOrganizationSubscription calls GET /organizations/:id/subscription", async () => {
      mockGet.mockResolvedValue({ id: 1, status: "active" });
      await getOrganizationSubscription(10);
      expect(mockGet).toHaveBeenCalledWith("/organizations/10/subscription");
    });

    it("getSubscriptionPlans calls GET /organizations/:id/subscription/plans", async () => {
      mockGet.mockResolvedValue({ data: [] });
      await getSubscriptionPlans(10);
      expect(mockGet).toHaveBeenCalledWith("/organizations/10/subscription/plans");
    });

    it("createSubscription calls POST with plan_name and months", async () => {
      mockPost.mockResolvedValue({ id: 1 });
      await createSubscription(10, "pro", 6);
      expect(mockPost).toHaveBeenCalledWith(
        "/organizations/10/subscription/create",
        { plan_name: "pro", months: 6 }
      );
    });

    it("createSubscription defaults months to 1", async () => {
      mockPost.mockResolvedValue({ id: 1 });
      await createSubscription(10, "pro");
      expect(mockPost).toHaveBeenCalledWith(
        "/organizations/10/subscription/create",
        { plan_name: "pro", months: 1 }
      );
    });

    it("updateSubscriptionPlan calls PUT", async () => {
      mockPut.mockResolvedValue({ id: 1 });
      await updateSubscriptionPlan(10, "enterprise");
      expect(mockPut).toHaveBeenCalledWith(
        "/organizations/10/subscription/plan",
        { plan_name: "enterprise" }
      );
    });

    it("updateSubscriptionSeats calls PUT", async () => {
      mockPut.mockResolvedValue({ id: 1 });
      await updateSubscriptionSeats(10, 25);
      expect(mockPut).toHaveBeenCalledWith(
        "/organizations/10/subscription/seats",
        { seat_count: 25 }
      );
    });

    it("updateSubscriptionCycle calls PUT", async () => {
      mockPut.mockResolvedValue({ id: 1 });
      await updateSubscriptionCycle(10, "yearly");
      expect(mockPut).toHaveBeenCalledWith(
        "/organizations/10/subscription/cycle",
        { billing_cycle: "yearly" }
      );
    });

    it("freezeSubscription calls POST", async () => {
      mockPost.mockResolvedValue({ id: 1, status: "frozen" });
      await freezeSubscription(10);
      expect(mockPost).toHaveBeenCalledWith("/organizations/10/subscription/freeze");
    });

    it("unfreezeSubscription calls POST", async () => {
      mockPost.mockResolvedValue({ id: 1, status: "active" });
      await unfreezeSubscription(10);
      expect(mockPost).toHaveBeenCalledWith("/organizations/10/subscription/unfreeze");
    });

    it("cancelSubscription calls POST", async () => {
      mockPost.mockResolvedValue({ id: 1, status: "canceled" });
      await cancelSubscription(10);
      expect(mockPost).toHaveBeenCalledWith("/organizations/10/subscription/cancel");
    });

    it("renewSubscription calls POST with months", async () => {
      mockPost.mockResolvedValue({ id: 1 });
      await renewSubscription(10, 12);
      expect(mockPost).toHaveBeenCalledWith(
        "/organizations/10/subscription/renew",
        { months: 12 }
      );
    });

    it("setSubscriptionAutoRenew calls PUT", async () => {
      mockPut.mockResolvedValue({ id: 1 });
      await setSubscriptionAutoRenew(10, false);
      expect(mockPut).toHaveBeenCalledWith(
        "/organizations/10/subscription/auto-renew",
        { auto_renew: false }
      );
    });

    it("setSubscriptionQuota calls PUT", async () => {
      mockPut.mockResolvedValue({ id: 1 });
      await setSubscriptionQuota(10, "max_runners", 50);
      expect(mockPut).toHaveBeenCalledWith(
        "/organizations/10/subscription/quotas",
        { resource: "max_runners", limit: 50 }
      );
    });
  });
});
