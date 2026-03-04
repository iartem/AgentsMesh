package supportticket

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/anthropics/agentsmesh/backend/internal/domain/supportticket"
)

// ============================================================
// User methods
// ============================================================

func TestCreate(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	req := &CreateRequest{
		Title:    "Login page broken",
		Category: domain.CategoryBug,
		Priority: domain.PriorityHigh,
	}

	ticket, err := svc.Create(ctx, 1, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket == nil {
		t.Fatal("expected non-nil ticket")
	}
	if ticket.Title != "Login page broken" {
		t.Errorf("expected title 'Login page broken', got %q", ticket.Title)
	}
	if ticket.Category != domain.CategoryBug {
		t.Errorf("expected category %q, got %q", domain.CategoryBug, ticket.Category)
	}
	if ticket.Priority != domain.PriorityHigh {
		t.Errorf("expected priority %q, got %q", domain.PriorityHigh, ticket.Priority)
	}
	if ticket.Status != domain.StatusOpen {
		t.Errorf("expected status %q, got %q", domain.StatusOpen, ticket.Status)
	}
	if ticket.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", ticket.UserID)
	}
	if ticket.ID == 0 {
		t.Error("expected non-zero ticket ID")
	}
}

func TestCreate_DefaultCategoryAndPriority(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	req := &CreateRequest{
		Title: "General question",
	}

	ticket, err := svc.Create(ctx, 1, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.Category != domain.CategoryOther {
		t.Errorf("expected default category %q, got %q", domain.CategoryOther, ticket.Category)
	}
	if ticket.Priority != domain.PriorityMedium {
		t.Errorf("expected default priority %q, got %q", domain.PriorityMedium, ticket.Priority)
	}
}

func TestCreate_InvalidCategory(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	req := &CreateRequest{
		Title:    "Bad category",
		Category: "nonexistent_category",
	}

	_, err := svc.Create(ctx, 1, req)
	if !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("expected ErrInvalidCategory, got %v", err)
	}
}

func TestCreate_InvalidPriority(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	req := &CreateRequest{
		Title:    "Bad priority",
		Category: domain.CategoryBug,
		Priority: "critical",
	}

	_, err := svc.Create(ctx, 1, req)
	if !errors.Is(err, ErrInvalidPriority) {
		t.Fatalf("expected ErrInvalidPriority, got %v", err)
	}
}

func TestCreate_WithContent(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	req := &CreateRequest{
		Title:    "Need help",
		Category: domain.CategoryUsageQuestion,
		Content:  "How do I configure the runner?",
	}

	ticket, err := svc.Create(ctx, 1, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the initial message was created
	var messages []domain.SupportTicketMessage
	if err := db.Where("ticket_id = ?", ticket.ID).Find(&messages).Error; err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "How do I configure the runner?" {
		t.Errorf("expected message content 'How do I configure the runner?', got %q", messages[0].Content)
	}
	if messages[0].IsAdminReply {
		t.Error("expected initial message to not be an admin reply")
	}
	if messages[0].UserID != 1 {
		t.Errorf("expected message user_id 1, got %d", messages[0].UserID)
	}
}

func TestListByUser(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	// Create tickets with a small delay to ensure ordering
	for i := 0; i < 3; i++ {
		_, err := svc.Create(ctx, 1, &CreateRequest{
			Title:    "Ticket " + string(rune('A'+i)),
			Category: domain.CategoryOther,
		})
		if err != nil {
			t.Fatalf("failed to create ticket %d: %v", i, err)
		}
	}
	// Create a ticket for a different user
	_, err := svc.Create(ctx, 2, &CreateRequest{
		Title:    "Other user ticket",
		Category: domain.CategoryOther,
	})
	if err != nil {
		t.Fatalf("failed to create other user ticket: %v", err)
	}

	resp, err := svc.ListByUser(ctx, 1, &ListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 tickets, got %d", len(resp.Data))
	}

	// Verify DESC ordering (newest first)
	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i-1].CreatedAt.Before(resp.Data[i].CreatedAt) {
			t.Errorf("expected descending order by created_at, but ticket at index %d is older than ticket at index %d", i-1, i)
		}
	}
}

