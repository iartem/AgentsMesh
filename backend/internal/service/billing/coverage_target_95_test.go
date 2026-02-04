package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Final Tests to Reach 95% Coverage
// ===========================================

// TestGetInvoicesByOrg_NoLimit tests getting all invoices without limit
func TestGetInvoicesByOrg_NoLimit(t *testing.T) {
	svc, _ := setupTestService(t)

	// Without limit (limit=0)
	invoices, err := svc.GetInvoicesByOrg(context.Background(), 1, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty is fine
	if invoices == nil {
		t.Error("expected non-nil slice, got nil")
	}
}

// TestGetUsage_MultipleRecords tests usage aggregation
func TestGetUsage_MultipleRecords(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	periodStart := now.AddDate(0, 0, -15)
	periodEnd := now.AddDate(0, 0, 15)

	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		SeatCount:          1,
	})

	// Create multiple usage records
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       100,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	})
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       50,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	})

	usage, err := svc.GetUsage(context.Background(), 1, billing.UsageTypePodMinutes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should aggregate: 100 + 50 = 150
	if usage != 150 {
		t.Errorf("expected usage 150, got %f", usage)
	}
}

// TestGetUsageHistory_Multiple tests history with multiple records
func TestGetUsageHistory_Multiple(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create multiple usage records
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       100,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      "other_type",
		Quantity:       50,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})

	// Get all history (empty usageType)
	records, err := svc.GetUsageHistory(context.Background(), 1, "", 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

// TestGetSeatUsage_ProPlan tests seat usage with pro plan
func TestGetSeatUsage_ProPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
	})

	usage, err := svc.GetSeatUsage(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.TotalSeats != 5 {
		t.Errorf("expected 5 total seats, got %d", usage.TotalSeats)
	}
	if !usage.CanAddSeats {
		t.Error("expected CanAddSeats to be true for pro plan")
	}
}

// TestCheckQuota_RunnersExceeded tests runners quota exceeded
func TestCheckQuota_RunnersExceeded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based: max 1 runner
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Add a runner to use the quota (test DB uses simplified schema)
	db.Exec("INSERT INTO runners (organization_id, name) VALUES (1, 'runner1')")

	// Should fail when trying to add another runner
	err := svc.CheckQuota(context.Background(), 1, "runners", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestCheckQuota_ReposExceeded tests repositories quota exceeded
func TestCheckQuota_ReposExceeded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based: max 5 repos
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Add repos to fill quota (test DB uses simplified schema)
	for i := 0; i < 5; i++ {
		db.Exec("INSERT INTO repositories (organization_id, name) VALUES (1, ?)",
			"repo"+string(rune('0'+i)))
	}

	// Should fail when trying to add another repo
	err := svc.CheckQuota(context.Background(), 1, "repositories", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestListPlansWithPrices_AllPricesExist tests when all plans have prices
func TestListPlansWithPrices_AllPricesExist(t *testing.T) {
	svc, _ := setupTestService(t)

	// List plans with USD (all seeded plans have USD prices)
	plans, err := svc.ListPlansWithPrices(context.Background(), "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have based, pro, enterprise plans
	if len(plans) < 3 {
		t.Errorf("expected at least 3 plans with USD prices, got %d", len(plans))
	}
}

// TestCalculateRenewalPrice_YearlyWithSeats tests renewal with yearly and multiple seats
func TestCalculateRenewalPrice_YearlyWithSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          5,
	})

	result, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Seats != 5 {
		t.Errorf("expected 5 seats, got %d", result.Seats)
	}
	if result.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", result.BillingCycle)
	}
}

