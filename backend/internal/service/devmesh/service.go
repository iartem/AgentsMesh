package devmesh

import (
	"context"
	"errors"

	"github.com/anthropics/agentmesh/backend/internal/domain/channel"
	"github.com/anthropics/agentmesh/backend/internal/domain/devmesh"
	"github.com/anthropics/agentmesh/backend/internal/domain/session"
	bindingService "github.com/anthropics/agentmesh/backend/internal/service/binding"
	channelService "github.com/anthropics/agentmesh/backend/internal/service/channel"
	sessionService "github.com/anthropics/agentmesh/backend/internal/service/session"
	"gorm.io/gorm"
)

var (
	ErrTicketNotFound = errors.New("ticket not found")
	ErrRunnerNotFound = errors.New("runner not found")
)

// Service handles DevMesh operations
type Service struct {
	db             *gorm.DB
	sessionService *sessionService.Service
	channelService *channelService.Service
	bindingService *bindingService.Service
}

// NewService creates a new DevMesh service
func NewService(
	db *gorm.DB,
	ss *sessionService.Service,
	cs *channelService.Service,
	bs *bindingService.Service,
) *Service {
	return &Service{
		db:             db,
		sessionService: ss,
		channelService: cs,
		bindingService: bs,
	}
}

// GetTopology returns the complete DevMesh topology for an organization
func (s *Service) GetTopology(ctx context.Context, orgID int64, teamID *int64) (*devmesh.DevMeshTopology, error) {
	// 1. Get active sessions
	sessions, _, err := s.sessionService.ListSessions(ctx, orgID, teamID, "", 100, 0)
	if err != nil {
		return nil, err
	}

	// Filter to only active sessions and convert to nodes
	nodes := make([]devmesh.DevMeshNode, 0)
	sessionKeys := make([]string, 0)

	for _, sess := range sessions {
		if sess.IsActive() {
			node := s.sessionToNode(sess)
			nodes = append(nodes, node)
			sessionKeys = append(sessionKeys, sess.SessionKey)
		}
	}

	// 2. Get bindings (edges) for active sessions
	edges := make([]devmesh.DevMeshEdge, 0)
	for _, key := range sessionKeys {
		activeStatus := channel.BindingStatusActive
		bindings, err := s.bindingService.GetBindingsForSession(ctx, key, &activeStatus)
		if err != nil {
			continue
		}
		for _, b := range bindings {
			if b.IsActive() {
				edges = append(edges, devmesh.DevMeshEdge{
					ID:            b.ID,
					Source:        b.InitiatorSession,
					Target:        b.TargetSession,
					GrantedScopes: []string(b.GrantedScopes),
					PendingScopes: []string(b.PendingScopes),
					Status:        b.Status,
				})
			}
		}
	}

	// 3. Get channels
	channels, _, err := s.channelService.ListChannels(ctx, orgID, teamID, false, 50, 0)
	if err != nil {
		return nil, err
	}

	channelInfos := make([]devmesh.ChannelInfo, 0, len(channels))
	for _, ch := range channels {
		// Get sessions in this channel
		channelSessions := s.getChannelSessions(ctx, ch.ID)

		// Get message count
		messageCount := s.getChannelMessageCount(ctx, ch.ID)

		channelInfos = append(channelInfos, devmesh.ChannelInfo{
			ID:           ch.ID,
			Name:         ch.Name,
			Description:  ch.Description,
			SessionKeys:  channelSessions,
			MessageCount: messageCount,
			IsArchived:   ch.IsArchived,
		})
	}

	return &devmesh.DevMeshTopology{
		Nodes:    nodes,
		Edges:    edges,
		Channels: channelInfos,
	}, nil
}

// sessionToNode converts a session to a DevMesh node
func (s *Service) sessionToNode(sess *session.Session) devmesh.DevMeshNode {
	return devmesh.DevMeshNode{
		SessionKey:   sess.SessionKey,
		Status:       sess.Status,
		AgentStatus:  sess.AgentStatus,
		Model:        sess.Model,
		TicketID:     sess.TicketID,
		RepositoryID: sess.RepositoryID,
		CreatedByID:  sess.CreatedByID,
		RunnerID:     sess.RunnerID,
		StartedAt:    sess.StartedAt,
	}
}

