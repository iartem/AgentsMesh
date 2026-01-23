package runner

import (
	"context"
	"sync"
)

const (
	// terminalShards is the number of shards for terminal data partitioning
	// 64 shards reduce lock contention for 500K pods
	terminalShards = 64

	// DefaultTerminalCols is the default terminal width
	DefaultTerminalCols = 80

	// DefaultTerminalRows is the default terminal height
	DefaultTerminalRows = 24
)

// PtySize represents the current PTY terminal size
type PtySize struct {
	Cols int
	Rows int
}

// PodInfoGetter interface for getting and updating pod information
type PodInfoGetter interface {
	GetPodOrganizationAndCreator(ctx context.Context, podKey string) (orgID, creatorID int64, err error)
	UpdatePodTitle(ctx context.Context, podKey, title string) error
}

// terminalShard holds a subset of terminal data with its own lock
// Note: After Relay migration, VirtualTerminal and client management moved to Runner/Relay
type terminalShard struct {
	podRunnerMap map[string]int64   // pod -> runner mapping
	ptySize      map[string]*PtySize // pod -> PTY size
	mu           sync.RWMutex
}

// newTerminalShard creates a new terminal shard with initialized maps
func newTerminalShard() *terminalShard {
	return &terminalShard{
		podRunnerMap: make(map[string]int64),
		ptySize:      make(map[string]*PtySize),
	}
}
