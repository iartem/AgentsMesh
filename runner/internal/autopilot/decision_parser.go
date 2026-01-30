// Package autopilot implements the AutopilotController for supervised Pod automation.
package autopilot

import (
	"encoding/json"
	"strings"
)

// DecisionType represents the type of decision made by Control.
type DecisionType string

const (
	DecisionCompleted     DecisionType = "TASK_COMPLETED"
	DecisionContinue      DecisionType = "CONTINUE"
	DecisionNeedHumanHelp DecisionType = "NEED_HUMAN_HELP"
	DecisionGiveUp        DecisionType = "GIVE_UP"
)

// ControlDecision represents the decision made by the control process.
type ControlDecision struct {
	Type         DecisionType // Decision type
	Summary      string       // Decision summary/reason
	Reasoning    string       // Detailed reasoning for the decision
	Confidence   float64      // Confidence level (0-1)
	FilesChanged []string     // List of changed files (for CONTINUE)

	// Progress tracking
	Progress *DecisionProgress // Progress information

	// Action taken
	Action *DecisionAction // Action taken in this iteration

	// Help request (when Type is NEED_HUMAN_HELP)
	HelpRequest *HelpRequestInfo // Detailed help request info
}

// DecisionProgress represents task progress information.
type DecisionProgress struct {
	Summary        string   // Brief progress summary
	CompletedSteps []string // Completed steps
	RemainingSteps []string // Remaining steps
	Percent        int      // Estimated progress percentage (0-100)
}

// DecisionAction represents the action taken in this iteration.
type DecisionAction struct {
	Type    string // observe, send_input, wait, none
	Content string // Action content/description
	Reason  string // Why this action was taken
}

// HelpRequestInfo contains detailed information for help requests.
type HelpRequestInfo struct {
	Reason          string           // Why help is needed
	Context         string           // Context information
	TerminalExcerpt string           // Relevant terminal output
	Suggestions     []HelpSuggestion // Suggested actions
}

// HelpSuggestion represents a suggested action for help request.
type HelpSuggestion struct {
	Action string // approve, skip, custom, etc.
	Label  string // Display label
}

// DecisionParser parses Control Process output to extract decisions.
type DecisionParser struct{}

// NewDecisionParser creates a new DecisionParser instance.
func NewDecisionParser() *DecisionParser {
	return &DecisionParser{}
}

// StructuredDecisionOutput represents the structured JSON output format from Control Agent.
type StructuredDecisionOutput struct {
	Decision struct {
		Type       string  `json:"type"`       // completed, continue, need_help, give_up
		Confidence float64 `json:"confidence"` // 0-1
		Reasoning  string  `json:"reasoning"`  // Detailed reasoning
	} `json:"decision"`
	Progress struct {
		Summary        string   `json:"summary"`
		CompletedSteps []string `json:"completed"`
		RemainingSteps []string `json:"remaining"`
		Percent        int      `json:"percent,omitempty"`
	} `json:"progress,omitempty"`
	Action struct {
		Type    string `json:"type"`    // observe, send_input, wait, none
		Content string `json:"content"` // Action content
		Reason  string `json:"reason"`  // Why this action
	} `json:"action,omitempty"`
	HelpRequest *struct {
		Reason          string `json:"reason"`
		Context         string `json:"context"`
		TerminalExcerpt string `json:"terminal_excerpt,omitempty"`
		Suggestions     []struct {
			Action string `json:"action"`
			Label  string `json:"label"`
		} `json:"suggestions,omitempty"`
	} `json:"help_request,omitempty"`
	FilesChanged []string `json:"files_changed,omitempty"`
}

// ParseDecision extracts a ControlDecision from the control process output.
// First tries to parse structured JSON output, then falls back to keyword-based parsing.
// For JSON output (from --output-format json), extracts the "result" field first.
func (dp *DecisionParser) ParseDecision(output string) *ControlDecision {
	// Try to extract result from JSON output (Claude Code --output-format json)
	textContent := ExtractResultFromJSON(output)
	if textContent == "" {
		textContent = output // Fallback to raw output
	}

	// Try structured JSON parsing first
	if decision := dp.parseStructuredDecision(textContent); decision != nil {
		return decision
	}

	// Fallback to keyword-based parsing
	return dp.parseKeywordDecision(textContent, output)
}