// getChannelSessions returns session keys in a channel
func (s *Service) getChannelSessions(ctx context.Context, channelID int64) []string {
	var channelSessions []devmesh.ChannelSession
	s.db.WithContext(ctx).
		Where("channel_id = ?", channelID).
		Find(&channelSessions)

	keys := make([]string, len(channelSessions))
	for i, cs := range channelSessions {
		keys[i] = cs.SessionKey
	}
	return keys
}

// getChannelMessageCount returns the message count for a channel
func (s *Service) getChannelMessageCount(ctx context.Context, channelID int64) int {
	var count int64
	s.db.WithContext(ctx).
		Model(&channel.Message{}).
		Where("channel_id = ?", channelID).
		Count(&count)
	return int(count)
}

// CreateSessionForTicket creates a new session associated with a ticket
func (s *Service) CreateSessionForTicket(ctx context.Context, req *devmesh.CreateSessionForTicketRequest) (*session.Session, error) {
	return s.sessionService.CreateSessionForTicket(ctx, &sessionService.CreateSessionRequest{
		OrganizationID: req.OrganizationID,
		TeamID:         req.TeamID,
		RunnerID:       req.RunnerID,
		TicketID:       &req.TicketID,
		CreatedByID:    req.CreatedByID,
		InitialPrompt:  req.InitialPrompt,
		Model:          req.Model,
		PermissionMode: req.PermissionMode,
		ThinkLevel:     req.ThinkLevel,
	})
}

// GetSessionsForTicket returns all sessions associated with a ticket
func (s *Service) GetSessionsForTicket(ctx context.Context, ticketID int64) ([]devmesh.DevMeshNode, error) {
	sessions, err := s.sessionService.GetSessionsByTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	nodes := make([]devmesh.DevMeshNode, len(sessions))
	for i, sess := range sessions {
		nodes[i] = s.sessionToNode(sess)
	}
	return nodes, nil
}

// GetActiveSessionsForTicket returns only active sessions for a ticket
func (s *Service) GetActiveSessionsForTicket(ctx context.Context, ticketID int64) ([]devmesh.DevMeshNode, error) {
	sessions, err := s.sessionService.GetSessionsByTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	nodes := make([]devmesh.DevMeshNode, 0)
	for _, sess := range sessions {
		if sess.IsActive() {
			nodes = append(nodes, s.sessionToNode(sess))
		}
	}
	return nodes, nil
}

// BatchGetTicketSessions returns sessions for multiple tickets
func (s *Service) BatchGetTicketSessions(ctx context.Context, ticketIDs []int64) (*devmesh.BatchTicketSessionsResponse, error) {
	// Get all sessions for the given ticket IDs
	var sessions []*session.Session
	if err := s.db.WithContext(ctx).
		Where("ticket_id IN ?", ticketIDs).
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	// Group by ticket ID
	result := make(map[int64][]devmesh.DevMeshNode)
	for _, sess := range sessions {
		if sess.TicketID != nil {
			ticketID := *sess.TicketID
			if _, exists := result[ticketID]; !exists {
				result[ticketID] = make([]devmesh.DevMeshNode, 0)
			}
			result[ticketID] = append(result[ticketID], s.sessionToNode(sess))
		}
	}

	// Ensure all requested ticket IDs are in the result (even if empty)
	for _, id := range ticketIDs {
		if _, exists := result[id]; !exists {
			result[id] = make([]devmesh.DevMeshNode, 0)
		}
	}

	return &devmesh.BatchTicketSessionsResponse{
		TicketSessions: result,
	}, nil
}

// JoinChannel adds a session to a channel
func (s *Service) JoinChannel(ctx context.Context, channelID int64, sessionKey string) error {
	cs := &devmesh.ChannelSession{
		ChannelID:  channelID,
		SessionKey: sessionKey,
	}
	return s.db.WithContext(ctx).Create(cs).Error
}

// LeaveChannel removes a session from a channel
func (s *Service) LeaveChannel(ctx context.Context, channelID int64, sessionKey string) error {
	return s.db.WithContext(ctx).
		Where("channel_id = ? AND session_key = ?", channelID, sessionKey).
		Delete(&devmesh.ChannelSession{}).Error
}

// RecordChannelAccess records access to a channel
func (s *Service) RecordChannelAccess(ctx context.Context, channelID int64, sessionKey *string, userID *int64) error {
	access := &devmesh.ChannelAccess{
		ChannelID:  channelID,
		SessionKey: sessionKey,
		UserID:     userID,
	}
	return s.db.WithContext(ctx).Create(access).Error
}