// TestCheckQuota_ConcurrentPodsExceeded tests concurrent pods quota
func TestCheckQuota_ConcurrentPodsExceeded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based: max 5 concurrent pods
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Add running pods to fill quota (test DB uses simplified schema with 'pods' table)
	for i := 0; i < 5; i++ {
		db.Exec("INSERT INTO pods (organization_id, name, status) VALUES (1, ?, 'running')",
			"pod"+string(rune('0'+i)))
	}

	// Should fail when trying to add another pod
	err := svc.CheckQuota(context.Background(), 1, "concurrent_pods", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestRecordUsage_NotFound tests recording usage without subscription
func TestRecordUsage_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.RecordUsage(context.Background(), 999, billing.UsageTypePodMinutes, 10, nil)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestSetCustomQuota_NotFound tests setting quota without subscription
func TestSetCustomQuota_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.SetCustomQuota(context.Background(), 999, "users", 100)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestUnfreezeSubscription_NotFound tests unfreezing without subscription
func TestUnfreezeSubscription_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.UnfreezeSubscription(context.Background(), 999, billing.BillingCycleMonthly)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestActivateTrialSubscription_NotFound tests activating trial without subscription
func TestActivateTrialSubscription_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.ActivateTrialSubscription(context.Background(), 999, billing.BillingCycleMonthly)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestUpdateSubscription_DowngradeSeatExceedsLimit tests downgrade with seat count exceeding limit
func TestUpdateSubscription_DowngradeSeatExceedsLimit(t *testing.T) {
	svc, db := setupTestService(t)

	// Seed free plan
	freePlan := seedFreePlan(t, db)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          10, // More than free plan's max_users (1)
	})

	// Downgrade from pro to free - should fail due to seat count exceeding limit
	_, err := svc.UpdateSubscription(context.Background(), 1, freePlan.Name)
	if err != ErrSeatCountExceedsLimit {
		t.Errorf("expected ErrSeatCountExceedsLimit, got %v", err)
	}
}

// TestCheckQuota_DefaultPlan tests quota when no based plan exists
func TestCheckQuota_NoPlanFound(t *testing.T) {
	svc, db := setupTestService(t)

	// Delete all plans to simulate no plans in database
	db.Exec("DELETE FROM plan_prices")
	db.Exec("DELETE FROM subscription_plans")

	// No subscription and no plans - should allow by default (nil error)
	err := svc.CheckQuota(context.Background(), 999, "users", 1)
	if err != nil {
		t.Errorf("expected nil when no plan found, got %v", err)
	}
}

// TestHandlePaymentSucceeded_WithExternalOrderNo tests finding order by external_order_no
func TestHandlePaymentSucceeded_WithExternalOrderNo(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-EXT-LOOKUP",
		ExternalOrderNo: ptrString("ext_order_123"),
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Seats:           1,
		BillingCycle:    billing.BillingCycleMonthly,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderStripe,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:         "evt_ext_lookup",
		EventType:       "checkout.session.completed",
		Provider:        billing.PaymentProviderStripe,
		ExternalOrderNo: "ext_order_123", // Match by external_order_no
		Amount:          19.99,
		Currency:        "USD",
		RawPayload:      map[string]interface{}{"test": true},
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGetUsageHistory_WithFilter tests usage history with type filter
func TestGetUsageHistory_WithFilter(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create multiple usage records of different types
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       100,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      "other_type",
		Quantity:       50,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})

	// Get history with specific type filter
	records, err := svc.GetUsageHistory(context.Background(), 1, billing.UsageTypePodMinutes, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return pod_minutes type
	if len(records) != 1 {
		t.Errorf("expected 1 record with type filter, got %d", len(records))
	}
}

// TestGetSeatUsage_BasedPlan tests seat usage with based plan (fixed seats)
func TestGetSeatUsage_BasedPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan - MaxUsers = 1
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	usage, err := svc.GetSeatUsage(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Based plan has fixed seats (MaxUsers = 1), so CanAddSeats should be false
	if usage.CanAddSeats {
		t.Error("expected CanAddSeats to be false for based plan")
	}
}

// Helper function for pointer string
func ptrString(s string) *string {
	return &s
}

// TestGetBillingOverview_PlanLoadedFromDB tests billing overview when plan is not preloaded
func TestGetBillingOverview_PlanLoadedFromDB(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create subscription without preloading plan
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          3,
	}
	db.Create(sub)

	overview, err := svc.GetBillingOverview(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.Plan == nil {
		t.Error("expected plan to be loaded")
	}
	if overview.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", overview.Status)
	}
}

