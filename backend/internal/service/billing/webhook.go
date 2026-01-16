package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
)

// ===========================================
// Webhook Event Handlers (implements BillingServiceInterface)
// ===========================================

// HandlePaymentSucceeded handles a successful payment webhook event
func (s *Service) HandlePaymentSucceeded(c *gin.Context, event *payment.WebhookEvent) error {
	ctx := c.Request.Context()

	// Try to find order by order_no first, then by external_order_no
	var order *billing.PaymentOrder
	var err error

	if event.OrderNo != "" {
		order, err = s.GetPaymentOrderByNo(ctx, event.OrderNo)
	}
	if order == nil && event.ExternalOrderNo != "" {
		order, err = s.GetPaymentOrderByExternalNo(ctx, event.ExternalOrderNo)
	}

	// For recurring payments (invoice.paid), there may not be an order in our system
	if order == nil && event.SubscriptionID != "" {
		// This is likely a recurring payment, find subscription by Stripe subscription ID
		return s.handleRecurringPaymentSuccess(ctx, event)
	}

	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	// Update order status
	if err := s.UpdatePaymentOrderStatus(ctx, order.OrderNo, billing.OrderStatusSucceeded, nil); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Create transaction record
	tx := &billing.PaymentTransaction{
		PaymentOrderID:        order.ID,
		TransactionType:       billing.TransactionTypePayment,
		ExternalTransactionID: &event.ExternalOrderNo,
		Amount:                event.Amount,
		Currency:              event.Currency,
		Status:                billing.TransactionStatusSucceeded,
		WebhookEventID:        &event.EventID,
		WebhookEventType:      &event.EventType,
		RawPayload:            billing.RawPayload(event.RawPayload),
	}
	if err := s.CreatePaymentTransaction(ctx, tx); err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Process based on order type
	switch order.OrderType {
	case billing.OrderTypeSubscription:
		return s.activateSubscription(ctx, order, event)
	case billing.OrderTypeSeatPurchase:
		return s.addSeats(ctx, order)
	case billing.OrderTypePlanUpgrade:
		return s.upgradePlan(ctx, order)
	case billing.OrderTypeRenewal:
		return s.renewSubscriptionFromOrder(ctx, order)
	}

	return nil
}

// HandlePaymentFailed handles a failed payment webhook event
func (s *Service) HandlePaymentFailed(c *gin.Context, event *payment.WebhookEvent) error {
	ctx := c.Request.Context()

	// For recurring payment failures, freeze the subscription
	if event.SubscriptionID != "" {
		return s.handleRecurringPaymentFailure(ctx, event)
	}

	// Try to find and update the order
	var order *billing.PaymentOrder
	var err error

	if event.OrderNo != "" {
		order, err = s.GetPaymentOrderByNo(ctx, event.OrderNo)
	}
	if order == nil && event.ExternalOrderNo != "" {
		order, err = s.GetPaymentOrderByExternalNo(ctx, event.ExternalOrderNo)
	}

	if err != nil || order == nil {
		return nil // Order not found, nothing to update
	}

	// Update order status
	return s.UpdatePaymentOrderStatus(ctx, order.OrderNo, billing.OrderStatusFailed, &event.FailedReason)
}

// HandleSubscriptionCanceled handles subscription cancellation webhook event
func (s *Service) HandleSubscriptionCanceled(c *gin.Context, event *payment.WebhookEvent) error {
	ctx := c.Request.Context()

	if event.SubscriptionID == "" {
		return nil
	}

	// Find subscription by Stripe subscription ID
	var sub billing.Subscription
	if err := s.db.WithContext(ctx).Where("stripe_subscription_id = ?", event.SubscriptionID).First(&sub).Error; err != nil {
		return nil // Subscription not found
	}

	// Update subscription status
	now := time.Now()
	sub.Status = billing.SubscriptionStatusCanceled
	sub.CanceledAt = &now

	return s.db.WithContext(ctx).Save(&sub).Error
}

// HandleSubscriptionUpdated handles subscription update webhook event
func (s *Service) HandleSubscriptionUpdated(c *gin.Context, event *payment.WebhookEvent) error {
	ctx := c.Request.Context()

	if event.SubscriptionID == "" {
		return nil
	}

	// Find subscription by Stripe subscription ID
	var sub billing.Subscription
	if err := s.db.WithContext(ctx).Where("stripe_subscription_id = ?", event.SubscriptionID).First(&sub).Error; err != nil {
		return nil // Subscription not found
	}

	// Map Stripe status to our status
	switch event.Status {
	case "active":
		sub.Status = billing.SubscriptionStatusActive
		sub.FrozenAt = nil
	case "past_due":
		sub.Status = billing.SubscriptionStatusPastDue
	case "canceled":
		sub.Status = billing.SubscriptionStatusCanceled
	case "trialing":
		sub.Status = billing.SubscriptionStatusTrialing
	}

	return s.db.WithContext(ctx).Save(&sub).Error
}

// ===========================================
// Internal Helper Methods
// ===========================================

