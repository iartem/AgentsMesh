package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/session"
	"gorm.io/gorm"
)

// SessionCoordinator coordinates session lifecycle events between backend and runners
type SessionCoordinator struct {
	db                *gorm.DB
	connectionManager *ConnectionManager
	terminalRouter    *TerminalRouter
	logger            *slog.Logger

	// Callbacks
	onStatusChange func(sessionID string, status string, agentStatus string)
}

// NewSessionCoordinator creates a new session coordinator
func NewSessionCoordinator(
	db *gorm.DB,
	cm *ConnectionManager,
	tr *TerminalRouter,
	logger *slog.Logger,
) *SessionCoordinator {
	sc := &SessionCoordinator{
		db:                db,
		connectionManager: cm,
		terminalRouter:    tr,
		logger:            logger,
	}

	// Set up callbacks from connection manager
	cm.SetHeartbeatCallback(sc.handleHeartbeat)
	cm.SetSessionCreatedCallback(sc.handleSessionCreated)
	cm.SetSessionTerminatedCallback(sc.handleSessionTerminated)
	cm.SetAgentStatusCallback(sc.handleAgentStatus)
	cm.SetDisconnectCallback(sc.handleRunnerDisconnect)

	return sc
}

// SetStatusChangeCallback sets the callback for status changes
func (sc *SessionCoordinator) SetStatusChangeCallback(fn func(sessionID string, status string, agentStatus string)) {
	sc.onStatusChange = fn
}

// handleHeartbeat handles heartbeat from a runner
func (sc *SessionCoordinator) handleHeartbeat(runnerID int64, data *HeartbeatData) {
	ctx := context.Background()

	// Update runner
	now := time.Now()
	updates := map[string]interface{}{
		"last_heartbeat":   now,
		"current_sessions": len(data.Sessions),
		"status":           "online",
	}
	if data.RunnerVersion != "" {
		updates["runner_version"] = data.RunnerVersion
	}

	if err := sc.db.WithContext(ctx).
		Table("runners").
		Where("id = ?", runnerID).
		Updates(updates).Error; err != nil {
		sc.logger.Error("failed to update runner heartbeat",
			"runner_id", runnerID,
			"error", err)
		return
	}

	// Reconcile sessions
	reportedSessionIDs := make(map[string]bool)
	for _, s := range data.Sessions {
		reportedSessionIDs[s.SessionKey] = true
	}

	sc.reconcileSessions(ctx, runnerID, reportedSessionIDs)
}

// reconcileSessions syncs session status between runner heartbeat and database
func (sc *SessionCoordinator) reconcileSessions(ctx context.Context, runnerID int64, reportedSessions map[string]bool) {
	now := time.Now()

	// First, ensure all reported sessions are registered with terminal router
	// and restore any orphaned sessions that runner reports as active
	for sessionKey := range reportedSessions {
		// Always register session with terminal router (idempotent operation)
		// This ensures routing works even after backend restart
		sc.terminalRouter.RegisterSession(sessionKey, runnerID)

		// Try to restore if session is orphaned
		result := sc.db.WithContext(ctx).
			Model(&session.Session{}).
			Where("session_key = ? AND runner_id = ? AND status = ?", sessionKey, runnerID, session.StatusOrphaned).
			Updates(map[string]interface{}{
				"status":        session.StatusRunning,
				"finished_at":   nil,
				"last_activity": now,
			})
		if result.Error != nil {
			sc.logger.Error("failed to restore orphaned session",
				"session_key", sessionKey,
				"error", result.Error)
		} else if result.RowsAffected > 0 {
			sc.logger.Info("restored orphaned session reported by runner",
				"session_key", sessionKey,
				"runner_id", runnerID)
		}
	}

	// Get active sessions for this runner from database
	var sessions []session.Session
	if err := sc.db.WithContext(ctx).
		Where("runner_id = ? AND status IN ?", runnerID, []string{session.StatusRunning, session.StatusInitializing}).
		Find(&sessions).Error; err != nil {
		sc.logger.Error("failed to get sessions for reconciliation",
			"runner_id", runnerID,
			"error", err)
		return
	}

	// Mark sessions that are in DB but not reported by runner as orphaned
	for _, s := range sessions {
		if !reportedSessions[s.SessionKey] {
			if err := sc.db.WithContext(ctx).
				Model(&s).
				Updates(map[string]interface{}{
					"status":      session.StatusOrphaned,
					"finished_at": now,
				}).Error; err != nil {
				sc.logger.Error("failed to mark session as orphaned",
					"session_key", s.SessionKey,
					"error", err)
			} else {
				sc.logger.Warn("session marked as orphaned (not reported by runner)",
					"session_key", s.SessionKey,
					"runner_id", runnerID)
				// Unregister from terminal router
				sc.terminalRouter.UnregisterSession(s.SessionKey)
			}
		}
	}
}

