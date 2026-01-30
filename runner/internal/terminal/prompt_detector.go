// Package terminal provides terminal state detection for AI agents.
package terminal

import (
	"strings"
)

// PromptDetector detects input prompts in terminal output using generic patterns.
// It doesn't depend on specific Agent implementations, but instead detects
// structural features common to most CLI prompts.
type PromptDetector struct {
	// Configuration
	maxPromptLength int // maximum length of a prompt line
}

// PromptDetectorConfig contains configuration for PromptDetector.
type PromptDetectorConfig struct {
	// MaxPromptLength is the maximum length of a line to consider as a prompt (default: 100)
	MaxPromptLength int
}

// NewPromptDetector creates a new prompt detector.
func NewPromptDetector(cfg PromptDetectorConfig) *PromptDetector {
	if cfg.MaxPromptLength == 0 {
		cfg.MaxPromptLength = 100
	}
	return &PromptDetector{
		maxPromptLength: cfg.MaxPromptLength,
	}
}

// PromptResult contains the result of prompt detection.
type PromptResult struct {
	// IsPrompt indicates if a prompt was detected
	IsPrompt bool
	// Confidence is the detection confidence (0.0-1.0)
	Confidence float64
	// PromptType indicates the type of prompt detected
	PromptType PromptType
	// Line is the detected prompt line
	Line string
	// LineIndex is the index of the prompt line
	LineIndex int
}

// PromptType indicates the type of prompt detected.
type PromptType string

const (
	PromptTypeNone        PromptType = ""
	PromptTypeCommand     PromptType = "command"     // >, $, #, etc.
	PromptTypeConfirm     PromptType = "confirm"     // y/n, yes/no
	PromptTypePermission  PromptType = "permission"  // allow, approve, permit
	PromptTypeContinue    PromptType = "continue"    // press any key, enter to continue
	PromptTypeInput       PromptType = "input"       // generic input request
)

// Common prompt ending symbols
var promptEndSymbols = []string{
	">", ">>>", "»", "›",    // Common input prompts
	"$", "#", "%",           // Shell prompts
	"?", ":",                // Question/input prompts
	"⟩", "❯", "➜", "→",      // Fancy prompts
}

// DetectPrompt analyzes terminal lines and detects if there's an input prompt.
// It checks the bottom N non-empty lines of the terminal.
func (d *PromptDetector) DetectPrompt(lines []string) PromptResult {
	if len(lines) == 0 {
		return PromptResult{}
	}

	// Collect non-empty lines from bottom to top (for agents using alternate screen mode)
	var nonEmptyLines []struct {
		line  string
		index int
	}
	for i := len(lines) - 1; i >= 0 && len(nonEmptyLines) < 10; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if len(trimmed) > 0 {
			nonEmptyLines = append(nonEmptyLines, struct {
				line  string
				index int
			}{line: lines[i], index: i})
		}
	}

	if len(nonEmptyLines) == 0 {
		return PromptResult{}
	}

	// Check the last few non-empty lines (most likely prompt is among them)
	linesToCheck := 5
	if len(nonEmptyLines) < linesToCheck {
		linesToCheck = len(nonEmptyLines)
	}

	for i := 0; i < linesToCheck; i++ {
		entry := nonEmptyLines[i]
		result := d.analyzeLine(entry.line, entry.index, i == 0) // i==0 means it's the last non-empty line
		if result.IsPrompt {
			return result
		}
	}

	return PromptResult{}
}

// analyzeLine analyzes a single line for prompt patterns.
func (d *PromptDetector) analyzeLine(line string, lineIndex int, isLastLine bool) PromptResult {
	// Trim trailing whitespace but preserve leading (for indentation detection)
	trimmedRight := strings.TrimRight(line, " \t\r\n")
	trimmed := strings.TrimSpace(line)

	// Empty lines are not prompts
	if len(trimmed) == 0 {
		return PromptResult{}
	}

	// Very long lines are unlikely to be prompts
	if len(trimmed) > d.maxPromptLength {
		return PromptResult{}
	}

	var confidence float64
	var promptType PromptType

	// Check for confirmation prompts (y/n, yes/no)
	if d.isConfirmPrompt(trimmed) {
		confidence = 0.9
		promptType = PromptTypeConfirm
	}

	// Check for permission prompts
	if d.isPermissionPrompt(trimmed) {
		confidence = 0.85
		promptType = PromptTypePermission
	}

	// Check for continue prompts
	if d.isContinuePrompt(trimmed) {
		confidence = 0.85
		promptType = PromptTypeContinue
	}

	// Check for keyboard shortcut prompts (e.g., "[Tab] Accept [Esc] Reject")
	if d.isKeyboardShortcutPrompt(trimmed) {
		if confidence == 0 {
			confidence = 0.85
			promptType = PromptTypeInput
		} else {
			confidence += 0.1
		}
	}

	// Check for command prompt symbols at the end
	if d.hasPromptSymbol(trimmedRight) {
		if confidence == 0 {
			confidence = 0.7
			promptType = PromptTypeCommand
		} else {
			confidence += 0.1 // Boost if also has other features
		}
	}

	// Check for generic input indicators
	if d.hasInputIndicator(trimmed) {
		if confidence == 0 {
			confidence = 0.6
			promptType = PromptTypeInput
		} else {
			confidence += 0.1
		}
	}

	// Boost confidence if it's the last line
	if isLastLine && confidence > 0 {
		confidence += 0.1
	}

	// Boost confidence for short lines (prompts are usually short)
	if len(trimmed) < 30 && confidence > 0 {
		confidence += 0.05
	}

	// Cap confidence at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	if confidence >= 0.5 {
		return PromptResult{
			IsPrompt:   true,
			Confidence: confidence,
			PromptType: promptType,
			Line:       line,
			LineIndex:  lineIndex,
		}
	}

	return PromptResult{}
}

