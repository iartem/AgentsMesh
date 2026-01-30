package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/client"
	"github.com/anthropics/agentsmesh/runner/internal/config"
	"github.com/anthropics/agentsmesh/runner/internal/logger"
	"github.com/anthropics/agentsmesh/runner/internal/mcp"
	"github.com/anthropics/agentsmesh/runner/internal/monitor"
	"github.com/anthropics/agentsmesh/runner/internal/autopilot"
	"github.com/anthropics/agentsmesh/runner/internal/relay"
	"github.com/anthropics/agentsmesh/runner/internal/terminal"
	"github.com/anthropics/agentsmesh/runner/internal/workspace"
)

// Runner is the main runner instance
type Runner struct {
	cfg       *config.Config
	conn      client.Connection         // gRPC connection interface
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

	// Autopilot management
	autopilots      map[string]*autopilot.AutopilotController // autopilotKey -> AutopilotController
	autopilotsMu    sync.RWMutex

	// Update management
	draining       bool                   // True when waiting for pods to finish before update
	drainingMu     sync.RWMutex           // Protects draining flag

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
	SandboxPath      string
	Terminal         *terminal.Terminal
	VirtualTerminal  *terminal.VirtualTerminal  // Virtual terminal for state management and snapshots
	Aggregator       *terminal.SmartAggregator  // Output aggregator for adaptive frame rate
	RelayClient      *relay.Client              // WebSocket client for Relay connection
	relayMu          sync.RWMutex               // Protects RelayClient field
	StartedAt        time.Time
	Status           string              // Pod status - use statusMu for thread-safe access
	statusMu         sync.RWMutex        // Protects Status field
	TicketIdentifier string              // Ticket ID for worktree-based pods
	OnOutput         func([]byte)        // Output callback
	OnExit           func(int)           // Exit callback
	PTYLogger        *terminal.PTYLogger // PTY logger for debugging (optional)

	// StateDetector for multi-signal state detection (used by Autopilot)
	stateDetector    *multiSignalDetectorAdapter
	stateDetectorMu  sync.RWMutex

	// Token refresh channel - used when relay token expires and needs to be refreshed
	// RelayClient sends token request via gRPC, Backend responds with new SubscribeTerminalCommand
	// This channel delivers the new token to the waiting goroutine
	tokenRefreshCh   chan string
	tokenRefreshMu   sync.Mutex
}

// NewVirtualTerminal creates a new VirtualTerminal.
// This is a wrapper for terminal.NewVirtualTerminal to avoid importing terminal package in message_handler.
func NewVirtualTerminal(cols, rows, historyLimit int) *terminal.VirtualTerminal {
	return terminal.NewVirtualTerminal(cols, rows, historyLimit)
}

// SetStatus sets the pod status in a thread-safe manner
func (p *Pod) SetStatus(status string) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	p.Status = status
}

// GetStatus returns the pod status in a thread-safe manner
func (p *Pod) GetStatus() string {
	p.statusMu.RLock()
	defer p.statusMu.RUnlock()
	return p.Status
}

// SetRelayClient sets the relay client in a thread-safe manner
func (p *Pod) SetRelayClient(client *relay.Client) {
	p.relayMu.Lock()
	defer p.relayMu.Unlock()
	p.RelayClient = client
}

// GetRelayClient returns the relay client in a thread-safe manner
func (p *Pod) GetRelayClient() *relay.Client {
	p.relayMu.RLock()
	defer p.relayMu.RUnlock()
	return p.RelayClient
}

// HasRelayClient returns whether a relay client is connected
func (p *Pod) HasRelayClient() bool {
	p.relayMu.RLock()
	defer p.relayMu.RUnlock()
	return p.RelayClient != nil && p.RelayClient.IsConnected()
}

// DisconnectRelay disconnects and clears the relay client
func (p *Pod) DisconnectRelay() {
	p.relayMu.Lock()
	defer p.relayMu.Unlock()
	if p.RelayClient != nil {
		p.RelayClient.Stop()
		p.RelayClient = nil
	}
	// Clear aggregator relay output - will fall back to gRPC
	if p.Aggregator != nil {
		p.Aggregator.SetRelayOutput(nil)
	}
}

