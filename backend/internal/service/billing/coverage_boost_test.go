package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Coverage Boost Tests - Target 95%
// ===========================================

// TestHandleSubscriptionUpdated_VariousStatuses tests all status mappings
func TestHandleSubscriptionUpdated_VariousStatuses(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		expectedStatus string
	}{
		{"active", "active", billing.SubscriptionStatusActive},
		{"past_due", "past_due", billing.SubscriptionStatusPastDue},
		{"canceled", "canceled", billing.SubscriptionStatusCanceled},
		{"cancelled_uk", "cancelled", billing.SubscriptionStatusCanceled},
		{"trialing", "trialing", billing.SubscriptionStatusTrialing},
		{"paused", "paused", billing.SubscriptionStatusPaused},
		{"expired", "expired", billing.SubscriptionStatusExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, db := setupTestService(t)

			stripeSubID := "sub_stripe_status_" + tt.name
			now := time.Now()
			db.Create(&billing.Subscription{
				OrganizationID:       1,
				PlanID:               2,
				Status:               billing.SubscriptionStatusActive,
				BillingCycle:         billing.BillingCycleMonthly,
				CurrentPeriodStart:   now,
				CurrentPeriodEnd:     now.AddDate(0, 1, 0),
				SeatCount:            1,
				StripeSubscriptionID: &stripeSubID,
				FrozenAt:             &now, // Set frozen to test clearing
			})

			c, _ := createTestGinContext()
			event := &payment.WebhookEvent{
				EventID:        "evt_status_" + tt.name,
				EventType:      "customer.subscription.updated",
				Provider:       billing.PaymentProviderStripe,
				SubscriptionID: stripeSubID,
				Status:         tt.status,
			}

			err := svc.HandleSubscriptionUpdated(c, event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			sub, _ := svc.GetSubscription(context.Background(), 1)
			if sub.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, sub.Status)
			}

			// For active status, FrozenAt should be cleared
			if tt.status == "active" && sub.FrozenAt != nil {
				t.Error("expected FrozenAt to be cleared for active status")
			}
		})
	}
}

// TestHandleSubscriptionUpdated_LemonSqueezyStatus tests LemonSqueezy status mapping
func TestHandleSubscriptionUpdated_LemonSqueezyStatus(t *testing.T) {
	svc, db := setupTestService(t)

	lsSubID := "ls_sub_updated"
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
		EventID:        "evt_ls_updated",
		EventType:      billing.WebhookEventLSSubscriptionUpdated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: lsSubID,
		Status:         "active", // LemonSqueezy status
	}

	err := svc.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", sub.Status)
	}
}

// TestHandleSubscriptionUpdated_EmptySubID tests with empty subscription ID
func TestHandleSubscriptionUpdated_EmptySubID(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_empty_sub_updated",
		EventType:      "customer.subscription.updated",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: "",
	}

	err := svc.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Errorf("expected nil for empty subscription ID, got %v", err)
	}
}

// TestHandleSubscriptionUpdated_NotFound tests with non-existent subscription
func TestHandleSubscriptionUpdated_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_not_found_updated",
		EventType:      "customer.subscription.updated",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: "nonexistent",
	}

	err := svc.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Errorf("expected nil for non-existent subscription, got %v", err)
	}
}

// TestHandleSubscriptionCanceled_NotFound_Stripe tests cancellation with non-existent subscription via Stripe
func TestHandleSubscriptionCanceled_NotFound_Stripe(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_cancel_not_found_stripe",
		EventType:      "customer.subscription.deleted",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: "nonexistent_stripe",
	}

	err := svc.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Errorf("expected nil for non-existent subscription, got %v", err)
	}
}

// TestHandlePaymentFailed_NoOrderNoSubID tests failed payment with neither order nor subscription
func TestHandlePaymentFailed_NoOrderNoSubID(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:      "evt_fail_nothing",
		EventType:    billing.WebhookEventLSPaymentFailed,
		Provider:     billing.PaymentProviderLemonSqueezy,
		OrderNo:      "",
		FailedReason: "Card declined",
	}

	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected nil when nothing found, got %v", err)
	}
}

