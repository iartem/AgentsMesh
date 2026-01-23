package billing

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Integration Tests with Mock Provider
// These tests verify the complete payment flow
// ===========================================

func setupIntegrationTestService(t *testing.T) (*Service, *payment.Factory) {
	db := setupTestDB(t)

	// Seed plans
	seedTestPlan(t, db)
	seedProPlan(t, db)
	seedEnterprisePlan(t, db)

	// Create service with mock provider enabled
	appCfg := &config.Config{
		PrimaryDomain: "localhost:3000",
		UseHTTPS:      false,
		Payment: config.PaymentConfig{
			DeploymentType: config.DeploymentGlobal,
			MockEnabled:    true,
			MockBaseURL:    "http://localhost:3000",
		},
	}

	service := NewServiceWithConfig(db, appCfg)
	factory := service.GetPaymentFactory()

	return service, factory
}

// TestIntegrationCreateSubscriptionFlow tests the complete subscription creation flow
func TestIntegrationCreateSubscriptionFlow(t *testing.T) {
	service, factory := setupIntegrationTestService(t)
	ctx := context.Background()

	// 1. Create a based subscription first
	sub, err := service.CreateSubscription(ctx, 1, "based")
	if err != nil {
		t.Fatalf("failed to create based subscription: %v", err)
	}
	if sub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", sub.Status)
	}

	// 2. Verify factory is available
	if factory == nil {
		t.Fatal("expected payment factory to be available")
	}
	if !factory.IsMockEnabled() {
		t.Error("expected mock to be enabled")
	}

	// 3. Get default provider (should be mock)
	provider, err := factory.GetDefaultProvider()
	if err != nil {
		t.Fatalf("failed to get default provider: %v", err)
	}
	if provider.GetProviderName() != "mock" {
		t.Errorf("expected mock provider, got %s", provider.GetProviderName())
	}
}

// TestIntegrationUpgradeWithCheckout tests the upgrade flow with checkout
func TestIntegrationUpgradeWithCheckout(t *testing.T) {
	service, factory := setupIntegrationTestService(t)
	ctx := context.Background()

	// 1. Create based subscription
	service.CreateSubscription(ctx, 1, "based")

	// 2. Get pro plan for upgrade
	proPlan, _ := service.GetPlan(ctx, "pro")

	// 3. Create checkout session for upgrade
	provider, _ := factory.GetDefaultProvider()
	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: 1,
		OrderType:      billing.OrderTypeSubscription,
		PlanID:         proPlan.ID,
		BillingCycle:   billing.BillingCycleMonthly,
		Seats:          1,
		Currency:       "USD",
		Amount:         proPlan.PricePerSeatMonthly,
		ActualAmount:   proPlan.PricePerSeatMonthly,
		SuccessURL:     "http://localhost:3000/success",
		CancelURL:      "http://localhost:3000/cancel",
		IdempotencyKey: "ORD-TEST-001",
	}

	resp, err := provider.CreateCheckoutSession(ctx, checkoutReq)
	if err != nil {
		t.Fatalf("failed to create checkout session: %v", err)
	}
	if resp.SessionID == "" {
		t.Error("expected session ID")
	}
	if resp.SessionURL == "" {
		t.Error("expected session URL")
	}

	// 4. Verify session status is pending
	status, err := provider.GetCheckoutStatus(ctx, resp.SessionID)
	if err != nil {
		t.Fatalf("failed to get checkout status: %v", err)
	}
	if status != billing.OrderStatusPending {
		t.Errorf("expected pending status, got %s", status)
	}

	// 5. Complete the checkout (simulate user payment)
	mockProvider := factory.GetMockProvider()
	session, err := mockProvider.CompleteSession(resp.SessionID)
	if err != nil {
		t.Fatalf("failed to complete session: %v", err)
	}
	if session.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected succeeded status, got %s", session.Status)
	}

	// 6. Verify status is now succeeded
	status, _ = provider.GetCheckoutStatus(ctx, resp.SessionID)
	if status != billing.OrderStatusSucceeded {
		t.Errorf("expected succeeded status, got %s", status)
	}
}