func TestListByUser_StatusFilter(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	// Create two open tickets and one resolved
	for i := 0; i < 2; i++ {
		svc.Create(ctx, 1, &CreateRequest{Title: "Open ticket", Category: domain.CategoryOther})
	}
	ticket, _ := svc.Create(ctx, 1, &CreateRequest{Title: "Resolved ticket", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket.ID).Update("status", domain.StatusResolved)

	resp, err := svc.ListByUser(ctx, 1, &ListQuery{Status: domain.StatusOpen, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 open tickets, got %d", resp.Total)
	}

	resp, err = svc.ListByUser(ctx, 1, &ListQuery{Status: domain.StatusResolved, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 resolved ticket, got %d", resp.Total)
	}
}

func TestListByUser_Pagination(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	// Create 5 tickets
	for i := 0; i < 5; i++ {
		svc.Create(ctx, 1, &CreateRequest{Title: "Ticket", Category: domain.CategoryOther})
	}

	// Page 1, size 2
	resp, err := svc.ListByUser(ctx, 1, &ListQuery{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Total)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(resp.Data))
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.PageSize != 2 {
		t.Errorf("expected pageSize 2, got %d", resp.PageSize)
	}
	if resp.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", resp.TotalPages)
	}

	// Page 3, size 2 (should have 1 item)
	resp, err = svc.ListByUser(ctx, 1, &ListQuery{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 item on last page, got %d", len(resp.Data))
	}
}

func TestListByUser_EmptyResult(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	resp, err := svc.ListByUser(ctx, 999, &ListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data slice, got nil")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty data slice, got %d items", len(resp.Data))
	}
}

func TestGetByID(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "My ticket",
		Category: domain.CategoryAccount,
		Priority: domain.PriorityLow,
	})

	ticket, err := svc.GetByID(ctx, created.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, ticket.ID)
	}
	if ticket.Title != "My ticket" {
		t.Errorf("expected title 'My ticket', got %q", ticket.Title)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	_, err := svc.GetByID(ctx, 99999, 1)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestGetByID_WrongUser(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "User 1 ticket",
		Category: domain.CategoryOther,
	})

	_, err := svc.GetByID(ctx, created.ID, 2)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound for wrong user, got %v", err)
	}
}

func TestAddMessage(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Test ticket",
		Category: domain.CategoryOther,
	})

	msg, err := svc.AddMessage(ctx, ticket.ID, 1, &AddMessageRequest{
		Content: "Here is more info",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if msg.Content != "Here is more info" {
		t.Errorf("expected content 'Here is more info', got %q", msg.Content)
	}
	if msg.IsAdminReply {
		t.Error("expected message to not be an admin reply")
	}
	if msg.TicketID != ticket.ID {
		t.Errorf("expected ticket_id %d, got %d", ticket.ID, msg.TicketID)
	}
	if msg.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", msg.UserID)
	}
}

func TestAddMessage_ReopensResolved(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Resolved ticket",
		Category: domain.CategoryOther,
	})

	// Manually set status to resolved
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket.ID).Update("status", domain.StatusResolved)

	// User adds a message
	_, err := svc.AddMessage(ctx, ticket.ID, 1, &AddMessageRequest{Content: "Still having issues"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify ticket is reopened
	updated, err := svc.GetByID(ctx, ticket.ID, 1)
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}
	if updated.Status != domain.StatusOpen {
		t.Errorf("expected status %q after reopen, got %q", domain.StatusOpen, updated.Status)
	}
}

func TestAddMessage_WrongUser(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "User 1 ticket",
		Category: domain.CategoryOther,
	})

	_, err := svc.AddMessage(ctx, ticket.ID, 2, &AddMessageRequest{Content: "I shouldn't be able to do this"})
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound for wrong user, got %v", err)
	}
}

