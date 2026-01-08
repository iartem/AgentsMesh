package binding

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/channel"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

var (
	ErrBindingNotFound      = errors.New("binding not found")
	ErrBindingExists        = errors.New("binding already exists")
	ErrSelfBinding          = errors.New("cannot bind a session to itself")
	ErrInvalidScope         = errors.New("invalid scope")
	ErrNotAuthorized        = errors.New("not authorized for this operation")
	ErrBindingNotPending    = errors.New("binding is not pending")
	ErrBindingNotActive     = errors.New("binding is not active")
	ErrNoValidPendingScopes = errors.New("no valid pending scopes to approve")
)

// Default expiry for pending bindings (24 hours)
const PendingExpiryHours = 24

// Service handles session binding operations
type Service struct {
	db             *gorm.DB
	sessionQuerier SessionQuerier
}

// SessionQuerier provides session information for policy evaluation
type SessionQuerier interface {
	GetSessionInfo(ctx context.Context, sessionKey string) (map[string]interface{}, error)
}

// NewService creates a new binding service
func NewService(db *gorm.DB, sessionQuerier SessionQuerier) *Service {
	return &Service{
		db:             db,
		sessionQuerier: sessionQuerier,
	}
}

// validateScopes validates that all scopes are valid
func (s *Service) validateScopes(scopes []string) error {
	for _, scope := range scopes {
		if !channel.ValidBindingScopes[scope] {
			return ErrInvalidScope
		}
	}
	return nil
}