// parseStructuredDecision attempts to parse structured JSON decision output.
func (dp *DecisionParser) parseStructuredDecision(content string) *ControlDecision {
	// Try to extract a JSON block from the content
	jsonData := ExtractJSONBlock(content)
	if jsonData == nil {
		return nil
	}

	// Check if this looks like a structured decision
	decisionField, hasDecision := jsonData["decision"]
	if !hasDecision {
		return nil
	}

	// Re-parse with proper struct
	jsonStr := extractJSONString(content)
	if jsonStr == "" {
		return nil
	}

	var structured StructuredDecisionOutput
	if err := json.Unmarshal([]byte(jsonStr), &structured); err != nil {
		return nil
	}

	// Convert to ControlDecision
	decision := &ControlDecision{
		Type:       mapDecisionType(structured.Decision.Type),
		Reasoning:  structured.Decision.Reasoning,
		Confidence: structured.Decision.Confidence,
	}

	// Map decision type to decisionField to ensure proper handling
	_ = decisionField

	// Set summary from reasoning or first line
	if structured.Decision.Reasoning != "" {
		decision.Summary = truncateSummary(structured.Decision.Reasoning, 200)
	}

	// Set progress
	if structured.Progress.Summary != "" || len(structured.Progress.CompletedSteps) > 0 {
		decision.Progress = &DecisionProgress{
			Summary:        structured.Progress.Summary,
			CompletedSteps: structured.Progress.CompletedSteps,
			RemainingSteps: structured.Progress.RemainingSteps,
			Percent:        structured.Progress.Percent,
		}
	}

	// Set action
	if structured.Action.Type != "" {
		decision.Action = &DecisionAction{
			Type:    structured.Action.Type,
			Content: structured.Action.Content,
			Reason:  structured.Action.Reason,
		}
	}

	// Set help request
	if structured.HelpRequest != nil {
		decision.HelpRequest = &HelpRequestInfo{
			Reason:          structured.HelpRequest.Reason,
			Context:         structured.HelpRequest.Context,
			TerminalExcerpt: structured.HelpRequest.TerminalExcerpt,
		}
		for _, s := range structured.HelpRequest.Suggestions {
			decision.HelpRequest.Suggestions = append(decision.HelpRequest.Suggestions, HelpSuggestion{
				Action: s.Action,
				Label:  s.Label,
			})
		}
	}

	// Set files changed
	decision.FilesChanged = structured.FilesChanged

	return decision
}

// parseKeywordDecision uses keyword-based parsing (legacy fallback).
func (dp *DecisionParser) parseKeywordDecision(textContent, originalOutput string) *ControlDecision {
	decision := &ControlDecision{
		Type:    DecisionContinue, // Default
		Summary: ExtractSummary(textContent),
	}

	// Check for decision markers at the start of lines
	decisionType := FindDecisionMarker(textContent)
	if decisionType != "" {
		decision.Type = decisionType
	}

	// Try to parse any JSON blocks that might contain structured data
	if jsonData := ExtractJSONBlock(originalOutput); jsonData != nil {
		if files, ok := jsonData["files_changed"].([]interface{}); ok {
			for _, f := range files {
				if s, ok := f.(string); ok {
					decision.FilesChanged = append(decision.FilesChanged, s)
				}
			}
		}
	}

	return decision
}

// mapDecisionType maps string type to DecisionType.
func mapDecisionType(typeStr string) DecisionType {
	switch strings.ToLower(typeStr) {
	case "completed", "task_completed":
		return DecisionCompleted
	case "continue":
		return DecisionContinue
	case "need_help", "need_human_help":
		return DecisionNeedHumanHelp
	case "give_up", "giveup":
		return DecisionGiveUp
	default:
		return DecisionContinue
	}
}

// extractJSONString extracts the first complete JSON object from content.
func extractJSONString(content string) string {
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	end := -1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end == -1 {
		return ""
	}

	return content[start:end]
}

