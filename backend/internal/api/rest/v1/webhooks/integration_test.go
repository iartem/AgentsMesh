package webhooks

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	billingService "github.com/anthropics/agentsmesh/backend/internal/service/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ===========================================
// Integration Test Setup
// ===========================================

func setupIntegrationDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create required tables
	db.Exec(`CREATE TABLE IF NOT EXISTS subscription_plans (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		display_name TEXT,
		tier TEXT DEFAULT 'based',
		price_per_seat_monthly REAL DEFAULT 0,
		price_per_seat_yearly REAL DEFAULT 0,
		max_users INTEGER DEFAULT 1,
		max_runners INTEGER DEFAULT 1,
		max_repositories INTEGER DEFAULT 10,
		max_concurrent_pods INTEGER DEFAULT 1,
		included_pod_minutes INTEGER DEFAULT 100,
		max_storage_gb INTEGER DEFAULT 1,
		stripe_price_id_monthly TEXT,
		stripe_price_id_yearly TEXT,
		features TEXT,
		is_active BOOLEAN DEFAULT TRUE
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY,
		organization_id INTEGER NOT NULL,
		plan_id INTEGER,
		status TEXT DEFAULT 'active',
		stripe_subscription_id TEXT,
		stripe_customer_id TEXT,
		billing_cycle TEXT DEFAULT 'monthly',
		seat_count INTEGER DEFAULT 1,
		auto_renew BOOLEAN DEFAULT TRUE,
		current_period_start DATETIME,
		current_period_end DATETIME,
		cancel_at_period_end BOOLEAN DEFAULT FALSE,
		downgrade_to_plan TEXT,
		next_billing_cycle TEXT,
		frozen_at DATETIME,
		canceled_at DATETIME,
		custom_quotas TEXT,
		payment_provider TEXT,
		payment_method TEXT,
		alipay_agreement_no TEXT,
		wechat_contract_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS payment_orders (
		id INTEGER PRIMARY KEY,
		organization_id INTEGER NOT NULL,
		order_no TEXT UNIQUE,
		external_order_no TEXT,
		order_type TEXT,
		plan_id INTEGER,
		billing_cycle TEXT,
		seats INTEGER DEFAULT 1,
		amount REAL,
		actual_amount REAL,
		payment_provider TEXT,
		payment_method TEXT,
		status TEXT,
		paid_at DATETIME,
		failure_reason TEXT,
		created_by_id INTEGER,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS payment_transactions (
		id INTEGER PRIMARY KEY,
		payment_order_id INTEGER,
		transaction_type TEXT,
		external_transaction_id TEXT,
		amount REAL,
		currency TEXT,
		status TEXT,
		webhook_event_id TEXT,
		webhook_event_type TEXT,
		raw_payload TEXT,
		created_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS usage_records (
		id INTEGER PRIMARY KEY,
		subscription_id INTEGER NOT NULL,
		usage_type TEXT,
		amount REAL,
		metadata TEXT,
		created_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS organization_members (
		organization_id INTEGER,
		user_id INTEGER,
		role TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS runners (
		id INTEGER PRIMARY KEY,
		organization_id INTEGER,
		name TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS repositories (
		id INTEGER PRIMARY KEY,
		organization_id INTEGER,
		name TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS pods (
		id INTEGER PRIMARY KEY,
		organization_id INTEGER,
		name TEXT,
		status TEXT
	)`)

	// Seed test data
	db.Exec(`INSERT INTO subscription_plans (name, display_name, tier, price_per_seat_monthly, price_per_seat_yearly, max_users, max_runners, max_repositories, max_concurrent_pods, included_pod_minutes, is_active)
		VALUES ('based', 'Based', 'based', 0, 0, 5, 1, 3, 2, 100, TRUE)`)
	db.Exec(`INSERT INTO subscription_plans (name, display_name, tier, price_per_seat_monthly, price_per_seat_yearly, max_users, max_runners, max_repositories, max_concurrent_pods, included_pod_minutes, is_active)
		VALUES ('pro', 'Pro', 'pro', 19.99, 199.90, 50, 10, 100, 10, 5000, TRUE)`)
	db.Exec(`INSERT INTO subscription_plans (name, display_name, tier, price_per_seat_monthly, price_per_seat_yearly, max_users, max_runners, max_repositories, max_concurrent_pods, included_pod_minutes, is_active)
		VALUES ('enterprise', 'Enterprise', 'enterprise', 99.99, 999.90, -1, -1, -1, -1, -1, TRUE)`)

	return db
}

func integrationTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createMockRouter(t *testing.T) (*WebhookRouter, *gorm.DB, *payment.Factory) {
	db := setupIntegrationDB(t)
	logger := integrationTestLogger()
	registry := NewHandlerRegistry(logger)
	SetupDefaultHandlers(registry, logger)

	// Create full config with mock payment enabled
	cfg := &config.Config{
		PrimaryDomain: "localhost:3000",
		UseHTTPS:      false,
		Payment: config.PaymentConfig{
			DeploymentType: config.DeploymentGlobal,
			MockEnabled:    true,
			MockBaseURL:    "http://localhost:3000",
		},
	}

	// Create billing service with mock (uses full config for URL derivation)
	billingSvc := billingService.NewServiceWithConfig(db, cfg)
	factory := billingSvc.GetPaymentFactory()

	return &WebhookRouter{
		db:             db,
		cfg:            cfg,
		logger:         logger,
		registry:       registry,
		billingSvc:     billingSvc,
		paymentFactory: factory,
	}, db, factory
}

// ===========================================
// Mock Payment Handler Integration Tests
// ===========================================

func TestMockCheckoutComplete_Enabled(t *testing.T) {
	router, _, factory := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/", nil)

	// Create a checkout session
	provider, _ := factory.GetDefaultProvider()
	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: 1,
		OrderType:      billing.OrderTypeSubscription,
		PlanID:         1,
		BillingCycle:   billing.BillingCycleMonthly,
		Seats:          1,
		Currency:       "USD",
		Amount:         19.99,
		ActualAmount:   19.99,
		SuccessURL:     "http://localhost:3000/success",
		CancelURL:      "http://localhost:3000/cancel",
		IdempotencyKey: "ORD-MOCK-INT-001",
	}

	resp, err := provider.CreateCheckoutSession(ctx.Request.Context(), checkoutReq)
	if err != nil {
		t.Fatalf("failed to create checkout session: %v", err)
	}

	// Call mock checkout complete (without order - tests the flow)
	w2 := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w2)

	payload := MockCheckoutCompleteRequest{
		SessionID: resp.SessionID,
		OrderNo:   "", // No order - tests error path
	}
	body, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("POST", "/webhooks/mock/complete", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	router.handleMockCheckoutComplete(c)

	// May return error due to no order, but this tests the code path
	// We're testing that the handler executes without panicking
	if w2.Code == http.StatusServiceUnavailable {
		t.Error("mock should be enabled")
	}
}

func TestMockCheckoutComplete_InvalidSession(t *testing.T) {
	router, _, _ := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	payload := MockCheckoutCompleteRequest{
		SessionID: "nonexistent_session",
		OrderNo:   "ORD-NONEXISTENT",
	}
	body, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("POST", "/webhooks/mock/complete", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	router.handleMockCheckoutComplete(c)

	// Should return 400 for invalid session
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestMockCheckoutComplete_InvalidJSON(t *testing.T) {
	router, _, _ := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("POST", "/webhooks/mock/complete", bytes.NewReader([]byte(`{invalid json`)))
	c.Request.Header.Set("Content-Type", "application/json")

	router.handleMockCheckoutComplete(c)

	// Should return 400 for invalid JSON
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetMockSession_Enabled(t *testing.T) {
	router, _, factory := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/", nil)

	// Create a checkout session
	provider, _ := factory.GetDefaultProvider()
	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: 1,
		OrderType:      billing.OrderTypeSubscription,
		PlanID:         1,
		BillingCycle:   billing.BillingCycleMonthly,
		Seats:          1,
		Currency:       "USD",
		Amount:         19.99,
		ActualAmount:   19.99,
		SuccessURL:     "http://localhost:3000/success",
		CancelURL:      "http://localhost:3000/cancel",
		IdempotencyKey: "ORD-GET-SESSION-001",
	}

	resp, err := provider.CreateCheckoutSession(ctx.Request.Context(), checkoutReq)
	if err != nil {
		t.Fatalf("failed to create checkout session: %v", err)
	}

	// Get session info
	w2 := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w2)

	c.Request = httptest.NewRequest("GET", "/webhooks/mock/session/"+resp.SessionID, nil)
	c.Params = []gin.Param{{Key: "session_id", Value: resp.SessionID}}

	router.getMockSession(c)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w2.Code, w2.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &result)
	if result["session_id"] != resp.SessionID {
		t.Errorf("expected session_id %s, got %s", resp.SessionID, result["session_id"])
	}
}

