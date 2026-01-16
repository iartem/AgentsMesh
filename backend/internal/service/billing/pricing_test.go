package billing

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// almostEqual compares two floats with a tolerance for floating point precision issues
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestCalculateSubscriptionPrice(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)    // free plan: 0/month
	seedProPlan(t, db)     // pro plan: 19.99/month, 199.90/year

	tests := []struct {
		name         string
		planName     string
		billingCycle string
		seats        int
		wantAmount   float64
		wantErr      bool
	}{
		{
			name:         "monthly pro with 1 seat",
			planName:     "pro",
			billingCycle: billing.BillingCycleMonthly,
			seats:        1,
			wantAmount:   19.99,
		},
		{
			name:         "yearly pro with 1 seat",
			planName:     "pro",
			billingCycle: billing.BillingCycleYearly,
			seats:        1,
			wantAmount:   199.90,
		},
		{
			name:         "monthly pro with 5 seats",
			planName:     "pro",
			billingCycle: billing.BillingCycleMonthly,
			seats:        5,
			wantAmount:   99.95,
		},
		{
			name:         "free plan",
			planName:     "free",
			billingCycle: billing.BillingCycleMonthly,
			seats:        1,
			wantAmount:   0,
		},
		{
			name:         "invalid plan",
			planName:     "nonexistent",
			billingCycle: billing.BillingCycleMonthly,
			seats:        1,
			wantErr:      true,
		},
		{
			name:         "zero seats defaults to 1",
			planName:     "pro",
			billingCycle: billing.BillingCycleMonthly,
			seats:        0,
			wantAmount:   19.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.CalculateSubscriptionPrice(ctx, tt.planName, tt.billingCycle, tt.seats)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !almostEqual(result.Amount, tt.wantAmount, 0.001) {
				t.Errorf("expected amount %f, got %f", tt.wantAmount, result.Amount)
			}
			if !almostEqual(result.Amount, result.ActualAmount, 0.001) {
				t.Errorf("expected ActualAmount to equal Amount for new subscription")
			}
		})
	}
}

func TestCalculateUpgradePrice(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)      // free plan
	seedProPlan(t, db)       // pro plan
	seedEnterprisePlan(t, db) // enterprise plan

	// Create a subscription halfway through the period
	freePlan, _ := service.GetPlan(ctx, "free")
	now := time.Now()
	periodStart := now.Add(-15 * 24 * time.Hour)  // Started 15 days ago
	periodEnd := now.Add(15 * 24 * time.Hour)     // Ends in 15 days

	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             freePlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          1,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
	}
	db.Create(sub)

	// Test upgrading from free to pro
	result, err := service.CalculateUpgradePrice(ctx, 1, "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Full price is 19.99, should be approximately half (prorated)
	if !almostEqual(result.Amount, 19.99, 0.001) {
		t.Errorf("expected full amount 19.99, got %f", result.Amount)
	}
	// ActualAmount should be roughly half (around 10, allowing for timing)
	if result.ActualAmount < 8 || result.ActualAmount > 12 {
		t.Errorf("expected prorated amount around 10, got %f", result.ActualAmount)
	}
}

func TestCalculateSeatPurchasePrice(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db)
	proPlan, _ := service.GetPlan(ctx, "pro")

	// Create a subscription with some time remaining
	now := time.Now()
	periodStart := now.Add(-10 * 24 * time.Hour)
	periodEnd := now.Add(20 * 24 * time.Hour) // 2/3 of period remaining

	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          1,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
	}
	db.Create(sub)

	// Test purchasing 2 additional seats
	result, err := service.CalculateSeatPurchasePrice(ctx, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Full price for 2 seats: 19.99 * 2 = 39.98
	expectedFullPrice := 39.98
	if !almostEqual(result.Amount, expectedFullPrice, 0.001) {
		t.Errorf("expected full amount %f, got %f", expectedFullPrice, result.Amount)
	}

	// Prorated should be about 2/3 of full price
	if result.ActualAmount < 24 || result.ActualAmount > 30 {
		t.Errorf("expected prorated amount around 26.65, got %f", result.ActualAmount)
	}
}

func TestCalculateSeatPurchasePrice_FreePlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	service.CreateSubscription(ctx, 1, "free")

	// Should fail for free plan
	_, err := service.CalculateSeatPurchasePrice(ctx, 1, 1)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan, got %v", err)
	}
}

func TestCalculateSeatPurchasePrice_ExceedsMax(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db) // max_users = 50
	proPlan, _ := service.GetPlan(ctx, "pro")

	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          49, // Already have 49 seats
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Try to add 2 seats (would exceed 50)
	_, err := service.CalculateSeatPurchasePrice(ctx, 1, 2)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestCalculateRenewalPrice(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db)
	proPlan, _ := service.GetPlan(ctx, "pro")

	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          3,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Renew with same cycle
	result, err := service.CalculateRenewalPrice(ctx, 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 19.99 * 3 seats
	expectedAmount := 59.97
	if !almostEqual(result.Amount, expectedAmount, 0.001) {
		t.Errorf("expected amount %f, got %f", expectedAmount, result.Amount)
	}
	if result.BillingCycle != billing.BillingCycleMonthly {
		t.Errorf("expected monthly cycle, got %s", result.BillingCycle)
	}
}

func TestCalculateRenewalPrice_ChangeCycle(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db)
	proPlan, _ := service.GetPlan(ctx, "pro")

	now := time.Now()
	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		SeatCount:          2,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	db.Create(sub)

	// Renew with yearly cycle
	result, err := service.CalculateRenewalPrice(ctx, 1, billing.BillingCycleYearly)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 199.90 * 2 seats
	expectedAmount := 399.80
	if !almostEqual(result.Amount, expectedAmount, 0.001) {
		t.Errorf("expected amount %f, got %f", expectedAmount, result.Amount)
	}
	if result.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected yearly cycle, got %s", result.BillingCycle)
	}
}

func TestGetPricePreview(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedProPlan(t, db)

	// Test subscription preview (no existing subscription needed)
	result, err := service.GetPricePreview(ctx, 0, billing.OrderTypeSubscription, "pro", billing.BillingCycleMonthly, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(result.Amount, 19.99, 0.001) {
		t.Errorf("expected amount 19.99, got %f", result.Amount)
	}

	// Test invalid order type
	_, err = service.GetPricePreview(ctx, 0, "invalid", "", "", 0)
	if err == nil {
		t.Error("expected error for invalid order type")
	}
}

func TestCalculateRemainingPeriodRatio(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		periodStart time.Time
		periodEnd   time.Time
		wantRatio   float64
		tolerance   float64
	}{
		{
			name:        "full period remaining",
			periodStart: now,
			periodEnd:   now.Add(30 * 24 * time.Hour),
			wantRatio:   1.0,
			tolerance:   0.01,
		},
		{
			name:        "half period remaining",
			periodStart: now.Add(-15 * 24 * time.Hour),
			periodEnd:   now.Add(15 * 24 * time.Hour),
			wantRatio:   0.5,
			tolerance:   0.01,
		},
		{
			name:        "period ended",
			periodStart: now.Add(-30 * 24 * time.Hour),
			periodEnd:   now.Add(-1 * time.Hour),
			wantRatio:   0,
			tolerance:   0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := calculateRemainingPeriodRatio(tt.periodStart, tt.periodEnd)
			diff := ratio - tt.wantRatio
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("expected ratio %f (±%f), got %f", tt.wantRatio, tt.tolerance, ratio)
			}
		})
	}
}
