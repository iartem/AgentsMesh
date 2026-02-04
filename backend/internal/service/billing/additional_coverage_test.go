package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Additional Coverage Tests for 95% Target
// ===========================================

// TestHandlePaymentSucceeded_SeatPurchase tests payment succeeded for seat purchase
func TestHandlePaymentSucceeded_SeatPurchase(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)
	seatCount := 3

	// Create payment order for seat purchase
	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-SEAT-001",
		OrderType:       billing.OrderTypeSeatPurchase,
		PlanID:          &planID,
		Seats:           seatCount,
		Amount:          59.97,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	// Create subscription
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_seat_purchase",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-SEAT-001",
		Amount:    59.97,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify seat count increased
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.SeatCount != 8 { // 5 + 3
		t.Errorf("expected seat count 8, got %d", sub.SeatCount)
	}
}

// TestHandlePaymentSucceeded_PlanUpgrade tests payment succeeded for plan upgrade
func TestHandlePaymentSucceeded_PlanUpgrade(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(3) // enterprise
	expiresAt := now.Add(time.Hour)

	// Create payment order for upgrade
	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-UPGRADE-001",
		OrderType:       billing.OrderTypePlanUpgrade,
		PlanID:          &planID,
		Amount:          79.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	// Create subscription on pro plan
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_plan_upgrade",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-UPGRADE-001",
		Amount:    79.99,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify plan upgraded
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.PlanID != 3 {
		t.Errorf("expected plan ID 3, got %d", sub.PlanID)
	}
}

// TestHandlePaymentSucceeded_Renewal tests payment succeeded for renewal
func TestHandlePaymentSucceeded_Renewal(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)

	// Create payment order for renewal
	db.Create(&billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-RENEW-001",
		OrderType:       billing.OrderTypeRenewal,
		PlanID:          &planID,
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusPending,
		PaymentProvider: billing.PaymentProviderLemonSqueezy,
		ExpiresAt:       &expiresAt,
	})

	// Create subscription that needs renewal
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now,
		SeatCount:          1,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_renewal",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
		OrderNo:   "ORD-RENEW-001",
		Amount:    19.99,
		Currency:  "USD",
	}

	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify period extended
	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.CurrentPeriodEnd.Before(now) {
		t.Error("expected period to be extended")
	}
}

// TestHandlePaymentFailed_ByExternalOrderNo tests payment failed lookup by external order no
func TestHandlePaymentFailed_ByExternalOrderNo(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	planID := int64(2)
	expiresAt := now.Add(time.Hour)
	externalNo := "ext_order_123"

	// Create payment order with external order no
	db.Create(&billing.PaymentOrder{
		OrganizationID:   1,
		OrderNo:          "ORD-EXT-001",
		ExternalOrderNo:  &externalNo,
		OrderType:        billing.OrderTypeSubscription,
		PlanID:           &planID,
		Amount:           19.99,
		Currency:         "USD",
		Status:           billing.OrderStatusPending,
		PaymentProvider:  billing.PaymentProviderLemonSqueezy,
		ExpiresAt:        &expiresAt,
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:         "evt_fail_ext",
		EventType:       billing.WebhookEventLSPaymentFailed,
		Provider:        billing.PaymentProviderLemonSqueezy,
		ExternalOrderNo: externalNo, // Use external order no instead of order_no
		FailedReason:    "Insufficient funds",
	}

	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify order status updated
	order, _ := svc.GetPaymentOrderByNo(context.Background(), "ORD-EXT-001")
	if order.Status != billing.OrderStatusFailed {
		t.Errorf("expected order status failed, got %s", order.Status)
	}
}

// TestHandleSubscriptionCanceled_Stripe tests cancellation with Stripe provider
func TestHandleSubscriptionCanceled_Stripe(t *testing.T) {
	svc, db := setupTestService(t)

	stripeSubID := "sub_stripe_test"
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
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_stripe_cancel_test",
		EventType:      "customer.subscription.deleted",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: stripeSubID,
	}

	err := svc.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusCanceled {
		t.Errorf("expected canceled status, got %s", sub.Status)
	}
}

// TestCalculateSubscriptionPriceWithCurrency_CNY tests pricing in CNY
func TestCalculateSubscriptionPriceWithCurrency_CNY(t *testing.T) {
	svc, _ := setupTestService(t)

	price, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "CNY", billing.BillingCycleMonthly, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Currency != "CNY" {
		t.Errorf("expected CNY currency, got %s", price.Currency)
	}
	// Pro plan CNY monthly is 139
	if price.Amount != 139 {
		t.Errorf("expected amount 139, got %f", price.Amount)
	}
}