// hasPromptSymbol checks if the line contains a common prompt symbol.
// It checks both suffix (traditional prompts) and contains (TUI-style prompts like Claude Code).
func (d *PromptDetector) hasPromptSymbol(line string) bool {
	trimmed := strings.TrimRight(line, " \t")
	if len(trimmed) == 0 {
		return false
	}

	// Check if line ends with a prompt symbol (traditional shell prompts)
	for _, symbol := range promptEndSymbols {
		if strings.HasSuffix(trimmed, symbol) {
			return true
		}
	}

	// Check for TUI-style prompts where the symbol is in the middle
	// (e.g., Claude Code's "❯ ───" input line)
	tuiPromptSymbols := []string{"❯", "➜", "›", "»"}
	for _, symbol := range tuiPromptSymbols {
		// Check if symbol exists followed by space or box-drawing chars
		idx := strings.Index(line, symbol)
		if idx >= 0 && idx < len(line)-len(symbol) {
			// Found TUI prompt symbol with content after it
			return true
		}
	}

	// Check for box-drawing prompts (│ > │ style)
	if strings.Contains(line, "│") && strings.Contains(line, ">") {
		return true
	}

	return false
}

// isConfirmPrompt checks for y/n style confirmation prompts.
func (d *PromptDetector) isConfirmPrompt(line string) bool {
	lower := strings.ToLower(line)

	patterns := []string{
		"(y/n)",
		"[y/n]",
		"(yes/no)",
		"[yes/no]",
		"y/n:",
		"yes/no:",
		"y or n",
		"yes or no",
	}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// Check if line ends with y/n pattern
	if strings.HasSuffix(lower, "?") {
		// Questions that likely need yes/no
		questionIndicators := []string{"continue", "proceed", "confirm", "sure", "want to", "would you"}
		for _, indicator := range questionIndicators {
			if strings.Contains(lower, indicator) {
				return true
			}
		}
	}

	return false
}

// isPermissionPrompt checks for permission/approval prompts.
func (d *PromptDetector) isPermissionPrompt(line string) bool {
	lower := strings.ToLower(line)

	keywords := []string{
		"permission",
		"approve",
		"allow",
		"authorize",
		"grant",
		"accept",
		"deny",
		"reject",
	}

	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			// Make sure it's a question or request, not just a statement
			if strings.Contains(lower, "?") ||
			   strings.Contains(lower, "allow") ||
			   strings.Contains(lower, "approve") {
				return true
			}
		}
	}

	return false
}

// isContinuePrompt checks for "press any key" style prompts.
func (d *PromptDetector) isContinuePrompt(line string) bool {
	lower := strings.ToLower(line)

	patterns := []string{
		"press any key",
		"press enter",
		"hit enter",
		"press return",
		"to continue",
		"to proceed",
		"when ready",
	}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

// isKeyboardShortcutPrompt checks for keyboard shortcut selection prompts.
// Examples: "[Tab] Accept [Esc] Reject", "[y] Yes [n] No"
func (d *PromptDetector) isKeyboardShortcutPrompt(line string) bool {
	lower := strings.ToLower(line)

	// Check for multiple [key] patterns indicating shortcut options
	bracketCount := 0
	for i := 0; i < len(lower); i++ {
		if lower[i] == '[' {
			// Look for closing bracket
			for j := i + 1; j < len(lower) && j < i+10; j++ {
				if lower[j] == ']' {
					bracketCount++
					i = j
					break
				}
			}
		}
	}

	// If we have 2+ bracketed items, likely a shortcut prompt
	if bracketCount >= 2 {
		// Verify it contains action words
		actionWords := []string{"accept", "reject", "yes", "no", "edit", "cancel", "apply", "skip", "abort"}
		for _, word := range actionWords {
			if strings.Contains(lower, word) {
				return true
			}
		}
	}

	return false
}

// hasInputIndicator checks for generic input request indicators.
func (d *PromptDetector) hasInputIndicator(line string) bool {
	lower := strings.ToLower(line)

	// Check for question marks at the end
	trimmed := strings.TrimRight(lower, " \t")
	if strings.HasSuffix(trimmed, "?") {
		return true
	}

	// Check for "label: (default)" pattern common in CLI prompts
	// e.g., "package name: (my-project)", "version: (1.0.0)"
	if strings.Contains(lower, ":") && strings.HasSuffix(trimmed, ")") {
		// Find the colon and check if there's a parenthetical default after it
		colonIdx := strings.LastIndex(lower, ":")
		parenIdx := strings.LastIndex(lower, "(")
		if colonIdx != -1 && parenIdx != -1 && parenIdx > colonIdx {
			return true
		}
	}

	// Check for input request keywords
	keywords := []string{
		"enter",
		"input",
		"type",
		"provide",
		"specify",
		"password",
		"passphrase",
		"name",
	}

	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			// Look for request context
			if strings.Contains(lower, "please") ||
				strings.Contains(lower, "your") ||
				strings.Contains(lower, ":") {
				return true
			}
		}
	}

	return false
}

// IsPromptChar checks if a character is commonly used in prompts.
func IsPromptChar(r rune) bool {
	promptChars := []rune{'>', '<', '$', '#', '%', '?', ':', '»', '›', '⟩', '❯', '➜', '→'}
	for _, c := range promptChars {
		if r == c {
			return true
		}
	}
	return false
}

// Note: StripANSI is defined in virtual_terminal.go to avoid redeclaration
