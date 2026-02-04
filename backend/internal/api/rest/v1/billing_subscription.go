package v1

import (
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	billingsvc "github.com/anthropics/agentsmesh/backend/internal/service/billing"
	"github.com/gin-gonic/gin"
)

// ===========================================
// Basic Subscription Operations (CRUD)
// ===========================================

// CreateSubscriptionRequest represents the subscription creation request
type CreateSubscriptionRequest struct {
	PlanName     string `json:"plan_name" binding:"required"`
	BillingCycle string `json:"billing_cycle"` // monthly or yearly
}

// GetSubscription returns the current subscription
func (h *BillingHandler) GetSubscription(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	sub, err := h.billingService.GetSubscription(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		if err == billingsvc.ErrSubscriptionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}

// CreateSubscription creates a new subscription
func (h *BillingHandler) CreateSubscription(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.billingService.CreateSubscription(c.Request.Context(), tenant.OrganizationID, req.PlanName)
	if err != nil {
		if err == billingsvc.ErrPlanNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"subscription": sub})
}

// UpdateSubscriptionRequest represents the subscription update request
type UpdateSubscriptionRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
}

// UpdateSubscription updates the subscription plan
func (h *BillingHandler) UpdateSubscription(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	var req UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.billingService.UpdateSubscription(c.Request.Context(), tenant.OrganizationID, req.PlanName)
	if err != nil {
		if err == billingsvc.ErrSubscriptionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
			return
		}
		if err == billingsvc.ErrPlanNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
			return
		}
		if err == billingsvc.ErrSeatCountExceedsLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current seat count exceeds target plan limit, please reduce seats first"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Check if downgrade is scheduled
	response := gin.H{"subscription": sub}
	if sub.DowngradeToPlan != nil {
		response["message"] = "downgrade scheduled for end of billing period"
		response["downgrade_to"] = *sub.DowngradeToPlan
	}

	c.JSON(http.StatusOK, response)
}

// CancelSubscription cancels the current subscription
func (h *BillingHandler) CancelSubscription(c *gin.Context) {
	tenant := c.MustGet("tenant").(*middleware.TenantContext)

	if err := h.billingService.CancelSubscription(c.Request.Context(), tenant.OrganizationID); err != nil {
		if err == billingsvc.ErrSubscriptionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "subscription cancelled"})
}
