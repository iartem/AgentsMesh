package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// ===========================================
// Subscription Tests
// ===========================================

func TestGetSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plan := seedTestPlan(t, db)

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

	result, err := service.GetSubscription(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get subscription: %v", err)
	}
	if result.OrganizationID != 1 {
		t.Errorf("expected org ID 1, got %d", result.OrganizationID)
	}
}

func TestGetSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetSubscription(ctx, 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestCreateSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	sub, err := service.CreateSubscription(ctx, 1, "based")
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	if sub.OrganizationID != 1 {
		t.Errorf("expected org ID 1, got %d", sub.OrganizationID)
	}
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
}

func TestCreateSubscriptionPlanNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.CreateSubscription(ctx, 1, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestUpdateSubscriptionUpgrade(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	seedProPlan(t, db)

	service.CreateSubscription(ctx, 1, "based")

	// Upgrade from based to pro requires payment, so plan should not change immediately
	// UpdateSubscription should return current subscription (payment handled via checkout)
	sub, err := service.UpdateSubscription(ctx, 1, "pro")
	if err != nil {
		t.Fatalf("failed to update subscription: %v", err)
	}
	// Since based is a paid plan ($9.9), upgrading to pro requires payment
	// The subscription should remain on based until payment is completed
	if sub.Plan.Name != "based" {
		t.Errorf("expected plan to remain 'based' until payment, got %s", sub.Plan.Name)
	}
}

func TestUpdateSubscriptionDowngrade(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	seedProPlan(t, db)

	// Create pro subscription first
	plan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          1, // Within based plan limit
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Downgrade to free - should schedule for period end
	result, err := service.UpdateSubscription(ctx, 1, "based")
	if err != nil {
		t.Fatalf("failed to schedule downgrade: %v", err)
	}
	if result.DowngradeToPlan == nil || *result.DowngradeToPlan != "based" {
		t.Error("expected downgrade to be scheduled")
	}
}

func TestUpdateSubscriptionDowngradeSeatExceeds(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // Free plan max_users = 5
	seedProPlan(t, db)

	// Create pro subscription with too many seats
	plan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          10, // Exceeds based plan limit of 5
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Downgrade to free - should fail
	_, err := service.UpdateSubscription(ctx, 1, "based")
	if err != ErrSeatCountExceedsLimit {
		t.Errorf("expected ErrSeatCountExceedsLimit, got %v", err)
	}
}

func TestCancelSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	err := service.CancelSubscription(ctx, 1)
	if err != nil {
		t.Fatalf("failed to cancel subscription: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusCanceled {
		t.Errorf("expected status canceled, got %s", sub.Status)
	}
}

func TestSetCancelAtPeriodEnd(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	err := service.SetCancelAtPeriodEnd(ctx, 1, true)
	if err != nil {
		t.Fatalf("failed to set cancel at period end: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if !sub.CancelAtPeriodEnd {
		t.Error("expected CancelAtPeriodEnd to be true")
	}
}

func TestSetNextBillingCycle(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	err := service.SetNextBillingCycle(ctx, 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("failed to set next billing cycle: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.NextBillingCycle == nil || *sub.NextBillingCycle != billing.BillingCycleYearly {
		t.Error("expected NextBillingCycle to be yearly")
	}
}

func TestRenewSubscriptionMonthly(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	sub, _ := service.GetSubscription(ctx, 1)
	originalEnd := sub.CurrentPeriodEnd

	err := service.RenewSubscription(ctx, 1)
	if err != nil {
		t.Fatalf("failed to renew subscription: %v", err)
	}

	sub, _ = service.GetSubscription(ctx, 1)
	if !sub.CurrentPeriodStart.Equal(originalEnd) {
		t.Error("expected new period start to equal old period end")
	}
}

func TestRenewSubscriptionYearly(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	now := time.Now()
	plan, _ := service.GetPlan(ctx, "based")
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
	}
	db.Create(sub)

	originalEnd := sub.CurrentPeriodEnd

	err := service.RenewSubscription(ctx, 1)
	if err != nil {
		t.Fatalf("failed to renew subscription: %v", err)
	}

	sub, _ = service.GetSubscription(ctx, 1)
	expectedEnd := originalEnd.AddDate(1, 0, 0)
	if !sub.CurrentPeriodEnd.Truncate(time.Second).Equal(expectedEnd.Truncate(time.Second)) {
		t.Errorf("expected yearly renewal, got %v vs %v", sub.CurrentPeriodEnd, expectedEnd)
	}
}

func TestUpdateSubscriptionPaidToPaid(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db)
	seedEnterprisePlan(t, db)

	// Create pro subscription
	plan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          1,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Upgrade from pro to enterprise (paid to paid)
	// Should just return current subscription (payment flow handles upgrade)
	result, err := service.UpdateSubscription(ctx, 1, "enterprise")
	if err != nil {
		t.Fatalf("failed to update subscription: %v", err)
	}
	// Should still be pro - upgrade requires payment flow
	if result.Plan.Name != "pro" {
		t.Errorf("expected plan 'pro', got %s", result.Plan.Name)
	}
}

func TestUpdateSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	_, err := service.UpdateSubscription(ctx, 999, "based")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestUpdateSubscriptionInvalidNewPlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	_, err := service.UpdateSubscription(ctx, 1, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestCancelSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	err := service.CancelSubscription(ctx, 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestRenewSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	err := service.RenewSubscription(ctx, 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ===========================================
// Trial Subscription Tests
// ===========================================

func TestCreateTrialSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	sub, err := service.CreateTrialSubscription(ctx, 1, "based", 30)
	if err != nil {
		t.Fatalf("failed to create trial subscription: %v", err)
	}
	if sub.Status != billing.SubscriptionStatusTrialing {
		t.Errorf("expected status trialing, got %s", sub.Status)
	}
	if sub.SeatCount != 1 {
		t.Errorf("expected seat count 1, got %d", sub.SeatCount)
	}

	// Verify period is 30 days
	expectedEnd := sub.CurrentPeriodStart.AddDate(0, 0, 30)
	if !sub.CurrentPeriodEnd.Truncate(time.Second).Equal(expectedEnd.Truncate(time.Second)) {
		t.Errorf("expected 30 day trial period, got %v", sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart))
	}
}

func TestCreateTrialSubscriptionDefaultDays(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// Pass 0 days - should use default
	sub, err := service.CreateTrialSubscription(ctx, 1, "based", 0)
	if err != nil {
		t.Fatalf("failed to create trial subscription: %v", err)
	}

	// Verify period uses default (30 days)
	expectedEnd := sub.CurrentPeriodStart.AddDate(0, 0, billing.DefaultTrialDays)
	if !sub.CurrentPeriodEnd.Truncate(time.Second).Equal(expectedEnd.Truncate(time.Second)) {
		t.Errorf("expected default trial period, got %v", sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart))
	}
}

func TestCreateTrialSubscriptionPlanNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.CreateTrialSubscription(ctx, 1, "nonexistent", 30)
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestActivateTrialSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// Create trial subscription
	service.CreateTrialSubscription(ctx, 1, "based", 30)

	// Activate with monthly billing
	err := service.ActivateTrialSubscription(ctx, 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("failed to activate trial: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.BillingCycle != billing.BillingCycleMonthly {
		t.Errorf("expected billing cycle monthly, got %s", sub.BillingCycle)
	}
}

func TestActivateTrialSubscriptionYearly(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// Create trial subscription
	service.CreateTrialSubscription(ctx, 1, "based", 30)

	// Activate with yearly billing
	err := service.ActivateTrialSubscription(ctx, 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("failed to activate trial: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected billing cycle yearly, got %s", sub.BillingCycle)
	}

	// Verify period is 1 year
	expectedEnd := sub.CurrentPeriodStart.AddDate(1, 0, 0)
	if !sub.CurrentPeriodEnd.Truncate(time.Second).Equal(expectedEnd.Truncate(time.Second)) {
		t.Errorf("expected 1 year period for yearly billing")
	}
}

func TestActivateTrialSubscriptionAlreadyActive(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// Create active subscription (not trialing)
	service.CreateSubscription(ctx, 1, "based")

	// Try to activate - should do nothing (already active)
	err := service.ActivateTrialSubscription(ctx, 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status to remain active")
	}
}

func TestActivateTrialSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	err := service.ActivateTrialSubscription(ctx, 999, billing.BillingCycleMonthly)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ===========================================
// Freeze/Unfreeze Subscription Tests
// ===========================================

func TestFreezeSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	err := service.FreezeSubscription(ctx, 1)
	if err != nil {
		t.Fatalf("failed to freeze subscription: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusFrozen {
		t.Errorf("expected status frozen, got %s", sub.Status)
	}
	if sub.FrozenAt == nil {
		t.Error("expected FrozenAt to be set")
	}
}

func TestUnfreezeSubscriptionMonthly(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")
	service.FreezeSubscription(ctx, 1)

	err := service.UnfreezeSubscription(ctx, 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("failed to unfreeze subscription: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.BillingCycle != billing.BillingCycleMonthly {
		t.Errorf("expected billing cycle monthly, got %s", sub.BillingCycle)
	}
}

func TestUnfreezeSubscriptionYearly(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")
	service.FreezeSubscription(ctx, 1)

	err := service.UnfreezeSubscription(ctx, 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("failed to unfreeze subscription: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected status active, got %s", sub.Status)
	}
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected billing cycle yearly, got %s", sub.BillingCycle)
	}
}

func TestUnfreezeSubscriptionNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	err := service.UnfreezeSubscription(ctx, 999, billing.BillingCycleMonthly)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// ===========================================
// Auto-Renew Tests
// ===========================================

func TestSetAutoRenew(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	// Enable auto-renew
	err := service.SetAutoRenew(ctx, 1, true)
	if err != nil {
		t.Fatalf("failed to set auto-renew: %v", err)
	}

	sub, _ := service.GetSubscription(ctx, 1)
	if !sub.AutoRenew {
		t.Error("expected AutoRenew to be true")
	}

	// Disable auto-renew
	err = service.SetAutoRenew(ctx, 1, false)
	if err != nil {
		t.Fatalf("failed to disable auto-renew: %v", err)
	}

	sub, _ = service.GetSubscription(ctx, 1)
	if sub.AutoRenew {
		t.Error("expected AutoRenew to be false")
	}
}

func TestUpdateSubscriptionSamePlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "based")

	// Update to same plan
	sub, err := service.UpdateSubscription(ctx, 1, "based")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Plan.Name != "based" {
		t.Errorf("expected plan 'based', got %s", sub.Plan.Name)
	}
}
