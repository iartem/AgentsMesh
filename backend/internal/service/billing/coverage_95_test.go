package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Tests to Push Coverage to 95%
// ===========================================

// TestGetUsage_WithActualUsageRecords tests getting usage with actual records
func TestGetUsage_WithActualUsageRecords(t *testing.T) {
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

	// Create usage record within period
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       150.5,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	})

	usage, err := svc.GetUsage(context.Background(), 1, billing.UsageTypePodMinutes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage != 150.5 {
		t.Errorf("expected usage 150.5, got %f", usage)
	}
}

// TestGetInvoices_WithLimit tests getting invoices with limit
func TestGetInvoices_WithLimit(t *testing.T) {
	svc, _ := setupTestService(t)

	invoices, err := svc.GetInvoicesByOrg(context.Background(), 1, 5, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty list (no invoices)
	if len(invoices) != 0 {
		t.Errorf("expected 0 invoices, got %d", len(invoices))
	}
}

// TestGetInvoices_WithoutLimit tests getting invoices without limit
func TestGetInvoices_WithoutLimit(t *testing.T) {
	svc, _ := setupTestService(t)

	invoices, err := svc.GetInvoicesByOrg(context.Background(), 1, 0, 0) // No limit
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty list (no invoices)
	if len(invoices) != 0 {
		t.Errorf("expected 0 invoices, got %d", len(invoices))
	}
}

// TestHandlePaymentSucceeded_NoOrderFound tests when order is not found
func TestHandlePaymentSucceeded_NoOrderFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_no_order_no_sub",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "nonexistent_order",
		Amount:    19.99,
		Currency:  "USD",
	}

	// Should return error since order not found and no subscription_id
	err := svc.HandlePaymentSucceeded(c, event)
	if err == nil {
		t.Error("expected error when order not found")
	}
}

// TestCheckAndMarkWebhookProcessed_DBError tests idempotency with non-duplicate error
func TestCheckAndMarkWebhookProcessed_Success(t *testing.T) {
	svc, _ := setupTestService(t)

	ctx := context.Background()
	// First call should succeed
	err := svc.CheckAndMarkWebhookProcessed(ctx, "unique_event_id", billing.PaymentProviderLemonSqueezy, "test_event")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should return duplicate error
	err = svc.CheckAndMarkWebhookProcessed(ctx, "unique_event_id", billing.PaymentProviderLemonSqueezy, "test_event")
	if err != ErrWebhookAlreadyProcessed {
		t.Errorf("expected ErrWebhookAlreadyProcessed, got %v", err)
	}
}

// TestUpdateSubscription_StripeEnabledPath tests update with Stripe enabled (code path)
func TestUpdateSubscription_UpgradeStripeEnabled(t *testing.T) {
	svc, db := setupTestService(t)

	// Seed free plan
	freePlan := seedFreePlan(t, db)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             freePlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Upgrade from free to pro - should apply immediately (free has price 0)
	sub, err := svc.UpdateSubscription(context.Background(), 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have the new plan reference
	if sub.Plan == nil || sub.Plan.Name != "pro" {
		t.Error("expected plan to be updated to pro")
	}
}

// TestHandleSubscriptionCreated_CustomerIDNotFound tests when customer ID lookup fails
func TestHandleSubscriptionCreated_CustomerIDNotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_cust_not_found",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_new",
		CustomerID:     "nonexistent_customer",
		// No OrderNo fallback either
	}

	// Should complete without error (nothing found, nothing to update)
	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestHandleSubscriptionCreated_NonLemonSqueezy tests with non-LemonSqueezy provider
func TestHandleSubscriptionCreated_NonLemonSqueezy(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_stripe_created",
		EventType:      "customer.subscription.created",
		Provider:       billing.PaymentProviderStripe, // Not LemonSqueezy
		SubscriptionID: "sub_stripe_new",
	}

	// Should complete without error (only LemonSqueezy is handled)
	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestGetSeatUsage_WithMembers tests seat usage with actual members
func TestGetSeatUsage_WithMembers(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          10,
	})

	// Add some members
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 2, 'member')")

	usage, err := svc.GetSeatUsage(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.TotalSeats != 10 {
		t.Errorf("expected 10 total seats, got %d", usage.TotalSeats)
	}
	if usage.UsedSeats != 2 {
		t.Errorf("expected 2 used seats, got %d", usage.UsedSeats)
	}
	if usage.AvailableSeats != 8 {
		t.Errorf("expected 8 available seats, got %d", usage.AvailableSeats)
	}
	if !usage.CanAddSeats {
		t.Error("expected CanAddSeats to be true for pro plan")
	}
}

