package billing

import (
	"testing"
	"time"
)

// ===========================================
// Test Features (plan.go)
// ===========================================

func TestFeaturesScanNil(t *testing.T) {
	var f Features
	err := f.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if f != nil {
		t.Error("expected nil Features")
	}
}

func TestFeaturesScanValid(t *testing.T) {
	var f Features
	err := f.Scan([]byte(`{"unlimited_seats":true,"max_runners":10}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if f["unlimited_seats"] != true {
		t.Errorf("expected unlimited_seats true, got %v", f["unlimited_seats"])
	}
}

func TestFeaturesScanInvalidType(t *testing.T) {
	var f Features
	err := f.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestFeaturesScanInvalidJSON(t *testing.T) {
	var f Features
	err := f.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFeaturesValueNil(t *testing.T) {
	var f Features
	val, err := f.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestFeaturesValueValid(t *testing.T) {
	f := Features{"feature_x": true}
	val, err := f.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test Constants (constants.go)
// ===========================================

func TestPlanConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PlanFree", PlanFree, "free"},
		{"PlanPro", PlanPro, "pro"},
		{"PlanEnterprise", PlanEnterprise, "enterprise"},
		{"PlanOnPremise", PlanOnPremise, "onpremise"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestSubscriptionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Active", SubscriptionStatusActive, "active"},
		{"PastDue", SubscriptionStatusPastDue, "past_due"},
		{"Canceled", SubscriptionStatusCanceled, "canceled"},
		{"Trialing", SubscriptionStatusTrialing, "trialing"},
		{"Frozen", SubscriptionStatusFrozen, "frozen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestPaymentProviderConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Stripe", PaymentProviderStripe, "stripe"},
		{"Alipay", PaymentProviderAlipay, "alipay"},
		{"WeChat", PaymentProviderWeChat, "wechat"},
		{"License", PaymentProviderLicense, "license"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestPaymentMethodConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Card", PaymentMethodCard, "card"},
		{"AlipayQR", PaymentMethodAlipayQR, "alipay_qr"},
		{"AlipayAgreement", PaymentMethodAlipayAgreement, "alipay_agreement"},
		{"WeChatNative", PaymentMethodWeChatNative, "wechat_native"},
		{"WeChatContract", PaymentMethodWeChatContract, "wechat_contract"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestBillingCycleConstants(t *testing.T) {
	if BillingCycleMonthly != "monthly" {
		t.Errorf("expected 'monthly', got %s", BillingCycleMonthly)
	}
	if BillingCycleYearly != "yearly" {
		t.Errorf("expected 'yearly', got %s", BillingCycleYearly)
	}
}

func TestUsageTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PodMinutes", UsageTypePodMinutes, "pod_minutes"},
		{"StorageGB", UsageTypeStorageGB, "storage_gb"},
		{"APIRequests", UsageTypeAPIRequests, "api_requests"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestOrderTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Subscription", OrderTypeSubscription, "subscription"},
		{"SeatPurchase", OrderTypeSeatPurchase, "seat_purchase"},
		{"PlanUpgrade", OrderTypePlanUpgrade, "plan_upgrade"},
		{"Renewal", OrderTypeRenewal, "renewal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestOrderStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Pending", OrderStatusPending, "pending"},
		{"Processing", OrderStatusProcessing, "processing"},
		{"Succeeded", OrderStatusSucceeded, "succeeded"},
		{"Failed", OrderStatusFailed, "failed"},
		{"Canceled", OrderStatusCanceled, "canceled"},
		{"Refunded", OrderStatusRefunded, "refunded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestTransactionTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Payment", TransactionTypePayment, "payment"},
		{"Refund", TransactionTypeRefund, "refund"},
		{"Chargeback", TransactionTypeChargeback, "chargeback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestTransactionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Pending", TransactionStatusPending, "pending"},
		{"Succeeded", TransactionStatusSucceeded, "succeeded"},
		{"Failed", TransactionStatusFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

func TestInvoiceStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Draft", InvoiceStatusDraft, "draft"},
		{"Issued", InvoiceStatusIssued, "issued"},
		{"Paid", InvoiceStatusPaid, "paid"},
		{"Void", InvoiceStatusVoid, "void"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
			}
		})
	}
}

// ===========================================
// Test SubscriptionPlan (plan.go)
// ===========================================

func TestSubscriptionPlanTableName(t *testing.T) {
	sp := SubscriptionPlan{}
	if sp.TableName() != "subscription_plans" {
		t.Errorf("expected 'subscription_plans', got %s", sp.TableName())
	}
}

func TestSubscriptionPlanGetPrice(t *testing.T) {
	sp := SubscriptionPlan{
		PricePerSeatMonthly: 19.99,
		PricePerSeatYearly:  199.90,
	}

	if price := sp.GetPrice(BillingCycleMonthly); price != 19.99 {
		t.Errorf("expected monthly price 19.99, got %f", price)
	}
	if price := sp.GetPrice(BillingCycleYearly); price != 199.90 {
		t.Errorf("expected yearly price 199.90, got %f", price)
	}
	// Default to monthly for unknown cycle
	if price := sp.GetPrice("unknown"); price != 19.99 {
		t.Errorf("expected default monthly price 19.99, got %f", price)
	}
}

func TestSubscriptionPlanStruct(t *testing.T) {
	now := time.Now()
	monthlyPriceID := "price_monthly_123"
	yearlyPriceID := "price_yearly_456"

	sp := SubscriptionPlan{
		ID:                   1,
		Name:                 PlanPro,
		DisplayName:          "Pro Plan",
		PricePerSeatMonthly:  19.99,
		PricePerSeatYearly:   199.90,
		IncludedPodMinutes:   1000,
		PricePerExtraMinute:  0.05,
		MaxUsers:             50,
		MaxRunners:           10,
		MaxConcurrentPods:    5,
		MaxRepositories:      100,
		Features:             Features{"priority_support": true},
		StripePriceIDMonthly: &monthlyPriceID,
		StripePriceIDYearly:  &yearlyPriceID,
		IsActive:             true,
		CreatedAt:            now,
	}

	if sp.ID != 1 {
		t.Errorf("expected ID 1, got %d", sp.ID)
	}
	if sp.Name != "pro" {
		t.Errorf("expected Name 'pro', got %s", sp.Name)
	}
	if sp.MaxConcurrentPods != 5 {
		t.Errorf("expected MaxConcurrentPods 5, got %d", sp.MaxConcurrentPods)
	}
	if *sp.StripePriceIDMonthly != "price_monthly_123" {
		t.Errorf("expected StripePriceIDMonthly, got %s", *sp.StripePriceIDMonthly)
	}
}

// ===========================================
// Test CustomQuotas (subscription.go)
// ===========================================

func TestCustomQuotasScanNil(t *testing.T) {
	var cq CustomQuotas
	err := cq.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cq != nil {
		t.Error("expected nil CustomQuotas")
	}
}

func TestCustomQuotasScanValid(t *testing.T) {
	var cq CustomQuotas
	err := cq.Scan([]byte(`{"max_runners":20,"extra_minutes":5000}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cq["max_runners"] != float64(20) {
		t.Errorf("expected max_runners 20, got %v", cq["max_runners"])
	}
}

func TestCustomQuotasScanInvalidType(t *testing.T) {
	var cq CustomQuotas
	err := cq.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestCustomQuotasScanInvalidJSON(t *testing.T) {
	var cq CustomQuotas
	err := cq.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCustomQuotasValueNil(t *testing.T) {
	var cq CustomQuotas
	val, err := cq.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestCustomQuotasValueValid(t *testing.T) {
	cq := CustomQuotas{"max_runners": 20}
	val, err := cq.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test Subscription (subscription.go)
// ===========================================

func TestSubscriptionTableName(t *testing.T) {
	s := Subscription{}
	if s.TableName() != "subscriptions" {
		t.Errorf("expected 'subscriptions', got %s", s.TableName())
	}
}

func TestSubscriptionIsFrozen(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		sub      Subscription
		expected bool
	}{
		{
			name:     "active subscription",
			sub:      Subscription{Status: SubscriptionStatusActive},
			expected: false,
		},
		{
			name:     "frozen status",
			sub:      Subscription{Status: SubscriptionStatusFrozen},
			expected: true,
		},
		{
			name:     "frozen_at set",
			sub:      Subscription{Status: SubscriptionStatusActive, FrozenAt: &now},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.IsFrozen(); got != tt.expected {
				t.Errorf("IsFrozen() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSubscriptionIsActive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		sub      Subscription
		expected bool
	}{
		{
			name:     "active subscription",
			sub:      Subscription{Status: SubscriptionStatusActive},
			expected: true,
		},
		{
			name:     "canceled subscription",
			sub:      Subscription{Status: SubscriptionStatusCanceled},
			expected: false,
		},
		{
			name:     "active but frozen",
			sub:      Subscription{Status: SubscriptionStatusActive, FrozenAt: &now},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSubscriptionCanAddSeats(t *testing.T) {
	freePlan := &SubscriptionPlan{Name: PlanFree}
	proPlan := &SubscriptionPlan{Name: PlanPro}

	tests := []struct {
		name     string
		sub      Subscription
		plan     *SubscriptionPlan
		expected bool
	}{
		{
			name:     "free plan cannot add seats",
			sub:      Subscription{Plan: freePlan},
			plan:     nil,
			expected: false,
		},
		{
			name:     "pro plan can add seats",
			sub:      Subscription{Plan: proPlan},
			plan:     nil,
			expected: true,
		},
		{
			name:     "explicit plan overrides subscription plan",
			sub:      Subscription{Plan: freePlan},
			plan:     proPlan,
			expected: true,
		},
		{
			name:     "nil plan",
			sub:      Subscription{},
			plan:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.CanAddSeats(tt.plan); got != tt.expected {
				t.Errorf("CanAddSeats() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSubscriptionGetAvailableSeats(t *testing.T) {
	sub := Subscription{SeatCount: 10}

	tests := []struct {
		name      string
		usedSeats int
		expected  int
	}{
		{"no seats used", 0, 10},
		{"half used", 5, 5},
		{"all used", 10, 0},
		{"over used", 12, -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sub.GetAvailableSeats(tt.usedSeats); got != tt.expected {
				t.Errorf("GetAvailableSeats(%d) = %d, want %d", tt.usedSeats, got, tt.expected)
			}
		})
	}
}

func TestSubscriptionStruct(t *testing.T) {
	now := time.Now()
	stripeCustomerID := "cus_123"
	stripeSubID := "sub_456"
	paymentProvider := PaymentProviderStripe
	paymentMethod := PaymentMethodCard

	s := Subscription{
		ID:                   1,
		OrganizationID:       100,
		PlanID:               2,
		Status:               SubscriptionStatusActive,
		BillingCycle:         BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
		PaymentProvider:      &paymentProvider,
		PaymentMethod:        &paymentMethod,
		AutoRenew:            true,
		SeatCount:            5,
		StripeCustomerID:     &stripeCustomerID,
		StripeSubscriptionID: &stripeSubID,
		CustomQuotas:         CustomQuotas{"extra_minutes": 1000},
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if s.ID != 1 {
		t.Errorf("expected ID 1, got %d", s.ID)
	}
	if s.SeatCount != 5 {
		t.Errorf("expected SeatCount 5, got %d", s.SeatCount)
	}
	if !s.AutoRenew {
		t.Error("expected AutoRenew true")
	}
}

// ===========================================
// Test UsageMetadata (usage.go)
// ===========================================

func TestUsageMetadataScanNil(t *testing.T) {
	var um UsageMetadata
	err := um.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if um != nil {
		t.Error("expected nil UsageMetadata")
	}
}

func TestUsageMetadataScanValid(t *testing.T) {
	var um UsageMetadata
	err := um.Scan([]byte(`{"pod_id":"pod-123","user_id":50}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if um["pod_id"] != "pod-123" {
		t.Errorf("expected pod_id 'pod-123', got %v", um["pod_id"])
	}
}

func TestUsageMetadataScanInvalidType(t *testing.T) {
	var um UsageMetadata
	err := um.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestUsageMetadataScanInvalidJSON(t *testing.T) {
	var um UsageMetadata
	err := um.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUsageMetadataValueNil(t *testing.T) {
	var um UsageMetadata
	val, err := um.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestUsageMetadataValueValid(t *testing.T) {
	um := UsageMetadata{"pod_id": "pod-123"}
	val, err := um.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test UsageRecord (usage.go)
// ===========================================

func TestUsageRecordTableName(t *testing.T) {
	ur := UsageRecord{}
	if ur.TableName() != "usage_records" {
		t.Errorf("expected 'usage_records', got %s", ur.TableName())
	}
}

func TestUsageRecordStruct(t *testing.T) {
	now := time.Now()

	ur := UsageRecord{
		ID:             1,
		OrganizationID: 100,
		UsageType:      UsageTypePodMinutes,
		Quantity:       120.5,
		PeriodStart:    now,
		PeriodEnd:      now.Add(24 * time.Hour),
		Metadata:       UsageMetadata{"pod_id": "pod-123"},
		CreatedAt:      now,
	}

	if ur.ID != 1 {
		t.Errorf("expected ID 1, got %d", ur.ID)
	}
	if ur.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", ur.OrganizationID)
	}
	if ur.UsageType != "pod_minutes" {
		t.Errorf("expected UsageType 'pod_minutes', got %s", ur.UsageType)
	}
	if ur.Quantity != 120.5 {
		t.Errorf("expected Quantity 120.5, got %f", ur.Quantity)
	}
}

// ===========================================
// Test OrderMetadata (order.go)
// ===========================================

func TestOrderMetadataScanNil(t *testing.T) {
	var om OrderMetadata
	err := om.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if om != nil {
		t.Error("expected nil OrderMetadata")
	}
}

func TestOrderMetadataScanValid(t *testing.T) {
	var om OrderMetadata
	err := om.Scan([]byte(`{"plan_name":"pro","seats":5}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if om["plan_name"] != "pro" {
		t.Errorf("expected plan_name 'pro', got %v", om["plan_name"])
	}
}

func TestOrderMetadataScanInvalidType(t *testing.T) {
	var om OrderMetadata
	err := om.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestOrderMetadataScanInvalidJSON(t *testing.T) {
	var om OrderMetadata
	err := om.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestOrderMetadataValueNil(t *testing.T) {
	var om OrderMetadata
	val, err := om.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestOrderMetadataValueValid(t *testing.T) {
	om := OrderMetadata{"plan_name": "pro"}
	val, err := om.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test PaymentOrder (order.go)
// ===========================================

func TestPaymentOrderTableName(t *testing.T) {
	po := PaymentOrder{}
	if po.TableName() != "payment_orders" {
		t.Errorf("expected 'payment_orders', got %s", po.TableName())
	}
}

func TestPaymentOrderIsPending(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"pending", OrderStatusPending, true},
		{"succeeded", OrderStatusSucceeded, false},
		{"failed", OrderStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			po := PaymentOrder{Status: tt.status}
			if got := po.IsPending(); got != tt.expected {
				t.Errorf("IsPending() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentOrderIsSucceeded(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"succeeded", OrderStatusSucceeded, true},
		{"pending", OrderStatusPending, false},
		{"failed", OrderStatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			po := PaymentOrder{Status: tt.status}
			if got := po.IsSucceeded(); got != tt.expected {
				t.Errorf("IsSucceeded() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentOrderIsExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{"no expiry", nil, false},
		{"expired", &past, true},
		{"not expired", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			po := PaymentOrder{ExpiresAt: tt.expiresAt}
			if got := po.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentOrderStruct(t *testing.T) {
	now := time.Now()
	externalOrderNo := "ext_123"
	planID := int64(2)
	paymentMethod := PaymentMethodCard
	idempotencyKey := "idem_456"

	po := PaymentOrder{
		ID:              1,
		OrganizationID:  100,
		OrderNo:         "ORD-20240101-001",
		ExternalOrderNo: &externalOrderNo,
		OrderType:       OrderTypeSubscription,
		PlanID:          &planID,
		BillingCycle:    BillingCycleMonthly,
		Seats:           5,
		Currency:        "USD",
		Amount:          99.95,
		DiscountAmount:  10.00,
		ActualAmount:    89.95,
		PaymentProvider: PaymentProviderStripe,
		PaymentMethod:   &paymentMethod,
		Status:          OrderStatusPending,
		Metadata:        OrderMetadata{"source": "web"},
		IdempotencyKey:  &idempotencyKey,
		CreatedAt:       now,
		UpdatedAt:       now,
		CreatedByID:     1,
	}

	if po.OrderNo != "ORD-20240101-001" {
		t.Errorf("expected OrderNo, got %s", po.OrderNo)
	}
	if po.ActualAmount != 89.95 {
		t.Errorf("expected ActualAmount 89.95, got %f", po.ActualAmount)
	}
}

// ===========================================
// Test RawPayload (transaction.go)
// ===========================================

func TestRawPayloadScanNil(t *testing.T) {
	var rp RawPayload
	err := rp.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if rp != nil {
		t.Error("expected nil RawPayload")
	}
}

func TestRawPayloadScanValid(t *testing.T) {
	var rp RawPayload
	err := rp.Scan([]byte(`{"event_id":"evt_123","type":"payment"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if rp["event_id"] != "evt_123" {
		t.Errorf("expected event_id 'evt_123', got %v", rp["event_id"])
	}
}

func TestRawPayloadScanInvalidType(t *testing.T) {
	var rp RawPayload
	err := rp.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestRawPayloadScanInvalidJSON(t *testing.T) {
	var rp RawPayload
	err := rp.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRawPayloadValueNil(t *testing.T) {
	var rp RawPayload
	val, err := rp.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestRawPayloadValueValid(t *testing.T) {
	rp := RawPayload{"event_id": "evt_123"}
	val, err := rp.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test PaymentTransaction (transaction.go)
// ===========================================

func TestPaymentTransactionTableName(t *testing.T) {
	pt := PaymentTransaction{}
	if pt.TableName() != "payment_transactions" {
		t.Errorf("expected 'payment_transactions', got %s", pt.TableName())
	}
}

func TestPaymentTransactionStruct(t *testing.T) {
	now := time.Now()
	extTxnID := "txn_123"
	webhookEventID := "evt_456"
	webhookEventType := "payment_intent.succeeded"

	pt := PaymentTransaction{
		ID:                    1,
		PaymentOrderID:        10,
		TransactionType:       TransactionTypePayment,
		ExternalTransactionID: &extTxnID,
		Amount:                99.95,
		Currency:              "USD",
		Status:                TransactionStatusSucceeded,
		WebhookEventID:        &webhookEventID,
		WebhookEventType:      &webhookEventType,
		RawPayload:            RawPayload{"data": "value"},
		CreatedAt:             now,
	}

	if pt.ID != 1 {
		t.Errorf("expected ID 1, got %d", pt.ID)
	}
	if pt.TransactionType != "payment" {
		t.Errorf("expected TransactionType 'payment', got %s", pt.TransactionType)
	}
}

// ===========================================
// Test BillingAddress (invoice.go)
// ===========================================

func TestBillingAddressScanNil(t *testing.T) {
	var ba BillingAddress
	err := ba.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ba != nil {
		t.Error("expected nil BillingAddress")
	}
}

func TestBillingAddressScanValid(t *testing.T) {
	var ba BillingAddress
	err := ba.Scan([]byte(`{"line1":"123 Main St","city":"NYC"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ba["city"] != "NYC" {
		t.Errorf("expected city 'NYC', got %v", ba["city"])
	}
}

func TestBillingAddressScanInvalidType(t *testing.T) {
	var ba BillingAddress
	err := ba.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestBillingAddressScanInvalidJSON(t *testing.T) {
	var ba BillingAddress
	err := ba.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBillingAddressValueNil(t *testing.T) {
	var ba BillingAddress
	val, err := ba.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestBillingAddressValueValid(t *testing.T) {
	ba := BillingAddress{"line1": "123 Main St"}
	val, err := ba.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// ===========================================
// Test LineItems (invoice.go)
// ===========================================

func TestLineItemsScanNil(t *testing.T) {
	var li LineItems
	err := li.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if li != nil {
		t.Error("expected nil LineItems")
	}
}

func TestLineItemsScanValid(t *testing.T) {
	var li LineItems
	err := li.Scan([]byte(`[{"description":"Pro Plan","quantity":1,"unit_price":19.99,"amount":19.99}]`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(li) != 1 {
		t.Errorf("expected 1 line item, got %d", len(li))
	}
	if li[0].Description != "Pro Plan" {
		t.Errorf("expected description 'Pro Plan', got %s", li[0].Description)
	}
}

func TestLineItemsScanInvalidType(t *testing.T) {
	var li LineItems
	err := li.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestLineItemsScanInvalidJSON(t *testing.T) {
	var li LineItems
	err := li.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLineItemsValueNil(t *testing.T) {
	var li LineItems
	val, err := li.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestLineItemsValueValid(t *testing.T) {
	li := LineItems{{Description: "Pro Plan", Quantity: 1, UnitPrice: 19.99, Amount: 19.99}}
	val, err := li.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

func TestLineItemStruct(t *testing.T) {
	li := LineItem{
		Description: "Pro Plan Subscription",
		Quantity:    5,
		UnitPrice:   19.99,
		Amount:      99.95,
	}

	if li.Description != "Pro Plan Subscription" {
		t.Errorf("expected Description, got %s", li.Description)
	}
	if li.Quantity != 5 {
		t.Errorf("expected Quantity 5, got %d", li.Quantity)
	}
}

// ===========================================
// Test Invoice (invoice.go)
// ===========================================

func TestInvoiceTableName(t *testing.T) {
	inv := Invoice{}
	if inv.TableName() != "invoices" {
		t.Errorf("expected 'invoices', got %s", inv.TableName())
	}
}

func TestInvoiceIsPaid(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"paid", InvoiceStatusPaid, true},
		{"draft", InvoiceStatusDraft, false},
		{"issued", InvoiceStatusIssued, false},
		{"void", InvoiceStatusVoid, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := Invoice{Status: tt.status}
			if got := inv.IsPaid(); got != tt.expected {
				t.Errorf("IsPaid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInvoiceStruct(t *testing.T) {
	now := time.Now()
	orderID := int64(10)
	billingName := "Acme Corp"
	billingEmail := "billing@acme.com"
	pdfURL := "https://example.com/invoice.pdf"

	inv := Invoice{
		ID:             1,
		OrganizationID: 100,
		PaymentOrderID: &orderID,
		InvoiceNo:      "INV-2024-001",
		Status:         InvoiceStatusIssued,
		Currency:       "USD",
		Subtotal:       99.95,
		TaxAmount:      8.00,
		Total:          107.95,
		BillingName:    &billingName,
		BillingEmail:   &billingEmail,
		BillingAddress: BillingAddress{"city": "NYC"},
		PeriodStart:    now,
		PeriodEnd:      now.Add(30 * 24 * time.Hour),
		LineItems:      LineItems{{Description: "Pro Plan", Amount: 99.95}},
		PDFURL:         &pdfURL,
		IssuedAt:       &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if inv.InvoiceNo != "INV-2024-001" {
		t.Errorf("expected InvoiceNo, got %s", inv.InvoiceNo)
	}
	if inv.Total != 107.95 {
		t.Errorf("expected Total 107.95, got %f", inv.Total)
	}
}

// ===========================================
// Test License (license.go)
// ===========================================

func TestLicenseTableName(t *testing.T) {
	lic := License{}
	if lic.TableName() != "licenses" {
		t.Errorf("expected 'licenses', got %s", lic.TableName())
	}
}

func TestLicenseIsValid(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name     string
		license  License
		expected bool
	}{
		{
			name:     "active valid license",
			license:  License{IsActive: true},
			expected: true,
		},
		{
			name:     "inactive license",
			license:  License{IsActive: false},
			expected: false,
		},
		{
			name:     "revoked license",
			license:  License{IsActive: true, RevokedAt: &past},
			expected: false,
		},
		{
			name:     "expired license",
			license:  License{IsActive: true, ExpiresAt: &past},
			expected: false,
		},
		{
			name:     "not expired license",
			license:  License{IsActive: true, ExpiresAt: &future},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.license.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLicenseIsActivated(t *testing.T) {
	now := time.Now()
	orgID := int64(100)

	tests := []struct {
		name     string
		license  License
		expected bool
	}{
		{
			name:     "not activated",
			license:  License{},
			expected: false,
		},
		{
			name:     "only activated_at set",
			license:  License{ActivatedAt: &now},
			expected: false,
		},
		{
			name:     "only org_id set",
			license:  License{ActivatedOrgID: &orgID},
			expected: false,
		},
		{
			name:     "fully activated",
			license:  License{ActivatedAt: &now, ActivatedOrgID: &orgID},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.license.IsActivated(); got != tt.expected {
				t.Errorf("IsActivated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLicenseDaysUntilExpiry(t *testing.T) {
	tests := []struct {
		name     string
		license  License
		expected int
	}{
		{
			name:     "no expiry",
			license:  License{},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.license.DaysUntilExpiry(); got != tt.expected {
				t.Errorf("DaysUntilExpiry() = %v, want %v", got, tt.expected)
			}
		})
	}

	// Test with future expiry (approximately)
	future := time.Now().Add(10 * 24 * time.Hour)
	lic := License{ExpiresAt: &future}
	days := lic.DaysUntilExpiry()
	if days < 9 || days > 11 {
		t.Errorf("DaysUntilExpiry() = %d, want approximately 10", days)
	}
}

func TestLicenseStruct(t *testing.T) {
	now := time.Now()
	future := now.Add(365 * 24 * time.Hour)
	fingerprint := "abc123"

	lic := License{
		ID:                   1,
		LicenseKey:           "AM-ENT-2024-XXXXX",
		OrganizationName:     "Acme Corp",
		ContactEmail:         "admin@acme.com",
		PlanName:             PlanEnterprise,
		MaxUsers:             -1,
		MaxRunners:           -1,
		MaxRepositories:      -1,
		MaxConcurrentPods:    -1,
		Features:             Features{"unlimited": true},
		IssuedAt:             now,
		ExpiresAt:            &future,
		Signature:            "BASE64SIGNATURE",
		PublicKeyFingerprint: &fingerprint,
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if lic.LicenseKey != "AM-ENT-2024-XXXXX" {
		t.Errorf("expected LicenseKey, got %s", lic.LicenseKey)
	}
	if lic.MaxUsers != -1 {
		t.Errorf("expected MaxUsers -1, got %d", lic.MaxUsers)
	}
}

func TestLicenseStatusStruct(t *testing.T) {
	future := time.Now().Add(365 * 24 * time.Hour)

	ls := LicenseStatus{
		IsActive:         true,
		LicenseKey:       "AM-ENT-2024-XXXXX",
		OrganizationName: "Acme Corp",
		Plan:             PlanEnterprise,
		ExpiresAt:        &future,
		MaxUsers:         -1,
		MaxRunners:       -1,
		MaxRepositories:  -1,
		MaxPodMinutes:    -1,
		Features:         []string{"unlimited_pods", "priority_support"},
		Message:          "License is valid",
	}

	if !ls.IsActive {
		t.Error("expected IsActive true")
	}
	if len(ls.Features) != 2 {
		t.Errorf("expected 2 features, got %d", len(ls.Features))
	}
}

// ===========================================
// Benchmark Tests
// ===========================================

func BenchmarkFeaturesScan(b *testing.B) {
	data := []byte(`{"unlimited_seats":true,"max_runners":10}`)
	for i := 0; i < b.N; i++ {
		var f Features
		f.Scan(data)
	}
}

func BenchmarkFeaturesValue(b *testing.B) {
	f := Features{"feature_x": true}
	for i := 0; i < b.N; i++ {
		f.Value()
	}
}

func BenchmarkSubscriptionPlanTableName(b *testing.B) {
	sp := SubscriptionPlan{}
	for i := 0; i < b.N; i++ {
		sp.TableName()
	}
}

func BenchmarkSubscriptionIsFrozen(b *testing.B) {
	s := Subscription{Status: SubscriptionStatusActive}
	for i := 0; i < b.N; i++ {
		s.IsFrozen()
	}
}

func BenchmarkLicenseIsValid(b *testing.B) {
	lic := License{IsActive: true}
	for i := 0; i < b.N; i++ {
		lic.IsValid()
	}
}
