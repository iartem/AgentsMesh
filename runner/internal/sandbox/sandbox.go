package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Sandbox represents a session's isolated runtime environment.
type Sandbox struct {
	// Session identification
	SessionKey string `json:"session_key"`
	RootPath   string `json:"root_path"` // Sandbox root directory

	// Outputs filled by plugin chain
	WorkDir    string            `json:"work_dir"`    // Final working directory
	EnvVars    map[string]string `json:"env_vars"`    // Environment variables
	LaunchArgs []string          `json:"launch_args"` // Additional launch arguments (e.g., --mcp-config)

	// Metadata from plugins
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Internal state (not serialized)
	plugins []Plugin `json:"-"` // Applied plugins (for Teardown)
}

// NewSandbox creates a new Sandbox instance.
func NewSandbox(sessionKey, rootPath string) *Sandbox {
	now := time.Now()
	return &Sandbox{
		SessionKey: sessionKey,
		RootPath:   rootPath,
		EnvVars:    make(map[string]string),
		LaunchArgs: make([]string, 0),
		Metadata:   make(map[string]interface{}),
		CreatedAt:  now,
		UpdatedAt:  now,
		plugins:    make([]Plugin, 0),
	}
}

// AddPlugin records a plugin that was applied (for Teardown).
func (s *Sandbox) AddPlugin(p Plugin) {
	s.plugins = append(s.plugins, p)
}

// GetPlugins returns the list of applied plugins.
func (s *Sandbox) GetPlugins() []Plugin {
	return s.plugins
}

// Save persists the sandbox metadata to sandbox.json.
func (s *Sandbox) Save() error {
	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	metaPath := filepath.Join(s.RootPath, "sandbox.json")
	return os.WriteFile(metaPath, data, 0644)
}

// Load reads sandbox metadata from sandbox.json.
func (s *Sandbox) Load() error {
	metaPath := filepath.Join(s.RootPath, "sandbox.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, s)
}

// GetLogsDir returns the path to the logs directory.
func (s *Sandbox) GetLogsDir() string {
	return filepath.Join(s.RootPath, "logs")
}

// EnsureLogsDir creates the logs directory if it doesn't exist.
func (s *Sandbox) EnsureLogsDir() error {
	return os.MkdirAll(s.GetLogsDir(), 0755)
}
