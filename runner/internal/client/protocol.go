// Package client provides communication with AgentsMesh server.
package client

import (
	"encoding/json"
)

// MessageType defines the type of control message.
type MessageType string

const (
	// ==================== 初始化流程 (三阶段握手) ====================
	// Runner -> Backend: 初始化请求
	MsgTypeInitialize MessageType = "initialize"
	// Backend -> Runner: 初始化响应
	MsgTypeInitializeResult MessageType = "initialize_result"
	// Runner -> Backend: 初始化完成确认
	MsgTypeInitialized MessageType = "initialized"

	// ==================== 运行时消息: Runner -> Backend ====================
	MsgTypeHeartbeat      MessageType = "heartbeat"
	MsgTypePodCreated     MessageType = "pod_created"
	MsgTypePodTerminated  MessageType = "pod_terminated"
	MsgTypeStatusChange   MessageType = "status_change"
	MsgTypePodList        MessageType = "pod_list"
	MsgTypeTerminalOutput MessageType = "terminal_output"
	MsgTypePtyResized     MessageType = "pty_resized"

	// ==================== 运行时消息: Backend -> Runner ====================
	MsgTypeCreatePod      MessageType = "create_pod"
	MsgTypeTerminatePod   MessageType = "terminate_pod"
	MsgTypeListPods       MessageType = "list_pods"
	MsgTypeTerminalInput  MessageType = "terminal_input"
	MsgTypeTerminalResize MessageType = "terminal_resize"
)

// ==================== 基础消息结构 ====================

// ProtocolMessage is the base message structure.
type ProtocolMessage struct {
	Type      MessageType     `json:"type"`
	PodKey    string          `json:"pod_key,omitempty"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// ==================== 初始化流程数据结构 ====================

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

// ==================== 心跳数据结构 ====================

// HeartbeatData contains heartbeat information.
type HeartbeatData struct {
	NodeID          string    `json:"node_id"`
	Pods            []PodInfo `json:"pods"`
	RunnerVersion   string    `json:"runner_version,omitempty"`
	ProtocolVersion int       `json:"protocol_version,omitempty"`
}

// PodInfo contains pod information for protocol messages.
type PodInfo struct {
	PodKey       string `json:"pod_key"`
	Status       string `json:"status"`
	ClaudeStatus string `json:"claude_status"`
	Pid          int    `json:"pid"`
	ClientCount  int    `json:"client_count"`
}

// ==================== Pod 操作数据结构 ====================

// FileToCreate represents a file to be created in the sandbox.
type FileToCreate struct {
	PathTemplate string `json:"path_template"`
	Content      string `json:"content"`
	Mode         int    `json:"mode,omitempty"`
	IsDirectory  bool   `json:"is_directory,omitempty"`
}

// WorkDirConfig represents the working directory configuration.
type WorkDirConfig struct {
	Type          string `json:"type"` // "worktree", "tempdir", "local"
	RepositoryURL string `json:"repository_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	TicketID      string `json:"ticket_id,omitempty"`
	GitToken      string `json:"git_token,omitempty"`
	SSHKeyPath    string `json:"ssh_key_path,omitempty"`
	LocalPath     string `json:"local_path,omitempty"`
}

// CreatePodRequest contains pod creation request data.
// Backend computes all config, Runner just executes.
type CreatePodRequest struct {
	PodKey        string            `json:"pod_key"`
	LaunchCommand string            `json:"launch_command"`
	LaunchArgs    []string          `json:"launch_args,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	FilesToCreate []FileToCreate    `json:"files_to_create,omitempty"`
	WorkDirConfig *WorkDirConfig    `json:"work_dir_config,omitempty"`
	InitialPrompt string            `json:"initial_prompt,omitempty"`
}

// TerminatePodRequest contains pod termination request data.
type TerminatePodRequest struct {
	PodKey string `json:"pod_key"`
}

// ==================== Pod 事件数据结构 ====================

// PodCreatedEvent is sent when a pod is created.
type PodCreatedEvent struct {
	PodKey       string `json:"pod_key"`
	Pid          int    `json:"pid"`
	WorktreePath string `json:"worktree_path,omitempty"`
	BranchName   string `json:"branch_name,omitempty"`
	PtyCols      uint16 `json:"pty_cols"`
	PtyRows      uint16 `json:"pty_rows"`
}

// PodTerminatedEvent is sent when a pod is terminated.
type PodTerminatedEvent struct {
	PodKey string `json:"pod_key"`
}

// StatusChangeEvent is sent when claude status changes.
type StatusChangeEvent struct {
	PodKey       string `json:"pod_key"`
	ClaudeStatus string `json:"claude_status"`
	ClaudePid    int    `json:"claude_pid,omitempty"`
}

// ==================== 终端数据结构 ====================

// TerminalOutputEvent is sent when there's PTY output.
type TerminalOutputEvent struct {
	PodKey string `json:"pod_key"`
	Data   string `json:"data"` // Base64 encoded binary data
}

// TerminalInputRequest is sent to write to PTY.
type TerminalInputRequest struct {
	PodKey string `json:"pod_key"`
	Data   string `json:"data"` // Base64 encoded binary data
}

// TerminalResizeRequest is sent to resize PTY.
type TerminalResizeRequest struct {
	PodKey string `json:"pod_key"`
	Cols   uint16 `json:"cols"`
	Rows   uint16 `json:"rows"`
}

// PtyResizedEvent is sent when PTY size changes.
type PtyResizedEvent struct {
	PodKey string `json:"pod_key"`
	Cols   uint16 `json:"cols"`
	Rows   uint16 `json:"rows"`
}

// ==================== 消息处理接口 ====================

// MessageHandler handles incoming messages from server.
type MessageHandler interface {
	OnCreatePod(req CreatePodRequest) error
	OnTerminatePod(req TerminatePodRequest) error
	OnListPods() []PodInfo
	OnTerminalInput(req TerminalInputRequest) error
	OnTerminalResize(req TerminalResizeRequest) error
}
