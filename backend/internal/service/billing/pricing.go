package billing

import (
	"context"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// PriceCalculation represents the result of a price calculation
type PriceCalculation struct {
	Amount       float64 `json:"amount"`        // Full price before proration
	ActualAmount float64 `json:"actual_amount"` // Prorated/discounted price to charge
	Currency     string  `json:"currency"`
	PlanID       int64   `json:"plan_id,omitempty"`
	Seats        int     `json:"seats"`
	BillingCycle string  `json:"billing_cycle"`
	Description  string  `json:"description,omitempty"`
}

// CalculateSubscriptionPriceWithCurrency calculates the price for a subscription in specific currency
func (s *Service) CalculateSubscriptionPriceWithCurrency(ctx context.Context, planName string, currency string, billingCycle string, seats int) (*PriceCalculation, error) {
	price, err := s.GetPlanPrice(ctx, planName, currency)
	if err != nil {
		return nil, err
	}

	if seats <= 0 {
		seats = 1
	}

	var amount float64
	if billingCycle == billing.BillingCycleYearly {
		amount = price.PriceYearly * float64(seats)
	} else {
		amount = price.PriceMonthly * float64(seats)
		billingCycle = billing.BillingCycleMonthly
	}

	return &PriceCalculation{
		Amount:       amount,
		ActualAmount: amount,
		Currency:     currency,
		PlanID:       price.PlanID,
		Seats:        seats,
		BillingCycle: billingCycle,
		Description:  price.Plan.DisplayName + " subscription",
	}, nil
}

// CalculateSubscriptionPrice calculates the price for a new subscription
// Uses USD by default. For multi-currency support, use CalculateSubscriptionPriceWithCurrency.
func (s *Service) CalculateSubscriptionPrice(ctx context.Context, planName string, billingCycle string, seats int) (*PriceCalculation, error) {
	// Use USD as default currency for backward compatibility
	return s.CalculateSubscriptionPriceWithCurrency(ctx, planName, billing.CurrencyUSD, billingCycle, seats)
}

// CalculateUpgradePrice calculates the prorated price for upgrading to a new plan
func (s *Service) CalculateUpgradePrice(ctx context.Context, orgID int64, newPlanName string) (*PriceCalculation, error) {
	// Get current subscription
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Get new plan
	newPlan, err := s.GetPlan(ctx, newPlanName)
	if err != nil {
		return nil, err
	}

	// Get current plan
	currentPlan, err := s.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	seats := sub.SeatCount
	if seats <= 0 {
		seats = 1
	}

	// Calculate full prices
	var currentPrice, newPrice float64
	if sub.BillingCycle == billing.BillingCycleYearly {
		currentPrice = currentPlan.PricePerSeatYearly * float64(seats)
		newPrice = newPlan.PricePerSeatYearly * float64(seats)
	} else {
		currentPrice = currentPlan.PricePerSeatMonthly * float64(seats)
		newPrice = newPlan.PricePerSeatMonthly * float64(seats)
	}

	// Calculate remaining period ratio
	ratio := calculateRemainingPeriodRatio(sub.CurrentPeriodStart, sub.CurrentPeriodEnd)

	// Calculate prorated difference
	actualAmount := (newPrice - currentPrice) * ratio
	if actualAmount < 0 {
		actualAmount = 0 // Downgrade should use different flow
	}

	return &PriceCalculation{
		Amount:       newPrice,
		ActualAmount: actualAmount,
		Currency:     billing.CurrencyUSD,
		PlanID:       newPlan.ID,
		Seats:        seats,
		BillingCycle: sub.BillingCycle,
		Description:  "Upgrade to " + newPlan.DisplayName,
	}, nil
}

// CalculateSeatPurchasePrice calculates the prorated price for purchasing additional seats
func (s *Service) CalculateSeatPurchasePrice(ctx context.Context, orgID int64, additionalSeats int) (*PriceCalculation, error) {
	if additionalSeats <= 0 {
		return nil, ErrInvalidPlan
	}

	// Get current subscription
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Get plan
	plan, err := s.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	// Check if plan allows adding seats (Based plan has fixed 1 seat)
	if plan.Name == billing.PlanBased {
		return nil, ErrInvalidPlan
	}

	// Check max seats
	if plan.MaxUsers != -1 && sub.SeatCount+additionalSeats > plan.MaxUsers {
		return nil, ErrQuotaExceeded
	}

	// Get price per seat based on billing cycle
	var pricePerSeat float64
	if sub.BillingCycle == billing.BillingCycleYearly {
		pricePerSeat = plan.PricePerSeatYearly
	} else {
		pricePerSeat = plan.PricePerSeatMonthly
	}

	// Calculate full amount
	amount := pricePerSeat * float64(additionalSeats)

	// Calculate prorated amount for remaining period
	ratio := calculateRemainingPeriodRatio(sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	actualAmount := amount * ratio

	return &PriceCalculation{
		Amount:       amount,
		ActualAmount: actualAmount,
		Currency:     billing.CurrencyUSD,
		PlanID:       sub.PlanID,
		Seats:        additionalSeats,
		BillingCycle: sub.BillingCycle,
		Description:  "Additional seats",
	}, nil
}

// CalculateRenewalPrice calculates the price for renewing a subscription
func (s *Service) CalculateRenewalPrice(ctx context.Context, orgID int64, newBillingCycle string) (*PriceCalculation, error) {
	// Get current subscription
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// Get plan
	plan, err := s.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	seats := sub.SeatCount
	if seats <= 0 {
		seats = 1
	}

	// Use new billing cycle if provided, otherwise keep current
	billingCycle := sub.BillingCycle
	if newBillingCycle != "" {
		billingCycle = newBillingCycle
	}

	var amount float64
	if billingCycle == billing.BillingCycleYearly {
		amount = plan.PricePerSeatYearly * float64(seats)
	} else {
		amount = plan.PricePerSeatMonthly * float64(seats)
	}

	return &PriceCalculation{
		Amount:       amount,
		ActualAmount: amount,
		Currency:     billing.CurrencyUSD,
		PlanID:       plan.ID,
		Seats:        seats,
		BillingCycle: billingCycle,
		Description:  "Renewal - " + plan.DisplayName,
	}, nil
}

// CalculateBillingCycleChangePrice calculates the price difference when changing billing cycle
func (s *Service) CalculateBillingCycleChangePrice(ctx context.Context, orgID int64, newBillingCycle string) (*PriceCalculation, error) {
	// Get current subscription
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// If same cycle, no change needed
	if sub.BillingCycle == newBillingCycle {
		return nil, nil
	}

	// Get plan
	plan, err := s.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	seats := sub.SeatCount
	if seats <= 0 {
		seats = 1
	}

	// Calculate prices for both cycles
	var currentPrice, newPrice float64
	if sub.BillingCycle == billing.BillingCycleYearly {
		currentPrice = plan.PricePerSeatYearly * float64(seats)
		newPrice = plan.PricePerSeatMonthly * float64(seats)
	} else {
		currentPrice = plan.PricePerSeatMonthly * float64(seats)
		newPrice = plan.PricePerSeatYearly * float64(seats)
	}

	// Calculate remaining period ratio
	ratio := calculateRemainingPeriodRatio(sub.CurrentPeriodStart, sub.CurrentPeriodEnd)

	// Calculate prorated difference
	// Positive = need to pay more (monthly to yearly)
	// Negative = credit (yearly to monthly, handled at renewal)
	actualAmount := (newPrice - currentPrice) * ratio
	if actualAmount < 0 {
		actualAmount = 0 // Credit will be applied at renewal
	}

	return &PriceCalculation{
		Amount:       newPrice,
		ActualAmount: actualAmount,
		Currency:     billing.CurrencyUSD,
		PlanID:       plan.ID,
		Seats:        seats,
		BillingCycle: newBillingCycle,
		Description:  "Billing cycle change",
	}, nil
}

// GetPricePreview returns a price preview without creating an order
func (s *Service) GetPricePreview(ctx context.Context, orgID int64, orderType string, planName string, billingCycle string, seats int) (*PriceCalculation, error) {
	switch orderType {
	case billing.OrderTypeSubscription:
		return s.CalculateSubscriptionPrice(ctx, planName, billingCycle, seats)

	case billing.OrderTypePlanUpgrade:
		return s.CalculateUpgradePrice(ctx, orgID, planName)

	case billing.OrderTypeSeatPurchase:
		return s.CalculateSeatPurchasePrice(ctx, orgID, seats)

	case billing.OrderTypeRenewal:
		return s.CalculateRenewalPrice(ctx, orgID, billingCycle)

	default:
		return nil, ErrInvalidOrderStatus
	}
}

// calculateRemainingPeriodRatio calculates the ratio of remaining time in the billing period
func calculateRemainingPeriodRatio(periodStart, periodEnd time.Time) float64 {
	now := time.Now()
	totalPeriod := periodEnd.Sub(periodStart).Hours()
	remainingPeriod := periodEnd.Sub(now).Hours()

	if remainingPeriod < 0 {
		remainingPeriod = 0
	}
	if totalPeriod <= 0 {
		return 0
	}

	return remainingPeriod / totalPeriod
}
