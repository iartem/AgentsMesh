package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// redisKeyLatestVersion is the Redis key for caching the latest runner version.
	redisKeyLatestVersion = "runner:latest_version"

	// defaultCheckInterval is the default interval for checking GitHub releases.
	defaultCheckInterval = 1 * time.Hour

	// redisCacheTTL is the TTL for the cached version in Redis.
	redisCacheTTL = 1 * time.Hour
)

const (
	// githubRepo is the hardcoded GitHub repository for runner releases.
	githubRepo = "AgentsMesh/AgentsMeshRunner"
)

// VersionChecker periodically fetches the latest Runner version from GitHub Releases
// and caches it in Redis for fast retrieval by the REST API.
type VersionChecker struct {
	redisClient *redis.Client
	interval    time.Duration // check interval, default 1h
	httpClient  *http.Client
	logger      *slog.Logger
}

// NewVersionChecker creates a new VersionChecker.
// Returns nil if redisClient is nil (feature disabled).
func NewVersionChecker(redisClient *redis.Client) *VersionChecker {
	if redisClient == nil {
		return nil
	}

	return &VersionChecker{
		redisClient: redisClient,
		interval:    defaultCheckInterval,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: slog.Default().With("component", "version_checker"),
	}
}

// Start begins the background polling loop. Non-blocking.
func (vc *VersionChecker) Start(ctx context.Context) {
	if vc == nil {
		return
	}

	// Immediate first check
	go func() {
		if _, err := vc.checkGitHubRelease(ctx); err != nil {
			vc.logger.Warn("Initial version check failed", "error", err)
		}
	}()

	// Periodic checks
	go func() {
		ticker := time.NewTicker(vc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := vc.checkGitHubRelease(ctx); err != nil {
					vc.logger.Warn("Periodic version check failed", "error", err)
				}
			}
		}
	}()

	vc.logger.Info("Version checker started", "repo", githubRepo, "interval", vc.interval)
}

// GetLatestVersion returns the cached latest version from Redis.
// Returns empty string if unavailable (cache miss, Redis down, or feature disabled).
func (vc *VersionChecker) GetLatestVersion(ctx context.Context) string {
	if vc == nil || vc.redisClient == nil {
		return ""
	}

	val, err := vc.redisClient.Get(ctx, redisKeyLatestVersion).Result()
	if err != nil {
		// Cache miss or Redis error — silently degrade
		return ""
	}
	return val
}

// githubRelease represents the minimal GitHub Release API response.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
}

// checkGitHubRelease fetches the latest release from GitHub and caches the version in Redis.
func (vc *VersionChecker) checkGitHubRelease(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AgentsMesh-Backend")

	resp, err := vc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	version := NormalizeVersion(release.TagName)
	if version == "" {
		return "", fmt.Errorf("empty version tag from GitHub")
	}

	// Cache to Redis
	if vc.redisClient != nil {
		if err := vc.redisClient.Set(ctx, redisKeyLatestVersion, version, redisCacheTTL).Err(); err != nil {
			vc.logger.Warn("Failed to cache latest version in Redis", "error", err)
		}
	}

	vc.logger.Info("Latest runner version updated", "version", version)
	return version, nil
}

// NormalizeVersion strips the "v" prefix from a version string.
// e.g., "v1.2.3" -> "1.2.3", "1.2.3" -> "1.2.3"
func NormalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}
