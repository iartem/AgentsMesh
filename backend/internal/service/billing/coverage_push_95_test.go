package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Final Push to 95% Coverage
// ===========================================

// TestCalculateRenewalPrice_YearlyCycleWithZeroSeats tests renewal with yearly and zero seats
func TestCalculateRenewalPrice_YearlyCycleWithZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          0, // Zero seats edge case
	})

	result, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
	if result.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", result.BillingCycle)
	}
}

// TestGetUsage_SubscriptionNotFound tests getting usage without subscription
func TestGetUsage_SubscriptionNotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.GetUsage(context.Background(), 999, billing.UsageTypePodMinutes)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestCheckQuota_ConcurrentPods tests quota check for concurrent pods
func TestCheckQuota_ConcurrentPods(t *testing.T) {
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

	err := svc.CheckQuota(context.Background(), 1, "concurrent_pods", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestCheckQuota_Repositories tests quota check for repositories
func TestCheckQuota_Repositories(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro: max 100 repositories
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	err := svc.CheckQuota(context.Background(), 1, "repositories", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestCheckQuota_Runners tests quota check for runners
func TestCheckQuota_Runners(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro: max 10 runners
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	err := svc.CheckQuota(context.Background(), 1, "runners", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestGetSeatUsage_PlanNilPath tests when Plan is nil and needs loading
func TestGetSeatUsage_PlanLoadPath(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create subscription without preloading plan
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          3,
	}
	db.Create(sub)

	usage, err := svc.GetSeatUsage(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.TotalSeats != 3 {
		t.Errorf("expected 3 total seats, got %d", usage.TotalSeats)
	}
}

// TestHandlePaymentSucceeded_TransactionCreation tests transaction creation
func TestHandlePaymentSucceeded_FullFlow(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-FULL-FLOW",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Seats:           2,
		BillingCycle:    billing.BillingCycleMonthly,
		Amount:          39.98,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderStripe,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:         "evt_full_flow",
		EventType:       "checkout.session.completed",
		Provider:        billing.PaymentProviderStripe,
		OrderNo:         "ORD-FULL-FLOW",
		ExternalOrderNo: "cs_ext_123",
		Amount:          39.98,
		Currency:        "USD",
		RawPayload:      map[string]interface{}{"test": true},
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify transaction was created
	var tx billing.PaymentTransaction
	db.Where("webhook_event_id = ?", "evt_full_flow").First(&tx)
	if tx.ID == 0 {
		t.Error("expected transaction to be created")
	}
}

// TestCreateStripeCustomer_Disabled tests when stripe is disabled
func TestCreateStripeCustomer_Disabled(t *testing.T) {
	svc, _ := setupTestService(t)

	// Service created without Stripe enabled
	customerID, err := svc.CreateStripeCustomer(context.Background(), 1, "test@example.com", "Test User")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if customerID != "" {
		t.Errorf("expected empty customer ID when Stripe disabled, got %s", customerID)
	}
}

// TestCheckSeatAvailability_WithMembers tests seat check with members
func TestCheckSeatAvailability_WithMembers(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          3, // 3 seats total
	})

	// Add members (uses 2 seats)
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 2, 'member')")

	// Should have 3 - 2 = 1 available seat
	err := svc.CheckSeatAvailability(context.Background(), 1, 1)
	if err != nil {
		t.Errorf("expected nil for 1 seat, got %v", err)
	}

	// Should fail when requesting more than available
	err = svc.CheckSeatAvailability(context.Background(), 1, 2)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded for 2 seats, got %v", err)
	}
}

// TestCheckSeatAvailability_NoSubscription tests default behavior without subscription
func TestCheckSeatAvailability_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	// No subscription = 1 default seat
	err := svc.CheckSeatAvailability(context.Background(), 999, 1)
	if err != nil {
		t.Errorf("expected nil for first seat, got %v", err)
	}

	// More than 1 should fail
	err = svc.CheckSeatAvailability(context.Background(), 999, 2)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestSetCustomQuota tests setting custom quota
func TestSetCustomQuota_Success(t *testing.T) {
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

	err := svc.SetCustomQuota(context.Background(), 1, "users", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify custom quota was set
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.CustomQuotas == nil || sub.CustomQuotas["users"] != float64(100) {
		t.Error("expected custom quota to be set")
	}
}

// TestGetCurrentConcurrentPods_Empty tests getting concurrent pods count when empty
func TestGetCurrentConcurrentPods_Empty(t *testing.T) {
	svc, _ := setupTestService(t)

	count, err := svc.GetCurrentConcurrentPods(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return 0 (no pods)
	if count != 0 {
		t.Errorf("expected 0 pods, got %d", count)
	}
}

// TestRecordUsage tests recording usage
func TestRecordUsage_Success(t *testing.T) {
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

	err := svc.RecordUsage(context.Background(), 1, billing.UsageTypePodMinutes, 50.5, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify usage was recorded
	usage, _ := svc.GetUsage(context.Background(), 1, billing.UsageTypePodMinutes)
	if usage != 50.5 {
		t.Errorf("expected usage 50.5, got %f", usage)
	}
}

// TestCreateTrialSubscription_InvalidPlanName tests trial with invalid plan
func TestCreateTrialSubscription_InvalidPlanName(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CreateTrialSubscription(context.Background(), 1, "nonexistent", 14)
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestUpdateSubscription_ClearDowngradePlan tests clearing downgrade when upgrading
func TestUpdateSubscription_ClearDowngradeOnUpgrade(t *testing.T) {
	svc, db := setupTestService(t)

	// Seed free plan
	freePlan := seedFreePlan(t, db)

	now := time.Now()
	downgradePlan := "free"
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             freePlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
		DowngradeToPlan:    &downgradePlan,
	})

	// Upgrade from free to based - should clear downgrade
	sub, err := svc.UpdateSubscription(context.Background(), 1, "based")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.DowngradeToPlan != nil {
		t.Error("expected DowngradeToPlan to be cleared")
	}
}

// TestHandleSubscriptionCreated_SetCustomerID tests setting customer ID when nil
func TestHandleSubscriptionCreated_SetCustomerID(t *testing.T) {
	svc, db := setupTestService(t)

	lsCustID := "ls_cust_set"
	now := time.Now()
	// Subscription with customer ID but no subscription ID
	db.Create(&billing.Subscription{
		OrganizationID:         1,
		PlanID:                 2,
		Status:                 billing.SubscriptionStatusActive,
		BillingCycle:           billing.BillingCycleMonthly,
		CurrentPeriodStart:     now,
		CurrentPeriodEnd:       now.AddDate(0, 1, 0),
		SeatCount:              1,
		LemonSqueezyCustomerID: &lsCustID,
		// LemonSqueezySubscriptionID is nil
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_set_cust_id",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_set_cust",
		CustomerID:     lsCustID,
	}

	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.LemonSqueezySubscriptionID == nil || *sub.LemonSqueezySubscriptionID != "ls_sub_set_cust" {
		t.Error("expected subscription ID to be set")
	}
}