// TestIntegrationWebhookPaymentSucceeded tests webhook handling for successful payment
func TestIntegrationWebhookPaymentSucceeded(t *testing.T) {
	service, factory := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create subscription
	service.CreateSubscription(ctx, 1, "based")

	// 2. Create a payment order
	proPlan, _ := service.GetPlan(ctx, "pro")
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-WEBHOOK-001",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          proPlan.PricePerSeatMonthly,
		ActualAmount:    proPlan.PricePerSeatMonthly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate webhook event
	event := &payment.WebhookEvent{
		EventID:         "evt_test_001",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-WEBHOOK-001",
		ExternalOrderNo: "mock_cs_123",
		CustomerID:      "mock_cus_456",
		SubscriptionID:  "mock_sub_789",
		Amount:          proPlan.PricePerSeatMonthly,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	// 4. Handle payment succeeded
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment succeeded: %v", err)
	}

	// 5. Verify order status updated
	updatedOrder, _ := service.GetPaymentOrderByNo(ctx, "ORD-WEBHOOK-001")
	if updatedOrder.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected order status succeeded, got %s", updatedOrder.Status)
	}
	if updatedOrder.PaidAt == nil {
		t.Error("expected PaidAt to be set")
	}

	// 6. Verify subscription upgraded
	sub, _ := service.GetSubscription(ctx, 1)
	if sub.StripeCustomerID == nil || *sub.StripeCustomerID != "mock_cus_456" {
		t.Error("expected customer ID to be set")
	}
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID != "mock_sub_789" {
		t.Error("expected subscription ID to be set")
	}

	// Verify factory mock status
	if !factory.IsMockEnabled() {
		t.Error("mock should be enabled")
	}
}

// TestIntegrationWebhookPaymentFailed tests webhook handling for failed payment
func TestIntegrationWebhookPaymentFailed(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create subscription
	service.CreateSubscription(ctx, 1, "based")

	// 2. Create a payment order
	proPlan, _ := service.GetPlan(ctx, "pro")
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-FAILED-001",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          proPlan.PricePerSeatMonthly,
		ActualAmount:    proPlan.PricePerSeatMonthly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate failed payment webhook
	event := &payment.WebhookEvent{
		EventID:      "evt_failed_001",
		EventType:    "invoice.payment_failed",
		Provider:     "mock",
		OrderNo:      "ORD-FAILED-001",
		Amount:       proPlan.PricePerSeatMonthly,
		Currency:     "USD",
		Status:       billing.OrderStatusFailed,
		FailedReason: "Card declined",
	}

	// 4. Handle payment failed
	err := service.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment failed: %v", err)
	}

	// 5. Verify order status updated
	updatedOrder, _ := service.GetPaymentOrderByNo(ctx, "ORD-FAILED-001")
	if updatedOrder.Status != billing.OrderStatusFailed {
		t.Errorf("expected order status failed, got %s", updatedOrder.Status)
	}
	if updatedOrder.FailureReason == nil || *updatedOrder.FailureReason != "Card declined" {
		t.Error("expected failure reason to be set")
	}
}

// TestIntegrationWebhookSubscriptionCanceled tests subscription cancellation webhook
func TestIntegrationWebhookSubscriptionCanceled(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create subscription with Stripe IDs
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_test_cancel"
	stripeCusID := "cus_test_cancel"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
	}
	service.db.Create(sub)

	// 2. Simulate subscription canceled webhook
	event := &payment.WebhookEvent{
		EventID:        "evt_cancel_001",
		EventType:      "customer.subscription.deleted",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Status:         billing.SubscriptionStatusCanceled,
	}

	// 3. Handle subscription canceled
	err := service.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Fatalf("failed to handle subscription canceled: %v", err)
	}

	// 4. Verify subscription status updated
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.Status != billing.SubscriptionStatusCanceled {
		t.Errorf("expected subscription status canceled, got %s", updatedSub.Status)
	}
}

// TestIntegrationWebhookSubscriptionUpdated tests subscription status update webhook
func TestIntegrationWebhookSubscriptionUpdated(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create subscription with Stripe IDs
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_test_update"
	stripeCusID := "cus_test_update"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		AutoRenew:            true,
	}
	service.db.Create(sub)

	// 2. Simulate subscription status change to past_due
	event := &payment.WebhookEvent{
		EventID:        "evt_update_001",
		EventType:      "customer.subscription.updated",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Status:         "past_due", // Changed status
	}

	// 3. Handle subscription updated
	err := service.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Fatalf("failed to handle subscription updated: %v", err)
	}

	// 4. Verify subscription status updated
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.Status != billing.SubscriptionStatusPastDue {
		t.Errorf("expected status past_due, got %s", updatedSub.Status)
	}
}

