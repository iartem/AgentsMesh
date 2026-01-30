// Package autopilot implements the AutopilotController for supervised Pod automation.
package autopilot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProgressTracker monitors file changes and git status to track task progress.
// It captures snapshots of the working directory state and detects changes.
type ProgressTracker struct {
	workDir      string
	snapshots    []ProgressSnapshot
	lastSnapshot *ProgressSnapshot
	mu           sync.RWMutex
	log          *slog.Logger
}

// ProgressSnapshot represents a point-in-time state of the working directory.
type ProgressSnapshot struct {
	Timestamp     time.Time
	FilesModified []string
	GitDiff       *GitDiffSummary
	ContentHash   string // Hash of terminal content or key files
}

// GitDiffSummary contains summarized git diff information.
type GitDiffSummary struct {
	FilesChanged  []string
	Insertions    int
	Deletions     int
	UnstagedFiles []string
	StagedFiles   []string
	UntrackedFiles []string
	HasChanges    bool
}

// ProgressTrackerConfig contains configuration for ProgressTracker.
type ProgressTrackerConfig struct {
	WorkDir string
	Logger  *slog.Logger
}

// NewProgressTracker creates a new ProgressTracker instance.
func NewProgressTracker(cfg ProgressTrackerConfig) *ProgressTracker {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	return &ProgressTracker{
		workDir:   cfg.WorkDir,
		snapshots: make([]ProgressSnapshot, 0),
		log:       log,
	}
}

// CaptureSnapshot captures the current state of the working directory.
func (pt *ProgressTracker) CaptureSnapshot() *ProgressSnapshot {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	snapshot := &ProgressSnapshot{
		Timestamp: time.Now(),
	}

	// Get git diff summary
	gitDiff := pt.getGitDiffSummary()
	snapshot.GitDiff = gitDiff
	snapshot.FilesModified = gitDiff.FilesChanged

	// Generate content hash for change detection
	snapshot.ContentHash = pt.generateContentHash(gitDiff)

	// Store snapshot
	pt.snapshots = append(pt.snapshots, *snapshot)
	pt.lastSnapshot = snapshot

	pt.log.Debug("Captured progress snapshot",
		"files_changed", len(snapshot.FilesModified),
		"has_changes", gitDiff.HasChanges,
		"insertions", gitDiff.Insertions,
		"deletions", gitDiff.Deletions)

	return snapshot
}

// HasProgress checks if there has been progress since the last snapshot.
func (pt *ProgressTracker) HasProgress() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.lastSnapshot == nil {
		return false
	}

	currentDiff := pt.getGitDiffSummary()

	// Check if there are new file changes
	if currentDiff.HasChanges && !pt.lastSnapshot.GitDiff.HasChanges {
		return true
	}

	// Check if different files are modified
	if len(currentDiff.FilesChanged) != len(pt.lastSnapshot.FilesModified) {
		return true
	}

	// Compare file lists
	currentFiles := make(map[string]bool)
	for _, f := range currentDiff.FilesChanged {
		currentFiles[f] = true
	}
	for _, f := range pt.lastSnapshot.FilesModified {
		if !currentFiles[f] {
			return true
		}
	}

	// Check insertions/deletions
	if currentDiff.Insertions != pt.lastSnapshot.GitDiff.Insertions ||
		currentDiff.Deletions != pt.lastSnapshot.GitDiff.Deletions {
		return true
	}

	return false
}

// IsStuck checks if no progress has been made for the specified duration.
func (pt *ProgressTracker) IsStuck(threshold time.Duration) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if len(pt.snapshots) < 2 {
		return false
	}

	// Check if the last N snapshots have the same content hash
	recentSnapshots := pt.snapshots
	if len(recentSnapshots) > 5 {
		recentSnapshots = recentSnapshots[len(recentSnapshots)-5:]
	}

	// If all recent snapshots have the same hash and enough time has passed
	if len(recentSnapshots) >= 2 {
		firstHash := recentSnapshots[0].ContentHash
		allSame := true
		for _, s := range recentSnapshots[1:] {
			if s.ContentHash != firstHash {
				allSame = false
				break
			}
		}

		if allSame {
			duration := recentSnapshots[len(recentSnapshots)-1].Timestamp.Sub(recentSnapshots[0].Timestamp)
			return duration >= threshold
		}
	}

	return false
}

// GenerateSummary generates a human-readable summary of progress.
func (pt *ProgressTracker) GenerateSummary() string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.lastSnapshot == nil {
		return "No progress data available"
	}

	var parts []string

	// Git changes summary
	if pt.lastSnapshot.GitDiff != nil && pt.lastSnapshot.GitDiff.HasChanges {
		parts = append(parts, fmt.Sprintf("%d file(s) modified", len(pt.lastSnapshot.GitDiff.FilesChanged)))
		if pt.lastSnapshot.GitDiff.Insertions > 0 || pt.lastSnapshot.GitDiff.Deletions > 0 {
			parts = append(parts, fmt.Sprintf("+%d/-%d lines", pt.lastSnapshot.GitDiff.Insertions, pt.lastSnapshot.GitDiff.Deletions))
		}
	} else {
		parts = append(parts, "No file changes detected")
	}

	// Untracked files
	if pt.lastSnapshot.GitDiff != nil && len(pt.lastSnapshot.GitDiff.UntrackedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("%d new file(s)", len(pt.lastSnapshot.GitDiff.UntrackedFiles)))
	}

	// Snapshot count
	parts = append(parts, fmt.Sprintf("%d snapshot(s) captured", len(pt.snapshots)))

	return strings.Join(parts, ", ")
}

