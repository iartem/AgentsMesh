package ticket

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/ticket"
	"github.com/anthropics/agentmesh/backend/internal/domain/user"
	"gorm.io/gorm"
)

var (
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrLabelNotFound     = errors.New("label not found")
	ErrDuplicateLabel    = errors.New("label already exists")
	ErrInvalidTransition = errors.New("invalid status transition")
)

// Service handles ticket operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new ticket service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateTicketRequest represents a ticket creation request
type CreateTicketRequest struct {
	OrganizationID int64
	TeamID         *int64
	RepositoryID   *int64
	ReporterID     int64
	ParentTicketID *int64
	Type           string
	Title          string
	Description    *string
	Content        *string
	Status         string
	Priority       string
	DueDate        *time.Time
	AssigneeIDs    []int64
	LabelIDs       []int64
	Labels         []string // Label names for convenience
}

// CreateTicket creates a new ticket
func (s *Service) CreateTicket(ctx context.Context, req *CreateTicketRequest) (*ticket.Ticket, error) {
	// Generate ticket number and identifier
	var maxNumber int
	s.db.WithContext(ctx).Model(&ticket.Ticket{}).
		Where("repository_id = ?", req.RepositoryID).
		Select("COALESCE(MAX(number), 0)").
		Scan(&maxNumber)

	number := maxNumber + 1
	identifier := fmt.Sprintf("TICKET-%d", number)

	// If repository has a prefix, use it
	if req.RepositoryID != nil {
		var prefix string
		s.db.WithContext(ctx).Table("repositories").
			Where("id = ?", *req.RepositoryID).
			Select("ticket_prefix").
			Scan(&prefix)
		if prefix != "" {
			identifier = fmt.Sprintf("%s-%d", prefix, number)
		}
	}

	status := req.Status
	if status == "" {
		status = ticket.TicketStatusBacklog
	}

	t := &ticket.Ticket{
		OrganizationID: req.OrganizationID,
		TeamID:         req.TeamID,
		Number:         number,
		Identifier:     identifier,
		Type:           req.Type,
		Title:          req.Title,
		Description:    req.Description,
		Content:        req.Content,
		Status:         status,
		Priority:       req.Priority,
		DueDate:        req.DueDate,
		RepositoryID:   req.RepositoryID,
		ReporterID:     req.ReporterID,
		ParentTicketID: req.ParentTicketID,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}

		// Add assignees
		for _, userID := range req.AssigneeIDs {
			assignee := &ticket.Assignee{
				TicketID: t.ID,
				UserID:   userID,
			}
			if err := tx.Create(assignee).Error; err != nil {
				return err
			}
		}

		// Add labels by ID
		for _, labelID := range req.LabelIDs {
			ticketLabel := &ticket.TicketLabel{
				TicketID: t.ID,
				LabelID:  labelID,
			}
			if err := tx.Create(ticketLabel).Error; err != nil {
				return err
			}
		}

		// Add labels by name (if provided)
		for _, labelName := range req.Labels {
			var label ticket.Label
			if err := tx.Where("organization_id = ? AND name = ?", req.OrganizationID, labelName).First(&label).Error; err != nil {
				continue // Skip if label not found
			}
			ticketLabel := &ticket.TicketLabel{
				TicketID: t.ID,
				LabelID:  label.ID,
			}
			if err := tx.Create(ticketLabel).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.GetTicket(ctx, t.ID)
}

// GetTicket returns a ticket by ID
func (s *Service) GetTicket(ctx context.Context, ticketID int64) (*ticket.Ticket, error) {
	var t ticket.Ticket
	if err := s.db.WithContext(ctx).
		Preload("Assignees").
		Preload("Labels").
		Preload("MergeRequests").
		Preload("SubTickets").
		First(&t, ticketID).Error; err != nil {
		return nil, ErrTicketNotFound
	}
	return &t, nil
}

// GetTicketByIdentifier returns a ticket by identifier
func (s *Service) GetTicketByIdentifier(ctx context.Context, identifier string) (*ticket.Ticket, error) {
	var t ticket.Ticket
	if err := s.db.WithContext(ctx).
		Preload("Assignees").
		Preload("Labels").
		Preload("MergeRequests").
		Preload("SubTickets").
		Where("identifier = ?", identifier).
		First(&t).Error; err != nil {
		return nil, ErrTicketNotFound
	}
	return &t, nil
}

// ListTicketsFilter represents filters for listing tickets
type ListTicketsFilter struct {
	OrganizationID int64
	TeamID         *int64 // Deprecated: kept for backward compatibility
	RepositoryID   *int64
	Status         string
	Type           string
	Priority       string
	AssigneeID     *int64
	ReporterID     *int64
	LabelIDs       []int64
	ParentTicketID *int64
	Query          string
	UserRole       string // Kept for future use, all org members can access all resources
	Limit          int
	Offset         int
}

// ListTickets returns tickets based on filters
func (s *Service) ListTickets(ctx context.Context, filter *ListTicketsFilter) ([]*ticket.Ticket, int64, error) {
	query := s.db.WithContext(ctx).Model(&ticket.Ticket{}).Where("organization_id = ?", filter.OrganizationID)

	if filter.TeamID != nil {
		query = query.Where("team_id = ?", *filter.TeamID)
	}
	if filter.RepositoryID != nil {
		query = query.Where("repository_id = ?", *filter.RepositoryID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.ReporterID != nil {
		query = query.Where("reporter_id = ?", *filter.ReporterID)
	}
	if filter.ParentTicketID != nil {
		query = query.Where("parent_ticket_id = ?", *filter.ParentTicketID)
	}
	if filter.Query != "" {
		query = query.Where("title ILIKE ? OR identifier ILIKE ?", "%"+filter.Query+"%", "%"+filter.Query+"%")
	}
	if filter.AssigneeID != nil {
		query = query.Joins("JOIN ticket_assignees ON ticket_assignees.ticket_id = tickets.id").
			Where("ticket_assignees.user_id = ?", *filter.AssigneeID)
	}
	if len(filter.LabelIDs) > 0 {
		query = query.Joins("JOIN ticket_labels ON ticket_labels.ticket_id = tickets.id").
			Where("ticket_labels.label_id IN ?", filter.LabelIDs)
	}

	// Team-based access control removed: all organization members can access all resources

	var total int64
	query.Count(&total)

	var tickets []*ticket.Ticket
	if err := query.
		Preload("Assignees").
		Preload("Labels").
		Order("created_at DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// UpdateTicket updates a ticket
func (s *Service) UpdateTicket(ctx context.Context, ticketID int64, updates map[string]interface{}) (*ticket.Ticket, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&ticket.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetTicket(ctx, ticketID)
}

// UpdateAssignees updates ticket assignees
func (s *Service) UpdateAssignees(ctx context.Context, ticketID int64, userIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing assignees
		if err := tx.Where("ticket_id = ?", ticketID).Delete(&ticket.Assignee{}).Error; err != nil {
			return err
		}
		// Add new assignees
		for _, userID := range userIDs {
			assignee := &ticket.Assignee{
				TicketID: ticketID,
				UserID:   userID,
			}
			if err := tx.Create(assignee).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateStatus updates a ticket's status
func (s *Service) UpdateStatus(ctx context.Context, ticketID int64, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	now := time.Now()
	switch status {
	case ticket.TicketStatusInProgress:
		updates["started_at"] = now
	case ticket.TicketStatusDone:
		updates["completed_at"] = now
	}

	return s.db.WithContext(ctx).Model(&ticket.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error
}

// DeleteTicket deletes a ticket
func (s *Service) DeleteTicket(ctx context.Context, ticketID int64) error {
	return s.db.WithContext(ctx).Delete(&ticket.Ticket{}, ticketID).Error
}

// Label operations

// CreateLabel creates a new label
func (s *Service) CreateLabel(ctx context.Context, orgID int64, repoID *int64, name, color string) (*ticket.Label, error) {
	// Check for duplicate
	var existing ticket.Label
	query := s.db.WithContext(ctx).Where("organization_id = ? AND name = ?", orgID, name)
	if repoID != nil {
		query = query.Where("repository_id = ?", *repoID)
	} else {
		query = query.Where("repository_id IS NULL")
	}
	if err := query.First(&existing).Error; err == nil {
		return nil, ErrDuplicateLabel
	}

	label := &ticket.Label{
		OrganizationID: orgID,
		RepositoryID:   repoID,
		Name:           name,
		Color:          color,
	}

	if err := s.db.WithContext(ctx).Create(label).Error; err != nil {
		return nil, err
	}

	return label, nil
}

// GetLabel returns a label by ID
func (s *Service) GetLabel(ctx context.Context, labelID int64) (*ticket.Label, error) {
	var label ticket.Label
	if err := s.db.WithContext(ctx).First(&label, labelID).Error; err != nil {
		return nil, ErrLabelNotFound
	}
	return &label, nil
}

// ListLabels returns labels for an organization/repository
func (s *Service) ListLabels(ctx context.Context, orgID int64, repoID *int64) ([]*ticket.Label, error) {
	query := s.db.WithContext(ctx).Where("organization_id = ?", orgID)

	if repoID != nil {
		// Include both org-level and repo-level labels
		query = query.Where("repository_id IS NULL OR repository_id = ?", *repoID)
	} else {
		// Only org-level labels
		query = query.Where("repository_id IS NULL")
	}

	var labels []*ticket.Label
	if err := query.Order("name ASC").Find(&labels).Error; err != nil {
		return nil, err
	}

	return labels, nil
}

// UpdateLabel updates a label
func (s *Service) UpdateLabel(ctx context.Context, orgID, labelID int64, updates map[string]interface{}) (*ticket.Label, error) {
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(&ticket.Label{}).Where("id = ? AND organization_id = ?", labelID, orgID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.GetLabel(ctx, labelID)
}

// DeleteLabel deletes a label
func (s *Service) DeleteLabel(ctx context.Context, orgID, labelID int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove label from all tickets
		if err := tx.Where("label_id = ?", labelID).Delete(&ticket.TicketLabel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND organization_id = ?", labelID, orgID).Delete(&ticket.Label{}).Error
	})
}

// AddAssignee adds an assignee to a ticket
func (s *Service) AddAssignee(ctx context.Context, ticketID, userID int64) error {
	assignee := &ticket.Assignee{
		TicketID: ticketID,
		UserID:   userID,
	}
	return s.db.WithContext(ctx).Create(assignee).Error
}

// RemoveAssignee removes an assignee from a ticket
func (s *Service) RemoveAssignee(ctx context.Context, ticketID, userID int64) error {
	return s.db.WithContext(ctx).Where("ticket_id = ? AND user_id = ?", ticketID, userID).Delete(&ticket.Assignee{}).Error
}

// AddLabel adds a label to a ticket
func (s *Service) AddLabel(ctx context.Context, ticketID, labelID int64) error {
	ticketLabel := &ticket.TicketLabel{
		TicketID: ticketID,
		LabelID:  labelID,
	}
	return s.db.WithContext(ctx).Create(ticketLabel).Error
}

// RemoveLabel removes a label from a ticket
func (s *Service) RemoveLabel(ctx context.Context, ticketID, labelID int64) error {
	return s.db.WithContext(ctx).Where("ticket_id = ? AND label_id = ?", ticketID, labelID).Delete(&ticket.TicketLabel{}).Error
}

// Merge Request operations

// LinkMergeRequest links a merge request to a ticket
func (s *Service) LinkMergeRequest(ctx context.Context, orgID, ticketID int64, sessionID *int64, mrIID int, mrURL, sourceBranch, targetBranch, title, state string) (*ticket.MergeRequest, error) {
	mr := &ticket.MergeRequest{
		OrganizationID: orgID,
		TicketID:       ticketID,
		SessionID:      sessionID,
		MRIID:          mrIID,
		MRURL:          mrURL,
		SourceBranch:   sourceBranch,
		TargetBranch:   targetBranch,
		Title:          title,
		State:          state,
	}

	if err := s.db.WithContext(ctx).Create(mr).Error; err != nil {
		return nil, err
	}

	return mr, nil
}

// UpdateMergeRequestState updates a merge request state
func (s *Service) UpdateMergeRequestState(ctx context.Context, mrID int64, state string) error {
	return s.db.WithContext(ctx).Model(&ticket.MergeRequest{}).
		Where("id = ?", mrID).
		Update("state", state).Error
}

// GetMergeRequestByURL returns a merge request by URL
func (s *Service) GetMergeRequestByURL(ctx context.Context, mrURL string) (*ticket.MergeRequest, error) {
	var mr ticket.MergeRequest
	if err := s.db.WithContext(ctx).Where("mr_url = ?", mrURL).First(&mr).Error; err != nil {
		return nil, err
	}
	return &mr, nil
}

// ListMergeRequests returns merge requests for a ticket
func (s *Service) ListMergeRequests(ctx context.Context, ticketID int64) ([]*ticket.MergeRequest, error) {
	var mrs []*ticket.MergeRequest
	if err := s.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Find(&mrs).Error; err != nil {
		return nil, err
	}
	return mrs, nil
}

// GetTicketStats returns ticket statistics for a repository
func (s *Service) GetTicketStats(ctx context.Context, orgID int64, repoID *int64) (map[string]int64, error) {
	stats := make(map[string]int64)

	query := s.db.WithContext(ctx).Model(&ticket.Ticket{}).Where("organization_id = ?", orgID)
	if repoID != nil {
		query = query.Where("repository_id = ?", *repoID)
	}

	statuses := []string{
		ticket.TicketStatusBacklog,
		ticket.TicketStatusTodo,
		ticket.TicketStatusInProgress,
		ticket.TicketStatusInReview,
		ticket.TicketStatusDone,
		ticket.TicketStatusCancelled,
	}

	for _, status := range statuses {
		var count int64
		query.Where("status = ?", status).Count(&count)
		stats[status] = count
	}

	return stats, nil
}

// GetAssignees returns assignees for a ticket
func (s *Service) GetAssignees(ctx context.Context, ticketID int64) ([]*user.User, error) {
	var assignees []ticket.Assignee
	if err := s.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Find(&assignees).Error; err != nil {
		return nil, err
	}

	if len(assignees) == 0 {
		return []*user.User{}, nil
	}

	userIDs := make([]int64, len(assignees))
	for i, a := range assignees {
		userIDs[i] = a.UserID
	}

	var users []*user.User
	if err := s.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

// GetTicketLabels returns labels for a ticket
func (s *Service) GetTicketLabels(ctx context.Context, ticketID int64) ([]*ticket.Label, error) {
	var ticketLabels []ticket.TicketLabel
	if err := s.db.WithContext(ctx).Where("ticket_id = ?", ticketID).Find(&ticketLabels).Error; err != nil {
		return nil, err
	}

	if len(ticketLabels) == 0 {
		return []*ticket.Label{}, nil
	}

	labelIDs := make([]int64, len(ticketLabels))
	for i, tl := range ticketLabels {
		labelIDs[i] = tl.LabelID
	}

	var labels []*ticket.Label
	if err := s.db.WithContext(ctx).Where("id IN ?", labelIDs).Find(&labels).Error; err != nil {
		return nil, err
	}

	return labels, nil
}

// GetChildTickets returns child tickets for a parent ticket
func (s *Service) GetChildTickets(ctx context.Context, parentTicketID int64) ([]*ticket.Ticket, error) {
	var tickets []*ticket.Ticket
	if err := s.db.WithContext(ctx).
		Preload("Assignees").
		Preload("Labels").
		Where("parent_ticket_id = ?", parentTicketID).
		Order("created_at ASC").
		Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

// ========== Board View (Kanban) ==========

// GetBoard returns a kanban board view of tickets
func (s *Service) GetBoard(ctx context.Context, filter *ListTicketsFilter) (*ticket.Board, error) {
	// Define board columns (ordered)
	columnStatuses := []string{
		ticket.TicketStatusBacklog,
		ticket.TicketStatusTodo,
		ticket.TicketStatusInProgress,
		ticket.TicketStatusInReview,
		ticket.TicketStatusDone,
	}

	board := &ticket.Board{
		Columns: make([]ticket.BoardColumn, len(columnStatuses)),
	}

	for i, status := range columnStatuses {
		filter.Status = status
		tickets, count, err := s.ListTickets(ctx, filter)
		if err != nil {
			return nil, err
		}

		column := ticket.BoardColumn{
			Status:  status,
			Count:   int(count),
			Tickets: make([]ticket.Ticket, len(tickets)),
		}
		for j, t := range tickets {
			column.Tickets[j] = *t
		}
		board.Columns[i] = column
	}

	return board, nil
}

// GetActiveTickets returns active (non-completed) tickets
func (s *Service) GetActiveTickets(ctx context.Context, orgID int64, repoID *int64, limit int) ([]*ticket.Ticket, error) {
	query := s.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Where("status NOT IN ?", []string{ticket.TicketStatusDone, ticket.TicketStatusCancelled})

	if repoID != nil {
		query = query.Where("repository_id = ?", *repoID)
	}

	var tickets []*ticket.Ticket
	if err := query.
		Preload("Assignees").
		Preload("Labels").
		Order("updated_at DESC").
		Limit(limit).
		Find(&tickets).Error; err != nil {
		return nil, err
	}

	return tickets, nil
}

// GetSubTicketCounts returns sub-ticket counts for multiple parent tickets
func (s *Service) GetSubTicketCounts(ctx context.Context, parentTicketIDs []int64) (map[int64]map[string]int64, error) {
	type countResult struct {
		ParentTicketID int64
		Status         string
		Count          int64
	}

	var results []countResult
	if err := s.db.WithContext(ctx).
		Model(&ticket.Ticket{}).
		Select("parent_ticket_id, status, COUNT(*) as count").
		Where("parent_ticket_id IN ?", parentTicketIDs).
		Group("parent_ticket_id, status").
		Find(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[int64]map[string]int64)
	for _, r := range results {
		if counts[r.ParentTicketID] == nil {
			counts[r.ParentTicketID] = make(map[string]int64)
		}
		counts[r.ParentTicketID][r.Status] = r.Count
	}

	return counts, nil
}

// ========== Ticket Relations ==========

var (
	ErrRelationNotFound = errors.New("relation not found")
	ErrSelfRelation     = errors.New("cannot create relation to self")
)

// GetReverseRelationType returns the reverse relation type
func GetReverseRelationType(relationType string) string {
	switch relationType {
	case ticket.RelationTypeBlocks:
		return ticket.RelationTypeBlockedBy
	case ticket.RelationTypeBlockedBy:
		return ticket.RelationTypeBlocks
	case ticket.RelationTypeDuplicate:
		return ticket.RelationTypeDuplicate
	default:
		return ticket.RelationTypeRelates
	}
}

// CreateRelation creates a relation between two tickets
func (s *Service) CreateRelation(ctx context.Context, orgID, sourceTicketID, targetTicketID int64, relationType string) (*ticket.Relation, error) {
	if sourceTicketID == targetTicketID {
		return nil, ErrSelfRelation
	}

	var result *ticket.Relation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the primary relation
		relation := &ticket.Relation{
			OrganizationID: orgID,
			SourceTicketID: sourceTicketID,
			TargetTicketID: targetTicketID,
			RelationType:   relationType,
		}
		if err := tx.Create(relation).Error; err != nil {
			return err
		}
		result = relation

		// Create the reverse relation
		reverseType := GetReverseRelationType(relationType)
		reverseRelation := &ticket.Relation{
			OrganizationID: orgID,
			SourceTicketID: targetTicketID,
			TargetTicketID: sourceTicketID,
			RelationType:   reverseType,
		}
		if err := tx.Create(reverseRelation).Error; err != nil {
			return err
		}

		return nil
	})

	return result, err
}

// Transaction helper to return value
func (s *Service) createRelationTx(ctx context.Context, orgID, sourceTicketID, targetTicketID int64, relationType string) (*ticket.Relation, error) {
	var result *ticket.Relation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the primary relation
		relation := &ticket.Relation{
			OrganizationID: orgID,
			SourceTicketID: sourceTicketID,
			TargetTicketID: targetTicketID,
			RelationType:   relationType,
		}
		if err := tx.Create(relation).Error; err != nil {
			return err
		}

		// Create the reverse relation
		reverseType := GetReverseRelationType(relationType)
		reverseRelation := &ticket.Relation{
			OrganizationID: orgID,
			SourceTicketID: targetTicketID,
			TargetTicketID: sourceTicketID,
			RelationType:   reverseType,
		}
		if err := tx.Create(reverseRelation).Error; err != nil {
			return err
		}

		result = relation
		return nil
	})
	return result, err
}

// DeleteRelation deletes a relation and its reverse
func (s *Service) DeleteRelation(ctx context.Context, relationID int64) error {
	// Get the relation first
	var relation ticket.Relation
	if err := s.db.WithContext(ctx).First(&relation, relationID).Error; err != nil {
		return ErrRelationNotFound
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the relation
		if err := tx.Delete(&relation).Error; err != nil {
			return err
		}

		// Delete the reverse relation
		reverseType := GetReverseRelationType(relation.RelationType)
		return tx.Where(
			"source_ticket_id = ? AND target_ticket_id = ? AND relation_type = ?",
			relation.TargetTicketID, relation.SourceTicketID, reverseType,
		).Delete(&ticket.Relation{}).Error
	})
}

// ListRelations returns relations for a ticket
func (s *Service) ListRelations(ctx context.Context, ticketID int64) ([]*ticket.Relation, error) {
	var relations []*ticket.Relation
	if err := s.db.WithContext(ctx).
		Preload("TargetTicket").
		Where("source_ticket_id = ?", ticketID).
		Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

// ========== Ticket Commits ==========

var ErrCommitNotFound = errors.New("commit not found")

// LinkCommit links a git commit to a ticket
func (s *Service) LinkCommit(ctx context.Context, orgID, ticketID, repoID int64, sessionID *int64, commitSHA, commitMessage string, commitURL, authorName, authorEmail *string, committedAt *time.Time) (*ticket.Commit, error) {
	commit := &ticket.Commit{
		OrganizationID: orgID,
		TicketID:       ticketID,
		RepositoryID:   repoID,
		SessionID:      sessionID,
		CommitSHA:      commitSHA,
		CommitMessage:  commitMessage,
		CommitURL:      commitURL,
		AuthorName:     authorName,
		AuthorEmail:    authorEmail,
		CommittedAt:    committedAt,
	}

	if err := s.db.WithContext(ctx).Create(commit).Error; err != nil {
		return nil, err
	}

	return commit, nil
}

// UnlinkCommit removes a commit link from a ticket
func (s *Service) UnlinkCommit(ctx context.Context, commitID int64) error {
	return s.db.WithContext(ctx).Delete(&ticket.Commit{}, commitID).Error
}

// ListCommits returns commits for a ticket
func (s *Service) ListCommits(ctx context.Context, ticketID int64) ([]*ticket.Commit, error) {
	var commits []*ticket.Commit
	if err := s.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("committed_at DESC, created_at DESC").
		Find(&commits).Error; err != nil {
		return nil, err
	}
	return commits, nil
}

// GetCommitBySHA returns a commit by SHA
func (s *Service) GetCommitBySHA(ctx context.Context, repoID int64, commitSHA string) (*ticket.Commit, error) {
	var commit ticket.Commit
	if err := s.db.WithContext(ctx).
		Where("repository_id = ? AND commit_sha = ?", repoID, commitSHA).
		First(&commit).Error; err != nil {
		return nil, ErrCommitNotFound
	}
	return &commit, nil
}
