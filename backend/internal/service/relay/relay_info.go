package relay

import "time"

// RelayInfo holds information about a relay server
type RelayInfo struct {
	ID                 string    `json:"id"`
	URL                string    `json:"url"`          // Public WebSocket URL for browsers (wss://relay.example.com)
	InternalURL        string    `json:"internal_url"` // Internal WebSocket URL for runners (ws://relay:8090 in Docker)
	Region             string    `json:"region"`       // Geographic region
	Capacity           int       `json:"capacity"`     // Maximum connections
	CurrentConnections int       `json:"connections"`  // Current active connections
	CPUUsage           float64   `json:"cpu_usage"`    // CPU usage percentage
	MemoryUsage        float64   `json:"memory_usage"` // Memory usage percentage
	LastHeartbeat      time.Time `json:"last_heartbeat"`
	Healthy            bool      `json:"healthy"`

	// Metrics for enhanced load balancing
	AvgLatencyMs int `json:"avg_latency_ms"` // Average heartbeat latency in milliseconds
}

// LoadBalancingConfig holds configuration for relay selection algorithm
type LoadBalancingConfig struct {
	ConnectionWeight float64 // Weight for connection count factor (default: 0.4)
	CPUWeight        float64 // Weight for CPU usage factor (default: 0.25)
	MemoryWeight     float64 // Weight for memory usage factor (default: 0.15)
	LatencyWeight    float64 // Weight for latency factor (default: 0.1)
	RegionBonus      float64 // Bonus score for same region (default: 50)
}

// DefaultLoadBalancingConfig returns default load balancing configuration
func DefaultLoadBalancingConfig() LoadBalancingConfig {
	return LoadBalancingConfig{
		ConnectionWeight: 0.4,
		CPUWeight:        0.25,
		MemoryWeight:     0.15,
		LatencyWeight:    0.1,
		RegionBonus:      50,
	}
}

// GetRunnerURL returns the URL that runners should use to connect.
// Returns InternalURL if set, otherwise falls back to URL.
func (r *RelayInfo) GetRunnerURL() string {
	if r.InternalURL != "" {
		return r.InternalURL
	}
	return r.URL
}
