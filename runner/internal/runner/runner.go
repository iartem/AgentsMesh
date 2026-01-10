package runner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/mcp"
	"github.com/anthropics/agentmesh/runner/internal/monitor"
	"github.com/anthropics/agentmesh/runner/internal/sandbox"
	"github.com/anthropics/agentmesh/runner/internal/sandbox/plugins"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// Runner is the main runner instance
type Runner struct {
	cfg       *config.Config
	conn      client.Connection         // New unified connection interface
	workspace *workspace.Manager
	pods      map[string]*Pod
	mu        sync.RWMutex

	// Pod management
	podStore       PodStore               // Pod state management
	messageHandler *RunnerMessageHandler  // Message handler implementing client.MessageHandler

	// Enhanced components
	mcpManager      *mcp.Manager          // MCP server management
	mcpServer       *mcp.HTTPServer       // MCP HTTP Server for Claude Code
	claudeMonitor   *monitor.Monitor      // Claude CLI status monitoring
	termManager     *terminal.Manager     // Enhanced terminal session management

	// Sandbox management
	sandboxManager *sandbox.Manager // Sandbox lifecycle management

	// Channels for coordination
	stopChan chan struct{}
}

// Pod represents an active terminal pod
type Pod struct {
	ID               string
	PodKey           string
	AgentType        string
	RepositoryURL    string
	Branch           string
	WorktreePath     string
	InitialPrompt    string
	Terminal         *terminal.Terminal
	StartedAt        time.Time
	Status           string
	TicketIdentifier string              // Ticket ID for worktree-based pods
	OnOutput         func([]byte)        // Output callback
	OnExit           func(int)           // Exit callback
	Forwarder        *PTYForwarder       // Output forwarder with backpressure
}

// PodStatus constants
const (
	PodStatusInitializing = "initializing"
	PodStatusRunning      = "running"
	PodStatusStopped      = "stopped"
	PodStatusFailed       = "failed"
)