// TestGetInvoices_Empty tests getting invoices when none exist
func TestGetInvoices_Empty(t *testing.T) {
	svc, _ := setupTestService(t)

	invoices, err := svc.GetInvoicesByOrg(context.Background(), 1, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(invoices) != 0 {
		t.Errorf("expected empty invoices, got %d", len(invoices))
	}
}

// TestGetInvoices_WithData tests getting invoices with data
// Skipped: Invoice table schema has complex JSON fields that don't work well with SQLite test db
func TestGetInvoices_WithData(t *testing.T) {
	t.Skip("Invoice table has complex JSON fields incompatible with SQLite test db")
}

// TestSetNextBillingCycle tests setting next billing cycle
func TestSetNextBillingCycle_ToMonthly(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          1,
	})

	err := svc.SetNextBillingCycle(context.Background(), 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.NextBillingCycle == nil || *sub.NextBillingCycle != billing.BillingCycleMonthly {
		t.Error("expected NextBillingCycle to be set to monthly")
	}
}

// TestCalculateUpgradePrice_CurrentPlanNotFound tests upgrade with invalid current plan
func TestCalculateUpgradePrice_CurrentPlanNotFound(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	// Create subscription with non-existent plan ID
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             9999, // non-existent
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	_, err := svc.CalculateUpgradePrice(context.Background(), 1, "enterprise")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestCheckQuota_ZeroLimit tests quota check with 0 limit (disabled quota type)
func TestCheckQuota_ZeroLimit(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan: max_users=1, but other quotas may be 0
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Check a quota type that isn't explicitly limited
	err := svc.CheckQuota(context.Background(), 1, "nonexistent_quota_type", 100)
	// Should pass because unknown quota types default to no limit
	if err != nil {
		t.Errorf("expected no error for unknown quota type, got %v", err)
	}
}

// TestGetSeatUsage_NoPlan tests seat usage when plan is not loaded
func TestGetSeatUsage_NoPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
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
}

// TestCalculateSubscriptionPriceWithCurrency_ZeroSeats tests pricing with zero seats (defaults to 1)
func TestCalculateSubscriptionPriceWithCurrency_ZeroSeats(t *testing.T) {
	svc, _ := setupTestService(t)

	price, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "USD", billing.BillingCycleMonthly, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", price.Seats)
	}
}

// TestCalculateSubscriptionPriceWithCurrency_NegativeSeats tests pricing with negative seats
func TestCalculateSubscriptionPriceWithCurrency_NegativeSeats(t *testing.T) {
	svc, _ := setupTestService(t)

	price, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "USD", billing.BillingCycleMonthly, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", price.Seats)
	}
}

// TestCalculateRenewalPrice_YearlyCycle tests renewal with yearly cycle
func TestCalculateRenewalPrice_YearlyCycle(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		SeatCount:          1,
	})

	// Calculate renewal with yearly cycle
	price, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use current yearly billing cycle
	if price.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", price.BillingCycle)
	}
}

// TestGetBillingOverview_WithPlanLoaded tests billing overview when plan is already loaded
func TestGetBillingOverview_WithPlanLoaded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          3,
	})

	overview, err := svc.GetBillingOverview(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.Plan == nil {
		t.Error("expected plan in overview")
	}
	if overview.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", overview.Status)
	}
}

// TestGetUsage_NoUsageRecords tests getting usage when no records exist
func TestGetUsage_NoUsageRecords(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now.AddDate(0, 0, -15),
		CurrentPeriodEnd:   now.AddDate(0, 0, 15),
		SeatCount:          1,
	})

	usage, err := svc.GetUsage(context.Background(), 1, "pod_minutes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage != 0 {
		t.Errorf("expected 0 usage, got %f", usage)
	}
}

// TestHandleSubscriptionCanceled_LemonSqueezy tests cancellation with LemonSqueezy
func TestHandleSubscriptionCanceled_LemonSqueezy(t *testing.T) {
	svc, db := setupTestService(t)

	lsSubID := "ls_sub_cancel_test"
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
		EventID:        "evt_ls_cancel_test",
		EventType:      billing.WebhookEventLSSubscriptionCancelled,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: lsSubID,
	}

	err := svc.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub, _ := svc.GetSubscription(context.Background(), 1)
	if sub.Status != billing.SubscriptionStatusCanceled {
		t.Errorf("expected canceled status, got %s", sub.Status)
	}
	if sub.CanceledAt == nil {
		t.Error("expected CanceledAt to be set")
	}
}

// TestCreateTrialSubscription_EnterprisePlan tests creating trial with enterprise plan
func TestCreateTrialSubscription_EnterprisePlan(t *testing.T) {
	svc, _ := setupTestService(t)

	sub, err := svc.CreateTrialSubscription(context.Background(), 1, "enterprise", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != billing.SubscriptionStatusTrialing {
		t.Errorf("expected trialing status, got %s", sub.Status)
	}
	// Trial plan should be enterprise
	if sub.PlanID != 3 {
		t.Errorf("expected enterprise plan (3), got %d", sub.PlanID)
	}
}
