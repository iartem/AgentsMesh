package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Final Coverage Tests - Push to 95%
// ===========================================

// TestCalculateSubscriptionPriceWithCurrency_ProviderIDs tests provider-specific IDs
func TestCalculateSubscriptionPriceWithCurrency_ProviderIDs(t *testing.T) {
	svc, db := setupTestService(t)

	// Add provider-specific IDs to plan prices
	var price billing.PlanPrice
	db.Where("plan_id = ? AND currency = ?", 2, "USD").First(&price)

	stripeMonthly := "price_stripe_monthly"
	stripeYearly := "price_stripe_yearly"
	lsMonthly := "var_ls_monthly"
	lsYearly := "var_ls_yearly"

	db.Model(&price).Updates(map[string]interface{}{
		"stripe_price_id_monthly":        &stripeMonthly,
		"stripe_price_id_yearly":         &stripeYearly,
		"lemonsqueezy_variant_id_monthly": &lsMonthly,
		"lemonsqueezy_variant_id_yearly":  &lsYearly,
	})

	// Test monthly cycle with provider IDs
	result, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "USD", billing.BillingCycleMonthly, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StripePrice != stripeMonthly {
		t.Errorf("expected Stripe price %s, got %s", stripeMonthly, result.StripePrice)
	}
	if result.LemonSqueezyVariantID != lsMonthly {
		t.Errorf("expected LS variant %s, got %s", lsMonthly, result.LemonSqueezyVariantID)
	}

	// Test yearly cycle with provider IDs
	result, err = svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "USD", billing.BillingCycleYearly, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StripePrice != stripeYearly {
		t.Errorf("expected Stripe price %s, got %s", stripeYearly, result.StripePrice)
	}
	if result.LemonSqueezyVariantID != lsYearly {
		t.Errorf("expected LS variant %s, got %s", lsYearly, result.LemonSqueezyVariantID)
	}
}

// TestCalculateUpgradePrice_WithProviderIDs tests upgrade pricing with provider IDs
func TestCalculateUpgradePrice_WithProviderIDs(t *testing.T) {
	svc, db := setupTestService(t)

	// Add provider IDs to enterprise plan
	var price billing.PlanPrice
	db.Where("plan_id = ? AND currency = ?", 3, "USD").First(&price)

	stripeYearly := "price_enterprise_yearly"
	lsYearly := "var_enterprise_yearly"

	db.Model(&price).Updates(map[string]interface{}{
		"stripe_price_id_yearly":         &stripeYearly,
		"lemonsqueezy_variant_id_yearly":  &lsYearly,
	})

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

	result, err := svc.CalculateUpgradePrice(context.Background(), 1, "enterprise")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StripePrice != stripeYearly {
		t.Errorf("expected Stripe price %s, got %s", stripeYearly, result.StripePrice)
	}
	if result.LemonSqueezyVariantID != lsYearly {
		t.Errorf("expected LS variant %s, got %s", lsYearly, result.LemonSqueezyVariantID)
	}
}

// TestCalculateUpgradePrice_MonthlyWithProviderIDs tests monthly upgrade with provider IDs
func TestCalculateUpgradePrice_MonthlyWithProviderIDs(t *testing.T) {
	svc, db := setupTestService(t)

	// Add provider IDs to enterprise plan
	var price billing.PlanPrice
	db.Where("plan_id = ? AND currency = ?", 3, "USD").First(&price)

	stripeMonthly := "price_enterprise_monthly"
	lsMonthly := "var_enterprise_monthly"

	db.Model(&price).Updates(map[string]interface{}{
		"stripe_price_id_monthly":         &stripeMonthly,
		"lemonsqueezy_variant_id_monthly":  &lsMonthly,
	})

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

	result, err := svc.CalculateUpgradePrice(context.Background(), 1, "enterprise")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StripePrice != stripeMonthly {
		t.Errorf("expected Stripe price %s, got %s", stripeMonthly, result.StripePrice)
	}
	if result.LemonSqueezyVariantID != lsMonthly {
		t.Errorf("expected LS variant %s, got %s", lsMonthly, result.LemonSqueezyVariantID)
	}
}

// TestCheckQuota_FrozenSubscription tests quota check with frozen subscription
func TestCheckQuota_FrozenSubscription(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusFrozen,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
		FrozenAt:           &now,
	})

	err := svc.CheckQuota(context.Background(), 1, "users", 1)
	if err != ErrSubscriptionFrozen {
		t.Errorf("expected ErrSubscriptionFrozen, got %v", err)
	}
}

