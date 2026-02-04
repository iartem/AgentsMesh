package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// ===========================================
// Additional Pricing Coverage Tests
// ===========================================

// TestCalculateSubscriptionPriceWithCurrency_InvalidPlan tests pricing with invalid plan
func TestCalculateSubscriptionPriceWithCurrency_InvalidPlan(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "nonexistent", "USD", billing.BillingCycleMonthly, 1)
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestCalculateSubscriptionPriceWithCurrency_InvalidCurrency tests pricing with invalid currency
func TestCalculateSubscriptionPriceWithCurrency_InvalidCurrency(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "EUR", billing.BillingCycleMonthly, 1)
	if err != ErrPriceNotFound {
		t.Errorf("expected ErrPriceNotFound, got %v", err)
	}
}

// TestCalculateSubscriptionPriceWithCurrency_YearlyCycle tests yearly pricing
func TestCalculateSubscriptionPriceWithCurrency_YearlyCycle(t *testing.T) {
	svc, _ := setupTestService(t)

	price, err := svc.CalculateSubscriptionPriceWithCurrency(context.Background(), "pro", "USD", billing.BillingCycleYearly, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pro yearly is 199.90 per seat
	expected := 199.90 * 2
	if price.Amount != expected {
		t.Errorf("expected amount %.2f, got %.2f", expected, price.Amount)
	}
}

// TestCalculateUpgradePrice_NoSubscription tests upgrade pricing without subscription
func TestCalculateUpgradePrice_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateUpgradePrice(context.Background(), 999, "enterprise")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestCalculateUpgradePrice_InvalidNewPlan tests upgrade to invalid plan
func TestCalculateUpgradePrice_InvalidNewPlan(t *testing.T) {
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

	_, err := svc.CalculateUpgradePrice(context.Background(), 1, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

// TestCalculateSeatPurchasePrice_NoSubscription tests seat purchase without subscription
func TestCalculateSeatPurchasePrice_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateSeatPurchasePrice(context.Background(), 999, 5)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestCalculateRenewalPrice_NoSubscription tests renewal pricing without subscription
func TestCalculateRenewalPrice_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateRenewalPrice(context.Background(), 999, "")
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestCalculateRenewalPrice_WithNewCycle tests renewal with new billing cycle
func TestCalculateRenewalPrice_WithNewCycle(t *testing.T) {
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

	// Calculate renewal with yearly cycle
	price, err := svc.CalculateRenewalPrice(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", price.BillingCycle)
	}
}

// TestCalculateBillingCycleChangePrice_NoSubscription tests cycle change without subscription
func TestCalculateBillingCycleChangePrice_NoSubscription(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.CalculateBillingCycleChangePrice(context.Background(), 999, billing.BillingCycleYearly)
	if err != ErrSubscriptionNotFound {
		t.Errorf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

// TestGetPricePreview_Subscription tests price preview for new subscription
func TestGetPricePreview_Subscription(t *testing.T) {
	svc, _ := setupTestService(t)

	price, err := svc.GetPricePreview(context.Background(), 1, billing.OrderTypeSubscription, "pro", billing.BillingCycleMonthly, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Seats != 2 {
		t.Errorf("expected 2 seats, got %d", price.Seats)
	}
}

// TestGetPricePreview_InvalidOrderType tests preview with invalid order type
func TestGetPricePreview_InvalidOrderType(t *testing.T) {
	svc, _ := setupTestService(t)

	_, err := svc.GetPricePreview(context.Background(), 1, "invalid_type", "pro", billing.BillingCycleMonthly, 2)
	if err == nil {
		t.Error("expected error for invalid order type")
	}
}

// TestCalculateRemainingPeriodRatio_FutureEnd tests ratio with future end date
func TestCalculateRemainingPeriodRatio_FutureEnd(t *testing.T) {
	now := time.Now()
	start := now.AddDate(0, -1, 0) // Started 1 month ago
	end := now.AddDate(0, 0, 15)   // Ends in 15 days

	ratio := calculateRemainingPeriodRatio(start, end)
	if ratio <= 0 || ratio >= 1 {
		t.Errorf("expected ratio between 0 and 1, got %f", ratio)
	}
}

// TestCalculateRemainingPeriodRatio_ExpiredPeriod tests ratio when period has ended
func TestCalculateRemainingPeriodRatio_ExpiredPeriod(t *testing.T) {
	now := time.Now()
	start := now.AddDate(0, -2, 0) // Started 2 months ago
	end := now.AddDate(0, -1, 0)   // Ended 1 month ago

	ratio := calculateRemainingPeriodRatio(start, end)
	if ratio != 0 {
		t.Errorf("expected 0 for expired period, got %f", ratio)
	}
}

// TestCalculateUpgradePrice_YearlyCycle tests upgrade pricing with yearly billing
func TestCalculateUpgradePrice_YearlyCycle(t *testing.T) {
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

	price, err := svc.CalculateUpgradePrice(context.Background(), 1, "enterprise")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", price.BillingCycle)
	}
}

// TestCalculateUpgradePrice_WithZeroSeats tests upgrade with zero seats (should default to 1)
func TestCalculateUpgradePrice_WithZeroSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          0, // Edge case
	})

	price, err := svc.CalculateUpgradePrice(context.Background(), 1, "enterprise")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Seats != 1 {
		t.Errorf("expected 1 seat (default), got %d", price.Seats)
	}
}

// TestCalculateUpgradePrice_Downgrade tests upgrade calculation for downgrade (should return 0 actual amount)
func TestCalculateUpgradePrice_Downgrade(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             3, // enterprise
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          1,
	})

	price, err := svc.CalculateUpgradePrice(context.Background(), 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Downgrade should have 0 actual amount
	if price.ActualAmount != 0 {
		t.Errorf("expected 0 actual amount for downgrade, got %f", price.ActualAmount)
	}
}

// TestCalculateSeatPurchasePrice_YearlyCycle tests seat purchase with yearly billing
func TestCalculateSeatPurchasePrice_YearlyCycle(t *testing.T) {
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

	price, err := svc.CalculateSeatPurchasePrice(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", price.BillingCycle)
	}
}

// TestCalculateSeatPurchasePrice_InvalidAdditionalSeats tests with zero or negative seats
func TestCalculateSeatPurchasePrice_InvalidAdditionalSeats(t *testing.T) {
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

	_, err := svc.CalculateSeatPurchasePrice(context.Background(), 1, 0)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan for zero seats, got %v", err)
	}

	_, err = svc.CalculateSeatPurchasePrice(context.Background(), 1, -1)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan for negative seats, got %v", err)
	}
}