// TestIntegrationRecurringPaymentSuccess tests recurring payment handling
func TestIntegrationRecurringPaymentSuccess(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create active subscription with Stripe IDs
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_recurring"
	stripeCusID := "cus_recurring"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now.AddDate(0, -1, 0), // Started last month
		CurrentPeriodEnd:     now,                   // Ending now
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		AutoRenew:            true,
		SeatCount:            1,
	}
	service.db.Create(sub)

	originalPeriodEnd := sub.CurrentPeriodEnd

	// 2. Simulate invoice.paid webhook for recurring payment
	event := &payment.WebhookEvent{
		EventID:        "evt_recurring_001",
		EventType:      "invoice.paid",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Amount:         proPlan.PricePerSeatMonthly,
		Currency:       "USD",
		Status:         billing.OrderStatusSucceeded,
	}

	// 3. Handle payment succeeded (recurring)
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle recurring payment: %v", err)
	}

	// 4. Verify subscription period was extended
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if !updatedSub.CurrentPeriodStart.After(originalPeriodEnd.Add(-time.Hour)) {
		t.Error("expected period start to be updated")
	}
}

// TestIntegrationAddSeatsFlow tests the seat addition flow
func TestIntegrationAddSeatsFlow(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create pro subscription
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_seats"
	stripeCusID := "cus_seats"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		SeatCount:            1,
	}
	service.db.Create(sub)

	// 2. Create order for adding seats
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-SEATS-001",
		OrderType:       billing.OrderTypeSeatPurchase,
		PlanID:          &proPlan.ID,
		Seats:           3, // Add 3 seats
		Amount:          proPlan.PricePerSeatMonthly * 3,
		ActualAmount:    proPlan.PricePerSeatMonthly * 3,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_seats_001",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-SEATS-001",
		ExternalOrderNo: "mock_cs_seats",
		CustomerID:      stripeCusID,
		SubscriptionID:  stripeSubID,
		Amount:          proPlan.PricePerSeatMonthly * 3,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle add seats payment: %v", err)
	}

	// 4. Verify seats were added
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.SeatCount != 4 { // 1 + 3 = 4
		t.Errorf("expected 4 seats, got %d", updatedSub.SeatCount)
	}
}

// TestIntegrationPlanUpgradeFlow tests plan upgrade via payment
func TestIntegrationPlanUpgradeFlow(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create pro subscription
	proPlan, _ := service.GetPlan(ctx, "pro")
	entPlan, _ := service.GetPlan(ctx, "enterprise")
	now := time.Now()
	stripeSubID := "sub_upgrade"
	stripeCusID := "cus_upgrade"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.AddDate(0, 1, 0),
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		SeatCount:            1,
	}
	service.db.Create(sub)

	// 2. Create upgrade order
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-UPGRADE-001",
		OrderType:       billing.OrderTypePlanUpgrade,
		PlanID:          &entPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          entPlan.PricePerSeatMonthly - proPlan.PricePerSeatMonthly, // Prorated
		ActualAmount:    entPlan.PricePerSeatMonthly - proPlan.PricePerSeatMonthly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_upgrade_001",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-UPGRADE-001",
		ExternalOrderNo: "mock_cs_upgrade",
		CustomerID:      stripeCusID,
		SubscriptionID:  stripeSubID,
		Amount:          entPlan.PricePerSeatMonthly - proPlan.PricePerSeatMonthly,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle upgrade payment: %v", err)
	}

	// 4. Verify plan was upgraded
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.PlanID != entPlan.ID {
		t.Errorf("expected plan ID %d, got %d", entPlan.ID, updatedSub.PlanID)
	}
}

// TestIntegrationRenewalFlow tests subscription renewal flow
func TestIntegrationRenewalFlow(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create subscription that needs renewal
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()

	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.Add(-time.Hour), // Expired
		SeatCount:          2,
	}
	service.db.Create(sub)

	originalPeriodEnd := sub.CurrentPeriodEnd

	// 2. Create renewal order
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-RENEWAL-001",
		OrderType:       billing.OrderTypeRenewal,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           2,
		Amount:          proPlan.PricePerSeatMonthly * 2,
		ActualAmount:    proPlan.PricePerSeatMonthly * 2,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_renewal_001",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-RENEWAL-001",
		ExternalOrderNo: "mock_cs_renewal",
		Amount:          proPlan.PricePerSeatMonthly * 2,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle renewal payment: %v", err)
	}

	// 4. Verify subscription was renewed
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if !updatedSub.CurrentPeriodEnd.After(originalPeriodEnd) {
		t.Error("expected period end to be extended")
	}
	if updatedSub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", updatedSub.Status)
	}
}