// handleSessionCreated handles session creation event from runner
func (sc *SessionCoordinator) handleSessionCreated(runnerID int64, data *SessionCreatedData) {
	ctx := context.Background()

	now := time.Now()
	updates := map[string]interface{}{
		"pty_pid":       data.Pid,
		"status":        session.StatusRunning,
		"started_at":    now,
		"last_activity": now,
	}

	if data.BranchName != "" {
		updates["branch_name"] = data.BranchName
	}
	if data.WorktreePath != "" {
		updates["worktree_path"] = data.WorktreePath
	}

	if err := sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ?", data.SessionID).
		Updates(updates).Error; err != nil {
		sc.logger.Error("failed to update session on creation",
			"session_id", data.SessionID,
			"error", err)
		return
	}

	// Register with terminal router
	sc.terminalRouter.RegisterSession(data.SessionID, runnerID)

	sc.logger.Info("session created",
		"session_id", data.SessionID,
		"runner_id", runnerID,
		"pid", data.Pid,
		"branch", data.BranchName)

	// Notify status change
	if sc.onStatusChange != nil {
		sc.onStatusChange(data.SessionID, session.StatusRunning, "")
	}
}

// handleSessionTerminated handles session termination event from runner
func (sc *SessionCoordinator) handleSessionTerminated(runnerID int64, data *SessionTerminatedData) {
	ctx := context.Background()

	now := time.Now()
	if err := sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ?", data.SessionID).
		Updates(map[string]interface{}{
			"status":      session.StatusCompleted,
			"finished_at": now,
			"pty_pid":     nil,
		}).Error; err != nil {
		sc.logger.Error("failed to update session on termination",
			"session_id", data.SessionID,
			"error", err)
		return
	}

	// Decrement runner session count
	sc.db.WithContext(ctx).Exec(
		"UPDATE runners SET current_sessions = GREATEST(current_sessions - 1, 0) WHERE id = ?",
		runnerID,
	)

	// Unregister from terminal router
	sc.terminalRouter.UnregisterSession(data.SessionID)

	sc.logger.Info("session terminated",
		"session_id", data.SessionID,
		"runner_id", runnerID,
		"exit_code", data.ExitCode)

	// Notify status change
	if sc.onStatusChange != nil {
		sc.onStatusChange(data.SessionID, session.StatusCompleted, "")
	}
}

// handleAgentStatus handles agent status change from runner
func (sc *SessionCoordinator) handleAgentStatus(runnerID int64, data *AgentStatusData) {
	ctx := context.Background()

	updates := map[string]interface{}{
		"agent_status": data.Status,
	}
	if data.Pid > 0 {
		updates["pty_pid"] = data.Pid
	}

	if err := sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ?", data.SessionID).
		Updates(updates).Error; err != nil {
		sc.logger.Error("failed to update agent status",
			"session_id", data.SessionID,
			"error", err)
		return
	}

	sc.logger.Debug("agent status changed",
		"session_id", data.SessionID,
		"status", data.Status)

	// Notify status change
	if sc.onStatusChange != nil {
		sc.onStatusChange(data.SessionID, "", data.Status)
	}
}

