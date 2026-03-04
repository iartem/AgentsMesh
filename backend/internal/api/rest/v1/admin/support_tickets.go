package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/anthropics/agentsmesh/backend/internal/domain/admin"
	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	adminservice "github.com/anthropics/agentsmesh/backend/internal/service/admin"
	"github.com/anthropics/agentsmesh/backend/internal/service/supportticket"
	"github.com/anthropics/agentsmesh/backend/pkg/apierr"
	"github.com/gin-gonic/gin"
)

// SupportTicketHandler handles admin support ticket management
type SupportTicketHandler struct {
	service      *supportticket.Service
	adminService *adminservice.Service
}

// NewSupportTicketHandler creates a new admin support ticket handler
func NewSupportTicketHandler(service *supportticket.Service, adminSvc *adminservice.Service) *SupportTicketHandler {
	return &SupportTicketHandler{
		service:      service,
		adminService: adminSvc,
	}
}

// RegisterRoutes registers admin support ticket routes
func (h *SupportTicketHandler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/support-tickets")
	{
		group.GET("", h.List)
		group.GET("/stats", h.GetStats)
		group.GET("/:id", h.GetByID)
		group.GET("/:id/messages", h.ListMessages)
		group.POST("/:id/reply", h.Reply)
		group.PATCH("/:id/status", h.UpdateStatus)
		group.POST("/:id/assign", h.Assign)
		group.GET("/attachments/:attachmentId/url", h.GetAttachmentURL)
	}
}

// logAction is a helper method that delegates to the shared LogAdminAction function
func (h *SupportTicketHandler) logAction(c *gin.Context, action admin.AuditAction, targetType admin.TargetType, targetID int64, oldData, newData interface{}) {
	LogAdminAction(c, h.adminService, action, targetType, targetID, oldData, newData)
}

// List returns all support tickets with filtering and pagination
// GET /api/v1/admin/support-tickets
func (h *SupportTicketHandler) List(c *gin.Context) {
	query := &supportticket.AdminListQuery{
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		Category: c.Query("category"),
		Priority: c.Query("priority"),
		Page:     1,
		PageSize: 20,
	}

	if page, err := strconv.Atoi(c.Query("page")); err == nil {
		query.Page = page
	}
	if pageSize, err := strconv.Atoi(c.Query("page_size")); err == nil {
		query.PageSize = pageSize
	}

	result, err := h.service.AdminList(c.Request.Context(), query)
	if err != nil {
		apierr.InternalError(c, "Failed to list support tickets")
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetStats returns support ticket statistics
// GET /api/v1/admin/support-tickets/stats
func (h *SupportTicketHandler) GetStats(c *gin.Context) {
	stats, err := h.service.AdminGetStats(c.Request.Context())
	if err != nil {
		apierr.InternalError(c, "Failed to get support ticket stats")
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetByID returns a single support ticket with messages
// GET /api/v1/admin/support-tickets/:id
func (h *SupportTicketHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid ticket ID")
		return
	}

	ticket, err := h.service.AdminGetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, supportticket.ErrTicketNotFound) {
			apierr.ResourceNotFound(c, "Support ticket not found")
			return
		}
		apierr.InternalError(c, "Failed to get support ticket")
		return
	}

	messages, err := h.service.AdminListMessages(c.Request.Context(), id)
	if err != nil {
		slog.Warn("failed to load messages for ticket", "ticket_id", id, "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket":   ticket,
		"messages": messages,
	})
}

// ListMessages returns all messages for a support ticket
// GET /api/v1/admin/support-tickets/:id/messages
func (h *SupportTicketHandler) ListMessages(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid ticket ID")
		return
	}

	messages, err := h.service.AdminListMessages(c.Request.Context(), ticketID)
	if err != nil {
		apierr.InternalError(c, "Failed to list messages")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": messages})
}

// Reply adds an admin reply to a support ticket
// POST /api/v1/admin/support-tickets/:id/reply
func (h *SupportTicketHandler) Reply(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid ticket ID")
		return
	}

	adminUserID := middleware.GetAdminUserID(c)

	content := c.PostForm("content")
	if content == "" {
		apierr.BadRequest(c, apierr.VALIDATION_FAILED, "Content is required")
		return
	}

	msg, err := h.service.AdminAddReply(c.Request.Context(), ticketID, adminUserID, &supportticket.AddMessageRequest{
		Content: content,
	})
	if err != nil {
		if errors.Is(err, supportticket.ErrTicketNotFound) {
			apierr.ResourceNotFound(c, "Support ticket not found")
			return
		}
		apierr.InternalError(c, "Failed to add reply")
		return
	}

	// Handle file uploads
	form, _ := c.MultipartForm()
	if form != nil && form.File["files[]"] != nil {
		for _, fileHeader := range form.File["files[]"] {
			func() {
				file, err := fileHeader.Open()
				if err != nil {
					slog.Warn("failed to open uploaded file", "filename", fileHeader.Filename, "error", err)
					return
				}
				defer file.Close()
				contentType := fileHeader.Header.Get("Content-Type")
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				if _, err := h.service.UploadAttachment(c.Request.Context(), ticketID, adminUserID, &msg.ID, true, &supportticket.UploadAttachmentRequest{
					FileName:    fileHeader.Filename,
					ContentType: contentType,
					Size:        fileHeader.Size,
					Reader:      file,
				}); err != nil {
					slog.Warn("failed to upload admin attachment", "filename", fileHeader.Filename, "error", err)
				}
			}()
		}
	}

	// Audit log
	h.logAction(c, admin.AuditActionSupportTicketReply, admin.TargetTypeSupportTicket, ticketID, nil, gin.H{"content": content})

	c.JSON(http.StatusCreated, msg)
}