// GetLastSnapshot returns the most recent snapshot.
func (pt *ProgressTracker) GetLastSnapshot() *ProgressSnapshot {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.lastSnapshot
}

// GetSnapshotCount returns the total number of snapshots.
func (pt *ProgressTracker) GetSnapshotCount() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.snapshots)
}

// getGitDiffSummary retrieves git diff information for the working directory.
func (pt *ProgressTracker) getGitDiffSummary() *GitDiffSummary {
	summary := &GitDiffSummary{
		FilesChanged:   make([]string, 0),
		UnstagedFiles:  make([]string, 0),
		StagedFiles:    make([]string, 0),
		UntrackedFiles: make([]string, 0),
	}

	// Check if directory exists
	if _, err := os.Stat(pt.workDir); os.IsNotExist(err) {
		return summary
	}

	// Check if it's a git repository
	gitDir := filepath.Join(pt.workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Not a git repository, return empty summary
		return summary
	}

	// Get git status (porcelain format for easy parsing)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = pt.workDir
	output, err := cmd.Output()
	if err != nil {
		pt.log.Debug("Failed to get git status", "error", err)
		return summary
	}

	// Parse git status output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[3:])

		if file == "" {
			continue
		}

		switch {
		case status == "??":
			summary.UntrackedFiles = append(summary.UntrackedFiles, file)
		case status[0] != ' ':
			summary.StagedFiles = append(summary.StagedFiles, file)
		case status[1] != ' ':
			summary.UnstagedFiles = append(summary.UnstagedFiles, file)
		}

		summary.FilesChanged = append(summary.FilesChanged, file)
	}

	// Get diff stats
	cmd = exec.Command("git", "diff", "--stat")
	cmd.Dir = pt.workDir
	output, err = cmd.Output()
	if err == nil {
		pt.parseDiffStats(string(output), summary)
	}

	// Also include staged changes
	cmd = exec.Command("git", "diff", "--cached", "--stat")
	cmd.Dir = pt.workDir
	output, err = cmd.Output()
	if err == nil {
		stagedSummary := &GitDiffSummary{}
		pt.parseDiffStats(string(output), stagedSummary)
		summary.Insertions += stagedSummary.Insertions
		summary.Deletions += stagedSummary.Deletions
	}

	summary.HasChanges = len(summary.FilesChanged) > 0

	return summary
}

// parseDiffStats parses the git diff --stat output.
func (pt *ProgressTracker) parseDiffStats(output string, summary *GitDiffSummary) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for summary line: " N files changed, X insertions(+), Y deletions(-)"
		// Also handles singular forms: "1 insertion(+)", "1 deletion(-)"
		hasInsertions := strings.Contains(line, "insertion(+)") || strings.Contains(line, "insertions(+)")
		hasDeletions := strings.Contains(line, "deletion(-)") || strings.Contains(line, "deletions(-)")

		if hasInsertions || hasDeletions {
			// Parse insertions (handles both singular and plural)
			for _, marker := range []string{"insertions(+)", "insertion(+)"} {
				if idx := strings.Index(line, marker); idx > 0 {
					parts := strings.Fields(line[:idx])
					if len(parts) >= 1 {
						fmt.Sscanf(parts[len(parts)-1], "%d", &summary.Insertions)
					}
					break
				}
			}

			// Parse deletions (handles both singular and plural)
			for _, marker := range []string{"deletions(-)", "deletion(-)"} {
				if idx := strings.Index(line, marker); idx > 0 {
					parts := strings.Fields(line[:idx])
					if len(parts) >= 1 {
						fmt.Sscanf(parts[len(parts)-1], "%d", &summary.Deletions)
					}
					break
				}
			}
		}
	}
}

// generateContentHash generates a hash of the current state for change detection.
func (pt *ProgressTracker) generateContentHash(diff *GitDiffSummary) string {
	hasher := sha256.New()

	// Include file list
	for _, f := range diff.FilesChanged {
		hasher.Write([]byte(f))
	}

	// Include stats
	hasher.Write([]byte(fmt.Sprintf("%d:%d", diff.Insertions, diff.Deletions)))

	// Include untracked files
	for _, f := range diff.UntrackedFiles {
		hasher.Write([]byte(f))
	}

	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// Reset clears all captured snapshots.
func (pt *ProgressTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.snapshots = make([]ProgressSnapshot, 0)
	pt.lastSnapshot = nil
}

// GetChangedFilesSince returns files changed since the given time.
func (pt *ProgressTracker) GetChangedFilesSince(since time.Time) []string {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	changedFiles := make(map[string]bool)

	for _, snapshot := range pt.snapshots {
		if snapshot.Timestamp.After(since) {
			for _, f := range snapshot.FilesModified {
				changedFiles[f] = true
			}
		}
	}

	result := make([]string, 0, len(changedFiles))
	for f := range changedFiles {
		result = append(result, f)
	}

	return result
}