// TestHandlePaymentSucceeded_DefaultOrderType tests payment with unknown order type
func TestHandlePaymentSucceeded_UnknownOrderType(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-UNKNOWN-001",
		OrderType:       "unknown_type", // Unknown order type
		PlanID:          &planID,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_unknown_type",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-UNKNOWN-001",
		Amount:    19.99,
		Currency:  "USD",
	}

	// Should complete without error (falls through switch)
	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify order status still updated
	order, _ := svc.GetPaymentOrderByNo(context.Background(), "ORD-UNKNOWN-001")
	if order.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected order status succeeded, got %s", order.Status)
	}
}

// TestListPlansWithPrices_SkipsPlansWithoutPrice tests skipping plans without prices
func TestListPlansWithPrices_SkipsPlansWithoutPrice(t *testing.T) {
	svc, db := setupTestService(t)

	// Create a plan without any prices
	planWithoutPrice := &billing.SubscriptionPlan{
		Name:                "no_price_plan",
		DisplayName:         "No Price Plan",
		PricePerSeatMonthly: 0,
		PricePerSeatYearly:  0,
		IsActive:            true,
	}
	db.Create(planWithoutPrice)

	// List plans with prices in EUR (no plans have EUR prices)
	plans, err := svc.ListPlansWithPrices(context.Background(), "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip plans without EUR prices
	if len(plans) != 0 {
		t.Errorf("expected 0 plans with EUR prices, got %d", len(plans))
	}

	// List plans with USD prices (should include standard plans)
	plans, err = svc.ListPlansWithPrices(context.Background(), "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have the standard plans with USD prices
	if len(plans) == 0 {
		t.Error("expected some plans with USD prices")
	}
}

// TestHandleRecurringPaymentSuccess_WithPendingChanges tests applying pending changes
func TestHandleRecurringPaymentSuccess_WithPendingChanges(t *testing.T) {
	svc, db := setupTestService(t)

	lsSubID := "ls_sub_pending_changes"
	now := time.Now()
	downgradePlan := "based"
	nextCycle := billing.BillingCycleYearly

	db.Create(&billing.Subscription{
		OrganizationID:             1,
		PlanID:                     2, // pro
		Status:                     billing.SubscriptionStatusActive,
		BillingCycle:               billing.BillingCycleMonthly,
		CurrentPeriodStart:         now.AddDate(0, -1, 0),
		CurrentPeriodEnd:           now,
		SeatCount:                  1,
		LemonSqueezySubscriptionID: &lsSubID,
		DowngradeToPlan:            &downgradePlan,
		NextBillingCycle:           &nextCycle,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_recurring_pending",
		EventType:      billing.WebhookEventLSPaymentSuccess,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: lsSubID,
		Amount:         19.99,
		Currency:       "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)

	// Downgrade should be applied
	if sub.PlanID != 1 { // based plan
		t.Errorf("expected plan ID 1 (based), got %d", sub.PlanID)
	}

	// DowngradeToPlan should be cleared
	if sub.DowngradeToPlan != nil {
		t.Error("expected DowngradeToPlan to be cleared")
	}

	// NextBillingCycle should be applied
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", sub.BillingCycle)
	}

	// NextBillingCycle should be cleared
	if sub.NextBillingCycle != nil {
		t.Error("expected NextBillingCycle to be cleared")
	}
}

// TestHandleRecurringPaymentSuccess_NotFound tests recurring payment with non-existent subscription
func TestHandleRecurringPaymentSuccess_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_recurring_not_found",
		EventType:      billing.WebhookEventLSPaymentSuccess,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "nonexistent",
		Amount:         19.99,
		Currency:       "USD",
	}

	// Should return nil (subscription not found is ignored)
	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Errorf("expected nil for non-existent subscription, got %v", err)
	}
}