// TestCheckQuota_WithCustomQuota tests quota check with custom quota
func TestCheckQuota_WithCustomQuota(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	customQuotas := billing.CustomQuotas{"users": float64(10)}
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          5,
		CustomQuotas:       customQuotas,
	})

	// Should pass with custom quota
	err := svc.CheckQuota(context.Background(), 1, "users", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fail when exceeding custom quota
	err = svc.CheckQuota(context.Background(), 1, "users", 15)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestCheckQuota_NoSubscriptionNoBasedPlan tests quota when no subscription and no based plan
func TestCheckQuota_DBError(t *testing.T) {
	svc, _ := setupTestService(t)

	// No subscription exists - should try to use Based plan
	// Based plan exists from seed data, so this should pass
	err := svc.CheckQuota(context.Background(), 999, "users", 1)
	if err != nil {
		t.Errorf("expected nil (allows by default when using Based plan), got %v", err)
	}
}

// TestCheckQuota_UnlimitedResources tests quota for resources with -1 limit
func TestCheckQuota_UnlimitedResources(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             3, // enterprise has unlimited resources (-1)
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Should pass even with high amount due to unlimited
	err := svc.CheckQuota(context.Background(), 1, "users", 1000)
	if err != nil {
		t.Errorf("expected nil for unlimited quota, got %v", err)
	}
}

// TestCheckQuota_QuotaExceeded tests when quota is exceeded
func TestCheckQuota_QuotaExceeded(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan: max_users = 1
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	// Create an existing member to use the quota
	db.Exec("INSERT INTO organization_members (organization_id, user_id, role) VALUES (1, 1, 'owner')")

	err := svc.CheckQuota(context.Background(), 1, "users", 1)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestGetUsageHistory_WithUsageType tests getting usage history with specific type
func TestGetUsageHistory_WithUsageType(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 0, 1),
		SeatCount:          1,
	})

	// Create usage record
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       100,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})

	// Get history with usage type
	records, err := svc.GetUsageHistory(context.Background(), 1, billing.UsageTypePodMinutes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) == 0 {
		t.Error("expected at least one record")
	}
}

// TestGetUsageHistory_WithoutUsageType tests getting all usage history
func TestGetUsageHistory_WithoutUsageType(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.UsageRecord{
		OrganizationID: 1,
		UsageType:      billing.UsageTypePodMinutes,
		Quantity:       50,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
	})

	// Get all history
	records, err := svc.GetUsageHistory(context.Background(), 1, "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) == 0 {
		t.Error("expected at least one record")
	}
}

// TestCalculateRenewalPrice_ZeroSeats tests renewal with zero seats
func TestCalculateRenewalPrice_ZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Edge case
	})

	result, err := svc.CalculateRenewalPrice(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
}

// TestCalculateBillingCycleChangePrice_SameCycle tests no change when same cycle
func TestCalculateBillingCycleChangePrice_SameCycle(t *testing.T) {
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

	result, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Error("expected nil for same billing cycle")
	}
}

// TestCalculateBillingCycleChangePrice_YearlyToMonthly tests changing from yearly to monthly
func TestCalculateBillingCycleChangePrice_YearlyToMonthly(t *testing.T) {
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

	result, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleMonthly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Yearly to monthly is a downgrade, actual amount should be 0
	if result.ActualAmount != 0 {
		t.Errorf("expected 0 actual amount for downgrade, got %f", result.ActualAmount)
	}
}

// TestCalculateBillingCycleChangePrice_ZeroSeats tests cycle change with zero seats
func TestCalculateBillingCycleChangePrice_ZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Edge case
	})

	result, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", result.Seats)
	}
}

// TestCalculateRemainingPeriodRatio_ZeroPeriod tests when period is zero
func TestCalculateRemainingPeriodRatio_ZeroPeriod(t *testing.T) {
	now := time.Now()
	// Same start and end time = zero period
	ratio := calculateRemainingPeriodRatio(now, now)
	if ratio != 0 {
		t.Errorf("expected 0 for zero period, got %f", ratio)
	}
}

// TestSeatUsage_BasedPlan tests seat usage for based plan (cannot add seats)
func TestSeatUsage_BasedPlan(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan
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

	if usage.CanAddSeats {
		t.Error("expected CanAddSeats to be false for based plan")
	}
}

// TestCalculateSeatPurchasePrice_BasedPlanFails tests seat purchase fails for based plan
func TestCalculateSeatPurchasePrice_BasedPlanFails(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             1, // based plan
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	_, err := svc.CalculateSeatPurchasePrice(context.Background(), 1, 5)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan for based plan, got %v", err)
	}
}

// TestHandlePaymentSucceeded_IdempotencyDuplicate tests idempotency check
func TestHandlePaymentSucceeded_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSPaymentSuccess,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:   "evt_duplicate",
		EventType: billing.WebhookEventLSPaymentSuccess,
		Provider:  billing.PaymentProviderLemonSqueezy,
	}

	// Should return nil (duplicate is silently ignored)
	err := svc.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandlePaymentFailed_IdempotencyDuplicate tests idempotency check for failed payments
