package billing

import (
	"context"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/config"
)

// ===========================================
// Service Core Tests
// ===========================================

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.stripeEnabled {
		t.Error("expected stripe to be disabled without key")
	}
}

func TestNewServiceWithStripeKey(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "sk_test_fake_key")

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if !service.stripeEnabled {
		t.Error("expected stripe to be enabled with key")
	}
}

// testConfigWithPayment creates a test config with payment settings
func testConfigWithPayment(payment *config.PaymentConfig) *config.Config {
	if payment == nil {
		return &config.Config{
			PrimaryDomain: "localhost:10000",
			UseHTTPS:      false,
		}
	}
	return &config.Config{
		PrimaryDomain: "localhost:10000",
		UseHTTPS:      false,
		Payment:       *payment,
	}
}

func TestNewServiceWithConfig(t *testing.T) {
	db := setupTestDB(t)

	// Test with nil config
	service := NewServiceWithConfig(db, nil)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.stripeEnabled {
		t.Error("expected stripe to be disabled without config")
	}

	// Test with mock enabled config
	appCfg := testConfigWithPayment(&config.PaymentConfig{
		DeploymentType: config.DeploymentGlobal,
		MockEnabled:    true,
	})
	service = NewServiceWithConfig(db, appCfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.GetPaymentFactory() == nil {
		t.Error("expected payment factory to be set")
	}
}

func TestGetPaymentFactory(t *testing.T) {
	db := setupTestDB(t)
	appCfg := testConfigWithPayment(&config.PaymentConfig{MockEnabled: true})
	service := NewServiceWithConfig(db, appCfg)

	factory := service.GetPaymentFactory()
	if factory == nil {
		t.Error("expected non-nil payment factory")
	}
}

func TestGetPlan(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	plan, err := service.GetPlan(ctx, "based")
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if plan.Name != "based" {
		t.Errorf("expected plan name 'free', got %s", plan.Name)
	}
}

func TestGetPlanNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetPlan(ctx, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestListPlans(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	seedProPlan(t, db)

	plans, err := service.ListPlans(ctx)
	if err != nil {
		t.Fatalf("failed to list plans: %v", err)
	}
	if len(plans) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plans))
	}
}

func TestGetPlanByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plan := seedTestPlan(t, db)

	result, err := service.GetPlanByID(ctx, plan.ID)
	if err != nil {
		t.Fatalf("failed to get plan by ID: %v", err)
	}
	if result.Name != "based" {
		t.Errorf("expected plan name 'free', got %s", result.Name)
	}
}

func TestGetPlanByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetPlanByID(ctx, 9999)
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestCreateStripeCustomerDisabled(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	customerID, err := service.CreateStripeCustomer(ctx, 1, "test@example.com", "Test Org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if customerID != "" {
		t.Error("expected empty customer ID when stripe is disabled")
	}
}

func TestGetDeploymentInfo(t *testing.T) {
	db := setupTestDB(t)

	// Without config
	service := NewService(db, "")
	info := service.GetDeploymentInfo()
	if info.DeploymentType != "global" {
		t.Errorf("expected 'global', got %s", info.DeploymentType)
	}

	// With config
	appCfg := testConfigWithPayment(&config.PaymentConfig{
		DeploymentType: config.DeploymentCN,
		MockEnabled:    true,
	})
	service = NewServiceWithConfig(db, appCfg)
	info = service.GetDeploymentInfo()
	if info.DeploymentType != "cn" {
		t.Errorf("expected 'cn', got %s", info.DeploymentType)
	}
}

func TestErrorVariables(t *testing.T) {
	errors := map[error]string{
		ErrSubscriptionNotFound:  "subscription not found",
		ErrPlanNotFound:          "plan not found",
		ErrQuotaExceeded:         "quota exceeded",
		ErrInvalidPlan:           "invalid plan",
		ErrOrderNotFound:         "order not found",
		ErrOrderExpired:          "order expired",
		ErrInvalidOrderStatus:    "invalid order status",
		ErrSeatCountExceedsLimit: "current seat count exceeds target plan limit",
	}

	for err, msg := range errors {
		if err.Error() != msg {
			t.Errorf("unexpected error message for %v: %s", err, err.Error())
		}
	}
}