// TestIntegrationMockProviderWebhook tests mock provider webhook handling
func TestIntegrationMockProviderWebhook(t *testing.T) {
	service, factory := setupIntegrationTestService(t)
	ctx := context.Background()

	// 1. Create checkout session
	provider, _ := factory.GetDefaultProvider()
	proPlan, _ := service.GetPlan(ctx, "pro")

	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: 1,
		OrderType:      billing.OrderTypeSubscription,
		PlanID:         proPlan.ID,
		BillingCycle:   billing.BillingCycleMonthly,
		Seats:          1,
		Currency:       "USD",
		Amount:         proPlan.PricePerSeatMonthly,
		ActualAmount:   proPlan.PricePerSeatMonthly,
		SuccessURL:     "http://localhost:3000/success",
		CancelURL:      "http://localhost:3000/cancel",
		IdempotencyKey: "ORD-MOCK-001",
	}

	resp, err := provider.CreateCheckoutSession(ctx, checkoutReq)
	if err != nil {
		t.Fatalf("failed to create checkout: %v", err)
	}

	// 2. Complete the session
	mockProvider := factory.GetMockProvider()
	_, err = mockProvider.CompleteSession(resp.SessionID)
	if err != nil {
		t.Fatalf("failed to complete session: %v", err)
	}

	// 3. Simulate webhook
	webhookPayload := []byte(`{"event_type": "checkout.session.completed", "session_id": "` + resp.SessionID + `", "order_no": "ORD-MOCK-001"}`)
	event, err := provider.HandleWebhook(ctx, webhookPayload, "")
	if err != nil {
		t.Fatalf("failed to handle webhook: %v", err)
	}

	if event.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected succeeded status, got %s", event.Status)
	}
	if event.EventType != "checkout.session.completed" {
		t.Errorf("expected checkout.session.completed, got %s", event.EventType)
	}
}

// TestIntegrationRefundPayment tests payment refund
func TestIntegrationRefundPayment(t *testing.T) {
	_, factory := setupIntegrationTestService(t)
	ctx := context.Background()

	provider, _ := factory.GetDefaultProvider()

	refundReq := &payment.RefundRequest{
		OrderNo: "ORD-REFUND-001",
		Amount:  19.99,
		Reason:  "Customer request",
	}

	resp, err := provider.RefundPayment(ctx, refundReq)
	if err != nil {
		t.Fatalf("failed to refund: %v", err)
	}

	if resp.Status != "succeeded" {
		t.Errorf("expected succeeded, got %s", resp.Status)
	}
	if resp.Amount != 19.99 {
		t.Errorf("expected amount 19.99, got %f", resp.Amount)
	}
}

// TestIntegrationCancelSubscription tests subscription cancellation via provider
func TestIntegrationCancelSubscription(t *testing.T) {
	_, factory := setupIntegrationTestService(t)
	ctx := context.Background()

	provider, _ := factory.GetDefaultProvider()

	// Mock provider always succeeds
	err := provider.CancelSubscription(ctx, "sub_test_123", false)
	if err != nil {
		t.Fatalf("failed to cancel subscription: %v", err)
	}
}

// TestIntegrationRecurringPaymentYearly tests recurring payment for yearly subscription
func TestIntegrationRecurringPaymentYearly(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create yearly subscription
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_yearly_recurring"
	stripeCusID := "cus_yearly_recurring"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleYearly, // Yearly
		CurrentPeriodStart:   now.AddDate(-1, 0, 0),      // Started last year
		CurrentPeriodEnd:     now,                        // Ending now
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		AutoRenew:            true,
		SeatCount:            1,
	}
	service.db.Create(sub)

	originalPeriodEnd := sub.CurrentPeriodEnd

	// 2. Simulate invoice.paid webhook
	event := &payment.WebhookEvent{
		EventID:        "evt_yearly_recurring",
		EventType:      "invoice.paid",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Amount:         proPlan.PricePerSeatYearly,
		Currency:       "USD",
		Status:         billing.OrderStatusSucceeded,
	}

	// 3. Handle recurring payment
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle recurring payment: %v", err)
	}

	// 4. Verify period extended by 1 year
	updatedSub, _ := service.GetSubscription(ctx, 1)
	expectedEnd := originalPeriodEnd.AddDate(1, 0, 0)
	if updatedSub.CurrentPeriodEnd.Before(expectedEnd.Add(-time.Hour)) {
		t.Errorf("expected period end to be extended by 1 year, got %v", updatedSub.CurrentPeriodEnd)
	}
}

// TestIntegrationRecurringPaymentWithDowngrade tests recurring payment with pending downgrade
func TestIntegrationRecurringPaymentWithDowngrade(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create pro subscription with pending downgrade to based
	proPlan, _ := service.GetPlan(ctx, "pro")
	basedPlan, _ := service.GetPlan(ctx, "based")
	now := time.Now()
	stripeSubID := "sub_downgrade"
	stripeCusID := "cus_downgrade"
	downgradePlan := "based"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now.AddDate(0, -1, 0),
		CurrentPeriodEnd:     now,
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		DowngradeToPlan:      &downgradePlan, // Pending downgrade
		AutoRenew:            true,
		SeatCount:            1,
	}
	service.db.Create(sub)

	// 2. Simulate invoice.paid webhook
	event := &payment.WebhookEvent{
		EventID:        "evt_with_downgrade",
		EventType:      "invoice.paid",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Amount:         basedPlan.PricePerSeatMonthly, // Paying based price
		Currency:       "USD",
		Status:         billing.OrderStatusSucceeded,
	}

	// 3. Handle recurring payment
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment: %v", err)
	}

	// 4. Verify downgrade applied
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.PlanID != basedPlan.ID {
		t.Errorf("expected plan to be downgraded to based, got plan ID %d", updatedSub.PlanID)
	}
	if updatedSub.DowngradeToPlan != nil {
		t.Error("expected DowngradeToPlan to be cleared")
	}
}

