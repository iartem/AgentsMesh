package billing

import (
	"context"
	"errors"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"gorm.io/gorm"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

var (
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrPlanNotFound          = errors.New("plan not found")
	ErrPriceNotFound         = errors.New("price not found for currency")
	ErrQuotaExceeded         = errors.New("quota exceeded")
	ErrInvalidPlan           = errors.New("invalid plan")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderExpired          = errors.New("order expired")
	ErrInvalidOrderStatus    = errors.New("invalid order status")
	ErrSeatCountExceedsLimit = errors.New("current seat count exceeds target plan limit")
	ErrSubscriptionFrozen    = errors.New("subscription is frozen, please renew to continue")
)

// Service handles billing operations
type Service struct {
	db             *gorm.DB
	stripeEnabled  bool
	paymentFactory *payment.Factory
	paymentConfig  *config.PaymentConfig
}

// NewService creates a new billing service without payment configuration.
// This is primarily for testing purposes where payment providers are not needed.
// For production use, prefer NewServiceWithConfig which supports all payment providers.
func NewService(db *gorm.DB, stripeKey string) *Service {
	if stripeKey != "" {
		stripe.Key = stripeKey
	}
	return &Service{
		db:            db,
		stripeEnabled: stripeKey != "",
	}
}

// NewServiceWithConfig creates a new billing service with full configuration
// appConfig is needed for URL derivation (AlipayNotifyURL, WeChatNotifyURL, etc.)
// If appConfig is nil, returns a service with no payment providers configured.
func NewServiceWithConfig(db *gorm.DB, appConfig *config.Config) *Service {
	svc := &Service{
		db: db,
	}

	// Handle nil config gracefully - return service without payment providers
	if appConfig == nil {
		return svc
	}

	cfg := &appConfig.Payment
	svc.paymentConfig = cfg

	// Use NewFactoryWithDB to support license provider and URL derivation
	svc.paymentFactory = payment.NewFactoryWithDB(appConfig, db)
	svc.stripeEnabled = cfg.StripeEnabled()

	// Set Stripe key if enabled
	if cfg.StripeEnabled() {
		stripe.Key = cfg.Stripe.SecretKey
	}

	return svc
}

// GetPaymentFactory returns the payment factory
func (s *Service) GetPaymentFactory() *payment.Factory {
	return s.paymentFactory
}

// GetPlan returns a plan by name
func (s *Service) GetPlan(ctx context.Context, planName string) (*billing.SubscriptionPlan, error) {
	var plan billing.SubscriptionPlan
	if err := s.db.WithContext(ctx).Where("name = ? AND is_active = ?", planName, true).First(&plan).Error; err != nil {
		return nil, ErrPlanNotFound
	}
	return &plan, nil
}

// ListPlans returns all active plans
func (s *Service) ListPlans(ctx context.Context) ([]*billing.SubscriptionPlan, error) {
	var plans []*billing.SubscriptionPlan
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("price_per_seat_monthly ASC").Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

// GetPlanByID returns a plan by ID
func (s *Service) GetPlanByID(ctx context.Context, planID int64) (*billing.SubscriptionPlan, error) {
	var plan billing.SubscriptionPlan
	if err := s.db.WithContext(ctx).First(&plan, planID).Error; err != nil {
		return nil, ErrPlanNotFound
	}
	return &plan, nil
}

// GetPlanPrice returns price for a plan in specific currency
func (s *Service) GetPlanPrice(ctx context.Context, planName, currency string) (*billing.PlanPrice, error) {
	plan, err := s.GetPlan(ctx, planName)
	if err != nil {
		return nil, err
	}

	var price billing.PlanPrice
	if err := s.db.WithContext(ctx).
		Where("plan_id = ? AND currency = ?", plan.ID, currency).
		First(&price).Error; err != nil {
		return nil, ErrPriceNotFound
	}

	price.Plan = plan
	return &price, nil
}

// GetPlanPriceByID returns price for a plan ID in specific currency
func (s *Service) GetPlanPriceByID(ctx context.Context, planID int64, currency string) (*billing.PlanPrice, error) {
	var price billing.PlanPrice
	if err := s.db.WithContext(ctx).
		Preload("Plan").
		Where("plan_id = ? AND currency = ?", planID, currency).
		First(&price).Error; err != nil {
		return nil, ErrPriceNotFound
	}
	return &price, nil
}

// GetPlanPrices returns all prices for a plan
func (s *Service) GetPlanPrices(ctx context.Context, planName string) ([]billing.PlanPrice, error) {
	plan, err := s.GetPlan(ctx, planName)
	if err != nil {
		return nil, err
	}

	var prices []billing.PlanPrice
	if err := s.db.WithContext(ctx).
		Where("plan_id = ?", plan.ID).
		Find(&prices).Error; err != nil {
		return nil, err
	}

	// Attach plan reference
	for i := range prices {
		prices[i].Plan = plan
	}

	return prices, nil
}

// ListPlansWithPrices returns all active plans with prices for a specific currency
func (s *Service) ListPlansWithPrices(ctx context.Context, currency string) ([]*PlanWithPrice, error) {
	plans, err := s.ListPlans(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*PlanWithPrice, 0, len(plans))
	for _, plan := range plans {
		var price billing.PlanPrice
		if err := s.db.WithContext(ctx).
			Where("plan_id = ? AND currency = ?", plan.ID, currency).
			First(&price).Error; err != nil {
			// Skip plans without price for this currency
			continue
		}

		result = append(result, &PlanWithPrice{
			Plan:  plan,
			Price: &price,
		})
	}

	return result, nil
}

// PlanWithPrice combines a plan with its price in a specific currency
type PlanWithPrice struct {
	Plan  *billing.SubscriptionPlan `json:"plan"`
	Price *billing.PlanPrice        `json:"price"`
}

// CreateStripeCustomer creates a Stripe customer for an organization
func (s *Service) CreateStripeCustomer(ctx context.Context, orgID int64, email, name string) (string, error) {
	if !s.stripeEnabled {
		return "", nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"organization_id": string(rune(orgID)),
		},
	}

	c, err := customer.New(params)
	if err != nil {
		return "", err
	}

	// Update subscription with Stripe customer ID
	s.db.WithContext(ctx).Model(&billing.Subscription{}).
		Where("organization_id = ?", orgID).
		Update("stripe_customer_id", c.ID)

	return c.ID, nil
}

// GetBillingOverview returns billing overview for an organization
func (s *Service) GetBillingOverview(ctx context.Context, orgID int64) (*BillingOverview, error) {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	plan := sub.Plan
	if plan == nil {
		plan, _ = s.GetPlanByID(ctx, sub.PlanID)
	}

	// Get current usage
	podMinutes, _ := s.GetUsage(ctx, orgID, billing.UsageTypePodMinutes)

	// Count resources
	var userCount, runnerCount, repoCount, concurrentPodCount int64
	s.db.WithContext(ctx).Table("organization_members").Where("organization_id = ?", orgID).Count(&userCount)
	s.db.WithContext(ctx).Table("runners").Where("organization_id = ?", orgID).Count(&runnerCount)
	s.db.WithContext(ctx).Table("repositories").Where("organization_id = ?", orgID).Count(&repoCount)
	s.db.WithContext(ctx).Table("pods").
		Where("organization_id = ? AND status IN ?", orgID, []string{"running", "initializing"}).
		Count(&concurrentPodCount)

	return &BillingOverview{
		Plan:               plan,
		Status:             sub.Status,
		BillingCycle:       sub.BillingCycle,
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CancelAtPeriodEnd:  sub.CancelAtPeriodEnd,
		Usage: UsageOverview{
			PodMinutes:         podMinutes,
			IncludedPodMinutes: float64(plan.IncludedPodMinutes),
			Users:              int(userCount),
			MaxUsers:           plan.MaxUsers,
			Runners:            int(runnerCount),
			MaxRunners:         plan.MaxRunners,
			ConcurrentPods:     int(concurrentPodCount),
			MaxConcurrentPods:  plan.MaxConcurrentPods,
			Repositories:       int(repoCount),
			MaxRepositories:    plan.MaxRepositories,
		},
	}, nil
}

// GetDeploymentInfo returns deployment type and available payment providers
func (s *Service) GetDeploymentInfo() *DeploymentInfo {
	if s.paymentConfig == nil {
		return &DeploymentInfo{
			DeploymentType:     "global",
			AvailableProviders: []string{},
		}
	}

	return &DeploymentInfo{
		DeploymentType:     string(s.paymentConfig.DeploymentType),
		AvailableProviders: s.paymentConfig.GetAvailableProviders(),
	}
}

// BillingOverview represents billing overview
type BillingOverview struct {
	Plan               *billing.SubscriptionPlan `json:"plan"`
	Status             string                    `json:"status"`
	BillingCycle       string                    `json:"billing_cycle"`
	CurrentPeriodStart time.Time                 `json:"current_period_start"`
	CurrentPeriodEnd   time.Time                 `json:"current_period_end"`
	CancelAtPeriodEnd  bool                      `json:"cancel_at_period_end"`
	Usage              UsageOverview             `json:"usage"`
}

// UsageOverview represents usage overview
type UsageOverview struct {
	PodMinutes         float64 `json:"pod_minutes"`
	IncludedPodMinutes float64 `json:"included_pod_minutes"`
	Users              int     `json:"users"`
	MaxUsers           int     `json:"max_users"`
	Runners            int     `json:"runners"`
	MaxRunners         int     `json:"max_runners"`
	ConcurrentPods     int     `json:"concurrent_pods"`
	MaxConcurrentPods  int     `json:"max_concurrent_pods"`
	Repositories       int     `json:"repositories"`
	MaxRepositories    int     `json:"max_repositories"`
}

// DeploymentInfo represents deployment information
type DeploymentInfo struct {
	DeploymentType     string   `json:"deployment_type"`
	AvailableProviders []string `json:"available_providers"`
}
