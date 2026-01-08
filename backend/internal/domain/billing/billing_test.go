package billing

import (
	"testing"
	"time"
)

// --- Test Features ---

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

// --- Test Plan Constants ---

func TestPlanConstants(t *testing.T) {
	if PlanFree != "free" {
		t.Errorf("expected 'free', got %s", PlanFree)
	}
	if PlanPro != "pro" {
		t.Errorf("expected 'pro', got %s", PlanPro)
	}
	if PlanEnterprise != "enterprise" {
		t.Errorf("expected 'enterprise', got %s", PlanEnterprise)
	}
}

// --- Test SubscriptionPlan ---

func TestSubscriptionPlanTableName(t *testing.T) {
	sp := SubscriptionPlan{}
	if sp.TableName() != "subscription_plans" {
		t.Errorf("expected 'subscription_plans', got %s", sp.TableName())
	}
}

func TestSubscriptionPlanStruct(t *testing.T) {
	now := time.Now()

	sp := SubscriptionPlan{
		ID:                     1,
		Name:                   PlanPro,
		DisplayName:            "Pro Plan",
		PricePerSeatMonthly:    19.99,
		IncludedSessionMinutes: 1000,
		PricePerExtraMinute:    0.05,
		MaxUsers:               50,
		MaxRunners:             10,
		MaxRepositories:        100,
		Features:               Features{"priority_support": true},
		IsActive:               true,
		CreatedAt:              now,
	}

	if sp.ID != 1 {
		t.Errorf("expected ID 1, got %d", sp.ID)
	}
	if sp.Name != "pro" {
		t.Errorf("expected Name 'pro', got %s", sp.Name)
	}
	if sp.PricePerSeatMonthly != 19.99 {
		t.Errorf("expected PricePerSeatMonthly 19.99, got %f", sp.PricePerSeatMonthly)
	}
	if sp.MaxUsers != 50 {
		t.Errorf("expected MaxUsers 50, got %d", sp.MaxUsers)
	}
}

// --- Test Subscription Status Constants ---

func TestSubscriptionStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{SubscriptionStatusActive, "active"},
		{SubscriptionStatusPastDue, "past_due"},
		{SubscriptionStatusCanceled, "canceled"},
		{SubscriptionStatusTrialing, "trialing"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

// --- Test Billing Cycle Constants ---

func TestBillingCycleConstants(t *testing.T) {
	if BillingCycleMonthly != "monthly" {
		t.Errorf("expected 'monthly', got %s", BillingCycleMonthly)
	}
	if BillingCycleYearly != "yearly" {
		t.Errorf("expected 'yearly', got %s", BillingCycleYearly)
	}
}

// --- Test CustomQuotas ---

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

// --- Test Subscription ---

func TestSubscriptionTableName(t *testing.T) {
	s := Subscription{}
	if s.TableName() != "subscriptions" {
		t.Errorf("expected 'subscriptions', got %s", s.TableName())
	}
}

func TestSubscriptionStruct(t *testing.T) {
	now := time.Now()
	stripeCustomerID := "cus_123"
	stripeSubID := "sub_456"

	s := Subscription{
		ID:                   1,
		OrganizationID:       100,
		PlanID:               2,
		Status:               SubscriptionStatusActive,
		BillingCycle:         BillingCycleMonthly,
		CurrentPeriodStart:   now,
		CurrentPeriodEnd:     now.Add(30 * 24 * time.Hour),
		StripeCustomerID:     &stripeCustomerID,
		StripeSubscriptionID: &stripeSubID,
		CustomQuotas:         CustomQuotas{"extra_minutes": 1000},
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if s.ID != 1 {
		t.Errorf("expected ID 1, got %d", s.ID)
	}
	if s.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", s.OrganizationID)
	}
	if s.Status != "active" {
		t.Errorf("expected Status 'active', got %s", s.Status)
	}
	if *s.StripeCustomerID != "cus_123" {
		t.Errorf("expected StripeCustomerID 'cus_123', got %s", *s.StripeCustomerID)
	}
}

// --- Test Usage Type Constants ---

func TestUsageTypeConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{UsageTypeSessionMinutes, "session_minutes"},
		{UsageTypeStorageGB, "storage_gb"},
		{UsageTypeAPIRequests, "api_requests"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

// --- Test UsageMetadata ---

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
	err := um.Scan([]byte(`{"session_id":"sess-123","user_id":50}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if um["session_id"] != "sess-123" {
		t.Errorf("expected session_id 'sess-123', got %v", um["session_id"])
	}
}

func TestUsageMetadataScanInvalidType(t *testing.T) {
	var um UsageMetadata
	err := um.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
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
	um := UsageMetadata{"session_id": "sess-123"}
	val, err := um.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// --- Test UsageRecord ---

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
		UsageType:      UsageTypeSessionMinutes,
		Quantity:       120.5,
		PeriodStart:    now,
		PeriodEnd:      now.Add(24 * time.Hour),
		Metadata:       UsageMetadata{"session_id": "sess-123"},
		CreatedAt:      now,
	}

	if ur.ID != 1 {
		t.Errorf("expected ID 1, got %d", ur.ID)
	}
	if ur.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", ur.OrganizationID)
	}
	if ur.UsageType != "session_minutes" {
		t.Errorf("expected UsageType 'session_minutes', got %s", ur.UsageType)
	}
	if ur.Quantity != 120.5 {
		t.Errorf("expected Quantity 120.5, got %f", ur.Quantity)
	}
}

// --- Benchmark Tests ---

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

func BenchmarkUsageRecordTableName(b *testing.B) {
	ur := UsageRecord{}
	for i := 0; i < b.N; i++ {
		ur.TableName()
	}
}
