package v1

import (
	"fmt"
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	billingdomain "github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	billingService "github.com/anthropics/agentsmesh/backend/internal/service/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateCheckoutRequest represents a checkout request
type CreateCheckoutRequest struct {
	OrderType    string `json:"order_type" binding:"required,oneof=subscription seat_purchase plan_upgrade renewal"`
	PlanName     string `json:"plan_name"`              // Required for subscription/plan_upgrade
	BillingCycle string `json:"billing_cycle"`          // monthly or yearly
	Seats        int    `json:"seats"`                  // Required for seat_purchase
	Provider     string `json:"provider"`               // stripe, alipay, wechat (auto-selected if not provided)
	SuccessURL   string `json:"success_url" binding:"required"`
	CancelURL    string `json:"cancel_url" binding:"required"`
}

// CreateCheckout creates a payment checkout session
func (h *BillingHandler) CreateCheckout(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	// Only owners can create checkouts
	if tenant.UserRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req CreateCheckoutRequest
	// Check if request was passed from another handler (e.g., PurchaseSeats)
	if passedReq, exists := c.Get("checkout_request"); exists {
		req = passedReq.(CreateCheckoutRequest)
	} else if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request based on order type
	if (req.OrderType == billing.OrderTypeSubscription || req.OrderType == billing.OrderTypePlanUpgrade) && req.PlanName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_name is required for subscription/plan_upgrade"})
		return
	}
	if req.OrderType == billing.OrderTypeSeatPurchase && req.Seats <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "seats must be positive for seat_purchase"})
		return
	}

	// Default billing cycle
	if req.BillingCycle == "" {
		req.BillingCycle = billing.BillingCycleMonthly
	}

	// Get payment factory
	factory := h.billingService.GetPaymentFactory()
	if factory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment service not configured"})
		return
	}

	// Determine provider
	providerName := req.Provider
	if providerName == "" {
		// Use default provider based on deployment type
		switch factory.GetDeploymentType() {
		case "cn":
			providerName = billing.PaymentProviderAlipay
		case "onpremise":
			c.JSON(http.StatusBadRequest, gin.H{"error": "payments not supported for on-premise deployment"})
			return
		default:
			providerName = billing.PaymentProviderStripe
		}
	}

	// Get provider
	provider, err := factory.GetProvider(providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate amount based on order type using pricing service
	ctx := c.Request.Context()
	var priceCalc *billingService.PriceCalculation
	var calcErr error

	switch req.OrderType {
	case billing.OrderTypeSubscription:
		priceCalc, calcErr = h.billingService.CalculateSubscriptionPrice(ctx, req.PlanName, req.BillingCycle, 1)
		if calcErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan or billing cycle"})
			return
		}

	case billing.OrderTypePlanUpgrade:
		priceCalc, calcErr = h.billingService.CalculateUpgradePrice(ctx, tenant.OrganizationID, req.PlanName)
		if calcErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("upgrade calculation failed: %v", calcErr)})
			return
		}

	case billing.OrderTypeSeatPurchase:
		priceCalc, calcErr = h.billingService.CalculateSeatPurchasePrice(ctx, tenant.OrganizationID, req.Seats)
		if calcErr != nil {
			errMsg := "seat purchase calculation failed"
			if calcErr == billingService.ErrInvalidPlan {
				errMsg = "cannot add seats to based plan, please upgrade first"
			} else if calcErr == billingService.ErrQuotaExceeded {
				errMsg = "exceeds maximum seats for this plan"
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

	case billing.OrderTypeRenewal:
		priceCalc, calcErr = h.billingService.CalculateRenewalPrice(ctx, tenant.OrganizationID, req.BillingCycle)
		if calcErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no subscription to renew"})
			return
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order type"})
		return
	}

	// Extract values from price calculation
	amount := priceCalc.Amount
	actualAmount := priceCalc.ActualAmount
	seats := priceCalc.Seats
	var planID *int64
	if priceCalc.PlanID > 0 {
		planID = &priceCalc.PlanID
	}

	// Generate order number
	orderNo := fmt.Sprintf("ORD-%d-%s", tenant.OrganizationID, uuid.New().String()[:8])

	// Note: User email is optional for checkout, Stripe will collect it during payment if needed
	var userEmail string

	// Create checkout session
	checkoutReq := &payment.CheckoutRequest{
		OrganizationID: tenant.OrganizationID,
		UserID:         tenant.UserID,
		UserEmail:      userEmail,
		OrderType:      req.OrderType,
		PlanID:         0,
		BillingCycle:   req.BillingCycle,
		Seats:          seats,
		Currency:       "usd",
		Amount:         amount,
		ActualAmount:   actualAmount,
		SuccessURL:     req.SuccessURL,
		CancelURL:      req.CancelURL,
		IdempotencyKey: orderNo,
		Metadata: map[string]string{
			"order_no": orderNo,
		},
	}
	if planID != nil {
		checkoutReq.PlanID = *planID
	}

	resp, err := provider.CreateCheckoutSession(c.Request.Context(), checkoutReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create checkout: %v", err)})
		return
	}

	// Store order in database
	order := &billingdomain.PaymentOrder{
		OrganizationID:  tenant.OrganizationID,
		OrderNo:         orderNo,
		ExternalOrderNo: &resp.ExternalOrderNo,
		OrderType:       req.OrderType,
		PlanID:          planID,
		BillingCycle:    req.BillingCycle,
		Seats:           seats,
		Amount:          amount,
		ActualAmount:    actualAmount,
		Currency:        "usd",
		Status:          billingdomain.OrderStatusPending,
		PaymentProvider: providerName,
		ExpiresAt:       &resp.ExpiresAt,
		CreatedByID:     tenant.UserID,
	}
	if err := h.billingService.CreatePaymentOrder(c.Request.Context(), order); err != nil {
		// Log error but don't fail - checkout session is already created
		fmt.Printf("Warning: failed to save order: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no":    orderNo,
		"session_id":  resp.SessionID,
		"session_url": resp.SessionURL,
		"qr_code_url": resp.QRCodeURL,
		"expires_at":  resp.ExpiresAt,
	})
}

// GetCheckoutStatus returns the status of a checkout/order
func (h *BillingHandler) GetCheckoutStatus(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)
	orderNo := c.Param("order_no")

	order, err := h.billingService.GetPaymentOrderByNo(c.Request.Context(), orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// Verify ownership
	if order.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no":   order.OrderNo,
		"status":     order.Status,
		"order_type": order.OrderType,
		"amount":     order.ActualAmount,
		"currency":   order.Currency,
		"created_at": order.CreatedAt,
		"paid_at":    order.PaidAt,
	})
}
