package billing

import (
	"context"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// GetSubscription returns subscription for an organization
func (s *Service) GetSubscription(ctx context.Context, orgID int64) (*billing.Subscription, error) {
	var sub billing.Subscription
	if err := s.db.WithContext(ctx).Preload("Plan").Where("organization_id = ?", orgID).First(&sub).Error; err != nil {
		return nil, ErrSubscriptionNotFound
	}
	return &sub, nil
}

// CreateSubscription creates a new subscription
func (s *Service) CreateSubscription(ctx context.Context, orgID int64, planName string) (*billing.Subscription, error) {
	plan, err := s.GetPlan(ctx, planName)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	periodEnd := now.AddDate(0, 1, 0) // 1 month

	sub := &billing.Subscription{
		OrganizationID:     orgID,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusActive,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
	}

	if err := s.db.WithContext(ctx).Create(sub).Error; err != nil {
		return nil, err
	}

	sub.Plan = plan
	return sub, nil
}

// CreateTrialSubscription creates a trial subscription for a new organization
func (s *Service) CreateTrialSubscription(ctx context.Context, orgID int64, planName string, trialDays int) (*billing.Subscription, error) {
	plan, err := s.GetPlan(ctx, planName)
	if err != nil {
		return nil, err
	}

	if trialDays <= 0 {
		trialDays = billing.DefaultTrialDays
	}

	now := time.Now()
	periodEnd := now.AddDate(0, 0, trialDays)

	sub := &billing.Subscription{
		OrganizationID:     orgID,
		PlanID:             plan.ID,
		Status:             billing.SubscriptionStatusTrialing,
		BillingCycle:       billing.BillingCycleMonthly,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
		SeatCount:          1,
	}

	if err := s.db.WithContext(ctx).Create(sub).Error; err != nil {
		return nil, err
	}

	sub.Plan = plan
	return sub, nil
}

// UpdateSubscription updates subscription plan (handles upgrade/downgrade)
func (s *Service) UpdateSubscription(ctx context.Context, orgID int64, planName string) (*billing.Subscription, error) {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	newPlan, err := s.GetPlan(ctx, planName)
	if err != nil {
		return nil, err
	}

	// Get current plan for comparison
	currentPlan, err := s.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	// Determine if this is an upgrade or downgrade based on price
	isDowngrade := newPlan.PricePerSeatMonthly < currentPlan.PricePerSeatMonthly

	if isDowngrade {
		// Downgrade: schedule for end of billing period
		// Check if current seat count exceeds new plan's max_users
		if newPlan.MaxUsers > 0 && sub.SeatCount > newPlan.MaxUsers {
			return nil, ErrSeatCountExceedsLimit
		}

		// Set downgrade to take effect at period end
		sub.DowngradeToPlan = &planName
		if err := s.db.WithContext(ctx).Save(sub).Error; err != nil {
			return nil, err
		}

		// Return current subscription with downgrade scheduled
		sub.Plan = currentPlan
		return sub, nil
	}

	// Upgrade: immediate effect (payment should be handled separately via checkout)
	// For free plan or if no payment required, apply immediately
	if currentPlan.PricePerSeatMonthly == 0 || newPlan.PricePerSeatMonthly == 0 {
		sub.PlanID = newPlan.ID
		sub.DowngradeToPlan = nil // Clear any pending downgrade

		// Update Stripe subscription if enabled
		if s.stripeEnabled && sub.StripeSubscriptionID != nil {
			// In a real implementation, update Stripe subscription here
		}

		if err := s.db.WithContext(ctx).Save(sub).Error; err != nil {
			return nil, err
		}

		sub.Plan = newPlan
		return sub, nil
	}

	// For paid upgrades, just return the subscription - payment flow handles the actual upgrade
	sub.Plan = currentPlan
	return sub, nil
}
