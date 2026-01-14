package v1

import (
	"net/http"
	"strconv"

	agentDomain "github.com/anthropics/agentmesh/backend/internal/domain/agent"
	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/agent"
	"github.com/gin-gonic/gin"
)

// AgentHandler handles agent-related requests
type AgentHandler struct {
	agentService  *agent.Service
	configBuilder *agent.ConfigBuilder
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(agentService *agent.Service) *AgentHandler {
	return &AgentHandler{
		agentService:  agentService,
		configBuilder: agent.NewConfigBuilder(agentService),
	}
}

// ListAgentTypes lists available agent types
// GET /api/v1/organizations/:slug/agents/types
func (h *AgentHandler) ListAgentTypes(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	// Get builtin types
	builtinTypes, err := h.agentService.ListBuiltinAgentTypes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list builtin agent types"})
		return
	}

	// Get custom types for organization
	customTypes, err := h.agentService.ListCustomAgentTypes(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list custom agent types"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"builtin_types": builtinTypes,
		"custom_types":  customTypes,
	})
}

// GetOrganizationAgentConfig returns organization's agent configuration
// GET /api/v1/organizations/:slug/agents/config
func (h *AgentHandler) GetOrganizationAgentConfig(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	agents, err := h.agentService.ListOrganizationAgents(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agent configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// EnableAgentRequest represents agent enable request
type EnableAgentRequest struct {
	AgentTypeID int64 `json:"agent_type_id" binding:"required"`
	IsDefault   bool  `json:"is_default"`
}

// EnableAgent enables an agent type for organization
// POST /api/v1/organizations/:slug/agents/config
func (h *AgentHandler) EnableAgent(c *gin.Context) {
	var req EnableAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	orgAgent, err := h.agentService.EnableAgentForOrganization(c.Request.Context(), tenant.OrganizationID, req.AgentTypeID, req.IsDefault)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": orgAgent})
}

// DisableAgent disables an agent type for organization
// DELETE /api/v1/organizations/:slug/agents/config/:agent_type_id
func (h *AgentHandler) DisableAgent(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	if err := h.agentService.DisableAgentForOrganization(c.Request.Context(), tenant.OrganizationID, agentTypeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent disabled"})
}

// SetOrgCredentialsRequest represents organization credentials request
type SetOrgCredentialsRequest struct {
	Credentials map[string]interface{} `json:"credentials" binding:"required"`
}

// SetOrganizationCredentials sets organization-level credentials
// PUT /api/v1/organizations/:slug/agents/config/:agent_type_id/credentials
func (h *AgentHandler) SetOrganizationCredentials(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	var req SetOrgCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// Convert map[string]interface{} to EncryptedCredentials
	creds := make(agentDomain.EncryptedCredentials)
	for k, v := range req.Credentials {
		if s, ok := v.(string); ok {
			creds[k] = s
		}
	}

	if err := h.agentService.SetOrganizationCredentials(c.Request.Context(), tenant.OrganizationID, agentTypeID, creds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Credentials updated"})
}

// GetUserCredentials returns user's agent credentials
// GET /api/v1/users/me/agents/credentials
func (h *AgentHandler) GetUserCredentials(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Get builtin types to list credentials for
	builtinTypes, err := h.agentService.ListBuiltinAgentTypes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agent types"})
		return
	}

	credentials := make(map[string]bool)
	for _, t := range builtinTypes {
		creds, err := h.agentService.GetUserCredentials(c.Request.Context(), userID, t.ID)
		credentials[t.Slug] = err == nil && creds != nil
	}

	c.JSON(http.StatusOK, gin.H{"credentials": credentials})
}

// SetUserCredentialsRequest represents user credentials request
type SetUserCredentialsRequest struct {
	Credentials map[string]interface{} `json:"credentials" binding:"required"`
}

// SetUserCredentials sets user-level credentials
// PUT /api/v1/users/me/agents/credentials/:agent_type_id
func (h *AgentHandler) SetUserCredentials(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	var req SetUserCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	// Convert map[string]interface{} to EncryptedCredentials
	creds := make(agentDomain.EncryptedCredentials)
	for k, v := range req.Credentials {
		if s, ok := v.(string); ok {
			creds[k] = s
		}
	}

	if err := h.agentService.SetUserCredentials(c.Request.Context(), userID, agentTypeID, creds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Credentials updated"})
}

// DeleteUserCredentials deletes user-level credentials
// DELETE /api/v1/users/me/agents/credentials/:agent_type_id
func (h *AgentHandler) DeleteUserCredentials(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	userID := middleware.GetUserID(c)

	if err := h.agentService.DeleteUserCredentials(c.Request.Context(), userID, agentTypeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Credentials deleted"})
}

// CreateCustomAgentRequest represents custom agent creation request
type CreateCustomAgentRequest struct {
	Slug             string                 `json:"slug" binding:"required,min=2,max=50,alphanum"`
	Name             string                 `json:"name" binding:"required,min=2,max=100"`
	Description      string                 `json:"description"`
	LaunchCommand    string                 `json:"launch_command" binding:"required"`
	DefaultArgs      string                 `json:"default_args"`
	CredentialSchema map[string]interface{} `json:"credential_schema"`
	StatusDetection  map[string]interface{} `json:"status_detection"`
}

// CreateCustomAgent creates a custom agent type
// POST /api/v1/organizations/:slug/agents/custom
func (h *AgentHandler) CreateCustomAgent(c *gin.Context) {
	var req CreateCustomAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// Convert request to service request
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}
	var args *string
	if req.DefaultArgs != "" {
		args = &req.DefaultArgs
	}

	// Convert credential schema
	var credSchema agentDomain.CredentialSchema
	if req.CredentialSchema != nil {
		// TODO: properly convert credential schema from map to CredentialSchema
	}

	// Convert status detection
	var statusDetection agentDomain.StatusDetection
	if req.StatusDetection != nil {
		statusDetection = make(agentDomain.StatusDetection)
		for k, v := range req.StatusDetection {
			statusDetection[k] = v
		}
	}

	customAgent, err := h.agentService.CreateCustomAgentType(c.Request.Context(), tenant.OrganizationID, &agent.CreateCustomAgentRequest{
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      desc,
		LaunchCommand:    req.LaunchCommand,
		DefaultArgs:      args,
		CredentialSchema: credSchema,
		StatusDetection:  statusDetection,
	})
	if err != nil {
		if err == agent.ErrAgentSlugExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Agent slug already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create custom agent"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"custom_agent": customAgent})
}

// UpdateCustomAgent updates a custom agent type
// PUT /api/v1/organizations/:slug/agents/custom/:id
func (h *AgentHandler) UpdateCustomAgent(c *gin.Context) {
	customAgentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid custom agent ID"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	customAgent, err := h.agentService.UpdateCustomAgentType(c.Request.Context(), customAgentID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update custom agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"custom_agent": customAgent})
}

// DeleteCustomAgent deletes a custom agent type
// DELETE /api/v1/organizations/:slug/agents/custom/:id
func (h *AgentHandler) DeleteCustomAgent(c *gin.Context) {
	customAgentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid custom agent ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	if err := h.agentService.DeleteCustomAgentType(c.Request.Context(), customAgentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete custom agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Custom agent deleted"})
}

// Organization Default Config API

// ListDefaultConfigs returns all default configs for the organization
// GET /api/v1/organizations/:slug/agents/default-configs
func (h *AgentHandler) ListDefaultConfigs(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	configs, err := h.agentService.ListOrganizationDefaultConfigs(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list default configs"})
		return
	}

	// Convert to response format
	responses := make([]*agentDomain.OrganizationAgentConfigResponse, len(configs))
	for i, cfg := range configs {
		responses[i] = cfg.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{"configs": responses})
}

// GetDefaultConfig returns the organization's default config for an agent type
// GET /api/v1/organizations/:slug/agents/:agent_type_id/default-config
func (h *AgentHandler) GetDefaultConfig(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	config, err := h.agentService.GetOrganizationDefaultConfig(c.Request.Context(), tenant.OrganizationID, agentTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get default config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config.ToResponse()})
}

// SetDefaultConfigRequest represents a request to set default config
type SetDefaultConfigRequest struct {
	ConfigValues map[string]interface{} `json:"config_values" binding:"required"`
}

// SetDefaultConfig sets the organization's default config for an agent type
// PUT /api/v1/organizations/:slug/agents/:agent_type_id/default-config
func (h *AgentHandler) SetDefaultConfig(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	var req SetDefaultConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// Convert to ConfigValues
	configValues := make(agentDomain.ConfigValues)
	for k, v := range req.ConfigValues {
		configValues[k] = v
	}

	config, err := h.agentService.SetOrganizationDefaultConfig(c.Request.Context(), tenant.OrganizationID, agentTypeID, configValues)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config.ToResponse()})
}

// DeleteDefaultConfig deletes the organization's default config for an agent type
// DELETE /api/v1/organizations/:slug/agents/:agent_type_id/default-config
func (h *AgentHandler) DeleteDefaultConfig(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	if err := h.agentService.DeleteOrganizationDefaultConfig(c.Request.Context(), tenant.OrganizationID, agentTypeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete default config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default config deleted"})
}

// GetAgentTypeConfigSchema returns the config schema for an agent type with i18n support
// GET /api/v1/organizations/:slug/agents/:agent_type_id/config-schema
func (h *AgentHandler) GetAgentTypeConfigSchema(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	// Get locale from query param or Accept-Language header
	locale := c.Query("locale")
	if locale == "" {
		locale = c.GetHeader("Accept-Language")
	}
	if locale == "" {
		locale = "en"
	}

	schema, err := h.configBuilder.GetConfigSchemaWithI18n(c.Request.Context(), agentTypeID, locale)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config schema"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"schema": schema})
}

// GetAgentType returns details of a specific agent type
// GET /api/v1/organizations/:slug/agents/types/:agent_type_id
func (h *AgentHandler) GetAgentType(c *gin.Context) {
	agentTypeID, err := strconv.ParseInt(c.Param("agent_type_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent type ID"})
		return
	}

	agentType, err := h.agentService.GetAgentType(c.Request.Context(), agentTypeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent type not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent_type": agentType})
}
