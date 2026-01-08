package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/session"
	"github.com/anthropics/agentmesh/backend/internal/domain/ticket"
	"gorm.io/gorm"
)

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrNoAvailableRunner = errors.New("no available runner")
	ErrSessionTerminated = errors.New("session already terminated")
	ErrRunnerNotFound    = errors.New("runner not found")
	ErrRunnerOffline     = errors.New("runner is offline")
)

// Service handles session operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new session service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateSessionRequest represents a session creation request
type CreateSessionRequest struct {
	OrganizationID    int64
	TeamID            *int64
	RunnerID          int64
	AgentTypeID       *int64
	CustomAgentTypeID *int64
	RepositoryID      *int64
	TicketID          *int64
	CreatedByID       int64
	InitialPrompt     string
	BranchName        *string

	// Enhanced fields (from Mainline)
	Model             string                    // opus/sonnet/haiku
	PermissionMode    string                    // plan/default/bypassPermissions
	SkipPermissions   bool                      // Whether to skip permission checks
	ThinkLevel        string                    // ultrathink/megathink
	PreparationConfig *session.PreparationConfig // Preparation script config
	EnvVars           map[string]string          // Environment variables (AI credentials)
}

// CreateSession creates a new session
func (s *Service) CreateSession(ctx context.Context, req *CreateSessionRequest) (*session.Session, error) {
	// Generate session key with user and ticket context
	keyBytes := make([]byte, 4)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}
	randomSuffix := hex.EncodeToString(keyBytes)

	ticketPart := "standalone"
	if req.TicketID != nil {
		ticketPart = fmt.Sprintf("%d", *req.TicketID)
	}
	sessionKey := fmt.Sprintf("%d-%s-%s", req.CreatedByID, ticketPart, randomSuffix)

	// Set default values
	model := req.Model
	if model == "" {
		model = "opus"
	}
	permissionMode := req.PermissionMode
	if permissionMode == "" {
		permissionMode = session.PermissionModePlan
	}
	thinkLevel := req.ThinkLevel
	if thinkLevel == "" {
		thinkLevel = session.ThinkLevelUltrathink
	}

	sess := &session.Session{
		OrganizationID:    req.OrganizationID,
		TeamID:            req.TeamID,
		SessionKey:        sessionKey,
		RunnerID:          req.RunnerID,
		AgentTypeID:       req.AgentTypeID,
		CustomAgentTypeID: req.CustomAgentTypeID,
		RepositoryID:      req.RepositoryID,
		TicketID:          req.TicketID,
		CreatedByID:       req.CreatedByID,
		Status:            session.SessionStatusInitializing,
		AgentStatus:       session.AgentStatusUnknown,
		InitialPrompt:     req.InitialPrompt,
		BranchName:        req.BranchName,
		Model:             &model,
		PermissionMode:    &permissionMode,
		ThinkLevel:        &thinkLevel,
	}

	if err := s.db.WithContext(ctx).Create(sess).Error; err != nil {
		return nil, err
	}

	// Increment runner session count
	s.db.WithContext(ctx).Exec("UPDATE runners SET current_sessions = current_sessions + 1 WHERE id = ?", req.RunnerID)

	return sess, nil
}

// CreateSessionForTicket creates a session with ticket context
func (s *Service) CreateSessionForTicket(ctx context.Context, req *CreateSessionRequest) (*session.Session, error) {
	if req.TicketID == nil {
		return nil, errors.New("ticket_id is required")
	}

	// Get ticket for identifier and context
	var t ticket.Ticket
	if err := s.db.WithContext(ctx).First(&t, *req.TicketID).Error; err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// Build prompt with ticket context if not provided
	if req.InitialPrompt == "" {
		req.InitialPrompt = BuildTicketPrompt(&t)
	}

	return s.CreateSession(ctx, req)
}

// BuildTicketPrompt builds an initial prompt from ticket context
func BuildTicketPrompt(t *ticket.Ticket) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Working on ticket: %s", t.Identifier))
	parts = append(parts, fmt.Sprintf("Title: %s", t.Title))
	if t.Description != nil && *t.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", *t.Description))
	}
	return strings.Join(parts, "\n")
}

// BuildAgentCommand builds the agent startup command (e.g., claude command)
func BuildAgentCommand(model, permissionMode string, skipPermissions bool) string {
	cmdParts := []string{"claude"}
	if skipPermissions {
		cmdParts = append(cmdParts, "--dangerously-skip-permissions")
	}
	if permissionMode != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--permission-mode %s", permissionMode))
	}
	if model != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--model %s", model))
	}
	return strings.Join(cmdParts, " ")
}

// BuildInitialPrompt builds the initial prompt with think level appended
func BuildInitialPrompt(prompt, thinkLevel string) string {
	if thinkLevel != "" && thinkLevel != session.ThinkLevelNone {
		return fmt.Sprintf("%s\n\n%s", prompt, thinkLevel)
	}
	return prompt
}