// TestIntegrationRecurringPaymentWithBillingCycleChange tests recurring payment with billing cycle change
func TestIntegrationRecurringPaymentWithBillingCycleChange(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create monthly subscription with pending change to yearly
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_cycle_change"
	stripeCusID := "cus_cycle_change"
	nextCycle := billing.BillingCycleYearly

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now.AddDate(0, -1, 0),
		CurrentPeriodEnd:     now,
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		NextBillingCycle:     &nextCycle, // Pending cycle change
		AutoRenew:            true,
		SeatCount:            1,
	}
	service.db.Create(sub)

	// 2. Simulate invoice.paid webhook
	event := &payment.WebhookEvent{
		EventID:        "evt_cycle_change",
		EventType:      "invoice.paid",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Amount:         proPlan.PricePerSeatYearly,
		Currency:       "USD",
		Status:         billing.OrderStatusSucceeded,
	}

	// 3. Handle recurring payment
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment: %v", err)
	}

	// 4. Verify billing cycle changed
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.BillingCycle != billing.BillingCycleYearly {
		t.Errorf("expected billing cycle to be yearly, got %s", updatedSub.BillingCycle)
	}
	if updatedSub.NextBillingCycle != nil {
		t.Error("expected NextBillingCycle to be cleared")
	}
}

// TestIntegrationRecurringPaymentFailure tests recurring payment failure and subscription freeze
func TestIntegrationRecurringPaymentFailure(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create active subscription
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()
	stripeSubID := "sub_fail_recurring"
	stripeCusID := "cus_fail_recurring"

	sub := &billing.Subscription{
		OrganizationID:       1,
		PlanID:               proPlan.ID,
		Status:               billing.SubscriptionStatusActive,
		BillingCycle:         billing.BillingCycleMonthly,
		CurrentPeriodStart:   now.AddDate(0, -1, 0),
		CurrentPeriodEnd:     now.Add(-time.Hour),
		StripeSubscriptionID: &stripeSubID,
		StripeCustomerID:     &stripeCusID,
		AutoRenew:            true,
		SeatCount:            1,
	}
	service.db.Create(sub)

	// 2. Simulate invoice.payment_failed webhook
	event := &payment.WebhookEvent{
		EventID:        "evt_fail_recurring",
		EventType:      "invoice.payment_failed",
		Provider:       "mock",
		SubscriptionID: stripeSubID,
		CustomerID:     stripeCusID,
		Amount:         proPlan.PricePerSeatMonthly,
		Currency:       "USD",
		Status:         billing.OrderStatusFailed,
		FailedReason:   "Card declined",
	}

	// 3. Handle payment failure
	err := service.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment failure: %v", err)
	}

	// 4. Verify subscription is frozen
	updatedSub, _ := service.GetSubscription(ctx, 1)
	if updatedSub.Status != billing.SubscriptionStatusFrozen {
		t.Errorf("expected status frozen, got %s", updatedSub.Status)
	}
	if updatedSub.FrozenAt == nil {
		t.Error("expected FrozenAt to be set")
	}
}

// TestIntegrationRenewalFlowYearly tests yearly subscription renewal
func TestIntegrationRenewalFlowYearly(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Create yearly subscription that needs renewal
	proPlan, _ := service.GetPlan(ctx, "pro")
	now := time.Now()

	sub := &billing.Subscription{
		OrganizationID:     1,
		PlanID:             proPlan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleYearly, // Yearly
		CurrentPeriodStart: now.AddDate(-1, 0, 0),
		CurrentPeriodEnd:   now.Add(-time.Hour), // Expired
		SeatCount:          2,
	}
	service.db.Create(sub)

	originalPeriodEnd := sub.CurrentPeriodEnd

	// 2. Create renewal order
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-YEARLY-RENEWAL",
		OrderType:       billing.OrderTypeRenewal,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleYearly,
		Seats:           2,
		Amount:          proPlan.PricePerSeatYearly * 2,
		ActualAmount:    proPlan.PricePerSeatYearly * 2,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_yearly_renewal",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-YEARLY-RENEWAL",
		ExternalOrderNo: "mock_cs_yearly_renewal",
		Amount:          proPlan.PricePerSeatYearly * 2,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle renewal payment: %v", err)
	}

	// 4. Verify subscription renewed for 1 year
	updatedSub, _ := service.GetSubscription(ctx, 1)
	expectedEnd := originalPeriodEnd.AddDate(1, 0, 0)
	if updatedSub.CurrentPeriodEnd.Before(expectedEnd.Add(-time.Hour)) {
		t.Error("expected period end to be extended by 1 year")
	}
}