// TestCheckQuota_PodMinutes tests pod minutes quota check
func TestCheckQuota_PodMinutes(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro plan: 1000 pod minutes
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Should pass with low pod minutes request
	err := svc.CheckQuota(context.Background(), 1, "pod_minutes", 100)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestCheckQuota_WithNilPlan tests quota when plan reference is nil
func TestCheckQuota_WithPreloadedPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
	}
	db.Create(sub)

	// The plan will be loaded from DB if not preloaded
	err := svc.CheckQuota(context.Background(), 1, "users", 1)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestHandlePaymentFailed_WithOrderNo tests failed payment with order lookup
func TestHandlePaymentFailed_WithOrderNoNotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:      "evt_fail_no_order",
		EventType:    billing.WebhookEventLSPaymentFailed,
		Provider:     billing.PaymentProviderLemonSqueezy,
		OrderNo:      "nonexistent",
		FailedReason: "Card declined",
	}

	// Should return nil when order not found
	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestActivateSubscription_UpdateExisting tests updating existing subscription
func TestActivateSubscription_UpdateExisting(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(3) // enterprise
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-UPDATE-EXIST",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Seats:           3,
		BillingCycle:    billing.BillingCycleMonthly,
		Amount:          299.97,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		PaymentMethod:   func() *string { s := "card"; return &s }(),
		ExpiresAt:       &expiresAt,
	})

	// Create existing subscription
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_update_existing",
		EventType:      billing.WebhookEventLSPaymentSuccess,
		Provider:       billing.PaymentProviderLemonSqueezy,
		OrderNo:        "ORD-UPDATE-EXIST",
		CustomerID:     "ls_cust_update",
		SubscriptionID: "ls_sub_update",
		Amount:         299.97,
		Currency:       "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)

	// activateSubscription updates PlanID, BillingCycle, SeatCount and provider IDs
	// Verify the update happened - check LemonSqueezy IDs were set
	if sub.LemonSqueezyCustomerID == nil || *sub.LemonSqueezyCustomerID != "ls_cust_update" {
		t.Error("expected LemonSqueezy customer ID to be set")
	}
	if sub.LemonSqueezySubscriptionID == nil || *sub.LemonSqueezySubscriptionID != "ls_sub_update" {
		t.Error("expected LemonSqueezy subscription ID to be set")
	}
}

// TestListPlansWithPrices_WithMissingCurrency tests listing plans when some don't have prices
func TestListPlansWithPrices_MixedAvailability(t *testing.T) {
	svc, db := setupTestService(t)

	// Create a plan with only USD price (no CNY)
	plan := &billing.SubscriptionPlan{
		Name:                "usd_only",
		DisplayName:         "USD Only Plan",
		PricePerSeatMonthly: 5.99,
		PricePerSeatYearly:  59.99,
		IsActive:            true,
	}
	db.Create(plan)

	// Add only USD price
	db.Create(&billing.PlanPrice{
		PlanID:       plan.ID,
		Currency:     billing.CurrencyUSD,
		PriceMonthly: 5.99,
		PriceYearly:  59.99,
	})

	// List plans with CNY - should exclude the USD-only plan
	plans, err := svc.ListPlansWithPrices(context.Background(), "CNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that usd_only plan is not included
	for _, p := range plans {
		if p.Plan.Name == "usd_only" {
			t.Error("expected USD-only plan to be excluded from CNY list")
		}
	}
}

// TestRenewSubscriptionFromOrder_YearlyCycle tests renewal order with yearly cycle
func TestRenewSubscriptionFromOrder_YearlyCycle(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-RENEW-YEARLY",
		OrderType:       billing.OrderTypeRenewal,
		PlanID:          &planID,
		Amount:          199.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	periodEnd := now.AddDate(0, 0, -1) // About to expire
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now.AddDate(-1, 0, 0),
		CurrentPeriodEnd:   periodEnd,
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_renew_yearly",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-RENEW-YEARLY",
		Amount:    199.99,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)

	// Should have period extended by 1 year
	expectedEnd := periodEnd.AddDate(1, 0, 0)
	if sub.CurrentPeriodEnd.Before(expectedEnd.AddDate(0, 0, -1)) {
		t.Error("expected period to be extended by 1 year")
	}
}

// TestHandleSubscriptionCreated_UpdateWithNoCustomerID tests updating subscription IDs
func TestHandleSubscriptionCreated_UpdateCustomerIDToo(t *testing.T) {
	svc, db := setupTestService(t)

	lsCustID := "ls_cust_partial"
	now := time.Now()
	// Create subscription with only customer ID (no subscription ID)
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
		EventID:        "evt_sub_update_ids",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_new_id",
		CustomerID:     lsCustID,
	}

	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.LemonSqueezySubscriptionID == nil || *sub.LemonSqueezySubscriptionID != "ls_sub_new_id" {
		t.Error("expected subscription ID to be set")
	}
}
