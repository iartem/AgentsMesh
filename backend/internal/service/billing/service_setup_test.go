package billing

import (
	"net/http/httptest"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create all required tables
	tables := []string{
		`CREATE TABLE IF NOT EXISTS subscription_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			price_per_seat_monthly REAL NOT NULL DEFAULT 0,
			price_per_seat_yearly REAL NOT NULL DEFAULT 0,
			included_pod_minutes INTEGER NOT NULL DEFAULT 0,
			price_per_extra_minute REAL NOT NULL DEFAULT 0,
			max_users INTEGER NOT NULL DEFAULT 0,
			max_runners INTEGER NOT NULL DEFAULT 0,
			max_concurrent_pods INTEGER NOT NULL DEFAULT 0,
			max_repositories INTEGER NOT NULL DEFAULT 0,
			features TEXT,
			stripe_price_id_monthly TEXT,
			stripe_price_id_yearly TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL UNIQUE,
			plan_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			billing_cycle TEXT NOT NULL DEFAULT 'monthly',
			current_period_start DATETIME NOT NULL,
			current_period_end DATETIME NOT NULL,
			payment_provider TEXT,
			payment_method TEXT,
			auto_renew INTEGER NOT NULL DEFAULT 0,
			seat_count INTEGER NOT NULL DEFAULT 1,
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			alipay_agreement_no TEXT,
			wechat_contract_id TEXT,
			canceled_at DATETIME,
			cancel_at_period_end INTEGER NOT NULL DEFAULT 0,
			frozen_at DATETIME,
			downgrade_to_plan TEXT,
			next_billing_cycle TEXT,
			custom_quotas TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS usage_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			usage_type TEXT NOT NULL,
			quantity REAL NOT NULL DEFAULT 0,
			period_start DATETIME NOT NULL,
			period_end DATETIME NOT NULL,
			metadata TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS payment_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			order_no TEXT NOT NULL UNIQUE,
			external_order_no TEXT,
			order_type TEXT NOT NULL,
			plan_id INTEGER,
			billing_cycle TEXT,
			seats INTEGER DEFAULT 1,
			currency TEXT NOT NULL DEFAULT 'USD',
			amount REAL NOT NULL,
			discount_amount REAL DEFAULT 0,
			actual_amount REAL NOT NULL,
			payment_provider TEXT NOT NULL,
			payment_method TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			metadata TEXT,
			failure_reason TEXT,
			idempotency_key TEXT UNIQUE,
			expires_at DATETIME,
			paid_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_by_id INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS payment_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payment_order_id INTEGER NOT NULL,
			transaction_type TEXT NOT NULL,
			external_transaction_id TEXT,
			amount REAL NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			status TEXT NOT NULL,
			webhook_event_id TEXT,
			webhook_event_type TEXT,
			raw_payload TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS invoices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			payment_order_id INTEGER,
			invoice_no TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'draft',
			currency TEXT NOT NULL DEFAULT 'USD',
			subtotal REAL NOT NULL,
			tax_amount REAL DEFAULT 0,
			total REAL NOT NULL,
			billing_name TEXT,
			billing_email TEXT,
			billing_address TEXT,
			period_start DATETIME NOT NULL,
			period_end DATETIME NOT NULL,
			line_items TEXT NOT NULL DEFAULT '[]',
			pdf_url TEXT,
			issued_at DATETIME,
			due_at DATETIME,
			paid_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS organization_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS runners (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pods (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS invitations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			email TEXT NOT NULL,
			accepted_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS plan_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id INTEGER NOT NULL,
			currency TEXT NOT NULL,
			price_monthly REAL NOT NULL,
			price_yearly REAL NOT NULL,
			stripe_price_id_monthly TEXT,
			stripe_price_id_yearly TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(plan_id, currency)
		)`,
	}

	for _, sql := range tables {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
	}

	return db
}

func seedTestPlan(t *testing.T, db *gorm.DB) *billing.SubscriptionPlan {
	plan := &billing.SubscriptionPlan{
		Name:                "based",
		DisplayName:         "Based Plan",
		PricePerSeatMonthly: 9.9,
		PricePerSeatYearly:  99,
		IncludedPodMinutes:  100,
		PricePerExtraMinute: 0,
		MaxUsers:            1, // Based plan has fixed 1 seat
		MaxRunners:          1,
		MaxConcurrentPods:   5,
		MaxRepositories:     5,
		IsActive:            true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed plan: %v", err)
	}

	// Seed plan prices (Single Source of Truth)
	prices := []billing.PlanPrice{
		{PlanID: plan.ID, Currency: billing.CurrencyUSD, PriceMonthly: 9.9, PriceYearly: 99},
		{PlanID: plan.ID, Currency: billing.CurrencyCNY, PriceMonthly: 69, PriceYearly: 690},
	}
	for _, price := range prices {
		if err := db.Create(&price).Error; err != nil {
			t.Fatalf("failed to seed plan price: %v", err)
		}
	}

	return plan
}

func seedProPlan(t *testing.T, db *gorm.DB) *billing.SubscriptionPlan {
	plan := &billing.SubscriptionPlan{
		Name:                "pro",
		DisplayName:         "Pro Plan",
		PricePerSeatMonthly: 19.99,
		PricePerSeatYearly:  199.90,
		IncludedPodMinutes:  1000,
		PricePerExtraMinute: 0.05,
		MaxUsers:            50,
		MaxRunners:          10,
		MaxConcurrentPods:   5,
		MaxRepositories:     100,
		IsActive:            true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed pro plan: %v", err)
	}

	// Seed plan prices (Single Source of Truth)
	prices := []billing.PlanPrice{
		{PlanID: plan.ID, Currency: billing.CurrencyUSD, PriceMonthly: 19.99, PriceYearly: 199.90},
		{PlanID: plan.ID, Currency: billing.CurrencyCNY, PriceMonthly: 139, PriceYearly: 1390},
	}
	for _, price := range prices {
		if err := db.Create(&price).Error; err != nil {
			t.Fatalf("failed to seed pro plan price: %v", err)
		}
	}

	return plan
}

func seedEnterprisePlan(t *testing.T, db *gorm.DB) *billing.SubscriptionPlan {
	plan := &billing.SubscriptionPlan{
		Name:                "enterprise",
		DisplayName:         "Enterprise Plan",
		PricePerSeatMonthly: 99.99,
		PricePerSeatYearly:  999.90,
		IncludedPodMinutes:  -1, // unlimited
		PricePerExtraMinute: 0,
		MaxUsers:            -1, // unlimited
		MaxRunners:          -1,
		MaxConcurrentPods:   -1,
		MaxRepositories:     -1,
		IsActive:            true,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed enterprise plan: %v", err)
	}

	// Seed plan prices (Single Source of Truth)
	prices := []billing.PlanPrice{
		{PlanID: plan.ID, Currency: billing.CurrencyUSD, PriceMonthly: 99.99, PriceYearly: 999.90},
		{PlanID: plan.ID, Currency: billing.CurrencyCNY, PriceMonthly: 690, PriceYearly: 6900},
	}
	for _, price := range prices {
		if err := db.Create(&price).Error; err != nil {
			t.Fatalf("failed to seed enterprise plan price: %v", err)
		}
	}

	return plan
}

func createTestGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	return c, w
}