// TestGetBillingOverview_NoSubscription tests billing overview when subscription not found
func TestGetBillingOverview_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.GetBillingOverview(context.Background(), 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestHandlePaymentFailed_RecurringWithSubscription tests recurring payment failure
func TestHandlePaymentFailed_RecurringWithSubscription(t *testing.T) {
	svc, db := setupTestService(t)

	lsSubID := "ls_sub_recurring_fail"
	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:             1,
		PlanID:                     2,
		Status:                     billing.SubscriptionStatusActive,
		BillingCycle:               billing.BillingCycleMonthly,
		CurrentPeriodStart:         now,
		CurrentPeriodEnd:           now.AddDate(0, 1, 0),
		SeatCount:                  1,
		LemonSqueezySubscriptionID: &lsSubID,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_recurring_fail",
		EventType:      billing.WebhookEventLSPaymentFailed,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: lsSubID, // Has subscription ID - triggers recurring payment failure path
		FailedReason:   "Card declined",
	}

	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Subscription should be frozen
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusFrozen {
		t.Errorf("expected frozen status, got %s", sub.Status)
	}
}

// TestCheckQuota_WithCustomQuota tests custom quota that is not -1
func TestCheckQuota_WithCustomQuotaLimit(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	customQuotas := billing.CustomQuotas{"users": float64(5)} // Custom limit of 5 users
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan - would normally have 1 user limit
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		CustomQuotas:       customQuotas,
	})

	// Should pass - custom quota allows 5 users
	err := svc.CheckQuota(context.Background(), 1, "users", 3)
	if err != nil {
		t.Errorf("expected nil for custom quota, got %v", err)
	}

	// Add 4 members to use quota
	for i := 0; i < 4; i++ {
		db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, ?, 'member')", i+1)
	}

	// Should fail - custom quota of 5, used 4, requesting 2 more (total 6 > 5)
	err = svc.CheckQuota(context.Background(), 1, "users", 2)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestCreateTrialSubscription_CustomDays tests trial with custom days
func TestCreateTrialSubscription_CustomDays(t *testing.T) {
	svc, _ := setupTestService(t)

	// Create with 30 days trial
	sub, err := svc.CreateTrialSubscription(context.Background(), 1, "pro", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check trial period is ~30 days
	duration := sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart)
	if duration.Hours() < 29*24 || duration.Hours() > 31*24 {
		t.Errorf("expected ~30 days trial, got %v", duration)
	}
}

// TestCreateTrialSubscription_ZeroDays tests trial with zero days (should use default 30 days)
func TestCreateTrialSubscription_ZeroDays(t *testing.T) {
	svc, _ := setupTestService(t)

	// Create with 0 days - should use default 30 days
	sub, err := svc.CreateTrialSubscription(context.Background(), 2, "pro", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check trial period is ~30 days (default)
	duration := sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart)
	if duration.Hours() < 29*24 || duration.Hours() > 31*24 {
		t.Errorf("expected ~30 days trial (default), got %v", duration)
	}
}

// TestUpdateSubscription_DowngradeScheduled tests scheduled downgrade (not exceeding limit)
func TestUpdateSubscription_DowngradeScheduled(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1, // Low seat count, won't exceed based plan limit
	})

	// Downgrade from pro to based - should be scheduled
	sub, err := svc.UpdateSubscription(context.Background(), 1, "based")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.DowngradeToPlan == nil || *sub.DowngradeToPlan != "based" {
		t.Error("expected downgrade to be scheduled")
	}
}