// RequestBinding creates a binding request between two sessions
func (s *Service) RequestBinding(ctx context.Context, orgID int64, initiatorSession, targetSession string, scopes []string, policy string) (*channel.SessionBinding, error) {
	// Validate scopes
	if err := s.validateScopes(scopes); err != nil {
		return nil, err
	}

	// Prevent self-binding
	if initiatorSession == targetSession {
		return nil, ErrSelfBinding
	}

	// Check for existing binding (active or pending)
	existing, err := s.GetExistingBinding(ctx, initiatorSession, targetSession)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Evaluate policy to determine if auto-approve
	autoApprove, initialStatus := s.evaluatePolicy(ctx, initiatorSession, targetSession, policy)

	// Calculate expiry for pending bindings
	var expiresAt *time.Time
	if initialStatus == channel.BindingStatusPending {
		t := time.Now().Add(time.Duration(PendingExpiryHours) * time.Hour)
		expiresAt = &t
	}

	// Determine granted vs pending scopes based on policy
	var grantedScopes, pendingScopes []string
	if autoApprove {
		grantedScopes = scopes
		pendingScopes = []string{}
	} else {
		grantedScopes = []string{}
		pendingScopes = scopes
	}

	now := time.Now()
	binding := &channel.SessionBinding{
		OrganizationID:   orgID,
		InitiatorSession: initiatorSession,
		TargetSession:    targetSession,
		GrantedScopes:    pq.StringArray(grantedScopes),
		PendingScopes:    pq.StringArray(pendingScopes),
		Status:           initialStatus,
		RequestedAt:      &now,
		ExpiresAt:        expiresAt,
	}

	if autoApprove {
		binding.RespondedAt = &now
	}

	if err := s.db.WithContext(ctx).Create(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// RequestScopes requests additional scopes on an existing binding
func (s *Service) RequestScopes(ctx context.Context, bindingID int64, requesterSession string, scopes []string) (*channel.SessionBinding, error) {
	if err := s.validateScopes(scopes); err != nil {
		return nil, err
	}

	binding, err := s.GetBinding(ctx, bindingID)
	if err != nil {
		return nil, err
	}

	if binding.InitiatorSession != requesterSession {
		return nil, ErrNotAuthorized
	}

	if !binding.IsActive() {
		return nil, ErrBindingNotActive
	}

	// Filter out already granted or pending scopes
	var newScopes []string
	for _, scope := range scopes {
		if !binding.HasScope(scope) && !binding.HasPendingScope(scope) {
			newScopes = append(newScopes, scope)
		}
	}

	if len(newScopes) == 0 {
		return binding, nil // No new scopes to request
	}

	// Check if we can auto-approve
	autoApprove, _ := s.evaluatePolicy(ctx, binding.InitiatorSession, binding.TargetSession, "")

	if autoApprove {
		binding.GrantedScopes = append(binding.GrantedScopes, newScopes...)
	} else {
		binding.PendingScopes = append(binding.PendingScopes, newScopes...)
	}

	if err := s.db.WithContext(ctx).Save(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// ApproveScopes approves pending scope requests
func (s *Service) ApproveScopes(ctx context.Context, bindingID int64, approverSession string, scopes []string) (*channel.SessionBinding, error) {
	binding, err := s.GetBinding(ctx, bindingID)
	if err != nil {
		return nil, err
	}

	if binding.TargetSession != approverSession {
		return nil, ErrNotAuthorized
	}

	// Only approve scopes that are actually pending
	var approved []string
	for _, scope := range scopes {
		if binding.HasPendingScope(scope) {
			approved = append(approved, scope)
		}
	}

	if len(approved) == 0 {
		return nil, ErrNoValidPendingScopes
	}

	// Move from pending to granted
	newGranted := append([]string{}, binding.GrantedScopes...)
	var newPending []string
	for _, s := range binding.PendingScopes {
		isApproved := false
		for _, a := range approved {
			if s == a {
				isApproved = true
				break
			}
		}
		if isApproved {
			newGranted = append(newGranted, s)
		} else {
			newPending = append(newPending, s)
		}
	}

	binding.GrantedScopes = pq.StringArray(newGranted)
	binding.PendingScopes = pq.StringArray(newPending)

	if err := s.db.WithContext(ctx).Save(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// AcceptBinding accepts a pending binding request (moves all pending scopes to granted)
func (s *Service) AcceptBinding(ctx context.Context, bindingID int64, targetSession string) (*channel.SessionBinding, error) {
	binding, err := s.GetBinding(ctx, bindingID)
	if err != nil {
		return nil, err
	}

	if binding.TargetSession != targetSession {
		return nil, ErrNotAuthorized
	}

	if !binding.IsPending() {
		return nil, ErrBindingNotPending
	}

	// Move pending scopes to granted
	newGranted := append([]string{}, binding.GrantedScopes...)
	newGranted = append(newGranted, binding.PendingScopes...)

	now := time.Now()
	binding.GrantedScopes = pq.StringArray(newGranted)
	binding.PendingScopes = pq.StringArray([]string{})
	binding.Status = channel.BindingStatusActive
	binding.RespondedAt = &now
	binding.ExpiresAt = nil // Active bindings don't expire

	if err := s.db.WithContext(ctx).Save(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// RejectBinding rejects a pending binding request
func (s *Service) RejectBinding(ctx context.Context, bindingID int64, targetSession string, reason string) (*channel.SessionBinding, error) {
	binding, err := s.GetBinding(ctx, bindingID)
	if err != nil {
		return nil, err
	}

	if binding.TargetSession != targetSession {
		return nil, ErrNotAuthorized
	}

	if !binding.IsPending() {
		return nil, ErrBindingNotPending
	}

	now := time.Now()
	binding.Status = channel.BindingStatusRejected
	binding.RespondedAt = &now
	if reason != "" {
		binding.RejectionReason = &reason
	}

	if err := s.db.WithContext(ctx).Save(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// Unbind removes an active binding between two sessions
func (s *Service) Unbind(ctx context.Context, initiatorSession, targetSession string) (bool, error) {
	// Try both directions
	binding, err := s.GetActiveBinding(ctx, initiatorSession, targetSession)
	if err != nil {
		binding, err = s.GetActiveBinding(ctx, targetSession, initiatorSession)
		if err != nil {
			return false, nil
		}
	}

	binding.Status = channel.BindingStatusInactive
	if err := s.db.WithContext(ctx).Save(binding).Error; err != nil {
		return false, err
	}

	return true, nil
}

// CreateAutoBinding creates a binding that is immediately active without approval
func (s *Service) CreateAutoBinding(ctx context.Context, orgID int64, initiatorSession, targetSession string, scopes []string) (*channel.SessionBinding, error) {
	if err := s.validateScopes(scopes); err != nil {
		return nil, err
	}

	if initiatorSession == targetSession {
		return nil, ErrSelfBinding
	}

	// Check for existing binding
	existing, err := s.GetExistingBinding(ctx, initiatorSession, targetSession)
	if err == nil && existing != nil {
		return existing, nil
	}

	now := time.Now()
	binding := &channel.SessionBinding{
		OrganizationID:   orgID,
		InitiatorSession: initiatorSession,
		TargetSession:    targetSession,
		GrantedScopes:    pq.StringArray(scopes),
		PendingScopes:    pq.StringArray([]string{}),
		Status:           channel.BindingStatusActive,
		RequestedAt:      &now,
		RespondedAt:      &now,
		ExpiresAt:        nil, // Active bindings don't expire
	}

	if err := s.db.WithContext(ctx).Create(binding).Error; err != nil {
		return nil, err
	}

	return binding, nil
}

// GetBinding returns a binding by ID
func (s *Service) GetBinding(ctx context.Context, bindingID int64) (*channel.SessionBinding, error) {
	var binding channel.SessionBinding
	if err := s.db.WithContext(ctx).First(&binding, bindingID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBindingNotFound
		}
		return nil, err
	}
	return &binding, nil
}

// GetActiveBinding returns an active binding between two sessions
func (s *Service) GetActiveBinding(ctx context.Context, initiatorSession, targetSession string) (*channel.SessionBinding, error) {
	var binding channel.SessionBinding
	if err := s.db.WithContext(ctx).
		Where("initiator_session = ? AND target_session = ? AND status = ?",
			initiatorSession, targetSession, channel.BindingStatusActive).
		First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBindingNotFound
		}
		return nil, err
	}
	return &binding, nil
}

// GetExistingBinding returns any existing binding (active or pending) between two sessions
func (s *Service) GetExistingBinding(ctx context.Context, initiatorSession, targetSession string) (*channel.SessionBinding, error) {
	var binding channel.SessionBinding
	if err := s.db.WithContext(ctx).
		Where("initiator_session = ? AND target_session = ? AND status IN ?",
			initiatorSession, targetSession, []string{channel.BindingStatusActive, channel.BindingStatusPending}).
		First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBindingNotFound
		}
		return nil, err
	}
	return &binding, nil
}

// GetBindingsForSession returns all bindings for a session (as initiator or target)
func (s *Service) GetBindingsForSession(ctx context.Context, sessionKey string, status *string) ([]*channel.SessionBinding, error) {
	query := s.db.WithContext(ctx).
		Where("initiator_session = ? OR target_session = ?", sessionKey, sessionKey)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	var bindings []*channel.SessionBinding
	if err := query.Order("created_at DESC").Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// GetBoundSessions returns session keys that are bound to a session
func (s *Service) GetBoundSessions(ctx context.Context, sessionKey string) ([]string, error) {
	active := channel.BindingStatusActive
	bindings, err := s.GetBindingsForSession(ctx, sessionKey, &active)
	if err != nil {
		return nil, err
	}

	var boundSessions []string
	for _, binding := range bindings {
		if binding.InitiatorSession == sessionKey {
			boundSessions = append(boundSessions, binding.TargetSession)
		} else {
			boundSessions = append(boundSessions, binding.InitiatorSession)
		}
	}

	return boundSessions, nil
}

// IsBound checks if two sessions are bound
func (s *Service) IsBound(ctx context.Context, sessionA, sessionB string) (bool, error) {
	_, err := s.GetActiveBinding(ctx, sessionA, sessionB)
	if err == nil {
		return true, nil
	}

	_, err = s.GetActiveBinding(ctx, sessionB, sessionA)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, ErrBindingNotFound) {
		return false, nil
	}

	return false, err
}

// GetPendingRequests returns pending binding requests for a target session
func (s *Service) GetPendingRequests(ctx context.Context, targetSession string) ([]*channel.SessionBinding, error) {
	var bindings []*channel.SessionBinding
	if err := s.db.WithContext(ctx).
		Where("target_session = ? AND status = ?", targetSession, channel.BindingStatusPending).
		Order("created_at ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

// CleanupExpiredBindings marks expired pending bindings as expired
func (s *Service) CleanupExpiredBindings(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&channel.SessionBinding{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?",
			channel.BindingStatusPending, time.Now()).
		Update("status", channel.BindingStatusExpired)

	return result.RowsAffected, result.Error
}

// evaluatePolicy evaluates the binding policy to determine if auto-approve
func (s *Service) evaluatePolicy(ctx context.Context, initiatorSession, targetSession, policy string) (bool, string) {
	// If explicit policy is set, use it
	if policy == channel.BindingPolicyExplicitOnly {
		return false, channel.BindingStatusPending
	}

	// If no session querier available, require explicit confirmation
	if s.sessionQuerier == nil {
		return false, channel.BindingStatusPending
	}

	// Get session info
	initiatorInfo, err := s.sessionQuerier.GetSessionInfo(ctx, initiatorSession)
	if err != nil {
		return false, channel.BindingStatusPending
	}

	targetInfo, err := s.sessionQuerier.GetSessionInfo(ctx, targetSession)
	if err != nil {
		return false, channel.BindingStatusPending
	}

	// Same user - auto approve
	initiatorUserID, ok1 := initiatorInfo["user_id"]
	targetUserID, ok2 := targetInfo["user_id"]
	if ok1 && ok2 && initiatorUserID == targetUserID {
		return true, channel.BindingStatusActive
	}

	// Same project - check policy
	if policy == channel.BindingPolicySameProjectAuto {
		initiatorProjectID, ok1 := initiatorInfo["project_id"]
		targetProjectID, ok2 := targetInfo["project_id"]
		if ok1 && ok2 && initiatorProjectID == targetProjectID {
			return true, channel.BindingStatusActive
		}
	}

	// Default: require explicit confirmation
	return false, channel.BindingStatusPending
}

// HasScope checks if initiator has a specific scope on target
func (s *Service) HasScope(ctx context.Context, initiatorSession, targetSession, scope string) (bool, error) {
	binding, err := s.GetActiveBinding(ctx, initiatorSession, targetSession)
	if err != nil {
		if errors.Is(err, ErrBindingNotFound) {
			return false, nil
		}
		return false, err
	}
	return binding.HasScope(scope), nil
}