// GetOrCreateStateDetector returns the state detector for this pod, creating one if needed.
// This ensures the same detector instance is used throughout the pod's lifecycle.
func (p *Pod) GetOrCreateStateDetector() *multiSignalDetectorAdapter {
	p.stateDetectorMu.RLock()
	if p.stateDetector != nil {
		defer p.stateDetectorMu.RUnlock()
		return p.stateDetector
	}
	p.stateDetectorMu.RUnlock()

	// Need to create - acquire write lock
	p.stateDetectorMu.Lock()
	defer p.stateDetectorMu.Unlock()

	// Double check after acquiring write lock
	if p.stateDetector != nil {
		return p.stateDetector
	}

	// Create new detector if VirtualTerminal is available
	if p.VirtualTerminal != nil {
		p.stateDetector = newMultiSignalDetectorAdapter(p.VirtualTerminal)
	}
	return p.stateDetector
}

// NotifyStateDetectorOutput notifies the state detector about new output.
// Deprecated: Use NotifyStateDetectorWithScreen for single-direction data flow.
func (p *Pod) NotifyStateDetectorOutput(bytes int) {
	p.stateDetectorMu.RLock()
	detector := p.stateDetector
	p.stateDetectorMu.RUnlock()

	if detector != nil {
		detector.OnOutput(bytes)
	}
}

// NotifyStateDetectorWithScreen notifies the state detector about new output
// and provides the current screen lines for state analysis.
// This implements single-direction data flow: PTY → VirtualTerminal.Feed → StateDetector
// No reverse lock acquisition needed.
func (p *Pod) NotifyStateDetectorWithScreen(bytes int, screenLines []string) {
	p.stateDetectorMu.RLock()
	detector := p.stateDetector
	p.stateDetectorMu.RUnlock()

	if detector != nil {
		detector.OnOutput(bytes)
		if screenLines != nil {
			detector.OnScreenUpdate(screenLines)
		}
	}
}

// StopStateDetector stops the state detector if running.
func (p *Pod) StopStateDetector() {
	p.stateDetectorMu.Lock()
	defer p.stateDetectorMu.Unlock()

	if p.stateDetector != nil {
		p.stateDetector.Stop()
		p.stateDetector = nil
	}
}

// WaitForNewToken waits for a new token to be delivered via tokenRefreshCh.
// Returns the new token or empty string if timeout occurs.
// timeout is the maximum time to wait for the new token.
func (p *Pod) WaitForNewToken(timeout time.Duration) string {
	p.tokenRefreshMu.Lock()
	// Create channel if not exists
	if p.tokenRefreshCh == nil {
		p.tokenRefreshCh = make(chan string, 1)
	}
	ch := p.tokenRefreshCh
	p.tokenRefreshMu.Unlock()

	select {
	case token := <-ch:
		return token
	case <-time.After(timeout):
		return ""
	}
}