// TestIntegrationActivateNewSubscription tests activation of brand new subscription
func TestIntegrationActivateNewSubscription(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Get pro plan (no existing subscription)
	proPlan, _ := service.GetPlan(ctx, "pro")

	// Use org ID 999 to avoid existing subscriptions
	// 2. Create payment order
	paymentMethod := billing.PaymentMethodCard
	order := &billing.PaymentOrder{
		OrganizationID:  999,
		OrderNo:         "ORD-NEW-SUB-001",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           3,
		Amount:          proPlan.PricePerSeatMonthly * 3,
		ActualAmount:    proPlan.PricePerSeatMonthly * 3,
		PaymentProvider: billing.PaymentProviderStripe,
		PaymentMethod:   &paymentMethod,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_new_sub",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-NEW-SUB-001",
		ExternalOrderNo: "mock_cs_new_sub",
		CustomerID:      "cus_new_999",
		SubscriptionID:  "sub_new_999",
		Amount:          proPlan.PricePerSeatMonthly * 3,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to activate new subscription: %v", err)
	}

	// 4. Verify new subscription created
	newSub, err := service.GetSubscription(ctx, 999)
	if err != nil {
		t.Fatalf("expected subscription to be created, got error: %v", err)
	}
	if newSub.PlanID != proPlan.ID {
		t.Errorf("expected plan ID %d, got %d", proPlan.ID, newSub.PlanID)
	}
	if newSub.SeatCount != 3 {
		t.Errorf("expected 3 seats, got %d", newSub.SeatCount)
	}
	if newSub.Status != billing.SubscriptionStatusActive {
		t.Errorf("expected active status, got %s", newSub.Status)
	}
	if newSub.StripeCustomerID == nil || *newSub.StripeCustomerID != "cus_new_999" {
		t.Error("expected Stripe customer ID to be set")
	}
	if newSub.StripeSubscriptionID == nil || *newSub.StripeSubscriptionID != "sub_new_999" {
		t.Error("expected Stripe subscription ID to be set")
	}
}

// TestIntegrationActivateNewSubscriptionYearly tests activation of yearly subscription
func TestIntegrationActivateNewSubscriptionYearly(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	// 1. Get pro plan
	proPlan, _ := service.GetPlan(ctx, "pro")

	// 2. Create payment order for yearly subscription
	order := &billing.PaymentOrder{
		OrganizationID:  998,
		OrderNo:         "ORD-NEW-YEARLY-001",
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleYearly,
		Seats:           1,
		Amount:          proPlan.PricePerSeatYearly,
		ActualAmount:    proPlan.PricePerSeatYearly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// 3. Simulate payment succeeded
	event := &payment.WebhookEvent{
		EventID:         "evt_new_yearly",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-NEW-YEARLY-001",
		ExternalOrderNo: "mock_cs_new_yearly",
		Amount:          proPlan.PricePerSeatYearly,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to activate yearly subscription: %v", err)
	}

	// 4. Verify subscription period is 1 year
	newSub, _ := service.GetSubscription(ctx, 998)
	expectedEnd := newSub.CurrentPeriodStart.AddDate(1, 0, 0)
	if !newSub.CurrentPeriodEnd.Truncate(time.Hour).Equal(expectedEnd.Truncate(time.Hour)) {
		t.Error("expected period end to be 1 year from start")
	}
}

// TestIntegrationPaymentSucceededByExternalOrderNo tests finding order by external order no
func TestIntegrationPaymentSucceededByExternalOrderNo(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	service.CreateSubscription(ctx, 1, "based")

	proPlan, _ := service.GetPlan(ctx, "pro")
	externalNo := "mock_cs_external_123"

	// Create order with external order no but no internal order no in event
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-INTERNAL-001",
		ExternalOrderNo: &externalNo,
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          proPlan.PricePerSeatMonthly,
		ActualAmount:    proPlan.PricePerSeatMonthly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// Event only has external order no
	event := &payment.WebhookEvent{
		EventID:         "evt_external",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		ExternalOrderNo: externalNo,
		Amount:          proPlan.PricePerSeatMonthly,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment: %v", err)
	}

	// Verify order found and updated
	updatedOrder, _ := service.GetPaymentOrderByNo(ctx, "ORD-INTERNAL-001")
	if updatedOrder.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected succeeded status, got %s", updatedOrder.Status)
	}
}

