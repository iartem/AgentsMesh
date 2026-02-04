package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// ===========================================
// Extra Tests to Push Coverage Higher
// ===========================================

// TestGetInvoices_WithLimitAndOffset tests getting invoices with pagination
func TestGetInvoices_WithLimitAndOffset(t *testing.T) {
	svc, _ := setupTestService(t)

	invoices, err := svc.GetInvoicesByOrg(context.Background(), 1, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty (no invoices in test db)
	if len(invoices) != 0 {
		t.Errorf("expected 0 invoices, got %d", len(invoices))
	}
}

// TestCalculateRenewalPrice_PlanNotFound tests renewal with missing plan
func TestCalculateRenewalPrice_PlanNotFound(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             9999, // Non-existent plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	_, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestCalculateBillingCycleChangePrice_PlanNotFound tests cycle change with missing plan
func TestCalculateBillingCycleChangePrice_PlanNotFound(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             9999, // Non-existent plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	_, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleYearly)
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestGetUsage_DBError tests getting usage with database query error path
func TestGetUsage_EmptyResult(t *testing.T) {
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

	// No usage records, should return 0
	usage, err := svc.GetUsage(context.Background(), 1, "custom_type")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage != 0 {
		t.Errorf("expected 0 usage, got %f", usage)
	}
}

// TestCheckQuota_UsersQuotaExceeded tests users quota exceeded
func TestCheckQuota_UsersQuotaExceeded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan: max 1 user
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Add a member to use the only slot
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")

	err := svc.CheckQuota(context.Background(), 1, "users", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestGetSeatUsage_NotFound tests seat usage with no subscription
func TestGetSeatUsage_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.GetSeatUsage(context.Background(), 999)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestListPlansWithPrices_DBError tests listing plans when query fails
func TestListPlansWithPrices_NoPlans(t *testing.T) {
	svc, db := setupTestService(t)

	// Delete all plans
	db.Exec("DELETE FROM plan_prices")
	db.Exec("DELETE FROM subscription_plans")

	plans, err := svc.ListPlansWithPrices(context.Background(), "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plans) != 0 {
		t.Errorf("expected 0 plans, got %d", len(plans))
	}
}

// TestUpdateSubscription_SamePlanNoChange tests update to same plan
func TestUpdateSubscription_SamePlanNoChange(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Update to same plan (pro to pro) - should be treated as paid upgrade
	sub, err := svc.UpdateSubscription(context.Background(), 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return current plan
	if sub.PlanID != 2 {
		t.Errorf("expected plan ID 2, got %d", sub.PlanID)
	}
}

// TestCheckQuota_WithUnlimitedCustomQuota tests custom quota with -1 (unlimited)
// When custom quota is -1, the code skips custom quota check and falls back to plan limits
func TestCheckQuota_WithUnlimitedCustomQuota(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	customQuotas := billing.CustomQuotas{"users": float64(-1)} // Unlimited custom quota
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             3, // enterprise: unlimited users (-1)
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		CustomQuotas:       customQuotas,
	})

	// With custom quota -1 and enterprise plan (unlimited), should pass
	err := svc.CheckQuota(context.Background(), 1, "users", 1000)
	if err != nil {
		t.Errorf("expected nil for unlimited quota, got %v", err)
	}
}

// TestActivateTrialSubscription_AlreadyActive tests trial activation when already active
func TestActivateTrialSubscription_AlreadyActive(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive, // Already active
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Should do nothing and return nil
	err := svc.ActivateTrialSubscription(context.Background(), 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Errorf("expected nil for already active subscription, got %v", err)
	}
}

// TestActivateTrialSubscription_YearlyCycle tests activating trial with yearly cycle
func TestActivateTrialSubscription_YearlyCycle(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusTrialing,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 0, 14),
		SeatCount:          1,
	})

	err := svc.ActivateTrialSubscription(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", sub.BillingCycle)
	}
}

// TestActivateTrialSubscription_DefaultCycle tests activating trial with default (monthly) cycle
func TestActivateTrialSubscription_DefaultCycle(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusTrialing,
		BillingCycle:       billing.BillingCycleYearly, // Will be changed to monthly
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 0, 14),
		SeatCount:          1,
	})

	// Empty billing cycle = default to monthly
	err := svc.ActivateTrialSubscription(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.BillingCycle != billing.BillingCycleMonthly {
		t.Errorf("expected monthly cycle, got %s", sub.BillingCycle)
	}
}

// TestSetAutoRenew_Disable tests disabling auto-renew
func TestSetAutoRenew_Disable(t *testing.T) {
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
		AutoRenew:          true,
	})

	// Disable auto-renew
	err := svc.SetAutoRenew(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.AutoRenew {
		t.Error("expected AutoRenew to be false")
	}
}

// TestSetCancelAtPeriodEnd_Enable tests setting cancel at period end flag
func TestSetCancelAtPeriodEnd_Enable(t *testing.T) {
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

	err := svc.SetCancelAtPeriodEnd(context.Background(), 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if !sub.CancelAtPeriodEnd {
		t.Error("expected CancelAtPeriodEnd to be true")
	}
}

// TestFreezeSubscription_Active tests freezing an active subscription
func TestFreezeSubscription_Active(t *testing.T) {
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

	err := svc.FreezeSubscription(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusFrozen {
		t.Errorf("expected frozen status, got %s", sub.Status)
	}
	if sub.FrozenAt == nil {
		t.Error("expected FrozenAt to be set")
	}
}

// TestUnfreezeSubscription_Yearly tests unfreezing with yearly billing
func TestUnfreezeSubscription_Yearly(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusFrozen,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		FrozenAt:           &now,
	})

	err := svc.UnfreezeSubscription(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", sub.Status)
	}
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", sub.BillingCycle)
	}
	if sub.FrozenAt != nil {
		t.Error("expected FrozenAt to be nil")
	}
}

// TestUnfreezeSubscription_DefaultCycle tests unfreezing with default (monthly) billing
func TestUnfreezeSubscription_DefaultCycle(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusFrozen,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          1,
		FrozenAt:           &now,
	})

	// Empty cycle = default to monthly
	err := svc.UnfreezeSubscription(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.BillingCycle != billing.BillingCycleMonthly {
		t.Errorf("expected monthly cycle, got %s", sub.BillingCycle)
	}
}