// DeliverNewToken delivers a new token to the waiting goroutine.
// This is called when Backend responds with a new SubscribeTerminalCommand.
func (p *Pod) DeliverNewToken(token string) {
	p.tokenRefreshMu.Lock()
	defer p.tokenRefreshMu.Unlock()

	// Create channel if not exists
	if p.tokenRefreshCh == nil {
		p.tokenRefreshCh = make(chan string, 1)
	}

	// Non-blocking send - if no one is waiting, the token is dropped
	select {
	case p.tokenRefreshCh <- token:
	default:
		// Channel full or no receiver - token is dropped
	}
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
	// Load gRPC config (certificates)
	if err := cfg.LoadGRPCConfig(); err != nil {
		return nil, fmt.Errorf("failed to load gRPC config: %w - please register the runner first using 'runner register'", err)
	}

	// Load org slug from file if not in config
	if err := cfg.LoadOrgSlug(); err != nil {
		logger.Runner().Warn("Failed to load org slug", "error", err)
	}

	// Validate required configuration
	if cfg.OrgSlug == "" {
		return nil, fmt.Errorf("org_slug is required - please re-register the runner")
	}

	if !cfg.UsesGRPC() {
		return nil, fmt.Errorf("gRPC configuration is required - please re-register the runner using 'runner register'")
	}

	// Create workspace manager
	ws, err := workspace.NewManager(cfg.WorkspaceRoot, cfg.GitConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace manager: %w", err)
	}

	// Create gRPC/mTLS connection
	logger.Runner().Info("Using gRPC/mTLS connection", "endpoint", cfg.GRPCEndpoint)

	grpcConn := client.NewGRPCConnection(
		cfg.GRPCEndpoint,
		cfg.NodeID,
		cfg.OrgSlug,
		cfg.CertFile,
		cfg.KeyFile,
		cfg.CAFile,
		client.WithGRPCServerURL(cfg.ServerURL), // For certificate renewal API calls
	)

	// Check certificate validity before connecting
	certInfo, err := grpcConn.GetCertificateExpiryInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to check certificate: %w", err)
	}

	if certInfo.IsExpired {
		return nil, fmt.Errorf("certificate has expired on %s. Please reactivate the runner using:\n  runner reactivate --token <token>\nGet a reactivation token from the web UI", certInfo.ExpiresAt.Format("2006-01-02"))
	}

	if certInfo.NeedsRenewal {
		logger.Runner().Warn("Certificate expires soon",
			"days_until_expiry", certInfo.DaysUntilExpiry,
			"expires_at", certInfo.ExpiresAt.Format("2006-01-02"))
	}

	// Create pod store
	podStore := NewInMemoryPodStore()

	r := &Runner{
		cfg:       cfg,
		conn:      grpcConn,
		workspace: ws,
		pods:      make(map[string]*Pod),
		podStore:  podStore,
		autopilots: make(map[string]*autopilot.AutopilotController),
		stopChan:  make(chan struct{}),
	}

	// Create message handler and set it on connection
	r.messageHandler = NewRunnerMessageHandler(r, podStore, grpcConn)
	grpcConn.SetHandler(r.messageHandler)

	// Initialize optional enhanced components
	r.initEnhancedComponents(cfg)

	return r, nil
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
	log := logger.Runner()

	// Initialize MCP manager (for legacy MCP config)
	r.mcpManager = mcp.NewManager()
	if cfg.MCPConfigPath != "" {
		if err := r.mcpManager.LoadConfig(cfg.MCPConfigPath); err != nil {
			log.Warn("Failed to load MCP config", "error", err)
		}
	}

	// Initialize and start MCP HTTP Server
	mcpPort := cfg.GetMCPPort()
	r.mcpServer = mcp.NewHTTPServer(cfg.ServerURL, mcpPort)
	// Set status provider so get_pod_status tool can query pod status
	r.mcpServer.SetStatusProvider(r)
	// Set terminal provider so terminal tools can access local pods directly
	// This is essential for Autopilot control process to interact with Pods
	r.mcpServer.SetTerminalProvider(r)
	go func() {
		log.Info("Starting MCP HTTP Server", "port", mcpPort)
		if err := r.mcpServer.Start(); err != nil {
			log.Warn("MCP HTTP Server failed", "error", err)
		}
	}()

	// Initialize and start Claude monitor for status tracking
	r.claudeMonitor = monitor.NewMonitor(5 * time.Second)
	r.claudeMonitor.Start()
}

// Run starts the runner and blocks until context is cancelled
func (r *Runner) Run(ctx context.Context) error {
	log := logger.Runner()
	log.Info("Runner starting", "node_id", r.cfg.NodeID, "org", r.cfg.OrgSlug)

	// Start connection (includes connect, heartbeat, reconnect loop)
	r.conn.Start()
	defer r.conn.Stop()

	// Wait for shutdown
	<-ctx.Done()
	log.Info("Shutting down runner...")

	// Stop all pods
	r.stopAllPods()

	return nil
}

