package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// ===========================================
// Usage and Quota Tests
// ===========================================

func TestRecordUsage(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	err := service.RecordUsage(ctx, 1, "pod_minutes", 5.0, billing.UsageMetadata{})
	if err != nil {
		t.Fatalf("failed to record usage: %v", err)
	}
}

func TestGetUsage(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	service.RecordUsage(ctx, 1, "pod_minutes", 5.0, billing.UsageMetadata{})
	service.RecordUsage(ctx, 1, "pod_minutes", 3.0, billing.UsageMetadata{})

	usage, err := service.GetUsage(ctx, 1, "pod_minutes")
	if err != nil {
		t.Fatalf("failed to get usage: %v", err)
	}
	if usage != 8.0 {
		t.Errorf("expected usage 8.0, got %f", usage)
	}
}

func TestGetUsageHistory(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	service.RecordUsage(ctx, 1, "pod_minutes", 5.0, billing.UsageMetadata{})

	records, err := service.GetUsageHistory(ctx, 1, "pod_minutes", 10)
	if err != nil {
		t.Fatalf("failed to get usage history: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestGetUsageHistoryAllTypes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	service.RecordUsage(ctx, 1, "pod_minutes", 5.0, billing.UsageMetadata{})
	service.RecordUsage(ctx, 1, "storage_gb", 1.0, billing.UsageMetadata{})

	records, err := service.GetUsageHistory(ctx, 1, "", 10) // Empty type = all types
	if err != nil {
		t.Fatalf("failed to get usage history: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

func TestCheckQuotaWithinLimit(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	err := service.CheckQuota(ctx, 1, "users", 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckQuotaExceeded(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // max_users = 5
	service.CreateSubscription(ctx, 1, "free")

	// Add existing members to exceed quota
	for i := 0; i < 5; i++ {
		db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, ?, 'member')", i+1)
	}

	err := service.CheckQuota(ctx, 1, "users", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckQuotaUnlimited(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedEnterprisePlan(t, db) // max_users = -1 (unlimited)

	plan, _ := service.GetPlan(ctx, "enterprise")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	err := service.CheckQuota(ctx, 1, "users", 1000)
	if err != nil {
		t.Errorf("expected no error for unlimited quota, got %v", err)
	}
}

func TestCheckQuotaCustomQuota(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")
	service.SetCustomQuota(ctx, 1, "users", 100)

	// Should allow up to 100 users now
	err := service.CheckQuota(ctx, 1, "users", 50)
	if err != nil {
		t.Errorf("expected no error with custom quota, got %v", err)
	}
}

func TestCheckQuotaNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// No subscription - should use free plan defaults
	err := service.CheckQuota(ctx, 999, "users", 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckQuotaAllResourceTypes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	resources := []string{"users", "runners", "concurrent_pods", "repositories", "pod_minutes", "unknown"}
	for _, resource := range resources {
		err := service.CheckQuota(ctx, 1, resource, 0)
		if err != nil {
			t.Errorf("unexpected error for resource %s: %v", resource, err)
		}
	}
}

func TestCheckSeatAvailability(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	plan, _ := service.GetPlan(ctx, "free")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		SeatCount:          5,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Add 2 members
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 2, 'member')")

	// Should have 3 available seats
	err := service.CheckSeatAvailability(ctx, 1, 3)
	if err != nil {
		t.Errorf("expected 3 available seats, got error: %v", err)
	}

	// Should fail for 4 seats
	err = service.CheckSeatAvailability(ctx, 1, 4)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckSeatAvailabilityWithPendingInvitations(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	plan, _ := service.GetPlan(ctx, "free")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		SeatCount:          3,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// 1 member
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	// 1 pending invitation
	db.Exec("INSERT INTO invitations (organization_id, email, expires_at) VALUES (1, 'test@example.com', ?)", now.Add(24*time.Hour))

	// 3 seats - 1 used - 1 pending = 1 available
	err := service.CheckSeatAvailability(ctx, 1, 1)
	if err != nil {
		t.Errorf("expected 1 available seat, got error: %v", err)
	}

	err = service.CheckSeatAvailability(ctx, 1, 2)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestGetCurrentConcurrentPods(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod1', 'running')")
	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod2', 'initializing')")
	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod3', 'stopped')")

	count, err := service.GetCurrentConcurrentPods(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get concurrent pods: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 concurrent pods, got %d", count)
	}
}

func TestSetCustomQuota(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	err := service.SetCustomQuota(ctx, 1, "users", 100)
	if err != nil {
		t.Fatalf("failed to set custom quota: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.CustomQuotas == nil {
		t.Fatal("expected custom quotas to be set")
	}
	if limit, ok := sub.CustomQuotas["users"].(float64); !ok || int(limit) != 100 {
		t.Errorf("expected users quota 100, got %v", sub.CustomQuotas["users"])
	}
}

// ===========================================
// Seat Usage Tests
// ===========================================

func TestGetSeatUsage(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db) // Use Pro plan for CanAddSeats = true

	plan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		SeatCount:          5,
		Status:             billing.SubscriptionStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Add 2 members
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 2, 'member')")

	usage, err := service.GetSeatUsage(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get seat usage: %v", err)
	}
	if usage.TotalSeats != 5 {
		t.Errorf("expected 5 total seats, got %d", usage.TotalSeats)
	}
	if usage.UsedSeats != 2 {
		t.Errorf("expected 2 used seats, got %d", usage.UsedSeats)
	}
	if usage.AvailableSeats != 3 {
		t.Errorf("expected 3 available seats, got %d", usage.AvailableSeats)
	}
	if !usage.CanAddSeats {
		t.Error("expected CanAddSeats to be true for non-free plan")
	}
}

func TestGetSeatUsageFreePlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	usage, err := service.GetSeatUsage(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get seat usage: %v", err)
	}
	if usage.CanAddSeats {
		t.Error("expected CanAddSeats to be false for free plan")
	}
}

func TestGetSeatUsageNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetSeatUsage(ctx, 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestGetSeatUsageWithNilPlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plan := seedProPlan(t, db)

	// Create subscription without plan preloaded
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		SeatCount:          5,
		Status:             billing.SubscriptionStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// GetSeatUsage should fetch plan if nil
	usage, err := service.GetSeatUsage(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get seat usage: %v", err)
	}
	if usage.MaxSeats != 50 { // Pro plan max_users
		t.Errorf("expected max seats 50, got %d", usage.MaxSeats)
	}
}

// ===========================================
// Billing Overview Tests
// ===========================================

func TestGetBillingOverview(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	// Add some resources
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO runners (organization_id, name) VALUES (1, 'runner1')")
	db.Exec("INSERT INTO repositories (organization_id, name) VALUES (1, 'repo1')")
	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod1', 'running')")

	overview, err := service.GetBillingOverview(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get billing overview: %v", err)
	}
	if overview.Plan.Name != "free" {
		t.Errorf("expected plan 'free', got %s", overview.Plan.Name)
	}
	if overview.Usage.Users != 1 {
		t.Errorf("expected 1 user, got %d", overview.Usage.Users)
	}
	if overview.Usage.Runners != 1 {
		t.Errorf("expected 1 runner, got %d", overview.Usage.Runners)
	}
}

func TestRecordUsageNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	// No subscription exists
	err := service.RecordUsage(ctx, 999, "pod_minutes", 5.0, billing.UsageMetadata{})
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestGetUsageNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetUsage(ctx, 999, "pod_minutes")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestCheckQuotaRunners(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // max_runners = 1
	service.CreateSubscription(ctx, 1, "free")

	// Add one runner
	db.Exec("INSERT INTO runners (organization_id, name) VALUES (1, 'runner1')")

	// Should fail to add another
	err := service.CheckQuota(ctx, 1, "runners", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckQuotaConcurrentPods(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // max_concurrent_pods = 2
	service.CreateSubscription(ctx, 1, "free")

	// Add two running pods
	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod1', 'running')")
	db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, 'pod2', 'initializing')")

	// Should fail to add another
	err := service.CheckQuota(ctx, 1, "concurrent_pods", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckQuotaRepositories(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // max_repositories = 3
	service.CreateSubscription(ctx, 1, "free")

	// Add three repositories
	for i := 1; i <= 3; i++ {
		db.Exec("INSERT INTO repositories (organization_id, name) VALUES (1, ?)", "repo"+string(rune('0'+i)))
	}

	// Should fail to add another
	err := service.CheckQuota(ctx, 1, "repositories", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckQuotaWithCustomQuotaExceeded(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")
	service.SetCustomQuota(ctx, 1, "users", 2) // Override to 2 users

	// Add 2 members
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 2, 'member')")

	// Should fail to add another (quota is 2)
	err := service.CheckQuota(ctx, 1, "users", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCheckQuotaPodMinutes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // included_pod_minutes = 100
	service.CreateSubscription(ctx, 1, "free")

	// Record 100 minutes of usage
	service.RecordUsage(ctx, 1, "pod_minutes", 100.0, billing.UsageMetadata{})

	// Should fail to use more
	err := service.CheckQuota(ctx, 1, "pod_minutes", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestSetCustomQuotaNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	err := service.SetCustomQuota(ctx, 999, "users", 100)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestCheckSeatAvailabilityNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	// No subscription = default 1 seat
	// Add 1 member
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")

	// Should fail to add another (no more seats available)
	err := service.CheckSeatAvailability(ctx, 1, 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestGetBillingOverviewNoSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// No subscription - should return error
	_, err := service.GetBillingOverview(ctx, 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestGetBillingOverviewWithNilPlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plan := seedTestPlan(t, db)

	// Create subscription without preloading plan
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// GetBillingOverview should still work by fetching plan by ID
	overview, err := service.GetBillingOverview(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get billing overview: %v", err)
	}
	if overview.Plan == nil {
		t.Error("expected plan to be loaded")
	}
}