// GetCreateSessionCommand returns the command to send to runner
func (s *Service) GetCreateSessionCommand(ctx context.Context, sess *session.Session, req *CreateSessionRequest) (*session.CreateSessionCommand, error) {
	model := "opus"
	if sess.Model != nil {
		model = *sess.Model
	}
	permissionMode := session.PermissionModePlan
	if sess.PermissionMode != nil {
		permissionMode = *sess.PermissionMode
	}
	thinkLevel := session.ThinkLevelUltrathink
	if sess.ThinkLevel != nil {
		thinkLevel = *sess.ThinkLevel
	}

	// Build command
	initialCommand := BuildAgentCommand(model, permissionMode, req.SkipPermissions)

	// Build prompt
	var formattedPrompt string
	if sess.InitialPrompt != "" {
		formattedPrompt = BuildInitialPrompt(sess.InitialPrompt, thinkLevel)
	}

	// Get ticket identifier for worktree
	var ticketIdentifier string
	if sess.TicketID != nil {
		var t ticket.Ticket
		if err := s.db.WithContext(ctx).First(&t, *sess.TicketID).Error; err == nil {
			ticketIdentifier = t.Identifier
		}
	}

	// Extract worktree suffix from session key
	parts := strings.Split(sess.SessionKey, "-")
	worktreeSuffix := parts[len(parts)-1]

	return &session.CreateSessionCommand{
		SessionKey:        sess.SessionKey,
		InitialCommand:    initialCommand,
		InitialPrompt:     formattedPrompt,
		PermissionMode:    permissionMode,
		TicketIdentifier:  ticketIdentifier,
		WorktreeSuffix:    worktreeSuffix,
		EnvVars:           req.EnvVars,
		PreparationConfig: req.PreparationConfig,
	}, nil
}