// stopAllPods stops all active pods
func (r *Runner) stopAllPods() {
	pods := r.podStore.All()
	for _, pod := range pods {
		// 1. Disconnect Relay first
		pod.DisconnectRelay()

		// 2. Stop state detector if running
		pod.StopStateDetector()

		// 3. Stop aggregator to flush remaining output
		if pod.Aggregator != nil {
			pod.Aggregator.Stop()
		}

		// 4. Stop terminal
		if pod.Terminal != nil {
			pod.Terminal.Stop()
		}

		r.podStore.Delete(pod.PodKey)
	}
}

// IsDraining returns true if the runner is waiting for pods to finish before update.
func (r *Runner) IsDraining() bool {
	r.drainingMu.RLock()
	defer r.drainingMu.RUnlock()
	return r.draining
}

// SetDraining sets the draining state.
func (r *Runner) SetDraining(draining bool) {
	r.drainingMu.Lock()
	defer r.drainingMu.Unlock()
	r.draining = draining
	if draining {
		logger.Runner().Info("Entering draining mode - no new pods will be accepted")
	} else {
		logger.Runner().Info("Exiting draining mode - accepting pods again")
	}
}

// CanAcceptPod returns true if the runner can accept new pods.
// Returns false if draining or at max capacity.
func (r *Runner) CanAcceptPod() bool {
	r.drainingMu.RLock()
	draining := r.draining
	r.drainingMu.RUnlock()

	if draining {
		return false
	}

	activePods := r.GetActivePodCount()
	return activePods < r.cfg.MaxConcurrentPods
}

// GetActivePodCount returns the number of currently active pods.
func (r *Runner) GetActivePodCount() int {
	return r.podStore.Count()
}

// GetPodCounter returns a function that counts active pods.
// This is used by the updater for graceful updates.
func (r *Runner) GetPodCounter() func() int {
	return func() int {
		return r.GetActivePodCount()
	}
}

// Config returns the runner configuration.
func (r *Runner) Config() *config.Config {
	return r.cfg
}

// GetPodStatus returns the agent status for a given pod key.
// Implements mcp.PodStatusProvider interface.
// Returns: agentStatus (executing/waiting/not_running/unknown), podStatus, shellPid, found
func (r *Runner) GetPodStatus(podKey string) (agentStatus string, podStatus string, shellPid int, found bool) {
	// Get pod from store
	pod, exists := r.podStore.Get(podKey)
	if !exists || pod == nil {
		return "not_running", "not_found", 0, false
	}

	podStatus = pod.GetStatus()
	shellPid = 0
	if pod.Terminal != nil {
		shellPid = pod.Terminal.PID()
	}

	// Get agent status from Claude monitor
	if r.claudeMonitor != nil && shellPid > 0 {
		status, exists := r.claudeMonitor.GetStatus(podKey)
		if exists {
			agentStatus = string(status.ClaudeStatus)
			return agentStatus, podStatus, shellPid, true
		}
	}

	// If monitor doesn't have status, check if terminal is running
	if pod.Terminal != nil && !pod.Terminal.IsClosed() {
		agentStatus = "unknown"
	} else {
		agentStatus = "not_running"
	}

	return agentStatus, podStatus, shellPid, true
}

// GetClaudeMonitor returns the Claude process monitor.
func (r *Runner) GetClaudeMonitor() *monitor.Monitor {
	return r.claudeMonitor
}

// Autopilot management methods

// GetAutopilot returns an AutopilotController by key.
func (r *Runner) GetAutopilot(key string) *autopilot.AutopilotController {
	r.autopilotsMu.RLock()
	defer r.autopilotsMu.RUnlock()
	return r.autopilots[key]
}

// AddAutopilot adds an AutopilotController.
func (r *Runner) AddAutopilot(ac *autopilot.AutopilotController) {
	r.autopilotsMu.Lock()
	defer r.autopilotsMu.Unlock()
	r.autopilots[ac.Key()] = ac
}