// TestHandleRecurringPaymentFailure_NotFound tests recurring failure with non-existent subscription
func TestHandleRecurringPaymentFailure_NotFound(t *testing.T) {
	svc, _ := setupTestService(t)

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_recurring_fail_not_found",
		EventType:      billing.WebhookEventLSPaymentFailed,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "nonexistent",
		FailedReason:   "Card declined",
	}

	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected nil for non-existent subscription, got %v", err)
	}
}

// TestActivateSubscription_NewSubscription tests creating new subscription from order
func TestActivateSubscription_NewSubscription(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  999, // New org with no subscription
		OrderNo:         "ORD-NEW-SUB-001",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Seats:           5,
		BillingCycle:    billing.BillingCycleYearly,
		Amount:          99.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_new_sub",
		EventType:      billing.WebhookEventLSPaymentSuccess,
		Provider:       billing.PaymentProviderLemonSqueezy,
		OrderNo:        "ORD-NEW-SUB-001",
		CustomerID:     "ls_cust_new",
		SubscriptionID: "ls_sub_new",
		Amount:         99.99,
		Currency:       "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify new subscription created
	sub, err := svc.GetSubscription(context.Background(), 999)
	if err != nil {
		t.Fatalf("subscription not created: %v", err)
	}

	if sub.PlanID != 2 {
		t.Errorf("expected plan ID 2, got %d", sub.PlanID)
	}
	if sub.SeatCount != 5 {
		t.Errorf("expected 5 seats, got %d", sub.SeatCount)
	}
	if sub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", sub.BillingCycle)
	}
	if sub.LemonSqueezyCustomerID == nil || *sub.LemonSqueezyCustomerID != "ls_cust_new" {
		t.Error("expected LemonSqueezy customer ID to be set")
	}
	if sub.LemonSqueezySubscriptionID == nil || *sub.LemonSqueezySubscriptionID != "ls_sub_new" {
		t.Error("expected LemonSqueezy subscription ID to be set")
	}
}

// TestActivateSubscription_StripeProvider tests setting Stripe IDs
func TestActivateSubscription_StripeProvider(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  998, // New org
		OrderNo:         "ORD-STRIPE-001",
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
		EventID:        "evt_stripe_new_sub",
		EventType:      "checkout.session.completed",
		Provider:       billing.PaymentProviderStripe,
		OrderNo:        "ORD-STRIPE-001",
		CustomerID:     "cus_stripe_new",
		SubscriptionID: "sub_stripe_new",
		Amount:         19.99,
		Currency:       "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 998)
	if sub.StripeCustomerID == nil || *sub.StripeCustomerID != "cus_stripe_new" {
		t.Error("expected Stripe customer ID to be set")
	}
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID != "sub_stripe_new" {
		t.Error("expected Stripe subscription ID to be set")
	}
}

// TestUpgradePlan_NilPlanID tests upgrade with nil plan ID
func TestUpgradePlan_NilPlanID(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	expiresAt := now.Add(time.Hour)

	// Create order without plan ID
	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-UPGRADE-NIL",
		OrderType:       billing.OrderTypePlanUpgrade,
		PlanID:          nil, // No plan ID
		Amount:          79.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_upgrade_nil",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-UPGRADE-NIL",
		Amount:    79.99,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan, got %v", err)
	}
}

// TestHandleSubscriptionCreated_AlreadySet tests when subscription ID already set
func TestHandleSubscriptionCreated_AlreadySet(t *testing.T) {
	svc, db := setupTestService(t)

	lsCustID := "ls_cust_existing"
	lsSubID := "ls_sub_existing"
	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:             1,
		PlanID:                     2,
		Status:                     billing.SubscriptionStatusActive,
		BillingCycle:               billing.BillingCycleMonthly,
		CurrentPeriodStart:         now,
		CurrentPeriodEnd:           now.AddDate(0, 1, 0),
		SeatCount:                  1,
		LemonSqueezyCustomerID:     &lsCustID,
		LemonSqueezySubscriptionID: &lsSubID, // Already set
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_sub_already_set",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_new_attempt",
		CustomerID:     lsCustID,
	}

	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify subscription ID was NOT changed (already set)
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if *sub.LemonSqueezySubscriptionID != lsSubID {
		t.Error("subscription ID should not have been changed")
	}
}