// GetSession returns a session by key
func (s *Service) GetSession(ctx context.Context, sessionKey string) (*session.Session, error) {
	var sess session.Session
	if err := s.db.WithContext(ctx).
		Preload("Runner").
		Preload("AgentType").
		Where("session_key = ?", sessionKey).
		First(&sess).Error; err != nil {
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// GetSessionByID returns a session by ID
func (s *Service) GetSessionByID(ctx context.Context, sessionID int64) (*session.Session, error) {
	var sess session.Session
	if err := s.db.WithContext(ctx).
		Preload("Runner").
		Preload("AgentType").
		First(&sess, sessionID).Error; err != nil {
		return nil, ErrSessionNotFound
	}
	return &sess, nil
}

// ListSessions returns sessions for an organization
func (s *Service) ListSessions(ctx context.Context, orgID int64, teamID *int64, status string, limit, offset int) ([]*session.Session, int64, error) {
	query := s.db.WithContext(ctx).Model(&session.Session{}).Where("organization_id = ?", orgID)

	if teamID != nil {
		query = query.Where("team_id = ?", *teamID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var sessions []*session.Session
	if err := query.
		Preload("Runner").
		Preload("AgentType").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

// ListActiveSessions returns active sessions for a runner
func (s *Service) ListActiveSessions(ctx context.Context, runnerID int64) ([]*session.Session, error) {
	var sessions []*session.Session
	if err := s.db.WithContext(ctx).
		Where("runner_id = ? AND status IN ?", runnerID, []string{
			session.SessionStatusInitializing,
			session.SessionStatusRunning,
			session.SessionStatusPaused,
			session.SessionStatusDisconnected,
		}).
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// UpdateSessionStatus updates session status
func (s *Service) UpdateSessionStatus(ctx context.Context, sessionKey, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == session.SessionStatusRunning {
		now := time.Now()
		updates["started_at"] = now
	} else if status == session.SessionStatusTerminated || status == session.SessionStatusOrphaned {
		now := time.Now()
		updates["finished_at"] = now
	}

	result := s.db.WithContext(ctx).Model(&session.Session{}).Where("session_key = ?", sessionKey).Updates(updates)
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	// If terminated, decrement runner session count
	if status == session.SessionStatusTerminated || status == session.SessionStatusOrphaned {
		var sess session.Session
		s.db.WithContext(ctx).Where("session_key = ?", sessionKey).First(&sess)
		s.db.WithContext(ctx).Exec("UPDATE runners SET current_sessions = GREATEST(current_sessions - 1, 0) WHERE id = ?", sess.RunnerID)
	}

	return nil
}

// UpdateAgentStatus updates agent status
func (s *Service) UpdateAgentStatus(ctx context.Context, sessionKey, agentStatus string, agentPID *int) error {
	now := time.Now()
	updates := map[string]interface{}{
		"agent_status":  agentStatus,
		"last_activity": now,
	}
	if agentPID != nil {
		updates["agent_pid"] = *agentPID
	}
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Updates(updates).Error
}

// UpdateSessionPTY updates session PTY PID
func (s *Service) UpdateSessionPTY(ctx context.Context, sessionKey string, ptyPID int) error {
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Update("pty_pid", ptyPID).Error
}

// UpdateWorktreePath updates session worktree path and branch
func (s *Service) UpdateWorktreePath(ctx context.Context, sessionKey, worktreePath, branchName string) error {
	updates := map[string]interface{}{
		"worktree_path": worktreePath,
	}
	if branchName != "" {
		updates["branch_name"] = branchName
	}
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Updates(updates).Error
}

// HandleSessionCreated handles the session_created event from runner
func (s *Service) HandleSessionCreated(ctx context.Context, sessionKey string, ptyPID int, worktreePath, branchName string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"pty_pid":       ptyPID,
		"status":        session.SessionStatusRunning,
		"started_at":    now,
		"last_activity": now,
	}
	if worktreePath != "" {
		updates["worktree_path"] = worktreePath
	}
	if branchName != "" {
		updates["branch_name"] = branchName
	}
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Updates(updates).Error
}

// HandleSessionTerminated handles the session_terminated event from runner
func (s *Service) HandleSessionTerminated(ctx context.Context, sessionKey string, exitCode *int) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Updates(map[string]interface{}{
			"status":      session.SessionStatusTerminated,
			"finished_at": now,
			"pty_pid":     nil,
		}).Error
}

// TerminateSession terminates a session
func (s *Service) TerminateSession(ctx context.Context, sessionKey string) error {
	sess, err := s.GetSession(ctx, sessionKey)
	if err != nil {
		return err
	}

	if !sess.IsActive() {
		return ErrSessionTerminated
	}

	return s.UpdateSessionStatus(ctx, sessionKey, session.SessionStatusTerminated)
}

// MarkDisconnected marks a session as disconnected (user closed browser)
func (s *Service) MarkDisconnected(ctx context.Context, sessionKey string) error {
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ? AND status = ?", sessionKey, session.SessionStatusRunning).
		Update("status", session.SessionStatusDisconnected).Error
}

// MarkReconnected marks a session as running again (user reconnected)
func (s *Service) MarkReconnected(ctx context.Context, sessionKey string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ? AND status = ?", sessionKey, session.SessionStatusDisconnected).
		Updates(map[string]interface{}{
			"status":        session.SessionStatusRunning,
			"last_activity": now,
		}).Error
}

// RecordActivity records session activity
func (s *Service) RecordActivity(ctx context.Context, sessionKey string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Update("last_activity", now).Error
}

// ReconcileSessions marks orphaned sessions that are not reported by runner
func (s *Service) ReconcileSessions(ctx context.Context, runnerID int64, reportedSessionKeys []string) error {
	// Get active sessions for this runner from database
	var dbSessions []*session.Session
	err := s.db.WithContext(ctx).
		Where("runner_id = ? AND status IN ?", runnerID, []string{
			session.SessionStatusRunning,
			session.SessionStatusInitializing,
		}).
		Find(&dbSessions).Error
	if err != nil {
		return err
	}

	// Create a set of reported session keys
	reportedSet := make(map[string]bool)
	for _, key := range reportedSessionKeys {
		reportedSet[key] = true
	}

	// Mark sessions not in heartbeat as orphaned
	now := time.Now()
	for _, sess := range dbSessions {
		if !reportedSet[sess.SessionKey] {
			s.db.WithContext(ctx).Model(sess).Updates(map[string]interface{}{
				"status":      session.SessionStatusOrphaned,
				"finished_at": now,
			})
		}
	}

	return nil
}

// GetSessionsByTicket returns sessions for a ticket
func (s *Service) GetSessionsByTicket(ctx context.Context, ticketID int64) ([]*session.Session, error) {
	var sessions []*session.Session
	if err := s.db.WithContext(ctx).
		Preload("Runner").
		Preload("AgentType").
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// CleanupStaleSessions marks stale sessions as terminated
func (s *Service) CleanupStaleSessions(ctx context.Context, maxIdleHours int) (int64, error) {
	threshold := time.Now().Add(-time.Duration(maxIdleHours) * time.Hour)
	now := time.Now()

	result := s.db.WithContext(ctx).Model(&session.Session{}).
		Where("status IN ? AND last_activity < ?", []string{
			session.SessionStatusDisconnected,
		}, threshold).
		Updates(map[string]interface{}{
			"status":      session.SessionStatusTerminated,
			"finished_at": now,
		})

	return result.RowsAffected, result.Error
}

// ListByRunner returns sessions for a runner with optional status filter
func (s *Service) ListByRunner(ctx context.Context, runnerID int64, status string) ([]*session.Session, error) {
	query := s.db.WithContext(ctx).Where("runner_id = ?", runnerID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var sessions []*session.Session
	if err := query.
		Preload("Runner").
		Preload("AgentType").
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// ListByTicket returns sessions for a ticket
func (s *Service) ListByTicket(ctx context.Context, ticketID int64) ([]*session.Session, error) {
	var sessions []*session.Session
	if err := s.db.WithContext(ctx).
		Preload("Runner").
		Preload("AgentType").
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// SessionUpdateFunc is a callback for session updates
type SessionUpdateFunc func(*session.Session)

// Subscribe subscribes to session updates and returns an unsubscribe function
func (s *Service) Subscribe(ctx context.Context, sessionKey string, callback SessionUpdateFunc) (func(), error) {
	// In a real implementation, this would use Redis pub/sub or similar
	// For now, return a simple unsubscribe function
	return func() {}, nil
}
