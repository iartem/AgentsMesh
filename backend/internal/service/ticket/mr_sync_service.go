package ticket

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/session"
	"github.com/anthropics/agentmesh/backend/internal/domain/ticket"
	"github.com/anthropics/agentmesh/backend/internal/infra/git"
	"gorm.io/gorm"
)

var (
	ErrMRNotFound       = errors.New("merge request not found")
	ErrNoGitProvider    = errors.New("git provider not available")
	ErrNoRepositoryLink = errors.New("ticket has no repository linked")
)

// ticketIdentifierRegex matches patterns like "AM-123" in branch names
var ticketIdentifierRegex = regexp.MustCompile(`([A-Z]+-\d+)`)

// MRSyncService handles MR synchronization with git providers
type MRSyncService struct {
	db          *gorm.DB
	gitProvider git.Provider
}

// NewMRSyncService creates a new MR sync service
func NewMRSyncService(db *gorm.DB, gitProvider git.Provider) *MRSyncService {
	return &MRSyncService{
		db:          db,
		gitProvider: gitProvider,
	}
}

// MRData represents MR data from git provider
type MRData struct {
	IID            int
	WebURL         string
	Title          string
	SourceBranch   string
	TargetBranch   string
	State          string
	PipelineStatus *string
	PipelineID     *int64
	PipelineURL    *string
	MergeCommitSHA *string
	MergedAt       *time.Time
}

// FindOrCreateMR finds or creates an MR record from git provider data
func (s *MRSyncService) FindOrCreateMR(ctx context.Context, orgID int64, t *ticket.Ticket, mrData *MRData, sessionID *int64) (*ticket.MergeRequest, error) {
	if mrData.WebURL == "" {
		return nil, errors.New("MR data must contain web URL")
	}

	// Try to find existing MR by URL
	var existing ticket.MergeRequest
	err := s.db.WithContext(ctx).
		Where("mr_url = ?", mrData.WebURL).
		First(&existing).Error

	if err == nil {
		// Update existing record
		s.updateMRFromData(&existing, mrData)
		if sessionID != nil && existing.SessionID == nil {
			existing.SessionID = sessionID
		}
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create new record
	now := time.Now()
	mr := &ticket.MergeRequest{
		OrganizationID: orgID,
		TicketID:       t.ID,
		SessionID:      sessionID,
		MRIID:          mrData.IID,
		MRURL:          mrData.WebURL,
		SourceBranch:   mrData.SourceBranch,
		TargetBranch:   mrData.TargetBranch,
		Title:          mrData.Title,
		State:          mrData.State,
		PipelineStatus: mrData.PipelineStatus,
		PipelineID:     mrData.PipelineID,
		PipelineURL:    mrData.PipelineURL,
		MergeCommitSHA: mrData.MergeCommitSHA,
		MergedAt:       mrData.MergedAt,
		LastSyncedAt:   &now,
	}

	if err := s.db.WithContext(ctx).Create(mr).Error; err != nil {
		return nil, err
	}

	return mr, nil
}

// CheckSessionForNewMR checks if a session's branch has an MR
func (s *MRSyncService) CheckSessionForNewMR(ctx context.Context, sess *session.Session) (*ticket.MergeRequest, error) {
	if sess.BranchName == nil || sess.TicketID == nil {
		return nil, nil
	}

	if s.gitProvider == nil {
		return nil, ErrNoGitProvider
	}

	// Get ticket with repository
	var t ticket.Ticket
	if err := s.db.WithContext(ctx).
		Preload("Repository").
		First(&t, *sess.TicketID).Error; err != nil {
		return nil, err
	}

	if t.RepositoryID == nil {
		return nil, ErrNoRepositoryLink
	}

	// Get repository info
	var repo struct {
		ExternalID string
	}
	if err := s.db.WithContext(ctx).
		Table("repositories").
		Select("external_id").
		Where("id = ?", *t.RepositoryID).
		First(&repo).Error; err != nil {
		return nil, err
	}

	// Fetch MRs from git provider
	mrs, err := s.gitProvider.ListMergeRequestsByBranch(ctx, repo.ExternalID, *sess.BranchName, "all")
	if err != nil {
		return nil, err
	}

	if len(mrs) == 0 {
		return nil, nil
	}

	// Convert to MRData
	mr := mrs[0]
	mrData := s.buildMRData(mr)

	return s.FindOrCreateMR(ctx, sess.OrganizationID, &t, mrData, &sess.ID)
}

// BatchCheckSessions checks active sessions for new MRs
func (s *MRSyncService) BatchCheckSessions(ctx context.Context) ([]*ticket.MergeRequest, error) {
	if s.gitProvider == nil {
		return nil, ErrNoGitProvider
	}

	// Find sessions with branch but no MR record
	var sessions []*session.Session
	subquery := s.db.WithContext(ctx).
		Table("ticket_merge_requests").
		Select("session_id").
		Where("session_id IS NOT NULL")

	err := s.db.WithContext(ctx).
		Where("branch_name IS NOT NULL").
		Where("ticket_id IS NOT NULL").
		Where("id NOT IN (?)", subquery).
		Where("status IN ?", []string{
			session.SessionStatusRunning,
			session.SessionStatusDisconnected,
		}).
		Find(&sessions).Error

	if err != nil {
		return nil, err
	}

	var newMRs []*ticket.MergeRequest
	for _, sess := range sessions {
		mr, err := s.CheckSessionForNewMR(ctx, sess)
		if err != nil {
			continue // Log and continue
		}
		if mr != nil {
			newMRs = append(newMRs, mr)
		}
	}

	return newMRs, nil
}

// BatchSyncMRStatus syncs status for open MRs
func (s *MRSyncService) BatchSyncMRStatus(ctx context.Context) ([]*ticket.MergeRequest, error) {
	if s.gitProvider == nil {
		return nil, ErrNoGitProvider
	}

	// Find non-merged MRs
	var mrs []*ticket.MergeRequest
	err := s.db.WithContext(ctx).
		Preload("Ticket").
		Where("state != ?", ticket.MRStateMerged).
		Find(&mrs).Error

	if err != nil {
		return nil, err
	}

	var updated []*ticket.MergeRequest
	for _, mr := range mrs {
		if mr.Ticket == nil || mr.Ticket.RepositoryID == nil {
			continue
		}

		// Get repository info
		var repo struct {
			ExternalID string
		}
		if err := s.db.WithContext(ctx).
			Table("repositories").
			Select("external_id").
			Where("id = ?", *mr.Ticket.RepositoryID).
			First(&repo).Error; err != nil {
			continue
		}

		// Fetch MR from git provider
		mrInfo, err := s.gitProvider.GetMergeRequest(ctx, repo.ExternalID, mr.MRIID)
		if err != nil {
			continue
		}

		mrData := s.buildMRData(mrInfo)
		s.updateMRFromData(mr, mrData)

		if err := s.db.WithContext(ctx).Save(mr).Error; err != nil {
			continue
		}

		updated = append(updated, mr)
	}

	return updated, nil
}

// GetTicketMRs returns all MRs for a ticket
func (s *MRSyncService) GetTicketMRs(ctx context.Context, ticketID int64) ([]*ticket.MergeRequest, error) {
	var mrs []*ticket.MergeRequest
	if err := s.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&mrs).Error; err != nil {
		return nil, err
	}
	return mrs, nil
}

