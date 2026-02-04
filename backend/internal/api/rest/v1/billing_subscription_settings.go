package v1

import (
	"fmt"
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
	"github.com/gin-gonic/gin"
)

// ===========================================
// Subscription Settings (Portal, Auto-Renew, Customer)
// ===========================================

// CreateStripeCustomerRequest represents the Stripe customer creation request
type CreateStripeCustomerRequest struct {
	Email string `json:"email" binding:"required,email"`
	Name  string `json:"name" binding:"required"`
}

// CreateStripeCustomer creates a Stripe customer for the organization
func (h *BillingHandler) CreateStripeCustomer(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	// Only owners can create Stripe customers
	if tenant.UserRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req CreateStripeCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customerID, err := h.billingService.CreateStripeCustomer(c.Request.Context(), tenant.OrganizationID, req.Email, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"customer_id": customerID})
}

// CustomerPortalRequest represents a customer portal request
type CustomerPortalRequest struct {
	ReturnURL string `json:"return_url" binding:"required"`
}

// GetCustomerPortal returns a customer portal URL (Stripe or LemonSqueezy)
func (h *BillingHandler) GetCustomerPortal(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	if tenant.UserRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req CustomerPortalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.billingService.GetSubscription(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
		return
	}

	factory := h.billingService.GetPaymentFactory()
	if factory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment service not configured"})
		return
	}

	// Determine which provider to use based on subscription IDs
	var provider payment.Provider
	var customerID string
	var subscriptionID string

	if sub.LemonSqueezyCustomerID != nil {
		provider, err = factory.GetProvider(billing.PaymentProviderLemonSqueezy)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		customerID = *sub.LemonSqueezyCustomerID
		if sub.LemonSqueezySubscriptionID != nil {
			subscriptionID = *sub.LemonSqueezySubscriptionID
		}
	} else if sub.StripeCustomerID != nil {
		provider, err = factory.GetProvider(billing.PaymentProviderStripe)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		customerID = *sub.StripeCustomerID
		if sub.StripeSubscriptionID != nil {
			subscriptionID = *sub.StripeSubscriptionID
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no payment provider associated with this subscription"})
		return
	}

	// Cast to SubscriptionProvider to access GetCustomerPortalURL
	subProvider, ok := provider.(payment.SubscriptionProvider)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "provider does not support customer portal"})
		return
	}

	portalReq := &payment.CustomerPortalRequest{
		CustomerID:     customerID,
		SubscriptionID: subscriptionID,
		ReturnURL:      req.ReturnURL,
	}

	resp, err := subProvider.GetCustomerPortalURL(c.Request.Context(), portalReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create portal session: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": resp.URL})
}

// UpdateAutoRenewRequest represents an auto-renew update request
type UpdateAutoRenewRequest struct {
	AutoRenew bool `json:"auto_renew"`
}

// UpdateAutoRenew updates the auto-renew setting for a subscription
func (h *BillingHandler) UpdateAutoRenew(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	if tenant.UserRole != "owner" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req UpdateAutoRenewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current subscription to verify it exists
	sub, err := h.billingService.GetSubscription(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
		return
	}

	// Update auto_renew setting
	if err := h.billingService.SetAutoRenew(c.Request.Context(), tenant.OrganizationID, req.AutoRenew); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated subscription
	sub.AutoRenew = req.AutoRenew
	c.JSON(http.StatusOK, gin.H{
		"subscription": sub,
		"auto_renew":   req.AutoRenew,
	})
}