// UpdateStatusRequest represents the request body for updating ticket status
type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateStatus updates the status of a support ticket
// PATCH /api/v1/admin/support-tickets/:id/status
func (h *SupportTicketHandler) UpdateStatus(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid ticket ID")
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierr.ValidationError(c, err.Error())
		return
	}

	// Get old ticket for audit log
	oldTicket, err := h.service.AdminGetByID(c.Request.Context(), ticketID)
	if err != nil {
		if errors.Is(err, supportticket.ErrTicketNotFound) {
			apierr.ResourceNotFound(c, "Support ticket not found")
			return
		}
		apierr.InternalError(c, "Failed to get support ticket")
		return
	}

	if err := h.service.AdminUpdateStatus(c.Request.Context(), ticketID, req.Status); err != nil {
		switch {
		case errors.Is(err, supportticket.ErrInvalidStatus):
			apierr.BadRequest(c, apierr.VALIDATION_FAILED, "Invalid status")
		case errors.Is(err, supportticket.ErrInvalidTransition):
			apierr.BadRequest(c, apierr.VALIDATION_FAILED, "Invalid status transition")
		case errors.Is(err, supportticket.ErrTicketNotFound):
			apierr.ResourceNotFound(c, "Support ticket not found")
		default:
			apierr.InternalError(c, "Failed to update status")
		}
		return
	}

	// Audit log
	h.logAction(c, admin.AuditActionSupportTicketStatus, admin.TargetTypeSupportTicket, ticketID,
		gin.H{"status": oldTicket.Status}, gin.H{"status": req.Status})

	c.JSON(http.StatusOK, gin.H{"message": "Status updated"})
}

// AssignRequest represents the request body for assigning a ticket
type AssignRequest struct {
	AdminID *int64 `json:"admin_id"`
}

// Assign assigns a support ticket to the current admin or a specified admin
// POST /api/v1/admin/support-tickets/:id/assign
func (h *SupportTicketHandler) Assign(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid ticket ID")
		return
	}

	adminUserID := middleware.GetAdminUserID(c)

	var req AssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default: assign to current admin
		req.AdminID = &adminUserID
	}
	if req.AdminID == nil {
		req.AdminID = &adminUserID
	}

	if err := h.service.AdminAssign(c.Request.Context(), ticketID, *req.AdminID); err != nil {
		if errors.Is(err, supportticket.ErrTicketNotFound) {
			apierr.ResourceNotFound(c, "Support ticket not found")
			return
		}
		apierr.InternalError(c, "Failed to assign ticket")
		return
	}

	// Audit log
	h.logAction(c, admin.AuditActionSupportTicketAssign, admin.TargetTypeSupportTicket, ticketID,
		nil, gin.H{"assigned_admin_id": *req.AdminID})

	c.JSON(http.StatusOK, gin.H{"message": "Ticket assigned"})
}

// GetAttachmentURL returns a presigned URL for downloading an attachment (admin)
// GET /api/v1/admin/support-tickets/attachments/:attachmentId/url
func (h *SupportTicketHandler) GetAttachmentURL(c *gin.Context) {
	attachmentID, err := strconv.ParseInt(c.Param("attachmentId"), 10, 64)
	if err != nil {
		apierr.InvalidInput(c, "Invalid attachment ID")
		return
	}

	url, err := h.service.AdminGetAttachmentURL(c.Request.Context(), attachmentID)
	if err != nil {
		if errors.Is(err, supportticket.ErrAttachmentNotFound) {
			apierr.ResourceNotFound(c, "Attachment not found")
			return
		}
		apierr.InternalError(c, "Failed to get attachment URL")
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}