// GetSessionMRs returns all MRs for a session
func (s *MRSyncService) GetSessionMRs(ctx context.Context, sessionID int64) ([]*ticket.MergeRequest, error) {
	var mrs []*ticket.MergeRequest
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Find(&mrs).Error; err != nil {
		return nil, err
	}
	return mrs, nil
}

// FindTicketByBranch finds a ticket by branch name pattern
func (s *MRSyncService) FindTicketByBranch(ctx context.Context, branchName string) (*ticket.Ticket, error) {
	match := ticketIdentifierRegex.FindString(branchName)
	if match == "" {
		return nil, nil
	}

	var t ticket.Ticket
	if err := s.db.WithContext(ctx).
		Where("identifier = ?", match).
		First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &t, nil
}

// SyncMRByURL syncs a single MR by its URL
func (s *MRSyncService) SyncMRByURL(ctx context.Context, mrURL string) (*ticket.MergeRequest, error) {
	var mr ticket.MergeRequest
	if err := s.db.WithContext(ctx).
		Preload("Ticket").
		Where("mr_url = ?", mrURL).
		First(&mr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMRNotFound
		}
		return nil, err
	}

	if mr.Ticket == nil || mr.Ticket.RepositoryID == nil {
		return nil, ErrNoRepositoryLink
	}

	// Get repository info
	var repo struct {
		ExternalID string
	}
	if err := s.db.WithContext(ctx).
		Table("repositories").
		Select("external_id").
		Where("id = ?", *mr.Ticket.RepositoryID).
		First(&repo).Error; err != nil {
		return nil, err
	}

	// Fetch MR from git provider
	mrInfo, err := s.gitProvider.GetMergeRequest(ctx, repo.ExternalID, mr.MRIID)
	if err != nil {
		return nil, err
	}

	mrData := s.buildMRData(mrInfo)
	s.updateMRFromData(&mr, mrData)

	if err := s.db.WithContext(ctx).Save(&mr).Error; err != nil {
		return nil, err
	}

	return &mr, nil
}

// updateMRFromData updates MR record from provider data
func (s *MRSyncService) updateMRFromData(mr *ticket.MergeRequest, data *MRData) {
	mr.Title = data.Title
	mr.State = data.State
	mr.PipelineStatus = data.PipelineStatus
	mr.PipelineID = data.PipelineID
	mr.PipelineURL = data.PipelineURL
	mr.MergeCommitSHA = data.MergeCommitSHA
	mr.MergedAt = data.MergedAt
	now := time.Now()
	mr.LastSyncedAt = &now
}

// buildMRData converts git provider MR to MRData
func (s *MRSyncService) buildMRData(mr *git.MergeRequest) *MRData {
	data := &MRData{
		IID:          mr.IID,
		WebURL:       mr.WebURL,
		Title:        mr.Title,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
	}

	if mr.PipelineStatus != "" {
		data.PipelineStatus = &mr.PipelineStatus
	}
	if mr.PipelineID != 0 {
		id := int64(mr.PipelineID)
		data.PipelineID = &id
	}
	if mr.PipelineURL != "" {
		data.PipelineURL = &mr.PipelineURL
	}
	if mr.MergeCommitSHA != "" {
		data.MergeCommitSHA = &mr.MergeCommitSHA
	}
	if mr.MergedAt != nil {
		data.MergedAt = mr.MergedAt
	}

	return data
}