func TestListMessages(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Ticket with messages",
		Category: domain.CategoryOther,
		Content:  "Initial message",
	})

	svc.AddMessage(ctx, ticket.ID, 1, &AddMessageRequest{Content: "Follow-up"})

	messages, err := svc.ListMessages(ctx, ticket.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "Initial message" {
		t.Errorf("expected first message 'Initial message', got %q", messages[0].Content)
	}
	if messages[1].Content != "Follow-up" {
		t.Errorf("expected second message 'Follow-up', got %q", messages[1].Content)
	}
}

func TestListMessages_WrongUser(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "User 1 ticket",
		Category: domain.CategoryOther,
		Content:  "Hello",
	})

	_, err := svc.ListMessages(ctx, ticket.ID, 2)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound for wrong user, got %v", err)
	}
}

// ============================================================
// Admin methods
// ============================================================

func TestAdminList(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	// Create tickets from different users
	svc.Create(ctx, 1, &CreateRequest{Title: "User 1 ticket", Category: domain.CategoryBug})
	svc.Create(ctx, 2, &CreateRequest{Title: "User 2 ticket", Category: domain.CategoryAccount})
	svc.Create(ctx, 3, &CreateRequest{Title: "User 3 ticket", Category: domain.CategoryOther})

	resp, err := svc.AdminList(ctx, &AdminListQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}
	if len(resp.Data) != 3 {
		t.Errorf("expected 3 tickets, got %d", len(resp.Data))
	}
}

func TestAdminList_SearchFilter(t *testing.T) {
	// SQLite does not support ILIKE; the AdminList method uses ILIKE which is
	// PostgreSQL-specific. We skip this test in SQLite-backed test runs.
	t.Skip("Skipping: AdminList search uses ILIKE which is not supported by SQLite")
}

func TestAdminList_StatusFilter(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	svc.Create(ctx, 1, &CreateRequest{Title: "Open", Category: domain.CategoryOther})
	ticket2, _ := svc.Create(ctx, 1, &CreateRequest{Title: "Closed", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket2.ID).Update("status", domain.StatusClosed)

	resp, err := svc.AdminList(ctx, &AdminListQuery{Status: domain.StatusOpen, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 open ticket, got %d", resp.Total)
	}
	if len(resp.Data) > 0 && resp.Data[0].Title != "Open" {
		t.Errorf("expected title 'Open', got %q", resp.Data[0].Title)
	}
}

func TestAdminList_CategoryFilter(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	svc.Create(ctx, 1, &CreateRequest{Title: "Bug report", Category: domain.CategoryBug})
	svc.Create(ctx, 1, &CreateRequest{Title: "Feature request", Category: domain.CategoryFeatureRequest})

	resp, err := svc.AdminList(ctx, &AdminListQuery{Category: domain.CategoryBug, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 bug ticket, got %d", resp.Total)
	}
}

func TestAdminList_PriorityFilter(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	svc.Create(ctx, 1, &CreateRequest{Title: "High prio", Category: domain.CategoryOther, Priority: domain.PriorityHigh})
	svc.Create(ctx, 1, &CreateRequest{Title: "Low prio", Category: domain.CategoryOther, Priority: domain.PriorityLow})
	svc.Create(ctx, 1, &CreateRequest{Title: "Default prio", Category: domain.CategoryOther}) // defaults to medium

	resp, err := svc.AdminList(ctx, &AdminListQuery{Priority: domain.PriorityHigh, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 high-priority ticket, got %d", resp.Total)
	}
}

func TestAdminGetByID(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Any user ticket",
		Category: domain.CategoryOther,
	})

	// Admin can access any ticket regardless of ownership
	ticket, err := svc.AdminGetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, ticket.ID)
	}
	if ticket.Title != "Any user ticket" {
		t.Errorf("expected title 'Any user ticket', got %q", ticket.Title)
	}
}