// New creates a new runner instance
func New(cfg *config.Config) (*Runner, error) {
	// Load auth token from file if not in config
	if err := cfg.LoadAuthToken(); err != nil {
		log.Printf("Warning: failed to load auth token: %v", err)
	}

	// Load org slug from file if not in config
	if err := cfg.LoadOrgSlug(); err != nil {
		log.Printf("Warning: failed to load org slug: %v", err)
	}

	// Validate org slug is present
	if cfg.OrgSlug == "" {
		return nil, fmt.Errorf("org_slug is required - please re-register the runner")
	}

	// Create workspace manager
	ws, err := workspace.NewManager(cfg.WorkspaceRoot, cfg.GitConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	// Build WebSocket base URL from server URL (just convert http to ws, no path)
	wsURL := buildWebSocketBaseURL(cfg.ServerURL)

	// Create new ServerConnection with org slug
	conn := client.NewServerConnection(wsURL, cfg.NodeID, cfg.AuthToken, cfg.OrgSlug)

	// Create pod store
	podStore := NewInMemoryPodStore()

	r := &Runner{
		cfg:       cfg,
		conn:      conn,
		workspace: ws,
		pods:      make(map[string]*Pod),
		podStore:  podStore,
		stopChan:  make(chan struct{}),
	}

	// Create message handler and set it on connection
	r.messageHandler = NewRunnerMessageHandler(r, podStore, conn)
	conn.SetHandler(r.messageHandler)

	// Initialize optional enhanced components
	r.initEnhancedComponents(cfg)

	return r, nil
}

// buildWebSocketBaseURL converts HTTP URL to WebSocket base URL (no path).
// The ServerConnection will append the org-scoped path.
func buildWebSocketBaseURL(serverURL string) string {
	// Parse and convert http(s) to ws(s)
	if len(serverURL) > 5 && serverURL[:5] == "https" {
		return "wss" + serverURL[5:]
	}
	if len(serverURL) > 4 && serverURL[:4] == "http" {
		return "ws" + serverURL[4:]
	}
	return serverURL
}

// WithConnection sets a custom connection implementation (useful for testing).
func (r *Runner) WithConnection(conn client.Connection) *Runner {
	r.conn = conn
	// Re-create message handler with new connection
	r.messageHandler = NewRunnerMessageHandler(r, r.podStore, conn)
	conn.SetHandler(r.messageHandler)
	return r
}

// initEnhancedComponents initializes optional enhanced components based on config.
func (r *Runner) initEnhancedComponents(cfg *config.Config) {
	// Initialize MCP manager (for legacy MCP config)
	r.mcpManager = mcp.NewManager()
	if cfg.MCPConfigPath != "" {
		if err := r.mcpManager.LoadConfig(cfg.MCPConfigPath); err != nil {
			log.Printf("[runner] Warning: failed to load MCP config: %v", err)
		}
	}

	// Initialize and start MCP HTTP Server
	mcpPort := cfg.GetMCPPort()
	r.mcpServer = mcp.NewHTTPServer(cfg.ServerURL, mcpPort)
	go func() {
		log.Printf("[runner] Starting MCP HTTP Server on port %d", mcpPort)
		if err := r.mcpServer.Start(); err != nil {
			log.Printf("[runner] Warning: MCP HTTP Server failed: %v", err)
		}
	}()

	// Initialize Sandbox Manager with plugins
	r.sandboxManager = sandbox.NewManager(cfg.GetWorkspace(), mcpPort)
	r.registerSandboxPlugins(cfg)
	log.Printf("[runner] Sandbox manager initialized: workspace=%s", cfg.GetWorkspace())

	// Initialize Claude monitor for status tracking
	r.claudeMonitor = monitor.NewMonitor(5 * time.Second)

	// Initialize enhanced terminal manager
	defaultShell := cfg.DefaultShell
	if defaultShell == "" {
		defaultShell = "/bin/sh"
	}
	r.termManager = terminal.NewManager(defaultShell, cfg.WorkspaceRoot)
}

// registerSandboxPlugins registers all sandbox plugins in order.
func (r *Runner) registerSandboxPlugins(cfg *config.Config) {
	// Plugin order: Worktree(10) -> TempDir(20) -> InitScript(30) -> Env(40) -> MCP(50)
	r.sandboxManager.RegisterPlugin(plugins.NewWorktreePlugin(cfg.GetReposDir()))
	r.sandboxManager.RegisterPlugin(plugins.NewTempDirPlugin())
	r.sandboxManager.RegisterPlugin(plugins.NewInitScriptPlugin())
	r.sandboxManager.RegisterPlugin(plugins.NewEnvPlugin())
	r.sandboxManager.RegisterPlugin(plugins.NewMCPPlugin(cfg.GetMCPPort()))

	log.Printf("[runner] Registered 5 sandbox plugins")
}

// GetSandboxManager returns the sandbox manager.
func (r *Runner) GetSandboxManager() *sandbox.Manager {
	return r.sandboxManager
}

// Run starts the runner and blocks until context is cancelled
func (r *Runner) Run(ctx context.Context) error {
	log.Printf("Runner starting with node_id: %s (org: %s)", r.cfg.NodeID, r.cfg.OrgSlug)

	// Register with server if needed
	if r.cfg.AuthToken == "" {
		if r.cfg.RegistrationToken == "" {
			return fmt.Errorf("no auth_token or registration_token provided")
		}

		log.Println("Registering runner with server...")
		resp, err := r.register(ctx)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		// Update connection with new auth token and org slug
		r.conn.SetAuthToken(resp.AuthToken)
		r.conn.SetOrgSlug(resp.OrgSlug)
		r.cfg.AuthToken = resp.AuthToken
		r.cfg.OrgSlug = resp.OrgSlug

		if err := r.cfg.SaveAuthToken(resp.AuthToken); err != nil {
			log.Printf("Warning: failed to save auth token: %v", err)
		}
		if err := r.cfg.SaveOrgSlug(resp.OrgSlug); err != nil {
			log.Printf("Warning: failed to save org slug: %v", err)
		}

		log.Printf("Registration successful (org: %s)", resp.OrgSlug)
	}

	// Start connection (includes connect, heartbeat, reconnect loop)
	r.conn.Start()
	defer r.conn.Stop()

	// Wait for shutdown
	<-ctx.Done()
	log.Println("Shutting down runner...")

	// Stop all pods
	r.stopAllPods()

	return nil
}

// register registers this runner with the server
func (r *Runner) register(ctx context.Context) (*client.RegistrationResponse, error) {
	req := client.RegistrationRequest{
		ServerURL:         r.cfg.ServerURL,
		NodeID:            r.cfg.NodeID,
		RegistrationToken: r.cfg.RegistrationToken,
		Description:       r.cfg.Description,
		MaxPods:           r.cfg.MaxConcurrentPods,
	}
	return client.Register(ctx, req)
}

// PodStartPayload represents the payload for pod start
type PodStartPayload struct {
	PodKey           string            `json:"pod_key"`
	AgentType        string            `json:"agent_type"`
	LaunchCommand    string            `json:"launch_command"`
	LaunchArgs       []string          `json:"launch_args"`
	EnvVars          map[string]string `json:"env_vars"`
	RepositoryURL    string            `json:"repository_url"`
	Branch           string            `json:"branch"`
	InitialPrompt    string            `json:"initial_prompt"`
	Rows             int               `json:"rows"`
	Cols             int               `json:"cols"`
	TicketIdentifier string            `json:"ticket_identifier,omitempty"`
	PrepScript       string            `json:"prep_script,omitempty"`
	PrepTimeout      int               `json:"prep_timeout,omitempty"`

	// PluginConfig is a flexible JSON dict passed to sandbox plugins
	// Can include: repository_url, branch, ticket_identifier, init_script, env_vars, git_token, etc.
	PluginConfig map[string]interface{} `json:"plugin_config,omitempty"`
}

// ToPluginConfig converts PodStartPayload to a plugin config map.
// This merges explicit fields with any PluginConfig values.
func (p *PodStartPayload) ToPluginConfig() map[string]interface{} {
	config := make(map[string]interface{})

	// Copy explicit fields
	if p.RepositoryURL != "" {
		config["repository_url"] = p.RepositoryURL
	}
	if p.Branch != "" {
		config["branch"] = p.Branch
	}
	if p.TicketIdentifier != "" {
		config["ticket_identifier"] = p.TicketIdentifier
	}
	if p.PrepScript != "" {
		config["init_script"] = p.PrepScript
	}
	if p.PrepTimeout > 0 {
		config["init_timeout"] = p.PrepTimeout
	}
	if len(p.EnvVars) > 0 {
		envMap := make(map[string]interface{})
		for k, v := range p.EnvVars {
			envMap[k] = v
		}
		config["env_vars"] = envMap
	}

	// Merge PluginConfig (can override above values)
	for k, v := range p.PluginConfig {
		config[k] = v
	}

	return config
}

// PodStopPayload represents the payload for pod stop
type PodStopPayload struct {
	PodKey string `json:"pod_key"`
}

// TerminalInputPayload represents terminal input
type TerminalInputPayload struct {
	PodKey string `json:"pod_key"`
	Data   []byte `json:"data"`
}

// TerminalResizePayload represents terminal resize
type TerminalResizePayload struct {
	PodKey string `json:"pod_key"`
	Rows   int    `json:"rows"`
	Cols   int    `json:"cols"`
}

// stopAllPods stops all active pods
func (r *Runner) stopAllPods() {
	pods := r.podStore.All()
	for _, pod := range pods {
		if pod.Terminal != nil {
			pod.Terminal.Stop()
		}
		r.podStore.Delete(pod.PodKey)
	}
}
