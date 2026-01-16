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
	cfg := &config.PaymentConfig{
		DeploymentType: config.DeploymentGlobal,
		MockEnabled:    true,
	}
	service = NewServiceWithConfig(db, cfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.GetPaymentFactory() == nil {
		t.Error("expected payment factory to be set")
	}
}

func TestGetPaymentFactory(t *testing.T) {
	db := setupTestDB(t)
	cfg := &config.PaymentConfig{MockEnabled: true}
	service := NewServiceWithConfig(db, cfg)

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

	plan, err := service.GetPlan(ctx, "free")
	if err != nil {
		t.Fatalf("failed to get plan: %v", err)
	}
	if plan.Name != "free" {
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
	if result.Name != "free" {
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
	cfg := &config.PaymentConfig{
		DeploymentType: config.DeploymentCN,
		MockEnabled:    true,
	}
	service = NewServiceWithConfig(db, cfg)
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
	cfg := &config.PaymentConfig{
		DeploymentType: config.DeploymentGlobal,
		Stripe: config.StripeConfig{
			SecretKey: "sk_test_fake",
		},
	}
	service := NewServiceWithConfig(db, cfg)
	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if !service.stripeEnabled {
		t.Error("expected stripe to be enabled with stripe config")
	}
}