// truncateSummary truncates a string to maxLen characters.
func truncateSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ExtractResultFromJSON extracts the "result" field from Claude Code JSON output.
func ExtractResultFromJSON(output string) string {
	var jsonResult struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &jsonResult); err == nil && jsonResult.Result != "" {
		return jsonResult.Result
	}
	return ""
}

// FindDecisionMarker searches for a decision marker at the start of a line.
// Returns the DecisionType if found, or empty string if not found.
// Markers must appear at the beginning of a line (after optional whitespace).
func FindDecisionMarker(output string) DecisionType {
	lines := strings.Split(output, "\n")

	// Search from the end of the output, as the decision is typically at the end
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		upperLine := strings.ToUpper(line)

		// Check if line starts with a decision marker
		if strings.HasPrefix(upperLine, "TASK_COMPLETED") {
			return DecisionCompleted
		}
		if strings.HasPrefix(upperLine, "NEED_HUMAN_HELP") {
			return DecisionNeedHumanHelp
		}
		if strings.HasPrefix(upperLine, "GIVE_UP") {
			return DecisionGiveUp
		}
		if strings.HasPrefix(upperLine, "CONTINUE") {
			return DecisionContinue
		}
	}

	return "" // No marker found, will use default (Continue)
}

// ExtractSummary extracts a brief summary from the output.
// It looks for content after decision markers that appear at line start.
func ExtractSummary(output string) string {
	lines := strings.Split(output, "\n")
	markers := []string{"TASK_COMPLETED", "CONTINUE", "NEED_HUMAN_HELP", "GIVE_UP"}

	// Find the line with a decision marker at start
	markerLineIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmedLine := strings.TrimSpace(lines[i])
		upperLine := strings.ToUpper(trimmedLine)
		for _, marker := range markers {
			if strings.HasPrefix(upperLine, marker) {
				markerLineIdx = i
				break
			}
		}
		if markerLineIdx >= 0 {
			break
		}
	}

	// Extract summary from lines after the marker
	if markerLineIdx >= 0 {
		var summaryLines []string
		for i := markerLineIdx + 1; i < len(lines) && i <= markerLineIdx+3; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed != "" && !strings.HasPrefix(trimmed, "{") {
				summaryLines = append(summaryLines, trimmed)
			}
		}
		if len(summaryLines) > 0 {
			summary := strings.Join(summaryLines, " ")
			if len(summary) > 200 {
				summary = summary[:200] + "..."
			}
			return summary
		}
	}

	// Fallback: Take last few non-empty lines as summary
	var summaryLines []string
	for i := len(lines) - 1; i >= 0 && len(summaryLines) < 3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "{") {
			summaryLines = append([]string{trimmed}, summaryLines...)
		}
	}

	summary := strings.Join(summaryLines, " ")
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}
	return summary
}

// ExtractJSONBlock tries to find and parse a JSON block in the output.
func ExtractJSONBlock(output string) map[string]interface{} {
	// Find JSON block between { and }
	start := strings.Index(output, "{")
	if start == -1 {
		return nil
	}

	// Find matching closing brace
	depth := 0
	end := -1
	for i := start; i < len(output); i++ {
		switch output[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end == -1 {
		return nil
	}

	jsonStr := output[start:end]
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil
	}

	return data
}

// ExtractSessionID extracts session_id from Claude's JSON output.
func ExtractSessionID(output string) string {
	// Try to parse as JSON to get session_id
	var result struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(output), &result); err == nil && result.SessionID != "" {
		return result.SessionID
	}

	// Try to find session_id in the output text
	if idx := strings.Index(output, `"session_id"`); idx != -1 {
		// Try to extract value after session_id
		remaining := output[idx:]
		if colonIdx := strings.Index(remaining, ":"); colonIdx != -1 {
			afterColon := strings.TrimSpace(remaining[colonIdx+1:])
			if len(afterColon) > 0 && afterColon[0] == '"' {
				endQuote := strings.Index(afterColon[1:], `"`)
				if endQuote != -1 {
					return afterColon[1 : endQuote+1]
				}
			}
		}
	}

	return ""
}