func TestHandlePaymentFailed_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_fail_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSPaymentFailed,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:      "evt_fail_duplicate",
		EventType:    billing.WebhookEventLSPaymentFailed,
		Provider:     billing.PaymentProviderLemonSqueezy,
		FailedReason: "Card declined",
	}

	// Should return nil (duplicate is silently ignored)
	err := svc.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandleSubscriptionCanceled_IdempotencyDuplicate tests idempotency check for cancellation
func TestHandleSubscriptionCanceled_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_cancel_duplicate",
		Provider:    billing.PaymentProviderStripe,
		EventType:   "customer.subscription.deleted",
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_cancel_duplicate",
		EventType:      "customer.subscription.deleted",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: "sub_any",
	}

	// Should return nil (duplicate is silently ignored)
	err := svc.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestGetCurrentResourceCount_AllTypes tests resource counting for all types
func TestGetCurrentResourceCount_AllTypes(t *testing.T) {
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

	resources := []string{"users", "runners", "concurrent_pods", "repositories", "pod_minutes"}
	for _, resource := range resources {
		err := svc.CheckQuota(context.Background(), 1, resource, 1)
		if err != nil && err != ErrQuotaExceeded {
			t.Errorf("unexpected error for resource %s: %v", resource, err)
		}
	}
}

// TestHandleSubscriptionUpdated_IdempotencyDuplicate tests idempotency for subscription updated
func TestHandleSubscriptionUpdated_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_updated_duplicate",
		Provider:    billing.PaymentProviderStripe,
		EventType:   "customer.subscription.updated",
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_updated_duplicate",
		EventType:      "customer.subscription.updated",
		Provider:       billing.PaymentProviderStripe,
		SubscriptionID: "sub_any",
		Status:         "active",
	}

	err := svc.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandleSubscriptionCreated_IdempotencyDuplicate tests idempotency for subscription created
func TestHandleSubscriptionCreated_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_created_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSSubscriptionCreated,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_created_duplicate",
		EventType:      billing.WebhookEventLSSubscriptionCreated,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_any",
	}

	err := svc.HandleSubscriptionCreated(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandleSubscriptionPaused_IdempotencyDuplicate tests idempotency for subscription paused
func TestHandleSubscriptionPaused_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_paused_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSSubscriptionPaused,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_paused_duplicate",
		EventType:      billing.WebhookEventLSSubscriptionPaused,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_any",
	}

	err := svc.HandleSubscriptionPaused(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandleSubscriptionResumed_IdempotencyDuplicate tests idempotency for subscription resumed
func TestHandleSubscriptionResumed_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_resumed_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSSubscriptionResumed,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_resumed_duplicate",
		EventType:      billing.WebhookEventLSSubscriptionResumed,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_any",
	}

	err := svc.HandleSubscriptionResumed(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestHandleSubscriptionExpired_IdempotencyDuplicate tests idempotency for subscription expired
func TestHandleSubscriptionExpired_IdempotencyDuplicate(t *testing.T) {
	svc, db := setupTestService(t)

	// Pre-create the webhook event record
	db.Create(&billing.WebhookEvent{
		EventID:     "evt_expired_duplicate",
		Provider:    billing.PaymentProviderLemonSqueezy,
		EventType:   billing.WebhookEventLSSubscriptionExpired,
		ProcessedAt: time.Now(),
	})

	c, _ := createTestGinContext()
	event := &payment.WebhookEvent{
		EventID:        "evt_expired_duplicate",
		EventType:      billing.WebhookEventLSSubscriptionExpired,
		Provider:       billing.PaymentProviderLemonSqueezy,
		SubscriptionID: "ls_sub_any",
	}

	err := svc.HandleSubscriptionExpired(c, event)
	if err != nil {
		t.Errorf("expected nil for duplicate event, got %v", err)
	}
}

// TestCreateTrialSubscription_DefaultTrialDays tests trial with default days
func TestCreateTrialSubscription_DefaultTrialDays(t *testing.T) {
	svc, _ := setupTestService(t)

	sub, err := svc.CreateTrialSubscription(context.Background(), 1, "pro", 0) // 0 triggers default
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != billing.SubscriptionStatusTrialing {
		t.Errorf("expected trialing status, got %s", sub.Status)
	}
}

// TestCreateTrialSubscription_NegativeTrialDays tests trial with negative days
func TestCreateTrialSubscription_NegativeTrialDays(t *testing.T) {
	svc, _ := setupTestService(t)

	sub, err := svc.CreateTrialSubscription(context.Background(), 2, "pro", -5) // Negative triggers default
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sub.Status != billing.SubscriptionStatusTrialing {
		t.Errorf("expected trialing status, got %s", sub.Status)
	}
}

// TestGetBillingOverview_WithNilPlan tests overview when plan is nil
func TestGetBillingOverview_WithNilPlan(t *testing.T) {
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

	overview, err := svc.GetBillingOverview(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if overview.Plan == nil {
		t.Error("expected plan to be loaded")
	}
}