// TestUpdateSubscription_PaidUpgradeReturnsCurrent tests paid upgrade (returns current plan)
func TestUpdateSubscription_PaidUpgradeReturnsCurrent(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan (paid)
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Upgrade from based to pro (both paid) - returns current plan (payment flow handles actual upgrade)
	sub, err := svc.UpdateSubscription(context.Background(), 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For paid upgrades, plan is NOT changed immediately - payment flow handles it
	// The returned subscription still has the current plan
	if sub.Plan == nil || sub.Plan.Name != "based" {
		t.Errorf("expected current plan (based), got %v", sub.Plan.Name)
	}
}

// TestUpdateSubscription_NotFoundPlan tests update to nonexistent plan
func TestUpdateSubscription_NotFoundPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	_, err := svc.UpdateSubscription(context.Background(), 1, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestCancelSubscription_NoSub tests cancelling nonexistent subscription
func TestCancelSubscription_NoSub(t *testing.T) {
	svc, _ := setupTestService(t)

	err := svc.CancelSubscription(context.Background(), 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestCancelSubscription_Active tests cancelling active subscription
func TestCancelSubscription_Active(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	err := svc.CancelSubscription(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusCanceled {
		t.Errorf("expected canceled status, got %s", sub.Status)
	}
}

// TestCalculateUpgradePrice_ZeroSeats tests upgrade price with zero seats (defaults to 1)
func TestCalculateUpgradePrice_ZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Zero seats - should default to 1
	})

	result, err := svc.CalculateUpgradePrice(context.Background(), 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 0 seats defaulting to 1, price should be calculated for 1 seat
	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
}

// TestCalculateSeatPurchasePrice_ZeroSeats tests seat purchase with zero existing seats
func TestCalculateSeatPurchasePrice_ZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Zero seats
	})

	// Purchase 3 additional seats
	result, err := svc.CalculateSeatPurchasePrice(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New total should be 3 (0 -> 3) - actually 1 + 3 = 4 if zero defaults to 1
	if result.Seats < 3 {
		t.Errorf("expected at least 3 seats, got %d", result.Seats)
	}
}

// TestCalculateBillingCycleChangePrice_NoSeats tests cycle change with zero seats
func TestCalculateBillingCycleChangePrice_NoSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Zero seats
	})

	result, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 0 seats defaulting to 1, should still calculate correctly
	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
}

// TestCalculateRenewalPrice_NoSeats tests renewal with zero seats
func TestCalculateRenewalPrice_NoSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Zero seats
	})

	result, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With 0 seats defaulting to 1
	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
}

// TestHandleSubscriptionCreated_FallbackByOrderNo tests subscription created with order_no fallback
func TestHandleSubscriptionCreated_FallbackByOrderNo(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	// Create order first
	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-LS-FALLBACK",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Seats:           1,
		BillingCycle:    billing.BillingCycleMonthly,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	// Create subscription (without LemonSqueezy IDs)
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		// No LemonSqueezy IDs
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_sub_created_fallback",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_fallback",
		CustomerID:     "nonexistent_customer", // Customer lookup will fail
		OrderNo:        "ORD-LS-FALLBACK",      // Fallback to order lookup
	}

	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify subscription was updated via fallback
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.LemonSqueezySubscriptionID == nil || *sub.LemonSqueezySubscriptionID != "ls_sub_fallback" {
		t.Error("expected subscription ID to be set via order fallback")
	}
}

// TestHandleSubscriptionCreated_BothCustomerAndOrder tests both customer and order lookups
func TestHandleSubscriptionCreated_BothCustomerAndOrderFail(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_sub_created_both_fail",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_both_fail",
		CustomerID:     "nonexistent_customer",
		OrderNo:        "nonexistent_order",
	}

	// Should return nil even when both lookups fail
	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Errorf("expected nil when both lookups fail, got %v", err)
	}
}

// TestGetSeatUsage_WithPlanNotPreloaded tests seat usage when plan is not preloaded
func TestGetSeatUsage_WithPlanNotPreloaded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create subscription - Plan will be nil initially
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
		// Plan is NOT set - will be loaded
	}
	db.Create(sub)

	usage, err := svc.GetSeatUsage(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.TotalSeats != 5 {
		t.Errorf("expected 5 total seats, got %d", usage.TotalSeats)
	}
	// Plan should have been loaded
	if usage.MaxSeats <= 0 {
		t.Error("expected MaxSeats to be loaded from plan")
	}
}

// TestCheckQuota_SubPlanNilNeedsLoad tests quota check when subscription plan is nil and needs to be loaded
func TestCheckQuota_SubPlanNilNeedsLoad(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create subscription without preloaded plan
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		// Plan field is nil - will need to be loaded
	}
	db.Create(sub)

	// The subscription's Plan field is nil, so CheckQuota will load it
	err := svc.CheckQuota(context.Background(), 1, "users", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
