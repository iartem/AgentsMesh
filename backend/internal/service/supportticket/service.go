package supportticket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"path"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/supportticket"
	"github.com/anthropics/agentsmesh/backend/internal/infra/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrTicketNotFound     = errors.New("support ticket not found")
	ErrAccessDenied       = errors.New("access denied")
	ErrInvalidCategory    = errors.New("invalid category")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrInvalidPriority    = errors.New("invalid priority")
	ErrStorageError       = errors.New("storage operation failed")
	ErrFileTooLarge       = errors.New("file exceeds maximum size")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

// Service handles support ticket operations
type Service struct {
	db      *gorm.DB
	storage storage.Storage
	config  config.StorageConfig
}

// NewService creates a new support ticket service
func NewService(db *gorm.DB, storage storage.Storage, cfg config.StorageConfig) *Service {
	return &Service{
		db:      db,
		storage: storage,
		config:  cfg,
	}
}

// --- Request/Response types ---

// CreateRequest represents a request to create a support ticket
type CreateRequest struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Content  string `json:"content"`
	Priority string `json:"priority"`
}

// AddMessageRequest represents a request to add a message to a ticket
type AddMessageRequest struct {
	Content string `json:"content"`
}

// UploadAttachmentRequest represents a file upload for a ticket attachment
type UploadAttachmentRequest struct {
	FileName    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

// ListQuery represents query parameters for listing user tickets
type ListQuery struct {
	Status   string
	Page     int
	PageSize int
}

// AdminListQuery represents query parameters for admin listing
type AdminListQuery struct {
	Search   string
	Status   string
	Category string
	Priority string
	Page     int
	PageSize int
}

// ListResponse represents a paginated list response
type ListResponse struct {
	Data       []supportticket.SupportTicket `json:"data"`
	Total      int64                         `json:"total"`
	Page       int                           `json:"page"`
	PageSize   int                           `json:"page_size"`
	TotalPages int                           `json:"total_pages"`
}

// Stats represents support ticket statistics
type Stats struct {
	Total      int64 `json:"total"`
	Open       int64 `json:"open"`
	InProgress int64 `json:"in_progress"`
	Resolved   int64 `json:"resolved"`
	Closed     int64 `json:"closed"`
}

// --- User-side methods ---

// Create creates a new support ticket with an initial message
func (s *Service) Create(ctx context.Context, userID int64, req *CreateRequest) (*supportticket.SupportTicket, error) {
	// Validate category
	category := req.Category
	if category == "" {
		category = supportticket.CategoryOther
	}
	if !supportticket.ValidCategories[category] {
		return nil, ErrInvalidCategory
	}

	// Validate priority
	priority := req.Priority
	if priority == "" {
		priority = supportticket.PriorityMedium
	}
	if !supportticket.ValidPriorities[priority] {
		return nil, ErrInvalidPriority
	}

	var ticket supportticket.SupportTicket

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create ticket
		ticket = supportticket.SupportTicket{
			UserID:   userID,
			Title:    req.Title,
			Category: category,
			Status:   supportticket.StatusOpen,
			Priority: priority,
		}
		if err := tx.Create(&ticket).Error; err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}

		// Create initial message
		if req.Content != "" {
			msg := supportticket.SupportTicketMessage{
				TicketID:     ticket.ID,
				UserID:       userID,
				Content:      req.Content,
				IsAdminReply: false,
			}
			if err := tx.Create(&msg).Error; err != nil {
				return fmt.Errorf("failed to create initial message: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}

// ListByUser returns paginated tickets for a specific user
func (s *Service) ListByUser(ctx context.Context, userID int64, query *ListQuery) (*ListResponse, error) {
	page, pageSize := normalizePagination(query.Page, query.PageSize)

	db := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	var total int64
	if err := db.Model(&supportticket.SupportTicket{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count tickets: %w", err)
	}

	var tickets []supportticket.SupportTicket
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tickets).Error; err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return &ListResponse{
		Data:       tickets,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
	}, nil
}

// GetByID returns a ticket by ID, verifying user ownership
func (s *Service) GetByID(ctx context.Context, id, userID int64) (*supportticket.SupportTicket, error) {
	var ticket supportticket.SupportTicket
	err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&ticket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

// AddMessage adds a user message to a ticket
func (s *Service) AddMessage(ctx context.Context, ticketID, userID int64, req *AddMessageRequest) (*supportticket.SupportTicketMessage, error) {
	// Verify ticket ownership
	if _, err := s.GetByID(ctx, ticketID, userID); err != nil {
		return nil, err
	}

	msg := &supportticket.SupportTicketMessage{
		TicketID:     ticketID,
		UserID:       userID,
		Content:      req.Content,
		IsAdminReply: false,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		// Reopen if resolved/closed
		if err := tx.Model(&supportticket.SupportTicket{}).
			Where("id = ? AND status IN ?", ticketID, []string{supportticket.StatusResolved, supportticket.StatusClosed}).
			Updates(map[string]interface{}{
				"status":     supportticket.StatusOpen,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to reopen ticket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// ListMessages returns all messages for a ticket (user-side, verifies ownership)
func (s *Service) ListMessages(ctx context.Context, ticketID, userID int64) ([]supportticket.SupportTicketMessage, error) {
	// Verify ticket ownership
	if _, err := s.GetByID(ctx, ticketID, userID); err != nil {
		return nil, err
	}

	return s.listMessagesByTicketID(ctx, ticketID)
}

// UploadAttachment uploads a file attachment and associates it with a ticket/message
func (s *Service) UploadAttachment(ctx context.Context, ticketID, userID int64, messageID *int64, isAdmin bool, req *UploadAttachmentRequest) (*supportticket.SupportTicketAttachment, error) {
	if s.storage == nil {
		return nil, ErrStorageError
	}

	// Verify ticket exists
	var ticket supportticket.SupportTicket
	err := s.db.WithContext(ctx).First(&ticket, ticketID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	// For regular users, verify ownership; admin users bypass this check
	if ticket.UserID != userID && !isAdmin {
		return nil, ErrAccessDenied
	}

	// Validate file size using config (fallback to 10MB)
	maxSize := s.config.MaxFileSize * 1024 * 1024
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if req.Size > maxSize {
		return nil, ErrFileTooLarge
	}

	// Generate storage key: support-tickets/{user_id}/{year}/{month}/{uuid}.{ext}
	ext := path.Ext(req.FileName)
	if ext == "" {
		ext = ".bin"
	}
	now := time.Now()
	storageKey := fmt.Sprintf("support-tickets/%d/%d/%02d/%s%s",
		userID, now.Year(), now.Month(), uuid.New().String(), ext)

	// Upload to storage
	if _, err := s.storage.Upload(ctx, storageKey, req.Reader, req.Size, req.ContentType); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageError, err)
	}

	// Create attachment record
	attachment := &supportticket.SupportTicketAttachment{
		TicketID:     ticketID,
		MessageID:    messageID,
		UploaderID:   userID,
		OriginalName: req.FileName,
		StorageKey:   storageKey,
		MimeType:     req.ContentType,
		Size:         req.Size,
	}
	if err := s.db.WithContext(ctx).Create(attachment).Error; err != nil {
		// Cleanup uploaded file on DB error
		if delErr := s.storage.Delete(ctx, storageKey); delErr != nil {
			slog.Warn("failed to cleanup uploaded file after DB error", "storage_key", storageKey, "error", delErr)
		}
		return nil, fmt.Errorf("failed to create attachment record: %w", err)
	}

	return attachment, nil
}

// GetAttachmentURL returns a presigned URL for downloading an attachment
func (s *Service) GetAttachmentURL(ctx context.Context, attachmentID, userID int64) (string, error) {
	if s.storage == nil {
		return "", ErrStorageError
	}

	var attachment supportticket.SupportTicketAttachment
	err := s.db.WithContext(ctx).First(&attachment, attachmentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrAttachmentNotFound
		}
		return "", err
	}

	// Verify user has access (owns the ticket)
	var ticket supportticket.SupportTicket
	err = s.db.WithContext(ctx).First(&ticket, attachment.TicketID).Error
	if err != nil {
		return "", ErrTicketNotFound
	}
	if ticket.UserID != userID {
		return "", ErrAccessDenied
	}

	return s.storage.GetURL(ctx, attachment.StorageKey, 1*time.Hour)
}

// --- Admin-side methods ---

// AdminList returns paginated tickets for admin (all users)
func (s *Service) AdminList(ctx context.Context, query *AdminListQuery) (*ListResponse, error) {
	page, pageSize := normalizePagination(query.Page, query.PageSize)

	db := s.db.WithContext(ctx).Model(&supportticket.SupportTicket{})
	db = db.Preload("User").Preload("AssignedAdmin")

	if query.Search != "" {
		db = db.Where("title ILIKE ?", "%"+query.Search+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Category != "" {
		db = db.Where("category = ?", query.Category)
	}
	if query.Priority != "" {
		db = db.Where("priority = ?", query.Priority)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count tickets: %w", err)
	}

	var tickets []supportticket.SupportTicket
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&tickets).Error; err != nil {
		return nil, fmt.Errorf("failed to list tickets: %w", err)
	}

	return &ListResponse{
		Data:       tickets,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
	}, nil
}

// AdminGetByID returns a ticket by ID (no ownership check)
func (s *Service) AdminGetByID(ctx context.Context, id int64) (*supportticket.SupportTicket, error) {
	var ticket supportticket.SupportTicket
	err := s.db.WithContext(ctx).
		Preload("User").
		Preload("AssignedAdmin").
		First(&ticket, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

// AdminListMessages returns all messages for a ticket (admin, no ownership check)
func (s *Service) AdminListMessages(ctx context.Context, ticketID int64) ([]supportticket.SupportTicketMessage, error) {
	return s.listMessagesByTicketID(ctx, ticketID)
}

// AdminAddReply adds an admin reply to a ticket
func (s *Service) AdminAddReply(ctx context.Context, ticketID, adminUserID int64, req *AddMessageRequest) (*supportticket.SupportTicketMessage, error) {
	// Verify ticket exists
	if _, err := s.AdminGetByID(ctx, ticketID); err != nil {
		return nil, err
	}

	msg := &supportticket.SupportTicketMessage{
		TicketID:     ticketID,
		UserID:       adminUserID,
		Content:      req.Content,
		IsAdminReply: true,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return fmt.Errorf("failed to create admin reply: %w", err)
		}

		// Auto-transition from open to in_progress when admin first replies
		if err := tx.Model(&supportticket.SupportTicket{}).
			Where("id = ? AND status = ?", ticketID, supportticket.StatusOpen).
			Updates(map[string]interface{}{
				"status":     supportticket.StatusInProgress,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to transition ticket status: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// AdminUpdateStatus updates the status of a ticket with transition validation
func (s *Service) AdminUpdateStatus(ctx context.Context, ticketID int64, status string) error {
	if !supportticket.ValidStatuses[status] {
		return ErrInvalidStatus
	}

	// Fetch current ticket to validate transition
	var ticket supportticket.SupportTicket
	if err := s.db.WithContext(ctx).First(&ticket, ticketID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTicketNotFound
		}
		return fmt.Errorf("failed to get ticket: %w", err)
	}

	// Same status is a no-op
	if ticket.Status == status {
		return nil
	}

	// Validate transition
	allowed, ok := supportticket.ValidTransitions[ticket.Status]
	if !ok || !allowed[status] {
		return ErrInvalidTransition
	}

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	// Only set resolved_at on first resolution (don't overwrite)
	if status == supportticket.StatusResolved && ticket.ResolvedAt == nil {
		now := time.Now()
		updates["resolved_at"] = &now
	}

	// Optimistic lock: WHERE includes current status to prevent race conditions
	result := s.db.WithContext(ctx).
		Model(&supportticket.SupportTicket{}).
		Where("id = ? AND status = ?", ticketID, ticket.Status).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update ticket status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Status was concurrently changed by another request
		return ErrInvalidTransition
	}
	return nil
}

// AdminAssign assigns a ticket to an admin
func (s *Service) AdminAssign(ctx context.Context, ticketID, adminUserID int64) error {
	result := s.db.WithContext(ctx).
		Model(&supportticket.SupportTicket{}).
		Where("id = ?", ticketID).
		Updates(map[string]interface{}{
			"assigned_admin_id": adminUserID,
			"updated_at":        time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to assign ticket: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTicketNotFound
	}
	return nil
}

// AdminGetStats returns ticket statistics
func (s *Service) AdminGetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	// Each query must use a fresh db session to avoid Where condition accumulation
	model := &supportticket.SupportTicket{}
	if err := s.db.WithContext(ctx).Model(model).Count(&stats.Total).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(model).Where("status = ?", supportticket.StatusOpen).Count(&stats.Open).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(model).Where("status = ?", supportticket.StatusInProgress).Count(&stats.InProgress).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(model).Where("status = ?", supportticket.StatusResolved).Count(&stats.Resolved).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(model).Where("status = ?", supportticket.StatusClosed).Count(&stats.Closed).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// AdminGetAttachmentURL returns a presigned URL for downloading an attachment (admin, no ownership check)
func (s *Service) AdminGetAttachmentURL(ctx context.Context, attachmentID int64) (string, error) {
	if s.storage == nil {
		return "", ErrStorageError
	}

	var attachment supportticket.SupportTicketAttachment
	err := s.db.WithContext(ctx).First(&attachment, attachmentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrAttachmentNotFound
		}
		return "", err
	}

	return s.storage.GetURL(ctx, attachment.StorageKey, 1*time.Hour)
}

// --- Internal helpers ---

func (s *Service) listMessagesByTicketID(ctx context.Context, ticketID int64) ([]supportticket.SupportTicketMessage, error) {
	var messages []supportticket.SupportTicketMessage
	err := s.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Preload("User").
		Preload("Attachments").
		Order("created_at ASC").
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	return messages, nil
}

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
