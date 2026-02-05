package relay

import "time"

// SelectRelayOptions holds options for relay selection
type SelectRelayOptions struct {
	Region          string   // Preferred region
	ExcludeRelayIDs []string // Relays to exclude from selection
}

// SelectRelay selects the best relay for a given runner region using weighted scoring
// Factors considered: connection load, CPU, memory, latency, and region affinity
func (m *Manager) SelectRelay(runnerRegion string) *RelayInfo {
	return m.SelectRelayWithOptions(SelectRelayOptions{Region: runnerRegion})
}

// SelectRelayWithOptions selects the best relay with additional options
func (m *Manager) SelectRelayWithOptions(opts SelectRelayOptions) *RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build exclusion set for O(1) lookup
	excluded := make(map[string]bool)
	for _, id := range opts.ExcludeRelayIDs {
		excluded[id] = true
	}

	var best *RelayInfo
	bestScore := float64(-1)
	cfg := m.loadBalancingConfig

	for _, relay := range m.relays {
		// Skip unhealthy, excluded, or at-capacity relays
		if !relay.Healthy || excluded[relay.ID] {
			continue
		}
		if relay.Capacity > 0 && relay.CurrentConnections >= relay.Capacity {
			continue
		}

		// Calculate weighted score (higher is better)
		score := m.calculateRelayScore(relay, opts.Region, cfg)

		if score > bestScore {
			bestScore = score
			best = relay
		}
	}

	if best != nil {
		m.logger.Debug("Selected relay",
			"relay_id", best.ID,
			"region", best.Region,
			"score", bestScore,
			"connections", best.CurrentConnections,
			"capacity", best.Capacity,
			"cpu", best.CPUUsage,
			"memory", best.MemoryUsage,
			"latency_ms", best.AvgLatencyMs)

		// Return copy to prevent data races
		bestCopy := *best
		return &bestCopy
	}

	return nil
}

// calculateRelayScore calculates the weighted score for relay selection
func (m *Manager) calculateRelayScore(relay *RelayInfo, region string, cfg LoadBalancingConfig) float64 {
	score := 100.0

	// Connection load factor: prefer relays with more available capacity
	if relay.Capacity > 0 {
		availableRatio := 1 - float64(relay.CurrentConnections)/float64(relay.Capacity)
		score += availableRatio * cfg.ConnectionWeight * 100
	}

	// CPU factor: prefer relays with lower CPU usage
	if relay.CPUUsage >= 0 {
		cpuAvailable := (100 - relay.CPUUsage) / 100
		score += cpuAvailable * cfg.CPUWeight * 100
	}

	// Memory factor: prefer relays with lower memory usage
	if relay.MemoryUsage >= 0 {
		memAvailable := (100 - relay.MemoryUsage) / 100
		score += memAvailable * cfg.MemoryWeight * 100
	}

	// Latency factor: prefer relays with lower latency
	// Normalize latency: 0ms = 1.0, 500ms = 0.0 (capped)
	if relay.AvgLatencyMs >= 0 {
		latencyFactor := 1 - float64(min(relay.AvgLatencyMs, 500))/500
		score += latencyFactor * cfg.LatencyWeight * 100
	}

	// Region affinity bonus
	if region != "" && relay.Region == region {
		score += cfg.RegionBonus
	}

	return score
}

// SelectRelayForPod selects relay for a pod, checking for existing active session
// Returns copies to prevent data races
func (m *Manager) SelectRelayForPod(podKey, runnerRegion string) (*RelayInfo, *ActiveSession) {
	m.mu.RLock()
	// Check for existing active session
	if session, ok := m.activeSessions[podKey]; ok {
		if time.Now().Before(session.ExpireAt) {
			if relay, exists := m.relays[session.RelayID]; exists && relay.Healthy {
				// Return copies to prevent data races
				relayCopy := *relay
				sessionCopy := *session
				m.mu.RUnlock()
				return &relayCopy, &sessionCopy
			}
		}
	}
	m.mu.RUnlock()

	// No existing session, select new relay
	relay := m.SelectRelay(runnerRegion)
	return relay, nil
}
