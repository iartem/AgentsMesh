package runner

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ========== Connection Types ==========

// RunnerConnection represents an active connection to a runner
type RunnerConnection struct {
	RunnerID int64
	Conn     *websocket.Conn
	Send     chan []byte
	LastPing time.Time
	mu       sync.Mutex

	// Close safety
	closeOnce sync.Once

	// 初始化状态
	initialized     bool     // 是否完成初始化
	availableAgents []string // Runner 可用的 Agent slug 列表

	// 连接时间，用于初始化超时检查
	ConnectedAt time.Time

	// Ping interval for WritePump (configurable)
	PingInterval time.Duration
}

// IsInitialized returns whether the connection has completed initialization.
// Thread-safe.
func (rc *RunnerConnection) IsInitialized() bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.initialized
}

// SetInitialized sets the initialization state and available agents.
// Thread-safe.
func (rc *RunnerConnection) SetInitialized(initialized bool, availableAgents []string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.initialized = initialized
	rc.availableAgents = availableAgents
}

// GetAvailableAgents returns the list of available agents.
// Thread-safe.
func (rc *RunnerConnection) GetAvailableAgents() []string {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.availableAgents
}

// ========== Message Types ==========

// Runner message types
const (
	// ==================== 初始化流程 (三阶段握手) ====================
	// Runner -> Backend: 初始化请求
	MsgTypeInitialize = "initialize"
	// Backend -> Runner: 初始化响应
	MsgTypeInitializeResult = "initialize_result"
	// Runner -> Backend: 初始化完成确认
	MsgTypeInitialized = "initialized"

	// ==================== 运行时消息: Runner -> Backend ====================
	MsgTypeHeartbeat      = "heartbeat"
	MsgTypePodCreated     = "pod_created"
	MsgTypePodTerminated  = "pod_terminated"
	MsgTypeTerminalOutput = "terminal_output"
	MsgTypeAgentStatus    = "agent_status"
	MsgTypePtyResized     = "pty_resized"
	MsgTypeError          = "error"

	// ==================== 运行时消息: Backend -> Runner ====================
	MsgTypeCreatePod      = "create_pod"
	MsgTypeTerminatePod   = "terminate_pod"
	MsgTypeTerminalInput  = "terminal_input"
	MsgTypeTerminalResize = "terminal_resize"
	MsgTypeSendPrompt     = "send_prompt"
)

// ========== 基础消息结构 ==========

// RunnerMessage represents a message from/to a runner
type RunnerMessage struct {
	Type      string          `json:"type"`
	PodKey    string          `json:"pod_key,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// ========== 初始化流程数据结构 ==========

// InitializeParams 是 Runner 发送的初始化参数
type InitializeParams struct {
	ProtocolVersion int        `json:"protocol_version"`
	RunnerInfo      RunnerInfo `json:"runner_info"`
}

// RunnerInfo 描述 Runner 的基本信息
type RunnerInfo struct {
	Version  string `json:"version"`
	NodeID   string `json:"node_id"`
	MCPPort  int    `json:"mcp_port"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
}

// InitializeResult 是 Backend 返回的初始化结果
type InitializeResult struct {
	ProtocolVersion int             `json:"protocol_version"`
	ServerInfo      ServerInfo      `json:"server_info"`
	AgentTypes      []AgentTypeInfo `json:"agent_types"`
	Features        []string        `json:"features"`
}

// ServerInfo 描述服务端的基本信息
type ServerInfo struct {
	Version string `json:"version"`
}

// AgentTypeInfo 描述单个 Agent 类型的信息
// 用于 Runner 检查本地是否可用
type AgentTypeInfo struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Executable    string `json:"executable"`
	LaunchCommand string `json:"launch_command"`
}

// InitializedParams 是 Runner 发送的初始化完成通知
type InitializedParams struct {
	AvailableAgents []string `json:"available_agents"`
}

// ========== 心跳数据结构 ==========

// HeartbeatData represents heartbeat message data
type HeartbeatData struct {
	Pods            []HeartbeatPod `json:"pods"`
	RunnerVersion   string         `json:"runner_version,omitempty"`
	ProtocolVersion int            `json:"protocol_version,omitempty"`
}

// HeartbeatPod represents a pod in heartbeat data
type HeartbeatPod struct {
	PodKey      string `json:"pod_key"`
	Status      string `json:"status,omitempty"`
	AgentStatus string `json:"agent_status,omitempty"`
}

// ========== Pod 事件数据结构 ==========

// PodCreatedData represents pod creation event data
type PodCreatedData struct {
	PodKey       string `json:"pod_key"`
	Pid          int    `json:"pid"`
	BranchName   string `json:"branch_name,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	Cols         int    `json:"cols,omitempty"`
	Rows         int    `json:"rows,omitempty"`
}

// PodTerminatedData represents pod termination event data
type PodTerminatedData struct {
	PodKey   string `json:"pod_key"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// TerminalOutputData represents terminal output data
type TerminalOutputData struct {
	PodKey string `json:"pod_key"`
	Data   []byte `json:"data"`
}

// AgentStatusData represents agent status change data
type AgentStatusData struct {
	PodKey string `json:"pod_key"`
	Status string `json:"status"`
	Pid    int    `json:"pid,omitempty"`
}

// PtyResizedData represents PTY resize event data
type PtyResizedData struct {
	PodKey string `json:"pod_key"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

// ========== Pod 操作请求结构 ==========

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

// CreatePodRequest represents a request to create a pod
// Backend computes all config, Runner just executes
type CreatePodRequest struct {
	PodKey        string            `json:"pod_key"`
	LaunchCommand string            `json:"launch_command"`
	LaunchArgs    []string          `json:"launch_args,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	FilesToCreate []FileToCreate    `json:"files_to_create,omitempty"`
	WorkDirConfig *WorkDirConfig    `json:"work_dir_config,omitempty"`
	InitialPrompt string            `json:"initial_prompt,omitempty"`
}

// TerminalInputRequest represents terminal input to send
type TerminalInputRequest struct {
	PodKey string `json:"pod_key"`
	Data   []byte `json:"data"`
}

// TerminalResizeRequest represents terminal resize request
type TerminalResizeRequest struct {
	PodKey string `json:"pod_key"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

// ========== 协议版本和特性 ==========

const (
	// CurrentProtocolVersion 当前协议版本
	CurrentProtocolVersion = 2

	// MinSupportedProtocolVersion 最低支持的协议版本
	MinSupportedProtocolVersion = 2

	// 协议特性标识
	FeatureFilesToCreate = "files_to_create"
	FeatureWorkDirConfig = "work_dir_config"
	FeatureInitialPrompt = "initial_prompt"
)

// SupportedFeatures 返回当前 Backend 支持的特性列表
func SupportedFeatures() []string {
	return []string{
		FeatureFilesToCreate,
		FeatureWorkDirConfig,
		FeatureInitialPrompt,
	}
}