// handleRecurringPaymentSuccess handles successful recurring payments
func (s *Service) handleRecurringPaymentSuccess(ctx context.Context, event *payment.WebhookEvent) error {
	// Find subscription by Stripe subscription ID
	var sub billing.Subscription
	if err := s.db.WithContext(ctx).Where("stripe_subscription_id = ?", event.SubscriptionID).First(&sub).Error; err != nil {
		return nil // Subscription not found, ignore
	}

	// Renew the subscription period
	sub.CurrentPeriodStart = sub.CurrentPeriodEnd
	if sub.BillingCycle == billing.BillingCycleYearly {
		sub.CurrentPeriodEnd = sub.CurrentPeriodStart.AddDate(1, 0, 0)
	} else {
		sub.CurrentPeriodEnd = sub.CurrentPeriodStart.AddDate(0, 1, 0)
	}

	// Unfreeze if was frozen
	sub.Status = billing.SubscriptionStatusActive
	sub.FrozenAt = nil

	// Apply pending changes if any
	if sub.DowngradeToPlan != nil {
		plan, err := s.GetPlan(ctx, *sub.DowngradeToPlan)
		if err == nil {
			sub.PlanID = plan.ID
		}
		sub.DowngradeToPlan = nil
	}
	if sub.NextBillingCycle != nil {
		sub.BillingCycle = *sub.NextBillingCycle
		sub.NextBillingCycle = nil
	}

	return s.db.WithContext(ctx).Save(&sub).Error
}

// handleRecurringPaymentFailure handles failed recurring payments
func (s *Service) handleRecurringPaymentFailure(ctx context.Context, event *payment.WebhookEvent) error {
	// Find subscription by Stripe subscription ID
	var sub billing.Subscription
	if err := s.db.WithContext(ctx).Where("stripe_subscription_id = ?", event.SubscriptionID).First(&sub).Error; err != nil {
		return nil // Subscription not found, ignore
	}

	// Freeze the subscription
	now := time.Now()
	sub.Status = billing.SubscriptionStatusFrozen
	sub.FrozenAt = &now

	return s.db.WithContext(ctx).Save(&sub).Error
}

// activateSubscription activates a new subscription after payment
func (s *Service) activateSubscription(ctx context.Context, order *billing.PaymentOrder, event *payment.WebhookEvent) error {
	sub, err := s.GetSubscription(ctx, order.OrganizationID)
	if err != nil {
		// Create new subscription
		now := time.Now()
		var periodEnd time.Time
		if order.BillingCycle == billing.BillingCycleYearly {
			periodEnd = now.AddDate(1, 0, 0)
		} else {
			periodEnd = now.AddDate(0, 1, 0)
		}

		provider := order.PaymentProvider
		sub = &billing.Subscription{
			OrganizationID:     order.OrganizationID,
			PlanID:             *order.PlanID,
			Status:             billing.SubscriptionStatusActive,
			BillingCycle:       order.BillingCycle,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   periodEnd,
			PaymentProvider:    &provider,
			PaymentMethod:      order.PaymentMethod,
			AutoRenew:          true,
			SeatCount:          order.Seats,
		}

		if event.CustomerID != "" {
			sub.StripeCustomerID = &event.CustomerID
		}
		if event.SubscriptionID != "" {
			sub.StripeSubscriptionID = &event.SubscriptionID
		}

		return s.db.WithContext(ctx).Create(sub).Error
	}

	// Update existing subscription
	sub.PlanID = *order.PlanID
	sub.Status = billing.SubscriptionStatusActive
	sub.BillingCycle = order.BillingCycle
	sub.SeatCount = order.Seats
	sub.FrozenAt = nil
	provider := order.PaymentProvider
	sub.PaymentProvider = &provider
	sub.PaymentMethod = order.PaymentMethod

	if event.CustomerID != "" {
		sub.StripeCustomerID = &event.CustomerID
	}
	if event.SubscriptionID != "" {
		sub.StripeSubscriptionID = &event.SubscriptionID
	}

	return s.db.WithContext(ctx).Save(sub).Error
}

// addSeats adds seats to a subscription after payment
func (s *Service) addSeats(ctx context.Context, order *billing.PaymentOrder) error {
	return s.db.WithContext(ctx).Model(&billing.Subscription{}).
		Where("organization_id = ?", order.OrganizationID).
		Update("seat_count", gorm.Expr("seat_count + ?", order.Seats)).Error
}

// upgradePlan upgrades a subscription to a new plan
func (s *Service) upgradePlan(ctx context.Context, order *billing.PaymentOrder) error {
	if order.PlanID == nil {
		return ErrInvalidPlan
	}
	return s.db.WithContext(ctx).Model(&billing.Subscription{}).
		Where("organization_id = ?", order.OrganizationID).
		Updates(map[string]interface{}{
			"plan_id":    *order.PlanID,
			"updated_at": time.Now(),
		}).Error
}

// renewSubscriptionFromOrder renews a subscription from an order
func (s *Service) renewSubscriptionFromOrder(ctx context.Context, order *billing.PaymentOrder) error {
	sub, err := s.GetSubscription(ctx, order.OrganizationID)
	if err != nil {
		return err
	}

	// Set new period
	sub.CurrentPeriodStart = sub.CurrentPeriodEnd
	if sub.BillingCycle == billing.BillingCycleYearly {
		sub.CurrentPeriodEnd = sub.CurrentPeriodStart.AddDate(1, 0, 0)
	} else {
		sub.CurrentPeriodEnd = sub.CurrentPeriodStart.AddDate(0, 1, 0)
	}
	sub.Status = billing.SubscriptionStatusActive
	sub.FrozenAt = nil

	return s.db.WithContext(ctx).Save(sub).Error
}