// handleRunnerDisconnect handles runner disconnection
func (sc *SessionCoordinator) handleRunnerDisconnect(runnerID int64) {
	ctx := context.Background()

	// Mark runner as offline, but don't immediately orphan sessions
	// Sessions will be orphaned by reconcileSessions if runner doesn't reconnect
	// and report them in heartbeat
	if err := sc.db.WithContext(ctx).
		Table("runners").
		Where("id = ?", runnerID).
		Update("status", "offline").Error; err != nil {
		sc.logger.Error("failed to mark runner as offline",
			"runner_id", runnerID,
			"error", err)
	}

	sc.logger.Info("runner disconnected, sessions will be reconciled on reconnect",
		"runner_id", runnerID)

	// Note: We intentionally don't mark sessions as orphaned here
	// The runner might reconnect quickly (network glitch) and sessions are still running
	// Sessions will be properly reconciled when:
	// 1. Runner reconnects and sends heartbeat - reconcileSessions will handle it
	// 2. Session cleanup task runs and finds stale sessions
}

// IncrementSessions increments session count for a runner
func (sc *SessionCoordinator) IncrementSessions(ctx context.Context, runnerID int64) error {
	return sc.db.WithContext(ctx).Exec(
		"UPDATE runners SET current_sessions = current_sessions + 1 WHERE id = ?",
		runnerID,
	).Error
}

// DecrementSessions decrements session count for a runner
func (sc *SessionCoordinator) DecrementSessions(ctx context.Context, runnerID int64) error {
	return sc.db.WithContext(ctx).Exec(
		"UPDATE runners SET current_sessions = GREATEST(current_sessions - 1, 0) WHERE id = ?",
		runnerID,
	).Error
}

// CreateSession creates a new session on a runner
func (sc *SessionCoordinator) CreateSession(ctx context.Context, runnerID int64, req *CreateSessionRequest) error {
	// Increment session count first
	if err := sc.IncrementSessions(ctx, runnerID); err != nil {
		return err
	}

	// Register with terminal router
	sc.terminalRouter.RegisterSession(req.SessionID, runnerID)

	// Send create session request to runner
	return sc.connectionManager.SendCreateSession(ctx, runnerID, req)
}

// TerminateSession terminates a session on a runner
func (sc *SessionCoordinator) TerminateSession(ctx context.Context, sessionKey string) error {
	// Get session to find runner
	var sess session.Session
	if err := sc.db.WithContext(ctx).
		Where("session_key = ?", sessionKey).
		First(&sess).Error; err != nil {
		return err
	}

	// Send terminate request to runner
	if err := sc.connectionManager.SendTerminateSession(ctx, sess.RunnerID, sessionKey); err != nil {
		sc.logger.Warn("failed to send terminate to runner, marking as completed",
			"session_key", sessionKey,
			"error", err)
	}

	// Update session status
	now := time.Now()
	if err := sc.db.WithContext(ctx).
		Model(&sess).
		Updates(map[string]interface{}{
			"status":      session.StatusCompleted,
			"finished_at": now,
		}).Error; err != nil {
		return err
	}

	// Unregister from terminal router
	sc.terminalRouter.UnregisterSession(sessionKey)

	// Decrement session count
	return sc.DecrementSessions(ctx, sess.RunnerID)
}

// UpdateActivity updates last activity timestamp for a session
func (sc *SessionCoordinator) UpdateActivity(ctx context.Context, sessionKey string) error {
	return sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ?", sessionKey).
		Update("last_activity", time.Now()).Error
}

// MarkDisconnected marks a session as disconnected (user closed browser)
func (sc *SessionCoordinator) MarkDisconnected(ctx context.Context, sessionKey string) error {
	return sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ? AND status = ?", sessionKey, session.StatusRunning).
		Update("status", session.StatusDisconnected).Error
}

// MarkReconnected marks a session as running again (user reconnected)
func (sc *SessionCoordinator) MarkReconnected(ctx context.Context, sessionKey string) error {
	return sc.db.WithContext(ctx).
		Model(&session.Session{}).
		Where("session_key = ? AND status = ?", sessionKey, session.StatusDisconnected).
		Updates(map[string]interface{}{
			"status":        session.StatusRunning,
			"last_activity": time.Now(),
		}).Error
}