// TestIntegrationPaymentFailedNoOrder tests payment failed with no order
func TestIntegrationPaymentFailedNoOrder(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	// Event for non-existent order (not a recurring payment)
	event := &payment.WebhookEvent{
		EventID:      "evt_fail_no_order",
		EventType:    "payment_intent.payment_failed",
		Provider:     "mock",
		OrderNo:      "ORD-NONEXISTENT",
		Amount:       19.99,
		Currency:     "USD",
		Status:       billing.OrderStatusFailed,
		FailedReason: "Card declined",
	}

	// Should not error
	err := service.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected no error for non-existent order, got %v", err)
	}
}

// TestIntegrationUpgradePlanWithNilPlanID tests upgrade with nil plan ID
func TestIntegrationUpgradePlanWithNilPlanID(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	service.CreateSubscription(ctx, 1, "based")

	// Create invalid order with nil plan ID
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-INVALID-UPGRADE",
		OrderType:       billing.OrderTypePlanUpgrade,
		PlanID:          nil, // Invalid
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          19.99,
		ActualAmount:    19.99,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	event := &payment.WebhookEvent{
		EventID:         "evt_invalid_upgrade",
		EventType:       "checkout.session.completed",
		Provider:        "mock",
		OrderNo:         "ORD-INVALID-UPGRADE",
		ExternalOrderNo: "mock_cs_invalid",
		Amount:          19.99,
		Currency:        "USD",
		Status:          billing.OrderStatusSucceeded,
	}

	// Should return error for invalid plan
	err := service.HandlePaymentSucceeded(c, event)
	if err != ErrInvalidPlan {
		t.Errorf("expected ErrInvalidPlan, got %v", err)
	}
}