// RemoveAutopilot removes an AutopilotController by key.
func (r *Runner) RemoveAutopilot(key string) {
	r.autopilotsMu.Lock()
	defer r.autopilotsMu.Unlock()
	delete(r.autopilots, key)
}

// GetAutopilotByPodKey returns an AutopilotController by its associated pod key.
func (r *Runner) GetAutopilotByPodKey(podKey string) *autopilot.AutopilotController {
	r.autopilotsMu.RLock()
	defer r.autopilotsMu.RUnlock()
	for _, ac := range r.autopilots {
		if ac.PodKey() == podKey {
			return ac
		}
	}
	return nil
}

// GetConnection returns the gRPC connection.
func (r *Runner) GetConnection() client.Connection {
	return r.conn
}

// LocalTerminalProvider implementation for MCP HTTP Server

// GetTerminalOutput returns the terminal output for a local pod.
// Implements mcp.LocalTerminalProvider interface.
func (r *Runner) GetTerminalOutput(podKey string, lines int) (string, error) {
	pod, exists := r.podStore.Get(podKey)
	if !exists || pod == nil {
		return "", fmt.Errorf("pod not found: %s", podKey)
	}

	if pod.VirtualTerminal == nil {
		return "", fmt.Errorf("virtual terminal not available for pod: %s", podKey)
	}

	// Get output from virtual terminal (includes scrollback and screen)
	output := pod.VirtualTerminal.GetOutput(lines)
	return output, nil
}

// SendTerminalText sends text to a local pod's terminal.
// Implements mcp.LocalTerminalProvider interface.
func (r *Runner) SendTerminalText(podKey string, text string) error {
	pod, exists := r.podStore.Get(podKey)
	if !exists || pod == nil {
		return fmt.Errorf("pod not found: %s", podKey)
	}

	if pod.Terminal == nil {
		return fmt.Errorf("terminal not available for pod: %s", podKey)
	}

	// Write text to terminal stdin
	err := pod.Terminal.Write([]byte(text))
	if err != nil {
		return fmt.Errorf("failed to write to terminal: %w", err)
	}

	logger.Runner().Debug("Sent text to local terminal", "pod_key", podKey, "text_length", len(text))
	return nil
}

// SendTerminalKey sends special keys to a local pod's terminal.
// Implements mcp.LocalTerminalProvider interface.
func (r *Runner) SendTerminalKey(podKey string, keys []string) error {
	pod, exists := r.podStore.Get(podKey)
	if !exists || pod == nil {
		return fmt.Errorf("pod not found: %s", podKey)
	}

	if pod.Terminal == nil {
		return fmt.Errorf("terminal not available for pod: %s", podKey)
	}

	// Map key names to escape sequences
	keyMap := map[string]string{
		"enter":     "\r",
		"escape":    "\x1b",
		"tab":       "\t",
		"backspace": "\x7f",
		"delete":    "\x1b[3~",
		"ctrl+c":    "\x03",
		"ctrl+d":    "\x04",
		"ctrl+u":    "\x15",
		"ctrl+l":    "\x0c",
		"ctrl+z":    "\x1a",
		"ctrl+a":    "\x01",
		"ctrl+e":    "\x05",
		"ctrl+k":    "\x0b",
		"ctrl+w":    "\x17",
		"up":        "\x1b[A",
		"down":      "\x1b[B",
		"right":     "\x1b[C",
		"left":      "\x1b[D",
		"home":      "\x1b[H",
		"end":       "\x1b[F",
		"pageup":    "\x1b[5~",
		"pagedown":  "\x1b[6~",
		"shift+tab": "\x1b[Z",
	}

	for _, key := range keys {
		seq, ok := keyMap[key]
		if !ok {
			return fmt.Errorf("unknown key: %s", key)
		}
		err := pod.Terminal.Write([]byte(seq))
		if err != nil {
			return fmt.Errorf("failed to send key %s: %w", key, err)
		}
	}

	logger.Runner().Debug("Sent keys to local terminal", "pod_key", podKey, "keys", keys)
	return nil
}