func TestListPlansEmpty(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plans, err := service.ListPlans(ctx)
	if err != nil {
		t.Fatalf("failed to list plans: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("expected 0 plans, got %d", len(plans))
	}
}

func TestNewServiceWithConfigStripe(t *testing.T) {
	db := setupTestDB(t)

	// Test with Stripe key
	appCfg := testConfigWithPayment(&config.PaymentConfig{
		DeploymentType: config.DeploymentGlobal,
		Stripe: config.StripeConfig{
			SecretKey: "sk_test_fake",
		},
	})
	service := NewServiceWithConfig(db, appCfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if !service.stripeEnabled {
		t.Error("expected stripe to be enabled with stripe config")
	}
}

// ===========================================
// Plan Price Tests
// ===========================================

func TestGetPlanPrice(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // Seeds based plan with USD and CNY prices

	// Test USD price
	price, err := service.GetPlanPrice(ctx, "based", "USD")
	if err != nil {
		t.Fatalf("failed to get plan price: %v", err)
	}
	if price.PriceMonthly != 9.9 {
		t.Errorf("expected USD monthly price 9.9, got %f", price.PriceMonthly)
	}
	if price.PriceYearly != 99 {
		t.Errorf("expected USD yearly price 99, got %f", price.PriceYearly)
	}
	if price.Plan == nil {
		t.Error("expected Plan to be populated")
	}

	// Test CNY price
	price, err = service.GetPlanPrice(ctx, "based", "CNY")
	if err != nil {
		t.Fatalf("failed to get plan price: %v", err)
	}
	if price.PriceMonthly != 69 {
		t.Errorf("expected CNY monthly price 69, got %f", price.PriceMonthly)
	}
}

func TestGetPlanPricePlanNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetPlanPrice(ctx, "nonexistent", "USD")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestGetPlanPriceCurrencyNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	_, err := service.GetPlanPrice(ctx, "based", "EUR")
	if err != ErrPriceNotFound {
		t.Errorf("expected ErrPriceNotFound, got %v", err)
	}
}

func TestGetPlanPriceByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	plan := seedTestPlan(t, db)

	price, err := service.GetPlanPriceByID(ctx, plan.ID, "USD")
	if err != nil {
		t.Fatalf("failed to get plan price by ID: %v", err)
	}
	if price.PriceMonthly != 9.9 {
		t.Errorf("expected monthly price 9.9, got %f", price.PriceMonthly)
	}
	if price.Plan == nil {
		t.Error("expected Plan to be preloaded")
	}
}

func TestGetPlanPriceByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetPlanPriceByID(ctx, 9999, "USD")
	if err != ErrPriceNotFound {
		t.Errorf("expected ErrPriceNotFound, got %v", err)
	}
}

func TestGetPlanPrices(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db) // Seeds with USD and CNY prices

	prices, err := service.GetPlanPrices(ctx, "based")
	if err != nil {
		t.Fatalf("failed to get plan prices: %v", err)
	}
	if len(prices) != 2 {
		t.Errorf("expected 2 prices (USD and CNY), got %d", len(prices))
	}

	// Verify both currencies are present
	currencies := make(map[string]bool)
	for _, p := range prices {
		currencies[p.Currency] = true
		if p.Plan == nil {
			t.Error("expected Plan to be attached to each price")
		}
	}
	if !currencies["USD"] || !currencies["CNY"] {
		t.Error("expected both USD and CNY prices")
	}
}

func TestGetPlanPricesPlanNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	_, err := service.GetPlanPrices(ctx, "nonexistent")
	if err != ErrPlanNotFound {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestListPlansWithPrices(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	seedProPlan(t, db)

	// List with USD prices
	plansWithPrices, err := service.ListPlansWithPrices(ctx, "USD")
	if err != nil {
		t.Fatalf("failed to list plans with prices: %v", err)
	}
	if len(plansWithPrices) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plansWithPrices))
	}

	for _, pwp := range plansWithPrices {
		if pwp.Plan == nil {
			t.Error("expected Plan to be set")
		}
		if pwp.Price == nil {
			t.Error("expected Price to be set")
		}
		if pwp.Price.Currency != "USD" {
			t.Errorf("expected USD currency, got %s", pwp.Price.Currency)
		}
	}
}

func TestListPlansWithPricesCNY(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)
	seedProPlan(t, db)

	// List with CNY prices
	plansWithPrices, err := service.ListPlansWithPrices(ctx, "CNY")
	if err != nil {
		t.Fatalf("failed to list plans with prices: %v", err)
	}
	if len(plansWithPrices) != 2 {
		t.Errorf("expected 2 plans, got %d", len(plansWithPrices))
	}

	for _, pwp := range plansWithPrices {
		if pwp.Price.Currency != "CNY" {
			t.Errorf("expected CNY currency, got %s", pwp.Price.Currency)
		}
	}
}

func TestListPlansWithPricesNoCurrency(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, "")
	ctx := context.Background()

	seedTestPlan(t, db)

	// List with non-existent currency - should return empty
	plansWithPrices, err := service.ListPlansWithPrices(ctx, "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plansWithPrices) != 0 {
		t.Errorf("expected 0 plans for EUR currency, got %d", len(plansWithPrices))
	}
}