func TestAdminGetByID_NotFound(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	_, err := svc.AdminGetByID(ctx, 99999)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestAdminAddReply(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()
	createTestUser(t, db, 100, "admin@test.com")

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "User ticket",
		Category: domain.CategoryOther,
	})

	msg, err := svc.AdminAddReply(ctx, ticket.ID, 100, &AddMessageRequest{
		Content: "We are looking into this.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if !msg.IsAdminReply {
		t.Error("expected message to be an admin reply")
	}
	if msg.Content != "We are looking into this." {
		t.Errorf("expected content 'We are looking into this.', got %q", msg.Content)
	}
	if msg.UserID != 100 {
		t.Errorf("expected user_id 100, got %d", msg.UserID)
	}
}

func TestAdminAddReply_AutoTransition(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Open ticket",
		Category: domain.CategoryOther,
	})

	// Verify ticket is open
	var status string
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket.ID).Pluck("status", &status)
	if status != domain.StatusOpen {
		t.Fatalf("expected initial status %q, got %q", domain.StatusOpen, status)
	}

	// Admin replies
	_, err := svc.AdminAddReply(ctx, ticket.ID, 100, &AddMessageRequest{Content: "Looking into it"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify status transitioned to in_progress
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket.ID).Pluck("status", &status)
	if status != domain.StatusInProgress {
		t.Errorf("expected status %q after admin reply, got %q", domain.StatusInProgress, status)
	}
}

func TestAdminAddReply_NotFound(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	_, err := svc.AdminAddReply(ctx, 99999, 100, &AddMessageRequest{Content: "Reply"})
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestAdminUpdateStatus(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Status test",
		Category: domain.CategoryOther,
	})

	err := svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusInProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var status string
	db.Model(&domain.SupportTicket{}).Where("id = ?", ticket.ID).Pluck("status", &status)
	if status != domain.StatusInProgress {
		t.Errorf("expected status %q, got %q", domain.StatusInProgress, status)
	}
}

func TestAdminUpdateStatus_Resolved(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Resolve test",
		Category: domain.CategoryOther,
	})

	err := svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusResolved)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated domain.SupportTicket
	db.First(&updated, ticket.ID)
	if updated.Status != domain.StatusResolved {
		t.Errorf("expected status %q, got %q", domain.StatusResolved, updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	} else {
		// Verify resolved_at is recent (within last 5 seconds)
		if time.Since(*updated.ResolvedAt) > 5*time.Second {
			t.Error("expected resolved_at to be recent")
		}
	}
}

func TestAdminUpdateStatus_InvalidStatus(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Status test",
		Category: domain.CategoryOther,
	})

	err := svc.AdminUpdateStatus(ctx, ticket.ID, "invalid_status")
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestAdminUpdateStatus_NotFound(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	err := svc.AdminUpdateStatus(ctx, 99999, domain.StatusClosed)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestAdminUpdateStatus_InvalidTransition(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Transition test",
		Category: domain.CategoryOther,
	})

	// Close the ticket first (open -> closed is valid)
	_ = svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusClosed)

	// closed -> in_progress is NOT a valid transition
	err := svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusInProgress)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestAdminUpdateStatus_SameStatusNoop(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Noop test",
		Category: domain.CategoryOther,
	})

	// open -> open should be a no-op (no error)
	err := svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusOpen)
	if err != nil {
		t.Fatalf("expected no error for same status, got %v", err)
	}
}

func TestAdminUpdateStatus_ResolvedAtNotOverwritten(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "ResolvedAt test",
		Category: domain.CategoryOther,
	})

	// First resolve: open -> resolved
	_ = svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusResolved)

	var first domain.SupportTicket
	db.First(&first, ticket.ID)
	if first.ResolvedAt == nil {
		t.Fatal("expected resolved_at to be set after first resolve")
	}
	firstResolvedAt := *first.ResolvedAt

	// Reopen: resolved -> open
	_ = svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusOpen)

	// Re-resolve: open -> resolved
	_ = svc.AdminUpdateStatus(ctx, ticket.ID, domain.StatusResolved)

	var second domain.SupportTicket
	db.First(&second, ticket.ID)
	if second.ResolvedAt == nil {
		t.Fatal("expected resolved_at to still be set")
	}
	// ResolvedAt should NOT have been overwritten
	if !firstResolvedAt.Equal(*second.ResolvedAt) {
		t.Errorf("resolved_at was overwritten: first=%v, second=%v", firstResolvedAt, *second.ResolvedAt)
	}
}