// TestIntegrationRecurringPaymentFailureNoSubscription tests failure when subscription not found
func TestIntegrationRecurringPaymentFailureNoSubscription(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	// Subscription ID that doesn't exist
	event := &payment.WebhookEvent{
		EventID:        "evt_no_sub_failure",
		EventType:      "invoice.payment_failed",
		Provider:       "mock",
		SubscriptionID: "sub_nonexistent",
		CustomerID:     "cus_nonexistent",
		Amount:         19.99,
		Currency:       "USD",
		Status:         billing.OrderStatusFailed,
		FailedReason:   "Card declined",
	}

	// Should not error - just ignores missing subscription
	err := service.HandlePaymentFailed(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationRecurringPaymentSuccessNoSubscription tests success when subscription not found
func TestIntegrationRecurringPaymentSuccessNoSubscription(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	// Subscription ID that doesn't exist (recurring payment with no matching subscription)
	event := &payment.WebhookEvent{
		EventID:        "evt_no_sub_success",
		EventType:      "invoice.paid",
		Provider:       "mock",
		SubscriptionID: "sub_nonexistent",
		CustomerID:     "cus_nonexistent",
		Amount:         19.99,
		Currency:       "USD",
		Status:         billing.OrderStatusSucceeded,
	}

	// Should not error - just ignores missing subscription
	err := service.HandlePaymentSucceeded(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationPaymentFailedByExternalOrderNo tests finding failed order by external order no
func TestIntegrationPaymentFailedByExternalOrderNo(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	service.CreateSubscription(ctx, 1, "based")

	proPlan, _ := service.GetPlan(ctx, "pro")
	externalNo := "mock_cs_external_fail"

	// Create order
	order := &billing.PaymentOrder{
		OrganizationID:  1,
		OrderNo:         "ORD-FAIL-EXTERNAL",
		ExternalOrderNo: &externalNo,
		OrderType:       billing.OrderTypeSubscription,
		PlanID:          &proPlan.ID,
		BillingCycle:    billing.BillingCycleMonthly,
		Seats:           1,
		Amount:          proPlan.PricePerSeatMonthly,
		ActualAmount:    proPlan.PricePerSeatMonthly,
		PaymentProvider: billing.PaymentProviderStripe,
		Status:          billing.OrderStatusPending,
		CreatedByID:     1,
	}
	service.CreatePaymentOrder(ctx, order)

	// Event only has external order no
	event := &payment.WebhookEvent{
		EventID:         "evt_fail_external",
		EventType:       "payment_intent.payment_failed",
		Provider:        "mock",
		ExternalOrderNo: externalNo,
		Amount:          proPlan.PricePerSeatMonthly,
		Currency:        "USD",
		Status:          billing.OrderStatusFailed,
		FailedReason:    "Card declined",
	}

	err := service.HandlePaymentFailed(c, event)
	if err != nil {
		t.Fatalf("failed to handle payment failed: %v", err)
	}

	// Verify order found and updated
	updatedOrder, _ := service.GetPaymentOrderByNo(ctx, "ORD-FAIL-EXTERNAL")
	if updatedOrder.Status != billing.OrderStatusFailed {
		t.Errorf("expected failed status, got %s", updatedOrder.Status)
	}
}

// TestIntegrationSubscriptionCanceledNoSubscription tests cancellation when subscription not found
func TestIntegrationSubscriptionCanceledNoSubscription(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	event := &payment.WebhookEvent{
		EventID:        "evt_cancel_no_sub",
		EventType:      "customer.subscription.deleted",
		Provider:       "mock",
		SubscriptionID: "sub_nonexistent",
		CustomerID:     "cus_nonexistent",
		Status:         billing.SubscriptionStatusCanceled,
	}

	// Should not error - just ignores missing subscription
	err := service.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationSubscriptionCanceledEmptySubscriptionID tests cancellation with empty ID
func TestIntegrationSubscriptionCanceledEmptySubscriptionID(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	event := &payment.WebhookEvent{
		EventID:        "evt_cancel_empty",
		EventType:      "customer.subscription.deleted",
		Provider:       "mock",
		SubscriptionID: "", // Empty
		CustomerID:     "cus_any",
		Status:         billing.SubscriptionStatusCanceled,
	}

	// Should not error - exits early
	err := service.HandleSubscriptionCanceled(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationSubscriptionUpdatedNoSubscription tests update when subscription not found
func TestIntegrationSubscriptionUpdatedNoSubscription(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	event := &payment.WebhookEvent{
		EventID:        "evt_update_no_sub",
		EventType:      "customer.subscription.updated",
		Provider:       "mock",
		SubscriptionID: "sub_nonexistent",
		CustomerID:     "cus_nonexistent",
		Status:         "active",
	}

	// Should not error - just ignores missing subscription
	err := service.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationSubscriptionUpdatedEmptySubscriptionID tests update with empty ID
func TestIntegrationSubscriptionUpdatedEmptySubscriptionID(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	c, _ := createTestGinContext()

	event := &payment.WebhookEvent{
		EventID:        "evt_update_empty",
		EventType:      "customer.subscription.updated",
		Provider:       "mock",
		SubscriptionID: "", // Empty
		CustomerID:     "cus_any",
		Status:         "active",
	}

	// Should not error - exits early
	err := service.HandleSubscriptionUpdated(c, event)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestIntegrationSubscriptionUpdatedVariousStatuses tests different status updates
func TestIntegrationSubscriptionUpdatedVariousStatuses(t *testing.T) {
	service, _ := setupIntegrationTestService(t)
	ctx := context.Background()
	c, _ := createTestGinContext()

	proPlan, _ := service.GetPlan(ctx, "pro")

	statuses := []struct {
		stripeStatus   string
		expectedStatus string
	}{
		{"active", billing.SubscriptionStatusActive},
		{"trialing", billing.SubscriptionStatusTrialing},
		{"canceled", billing.SubscriptionStatusCanceled},
	}

	for i, tc := range statuses {
		stripeSubID := "sub_status_test_" + tc.stripeStatus
		stripeCusID := "cus_status_test"
		now := time.Now()

		// Create subscription
		sub := &billing.Subscription{
			OrganizationID:       int64(100 + i),
			PlanID:               proPlan.ID,
			Status:               billing.SubscriptionStatusActive,
			BillingCycle:         billing.BillingCycleMonthly,
			CurrentPeriodStart:   now,
			CurrentPeriodEnd:     now.AddDate(0, 1, 0),
			StripeSubscriptionID: &stripeSubID,
			StripeCustomerID:     &stripeCusID,
		}
		service.db.Create(sub)

		// Send update event
		event := &payment.WebhookEvent{
			EventID:        "evt_status_" + tc.stripeStatus,
			EventType:      "customer.subscription.updated",
			Provider:       "mock",
			SubscriptionID: stripeSubID,
			CustomerID:     stripeCusID,
			Status:         tc.stripeStatus,
		}

		err := service.HandleSubscriptionUpdated(c, event)
		if err != nil {
			t.Errorf("failed to update status to %s: %v", tc.stripeStatus, err)
		}

		updatedSub, _ := service.GetSubscription(ctx, int64(100+i))
		if updatedSub.Status != tc.expectedStatus {
			t.Errorf("expected status %s, got %s", tc.expectedStatus, updatedSub.Status)
		}
	}
}