func TestGetMockSession_NotFound(t *testing.T) {
	router, _, _ := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/webhooks/mock/session/nonexistent", nil)
	c.Params = []gin.Param{{Key: "session_id", Value: "nonexistent"}}

	router.getMockSession(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetMockSession_EmptyID(t *testing.T) {
	router, _, _ := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request = httptest.NewRequest("GET", "/webhooks/mock/session/", nil)
	c.Params = []gin.Param{{Key: "session_id", Value: ""}}

	router.getMockSession(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// ===========================================
// Handler Registry Tests
// ===========================================

func TestHandlerRegistryWithMockRouter(t *testing.T) {
	router, _, _ := createMockRouter(t)

	// Verify registry is set up
	if router.registry == nil {
		t.Error("expected registry to be set")
	}

	// Verify payment factory is set
	if router.paymentFactory == nil {
		t.Error("expected payment factory to be set")
	}

	// Verify mock is enabled
	if !router.paymentFactory.IsMockEnabled() {
		t.Error("expected mock to be enabled")
	}
}

// ===========================================
// Routes Registration Tests
// ===========================================

func TestRegisterRoutesWithMock(t *testing.T) {
	router, _, _ := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	rg := engine.Group("/webhooks")

	// Should not panic when registering routes
	router.RegisterRoutes(rg)

	// Verify routes are registered by testing GitLab webhook
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/webhooks/gitlab", bytes.NewReader([]byte(`{"object_kind": "push"}`)))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	// Should not be 404 (route exists)
	if w.Code == http.StatusNotFound {
		t.Error("expected route to be registered")
	}
}

// ===========================================
// Webhook Processing Tests
// ===========================================

func TestProcessWebhookWithMockProvider(t *testing.T) {
	router, _, factory := createMockRouter(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/", nil)

	// Create checkout session
	provider, _ := factory.GetDefaultProvider()
	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: 1,
		OrderType:      billing.OrderTypeSubscription,
		PlanID:         1,
		BillingCycle:   billing.BillingCycleMonthly,
		Seats:          1,
		Currency:       "USD",
		Amount:         19.99,
		ActualAmount:   19.99,
		SuccessURL:     "http://localhost:3000/success",
		CancelURL:      "http://localhost:3000/cancel",
		IdempotencyKey: "ORD-PROCESS-001",
	}

	resp, err := provider.CreateCheckoutSession(ctx.Request.Context(), checkoutReq)
	if err != nil {
		t.Fatalf("failed to create checkout session: %v", err)
	}

	// Complete the session
	mockProvider := factory.GetMockProvider()
	_, err = mockProvider.CompleteSession(resp.SessionID)
	if err != nil {
		t.Fatalf("failed to complete session: %v", err)
	}

	// Handle webhook
	webhookPayload := []byte(`{"event_type": "checkout.session.completed", "session_id": "` + resp.SessionID + `", "order_no": "ORD-PROCESS-001"}`)
	event, err := provider.HandleWebhook(ctx.Request.Context(), webhookPayload, "")
	if err != nil {
		t.Fatalf("failed to handle webhook: %v", err)
	}

	// Process with billing service - will fail due to no order, but tests the flow
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	_ = router.billingSvc.HandlePaymentSucceeded(c, event) // Error expected

	// Verify event was parsed correctly
	if event.EventType != "checkout.session.completed" {
		t.Errorf("expected event type checkout.session.completed, got %s", event.EventType)
	}
}

// ===========================================
// Edge Cases
// ===========================================

func TestMockRouterWithNilFactory(t *testing.T) {
	db := setupIntegrationDB(t)
	logger := testLogger()
	registry := NewHandlerRegistry(logger)
	SetupDefaultHandlers(registry, logger)

	// Create billing service without mock
	billingSvc := billingService.NewService(db, "")

	cfg := &config.Config{}

	router := &WebhookRouter{
		db:             db,
		cfg:            cfg,
		logger:         logger,
		registry:       registry,
		billingSvc:     billingSvc,
		paymentFactory: nil, // No factory
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	payload := `{"session_id": "test123"}`
	c.Request = httptest.NewRequest("POST", "/webhooks/mock/complete", bytes.NewReader([]byte(payload)))
	c.Request.Header.Set("Content-Type", "application/json")

	router.handleMockCheckoutComplete(c)

	// Should return 403 when factory is nil
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}
