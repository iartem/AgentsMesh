package agent

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ConfigValues represents dynamic configuration values (JSONB)
type ConfigValues map[string]interface{}

// Scan implements sql.Scanner for ConfigValues
func (cv *ConfigValues) Scan(value interface{}) error {
	if value == nil {
		*cv = make(ConfigValues)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}
	return json.Unmarshal(bytes, cv)
}

// Value implements driver.Valuer for ConfigValues
func (cv ConfigValues) Value() (driver.Value, error) {
	if cv == nil {
		return json.Marshal(make(map[string]interface{}))
	}
	return json.Marshal(cv)
}

// OrganizationAgentConfig represents organization-level default agent configuration
// Pod creation can override these defaults
type OrganizationAgentConfig struct {
	ID             int64 `gorm:"primaryKey" json:"id"`
	OrganizationID int64 `gorm:"not null;index" json:"organization_id"`
	AgentTypeID    int64 `gorm:"not null;index" json:"agent_type_id"`

	// Dynamic configuration values (JSON)
	ConfigValues ConfigValues `gorm:"type:jsonb;not null;default:'{}'" json:"config_values"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	AgentType *AgentType `gorm:"foreignKey:AgentTypeID" json:"agent_type,omitempty"`
}

func (OrganizationAgentConfig) TableName() string {
	return "organization_agent_configs"
}

// OrganizationAgentConfigResponse is the API response for organization agent config
type OrganizationAgentConfigResponse struct {
	ID             int64                  `json:"id"`
	OrganizationID int64                  `json:"organization_id"`
	AgentTypeID    int64                  `json:"agent_type_id"`
	AgentTypeName  string                 `json:"agent_type_name,omitempty"`
	AgentTypeSlug  string                 `json:"agent_type_slug,omitempty"`
	ConfigValues   map[string]interface{} `json:"config_values"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

// ToResponse converts OrganizationAgentConfig to API response
func (c *OrganizationAgentConfig) ToResponse() *OrganizationAgentConfigResponse {
	resp := &OrganizationAgentConfigResponse{
		ID:             c.ID,
		OrganizationID: c.OrganizationID,
		AgentTypeID:    c.AgentTypeID,
		ConfigValues:   c.ConfigValues,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.Format(time.RFC3339),
	}

	if c.AgentType != nil {
		resp.AgentTypeName = c.AgentType.Name
		resp.AgentTypeSlug = c.AgentType.Slug
	}

	return resp
}

// MergeConfigs merges multiple config maps with priority (later maps override earlier)
// Used for: system defaults -> organization defaults -> pod overrides
func MergeConfigs(configs ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, config := range configs {
		if config == nil {
			continue
		}
		for k, v := range config {
			result[k] = v
		}
	}

	return result
}
