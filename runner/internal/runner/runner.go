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
	"github.com/anthropics/agentmesh/runner/internal/terminal"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
	"github.com/anthropics/agentmesh/runner/internal/worktree"
)

// Runner is the main runner instance
type Runner struct {
	cfg       *config.Config
	conn      client.Connection         // New unified connection interface
	workspace *workspace.Manager
	sessions  map[string]*Session
	mu        sync.RWMutex

	// Session management
	sessionStore   SessionStore           // Session state management
	messageHandler *RunnerMessageHandler  // Message handler implementing client.MessageHandler

	// Enhanced components
	worktreeService *worktree.Service     // Git worktree management for ticket-based development
	mcpManager      *mcp.Manager          // MCP server management
	claudeMonitor   *monitor.Monitor      // Claude CLI status monitoring
	termManager     *terminal.Manager     // Enhanced terminal session management

	// Channels for coordination
	stopChan chan struct{}
}

// Session represents an active terminal session
type Session struct {
	ID               string
	SessionKey       string
	AgentType        string
	RepositoryURL    string
	Branch           string
	WorktreePath     string
	InitialPrompt    string
	Terminal         *terminal.Terminal
	StartedAt        time.Time
	Status           string
	TicketIdentifier string              // Ticket ID for worktree-based sessions
	OnOutput         func([]byte)        // Output callback
	OnExit           func(int)           // Exit callback
	Forwarder        *PTYForwarder       // Output forwarder with backpressure
}

// SessionStatus constants
const (
	SessionStatusInitializing = "initializing"
	SessionStatusRunning      = "running"
	SessionStatusStopped      = "stopped"
	SessionStatusFailed       = "failed"
)

// New creates a new runner instance
func New(cfg *config.Config) (*Runner, error) {
	// Load auth token from file if not in config
	if err := cfg.LoadAuthToken(); err != nil {
		log.Printf("Warning: failed to load auth token: %v", err)
	}

	// Create workspace manager
	ws, err := workspace.NewManager(cfg.WorkspaceRoot, cfg.GitConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	// Build WebSocket URL from server URL
	wsURL := buildWebSocketURL(cfg.ServerURL)

	// Create new ServerConnection
	conn := client.NewServerConnection(wsURL, cfg.NodeID, cfg.AuthToken)

	// Create session store
	sessionStore := NewInMemorySessionStore()

	r := &Runner{
		cfg:          cfg,
		conn:         conn,
		workspace:    ws,
		sessions:     make(map[string]*Session),
		sessionStore: sessionStore,
		stopChan:     make(chan struct{}),
	}

	// Create message handler and set it on connection
	r.messageHandler = NewRunnerMessageHandler(r, sessionStore, conn)
	conn.SetHandler(r.messageHandler)

	// Initialize optional enhanced components
	r.initEnhancedComponents(cfg)

	return r, nil
}

// buildWebSocketURL converts HTTP URL to WebSocket URL
func buildWebSocketURL(serverURL string) string {
	// Parse and convert http(s) to ws(s)
	if len(serverURL) > 5 && serverURL[:5] == "https" {
		return "wss" + serverURL[5:] + "/api/v1/runners/ws"
	}
	if len(serverURL) > 4 && serverURL[:4] == "http" {
		return "ws" + serverURL[4:] + "/api/v1/runners/ws"
	}
	return serverURL + "/api/v1/runners/ws"
}

// WithConnection sets a custom connection implementation (useful for testing).
func (r *Runner) WithConnection(conn client.Connection) *Runner {
	r.conn = conn
	// Re-create message handler with new connection
	r.messageHandler = NewRunnerMessageHandler(r, r.sessionStore, conn)
	conn.SetHandler(r.messageHandler)
	return r
}

// initEnhancedComponents initializes optional enhanced components based on config.
func (r *Runner) initEnhancedComponents(cfg *config.Config) {
	// Initialize worktree service if configured
	if cfg.RepositoryPath != "" && cfg.WorktreesDir != "" {
		r.worktreeService = worktree.New(cfg.RepositoryPath, cfg.WorktreesDir, cfg.BaseBranch)
		if r.worktreeService != nil {
			log.Printf("[runner] Worktree service initialized: repo=%s, worktrees=%s",
				cfg.RepositoryPath, cfg.WorktreesDir)
		}
	}

	// Initialize MCP manager
	r.mcpManager = mcp.NewManager()
	if cfg.MCPConfigPath != "" {
		if err := r.mcpManager.LoadConfig(cfg.MCPConfigPath); err != nil {
			log.Printf("[runner] Warning: failed to load MCP config: %v", err)
		}
	}

	// Initialize Claude monitor for status tracking
	r.claudeMonitor = monitor.NewMonitor(5 * time.Second)

	// Initialize enhanced terminal manager
	defaultShell := cfg.DefaultShell
	if defaultShell == "" {
		defaultShell = "/bin/sh"
	}
	r.termManager = terminal.NewManager(defaultShell, cfg.WorkspaceRoot)
}

// Run starts the runner and blocks until context is cancelled
func (r *Runner) Run(ctx context.Context) error {
	log.Printf("Runner starting with node_id: %s", r.cfg.NodeID)

	// Register with server if needed
	if r.cfg.AuthToken == "" {
		if r.cfg.RegistrationToken == "" {
			return fmt.Errorf("no auth_token or registration_token provided")
		}

		log.Println("Registering runner with server...")
		token, err := r.register(ctx)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		// Update connection with new auth token
		r.conn.SetAuthToken(token)
		r.cfg.AuthToken = token

		if err := r.cfg.SaveAuthToken(token); err != nil {
			log.Printf("Warning: failed to save auth token: %v", err)
		}

		log.Println("Registration successful")
	}

	// Start connection (includes connect, heartbeat, reconnect loop)
	r.conn.Start()
	defer r.conn.Stop()

	// Wait for shutdown
	<-ctx.Done()
	log.Println("Shutting down runner...")

	// Stop all sessions
	r.stopAllSessions()

	return nil
}

// register registers this runner with the server
func (r *Runner) register(ctx context.Context) (string, error) {
	req := client.RegistrationRequest{
		ServerURL:         r.cfg.ServerURL,
		NodeID:            r.cfg.NodeID,
		RegistrationToken: r.cfg.RegistrationToken,
		Description:       r.cfg.Description,
		MaxSessions:       r.cfg.MaxConcurrentSessions,
	}
	resp, err := client.Register(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.AuthToken, nil
}

// SessionStartPayload represents the payload for session start (legacy, kept for session_handler.go)
type SessionStartPayload struct {
	SessionKey       string            `json:"session_key"`
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
}

// SessionStopPayload represents the payload for session stop (legacy, kept for session_handler.go)
type SessionStopPayload struct {
	SessionKey string `json:"session_key"`
}

// TerminalInputPayload represents terminal input (legacy, kept for session_handler.go)
type TerminalInputPayload struct {
	SessionKey string `json:"session_key"`
	Data       []byte `json:"data"`
}

// TerminalResizePayload represents terminal resize (legacy, kept for session_handler.go)
type TerminalResizePayload struct {
	SessionKey string `json:"session_key"`
	Rows       int    `json:"rows"`
	Cols       int    `json:"cols"`
}

// stopAllSessions stops all active sessions
func (r *Runner) stopAllSessions() {
	sessions := r.sessionStore.All()
	for _, session := range sessions {
		if session.Terminal != nil {
			session.Terminal.Stop()
		}
		r.sessionStore.Delete(session.SessionKey)
	}
}