func TestAdminAssign(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	ticket, _ := svc.Create(ctx, 1, &CreateRequest{
		Title:    "Assign test",
		Category: domain.CategoryOther,
	})

	err := svc.AdminAssign(ctx, ticket.ID, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated domain.SupportTicket
	db.First(&updated, ticket.ID)
	if updated.AssignedAdminID == nil {
		t.Fatal("expected assigned_admin_id to be set")
	}
	if *updated.AssignedAdminID != 42 {
		t.Errorf("expected assigned_admin_id 42, got %d", *updated.AssignedAdminID)
	}
}

func TestAdminAssign_NotFound(t *testing.T) {
	svc, _ := createTestService(t)
	ctx := context.Background()

	err := svc.AdminAssign(ctx, 99999, 42)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestAdminGetStats(t *testing.T) {
	svc, db := createTestService(t)
	ctx := context.Background()

	// Create tickets with various statuses
	svc.Create(ctx, 1, &CreateRequest{Title: "Open 1", Category: domain.CategoryOther})
	svc.Create(ctx, 1, &CreateRequest{Title: "Open 2", Category: domain.CategoryOther})

	t3, _ := svc.Create(ctx, 1, &CreateRequest{Title: "In Progress", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", t3.ID).Update("status", domain.StatusInProgress)

	t4, _ := svc.Create(ctx, 1, &CreateRequest{Title: "Resolved", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", t4.ID).Update("status", domain.StatusResolved)

	t5, _ := svc.Create(ctx, 1, &CreateRequest{Title: "Closed 1", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", t5.ID).Update("status", domain.StatusClosed)

	t6, _ := svc.Create(ctx, 1, &CreateRequest{Title: "Closed 2", Category: domain.CategoryOther})
	db.Model(&domain.SupportTicket{}).Where("id = ?", t6.ID).Update("status", domain.StatusClosed)

	stats, err := svc.AdminGetStats(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Total != 6 {
		t.Errorf("expected total 6, got %d", stats.Total)
	}
	if stats.Open != 2 {
		t.Errorf("expected open 2, got %d", stats.Open)
	}
	if stats.InProgress != 1 {
		t.Errorf("expected in_progress 1, got %d", stats.InProgress)
	}
	if stats.Resolved != 1 {
		t.Errorf("expected resolved 1, got %d", stats.Resolved)
	}
	if stats.Closed != 2 {
		t.Errorf("expected closed 2, got %d", stats.Closed)
	}
}

// ============================================================
// Helpers
// ============================================================

func TestNormalizePagination(t *testing.T) {
	tests := []struct {
		name             string
		page, pageSize   int
		wantPage, wantPS int
	}{
		{"zero values default", 0, 0, 1, 20},
		{"negative values default", -5, -1, 1, 20},
		{"valid values pass through", 3, 50, 3, 50},
		{"pageSize over 100 defaults", 1, 200, 1, 20},
		{"page 1 pageSize 1", 1, 1, 1, 1},
		{"boundary pageSize 100", 1, 100, 1, 100},
		{"boundary pageSize 101 defaults", 1, 101, 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPS := normalizePagination(tt.page, tt.pageSize)
			if gotPage != tt.wantPage {
				t.Errorf("normalizePagination(%d, %d) page = %d, want %d", tt.page, tt.pageSize, gotPage, tt.wantPage)
			}
			if gotPS != tt.wantPS {
				t.Errorf("normalizePagination(%d, %d) pageSize = %d, want %d", tt.page, tt.pageSize, gotPS, tt.wantPS)
			}
		})
	}
}