// TestGetDeploymentInfo_WithConfig tests deployment info with payment config
func TestGetDeploymentInfo_WithConfig(t *testing.T) {
	svc, _ := setupTestService(t)

	// Service created without config, so should return defaults
	info := svc.GetDeploymentInfo()
	if info.DeploymentType != "global" {
		t.Errorf("expected global deployment type, got %s", info.DeploymentType)
	}
}

// TestRenewSubscriptionFromOrder_NotFound tests renewal with non-existent subscription
func TestRenewSubscriptionFromOrder_NotFound(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	db.Create(&billing.PaymentOrder{
		OrganizationID:  999, // Non-existent org
		OrderNo:         "ORD-RENEW-NOTFOUND",
		OrderType:       billing.OrderTypeRenewal,
		PlanID:          &planID,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_renew_notfound",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-RENEW-NOTFOUND",
		Amount:    19.99,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestHandlePaymentSucceeded_ExternalOrderNoLookup tests finding order by external order no
func TestHandlePaymentSucceeded_ExternalOrderNoLookup(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)
	externalNo := "ext_order_lookup_123"

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-INT-001",
		ExternalOrderNo: &externalNo,
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:         "evt_ext_order_lookup",
		EventType:       billing.WebhookEventLSPaymentSuccess,
		Provider:        billing.PaymentProviderLemonSqueezy,
		OrderNo:         "",        // No internal order no
		ExternalOrderNo: externalNo, // Use external order no
		Amount:          19.99,
		Currency:        "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order, _ := svc.GetPaymentOrderByNo(context.Background(), "ORD-INT-001")
	if order.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected order status succeeded, got %s", order.Status)
	}
}

// TestHandlePaymentFailed_ExternalOrderNoLookup tests finding order by external order no for failure
func TestHandlePaymentFailed_ExternalOrderNoLookup(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)
	externalNo := "ext_fail_order_456"

	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-FAIL-INT-001",
		ExternalOrderNo: &externalNo,
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &planID,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:         "evt_ext_fail_lookup",
		EventType:       billing.WebhookEventLSPaymentFailed,
		Provider:        billing.PaymentProviderLemonSqueezy,
		OrderNo:         "",        // No internal order no
		ExternalOrderNo: externalNo, // Use external order no
		FailedReason:    "Card declined",
	}

	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order, _ := svc.GetPaymentOrderByNo(context.Background(), "ORD-FAIL-INT-001")
	if order.Status != billing.OrderStatusFailed {
		t.Errorf("expected order status failed, got %s", order.Status)
	}
}

// TestRecurringPaymentSuccess_YearlyCycle tests recurring payment with yearly billing
func TestRecurringPaymentSuccess_YearlyCycle(t *testing.T) {
	svc, db := setupTestService(t)

	lsSubID := "ls_sub_yearly_recurring"
	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:             1,
		PlanID:                     2,
		Status:                     billing.SubscriptionStatusActive,
		BillingCycle:               billing.BillingCycleYearly,
		CurrentPeriodStart:         now.AddDate(-1, 0, 0),
		CurrentPeriodEnd:           now,
		SeatCount:                  1,
		LemonSqueezySubscriptionID: &lsSubID,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_yearly_recurring",
		EventType:      billing.WebhookEventLSPaymentSuccess,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: lsSubID,
		Amount:         199.99,
		Currency:       "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)

	// Period should be extended by 1 year
	expectedEnd := now.AddDate(1, 0, 0)
	if sub.CurrentPeriodEnd.Before(expectedEnd.AddDate(0, 0, -1)) {
		t.Error("expected period to be extended by 1 year")
	}
}
