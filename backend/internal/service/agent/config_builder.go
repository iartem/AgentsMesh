package agent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/anthropics/agentmesh/backend/internal/domain/agent"
)

// PodConfig represents the computed configuration to send to Runner
type PodConfig struct {
	LaunchCommand string            `json:"launch_command"`
	LaunchArgs    []string          `json:"launch_args"`
	EnvVars       map[string]string `json:"env_vars"`
	FilesToCreate []FileToCreate    `json:"files_to_create"`
	WorkDirConfig WorkDirConfig     `json:"work_dir_config"`
	InitialPrompt string            `json:"initial_prompt,omitempty"`
}

// FileToCreate represents a file to be created in the sandbox
type FileToCreate struct {
	PathTemplate string `json:"path_template"`
	Content      string `json:"content"`
	Mode         int    `json:"mode,omitempty"`
	IsDirectory  bool   `json:"is_directory,omitempty"`
}

// WorkDirConfig represents the working directory configuration
type WorkDirConfig struct {
	Type          string `json:"type"` // "worktree", "tempdir", "local"
	RepositoryURL string `json:"repository_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	TicketID      string `json:"ticket_id,omitempty"`
	GitToken      string `json:"git_token,omitempty"`
	SSHKeyPath    string `json:"ssh_key_path,omitempty"`
	LocalPath     string `json:"local_path,omitempty"`
}

// ConfigBuildRequest contains all the information needed to build a pod config
type ConfigBuildRequest struct {
	AgentTypeID         int64
	OrganizationID      int64
	UserID              int64
	CredentialProfileID *int64

	// Repository info
	RepositoryURL string
	Branch        string
	TicketID      string
	GitToken      string
	SSHKeyPath    string
	LocalPath     string

	// User-provided config overrides
	ConfigOverrides map[string]interface{}

	// Initial prompt
	InitialPrompt string

	// Runtime info (provided by Runner during handshake)
	MCPPort int
	PodKey  string
}

// ConfigBuilder builds pod configurations from agent type templates
type ConfigBuilder struct {
	agentService *Service
}

// NewConfigBuilder creates a new ConfigBuilder
func NewConfigBuilder(agentService *Service) *ConfigBuilder {
	return &ConfigBuilder{
		agentService: agentService,
	}
}

// BuildPodConfig builds the complete pod configuration
func (b *ConfigBuilder) BuildPodConfig(ctx context.Context, req *ConfigBuildRequest) (*PodConfig, error) {
	// 1. Get agent type
	agentType, err := b.agentService.GetAgentType(ctx, req.AgentTypeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent type: %w", err)
	}

	// 2. Merge configs: org defaults + user overrides
	config := b.agentService.GetEffectiveConfig(ctx, req.OrganizationID, req.AgentTypeID, agent.ConfigValues(req.ConfigOverrides))

	// 3. Get credentials and inject as env vars
	envVars, err := b.buildEnvVars(ctx, req, agentType)
	if err != nil {
		return nil, fmt.Errorf("failed to build env vars: %w", err)
	}

	// 4. Build template context
	templateCtx := b.buildTemplateContext(req, config)

	// 5. Build launch args from CommandTemplate
	launchArgs, err := b.buildLaunchArgs(agentType.CommandTemplate, config, templateCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build launch args: %w", err)
	}

	// 6. Build files to create from FilesTemplate
	filesToCreate, err := b.buildFilesToCreate(agentType.FilesTemplate, config, templateCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build files to create: %w", err)
	}

	// 7. Build work dir config
	workDirConfig := b.buildWorkDirConfig(req)

	return &PodConfig{
		LaunchCommand: agentType.LaunchCommand,
		LaunchArgs:    launchArgs,
		EnvVars:       envVars,
		FilesToCreate: filesToCreate,
		WorkDirConfig: workDirConfig,
		InitialPrompt: req.InitialPrompt,
	}, nil
}

// buildEnvVars builds environment variables including credentials
func (b *ConfigBuilder) buildEnvVars(ctx context.Context, req *ConfigBuildRequest, agentType *agent.AgentType) (map[string]string, error) {
	envVars := make(map[string]string)

	// Get credentials from profile
	creds, isRunnerHost, err := b.agentService.GetEffectiveCredentialsForPod(ctx, req.UserID, req.AgentTypeID, req.CredentialProfileID)
	if err != nil {
		return nil, err
	}

	// If using RunnerHost mode, don't inject credentials
	if isRunnerHost {
		return envVars, nil
	}

	// Map credentials to env vars based on credential schema
	for _, field := range agentType.CredentialSchema {
		if value, ok := creds[field.Name]; ok && value != "" {
			envVars[field.EnvVar] = value
		}
	}

	return envVars, nil
}

// buildTemplateContext builds the context for template rendering
func (b *ConfigBuilder) buildTemplateContext(req *ConfigBuildRequest, config agent.ConfigValues) map[string]interface{} {
	return map[string]interface{}{
		"config": config,
		"sandbox": map[string]interface{}{
			"root_path": "{{.sandbox.root_path}}", // Placeholder, resolved by Runner
			"work_dir":  "{{.sandbox.work_dir}}",  // Placeholder, resolved by Runner
		},
		"mcp_port": req.MCPPort,
		"pod_key":  req.PodKey,
	}
}

// buildLaunchArgs builds launch arguments from CommandTemplate
func (b *ConfigBuilder) buildLaunchArgs(cmdTemplate agent.CommandTemplate, config agent.ConfigValues, templateCtx map[string]interface{}) ([]string, error) {
	var args []string

	for _, rule := range cmdTemplate.Args {
		// Check condition
		if rule.Condition != nil && !rule.Condition.Evaluate(config) {
			continue
		}

		// Render each arg template
		for _, argTemplate := range rule.Args {
			rendered, err := b.renderTemplate(argTemplate, templateCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to render arg template %q: %w", argTemplate, err)
			}
			if rendered != "" {
				args = append(args, rendered)
			}
		}
	}

	return args, nil
}

// buildFilesToCreate builds the list of files to create from FilesTemplate
func (b *ConfigBuilder) buildFilesToCreate(filesTemplate agent.FilesTemplate, config agent.ConfigValues, templateCtx map[string]interface{}) ([]FileToCreate, error) {
	var files []FileToCreate

	for _, ft := range filesTemplate {
		// Check condition
		if ft.Condition != nil && !ft.Condition.Evaluate(config) {
			continue
		}

		// For directories, just add the path
		if ft.IsDirectory {
			files = append(files, FileToCreate{
				PathTemplate: ft.PathTemplate,
				IsDirectory:  true,
			})
			continue
		}

		// Render content template
		content, err := b.renderTemplate(ft.ContentTemplate, templateCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render content template for %q: %w", ft.PathTemplate, err)
		}

		mode := ft.Mode
		if mode == 0 {
			mode = 0644 // Default permission
		}

		files = append(files, FileToCreate{
			PathTemplate: ft.PathTemplate,
			Content:      content,
			Mode:         mode,
		})
	}

	return files, nil
}

// buildWorkDirConfig builds the working directory configuration
func (b *ConfigBuilder) buildWorkDirConfig(req *ConfigBuildRequest) WorkDirConfig {
	// Determine work dir type
	workDirType := "tempdir"
	if req.RepositoryURL != "" {
		workDirType = "worktree"
	} else if req.LocalPath != "" {
		workDirType = "local"
	}

	return WorkDirConfig{
		Type:          workDirType,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		TicketID:      req.TicketID,
		GitToken:      req.GitToken,
		SSHKeyPath:    req.SSHKeyPath,
		LocalPath:     req.LocalPath,
	}
}

// renderTemplate renders a Go template string with the given context
func (b *ConfigBuilder) renderTemplate(templateStr string, ctx map[string]interface{}) (string, error) {
	// Skip if no template markers
	if !strings.Contains(templateStr, "{{") {
		return templateStr, nil
	}

	tmpl, err := template.New("").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetConfigSchemaWithI18n returns the config schema with translated labels
func (b *ConfigBuilder) GetConfigSchemaWithI18n(ctx context.Context, agentTypeID int64, locale string) (*TranslatedConfigSchema, error) {
	agentType, err := b.agentService.GetAgentType(ctx, agentTypeID)
	if err != nil {
		return nil, err
	}

	// TODO: Implement i18n translation using locale
	// For now, return the schema with keys as labels
	return b.translateConfigSchema(&agentType.ConfigSchema, locale), nil
}

// TranslatedConfigSchema is the config schema with translated labels
type TranslatedConfigSchema struct {
	Fields []TranslatedConfigField `json:"fields"`
}

// TranslatedConfigField is a config field with translated labels
type TranslatedConfigField struct {
	Name        string                    `json:"name"`
	Type        string                    `json:"type"`
	Default     interface{}               `json:"default,omitempty"`
	Required    bool                      `json:"required,omitempty"`
	Label       string                    `json:"label"`
	Description string                    `json:"description,omitempty"`
	Options     []TranslatedFieldOption   `json:"options,omitempty"`
	Validation  *agent.Validation         `json:"validation,omitempty"`
	ShowWhen    *agent.Condition          `json:"show_when,omitempty"`
}

// TranslatedFieldOption is a field option with translated label
type TranslatedFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// translateConfigSchema translates the config schema labels
func (b *ConfigBuilder) translateConfigSchema(schema *agent.ConfigSchema, locale string) *TranslatedConfigSchema {
	result := &TranslatedConfigSchema{
		Fields: make([]TranslatedConfigField, 0, len(schema.Fields)),
	}

	for _, field := range schema.Fields {
		translatedField := TranslatedConfigField{
			Name:       field.Name,
			Type:       field.Type,
			Default:    field.Default,
			Required:   field.Required,
			Label:      b.translate(field.LabelKey, locale),
			Description: b.translate(field.DescKey, locale),
			Validation: field.Validation,
			ShowWhen:   field.ShowWhen,
		}

		// Translate options
		if len(field.Options) > 0 {
			translatedField.Options = make([]TranslatedFieldOption, 0, len(field.Options))
			for _, opt := range field.Options {
				translatedField.Options = append(translatedField.Options, TranslatedFieldOption{
					Value: opt.Value,
					Label: b.translate(opt.LabelKey, locale),
				})
			}
		}

		result.Fields = append(result.Fields, translatedField)
	}

	return result
}

// translate translates an i18n key to the given locale
// TODO: Implement actual i18n translation
func (b *ConfigBuilder) translate(key string, locale string) string {
	// For now, just return the key as a placeholder
	// In the future, this should look up the translation from a translation service
	if key == "" {
		return ""
	}

	// Extract the last part of the key as a fallback label
	parts := strings.Split(key, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}