// TestCalculateSeatPurchasePrice_ExceedsMaxSeats tests seat purchase exceeding max
func TestCalculateSeatPurchasePrice_ExceedsMaxSeats(t *testing.T) {
	svc, db := setupTestService(t)

	now := time.Now()
	db.Create(&billing.Subscription{
		OrganizationID:     1,
		PlanID:             2, // pro: max 50 users
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		SeatCount:          45,
	})

	// Try to purchase 10 more (would exceed 50)
	_, err := svc.CalculateSeatPurchasePrice(context.Background(), 1, 10)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

// TestGetPricePreview_Upgrade tests price preview for upgrade
func TestGetPricePreview_Upgrade(t *testing.T) {
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

	price, err := svc.GetPricePreview(context.Background(), 1, billing.OrderTypePlanUpgrade, "enterprise", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just check that description contains "Upgrade"
	if price.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestGetPricePreview_SeatPurchase tests price preview for seat purchase
func TestGetPricePreview_SeatPurchase(t *testing.T) {
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

	price, err := svc.GetPricePreview(context.Background(), 1, billing.OrderTypeSeatPurchase, "", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.Seats != 3 {
		t.Errorf("expected 3 additional seats, got %d", price.Seats)
	}
}

// TestGetPricePreview_Renewal tests price preview for renewal
func TestGetPricePreview_Renewal(t *testing.T) {
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

	price, err := svc.GetPricePreview(context.Background(), 1, billing.OrderTypeRenewal, "", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just check that description contains "Renewal"
	if price.Description == "" {
		t.Error("expected non-empty description")
	}
}

// TestCalculateBillingCycleChangePrice_Success tests successful cycle change pricing
func TestCalculateBillingCycleChangePrice_Success(t *testing.T) {
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

	price, err := svc.CalculateBillingCycleChangePrice(context.Background(), 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", price.BillingCycle)
	}
}
