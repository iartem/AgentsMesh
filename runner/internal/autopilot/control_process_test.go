package autopilot

import (
	"testing"

	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestBuildInitialPrompt_Default(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Implement user authentication",
		MaxIterations: 10,
	}

	workerCtrl := &MockPodController{
		workDir: "/workspace",
		podKey:  "worker-123",
	}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
		MCPPort:      19000,
	})

	prompt := rp.buildInitialPrompt()

	// Should contain the original task
	assert.Contains(t, prompt, "Implement user authentication")
	// Should contain the JSON decision type instructions (new format)
	assert.Contains(t, prompt, `"completed"`)
	assert.Contains(t, prompt, `"continue"`)
	assert.Contains(t, prompt, `"need_help"`)
	assert.Contains(t, prompt, `"give_up"`)
	// Should contain MCP tool instructions
	assert.Contains(t, prompt, "observe_terminal")
	assert.Contains(t, prompt, "send_terminal_text")
	assert.Contains(t, prompt, "send_terminal_key")
	assert.Contains(t, prompt, "get_pod_status")
	// Should contain pod key
	assert.Contains(t, prompt, "worker-123")
	// Should contain the important restrictions
	assert.Contains(t, prompt, "重要限制")
	assert.Contains(t, prompt, "你不能直接完成任务")
}

func TestBuildInitialPrompt_CustomTemplate(t *testing.T) {
	customTemplate := "Custom prompt template for {{task}}"
	config := &runnerv1.AutopilotConfig{
		InitialPrompt:         "Test task",
		ControlPromptTemplate: customTemplate,
	}

	workerCtrl := &MockPodController{
		workDir: "/workspace",
		podKey:  "worker-123",
	}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	prompt := rp.buildInitialPrompt()

	// Should use custom template
	assert.Equal(t, customTemplate, prompt)
}

func TestBuildResumePrompt(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test task",
		MaxIterations: 10,
	}

	workerCtrl := &MockPodController{
		workDir: "/workspace",
		podKey:  "worker-123",
	}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	prompt := rp.buildResumePrompt(5)

	// Should contain iteration info
	assert.Contains(t, prompt, "5")
	assert.Contains(t, prompt, "10")
	// Should contain instructions
	assert.Contains(t, prompt, "观察 Pod 终端")
	assert.Contains(t, prompt, "判断任务是否完成")
}

func TestParseDecision_TaskCompleted(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	output := `Analysis complete.
TASK_COMPLETED
Successfully implemented the user authentication feature.
All tests passing.`

	decision := rp.parseDecision(output)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Contains(t, decision.Summary, "Successfully implemented")
}

func TestParseDecision_Continue(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	output := `Current progress: 50%
CONTINUE
Need to implement the login form next.`

	decision := rp.parseDecision(output)

	assert.Equal(t, DecisionContinue, decision.Type)
	assert.Contains(t, decision.Summary, "Need to implement")
}

func TestParseDecision_NeedHumanHelp(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	output := `I've encountered a problem.
NEED_HUMAN_HELP
The API credentials are missing from the environment.
Cannot proceed without them.`

	decision := rp.parseDecision(output)

	assert.Equal(t, DecisionNeedHumanHelp, decision.Type)
	assert.Contains(t, decision.Summary, "API credentials")
}

func TestParseDecision_GiveUp(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	output := `After multiple attempts, I cannot complete this task.
GIVE_UP
The codebase architecture is incompatible with the requested feature.
This would require a complete rewrite.`

	decision := rp.parseDecision(output)

	assert.Equal(t, DecisionGiveUp, decision.Type)
	assert.Contains(t, decision.Summary, "codebase architecture")
}

func TestParseDecision_DefaultToContinue(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	// Output without any decision marker
	output := `Working on the task...
Still processing.`

	decision := rp.parseDecision(output)

	// Should default to CONTINUE
	assert.Equal(t, DecisionContinue, decision.Type)
}

func TestParseDecision_WithJSONBlock(t *testing.T) {
	config := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
	}

	workerCtrl := &MockPodController{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  config,
		PodCtrl:   workerCtrl,
		Reporter:     &MockEventReporter{},
	})

	output := `Task analysis complete.
TASK_COMPLETED
{"files_changed": ["auth.go", "user.go", "test.go"]}
Summary: All files updated.`

	decision := rp.parseDecision(output)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Len(t, decision.FilesChanged, 3)
	assert.Contains(t, decision.FilesChanged, "auth.go")
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple summary",
			input:    "This is a summary.\nMore details here.",
			expected: "This is a summary. More details here.",
		},
		{
			name:     "skip json lines",
			input:    "Summary line\n{\"key\": \"value\"}\nAnother line",
			expected: "Summary line Another line",
		},
		{
			name:     "fallback last 3 lines",
			input:    "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7",
			expected: "Line 5 Line 6 Line 7",
		},
		{
			name:     "long summary truncated",
			input:    "This is a very long line that exceeds 200 characters. " + "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco.",
			expected: "This is a very long line that exceeds 200 characters. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim venia...",
		},
		{
			name:     "extract after TASK_COMPLETED",
			input:    "Some noise\nTASK_COMPLETED\nThis is the actual summary",
			expected: "This is the actual summary",
		},
		{
			name:     "extract after CONTINUE",
			input:    "Prefix text\nCONTINUE\nSent instruction to worker",
			expected: "Sent instruction to worker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSummary(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractJSONBlock(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectNil   bool
		expectKey   string
		expectValue interface{}
	}{
		{
			name:        "simple json",
			input:       `Some text {"key": "value"} more text`,
			expectNil:   false,
			expectKey:   "key",
			expectValue: "value",
		},
		{
			name:        "nested json",
			input:       `Text {"outer": {"inner": 123}} end`,
			expectNil:   false,
			expectKey:   "outer",
			expectValue: map[string]interface{}{"inner": float64(123)},
		},
		{
			name:      "no json",
			input:     "No JSON content here",
			expectNil: true,
		},
		{
			name:      "incomplete json",
			input:     `Start {"key": "value" without closing`,
			expectNil: true,
		},
		{
			name:      "invalid json",
			input:     `{invalid json content}`,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractJSONBlock(tt.input)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectValue, result[tt.expectKey])
			}
		})
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		shouldSet bool
	}{
		{
			name:      "json format",
			input:     `{"session_id": "abc-123-def", "result": "ok"}`,
			expected:  "abc-123-def",
			shouldSet: true,
		},
		{
			name:      "embedded in text",
			input:     `Response text "session_id": "xyz-789" more text`,
			expected:  "xyz-789",
			shouldSet: true,
		},
		{
			name:      "no session_id",
			input:     `{"result": "ok"}`,
			expected:  "",
			shouldSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSessionID(tt.input)

			if tt.shouldSet {
				assert.Equal(t, tt.expected, result)
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestDecisionType_Constants(t *testing.T) {
	// Verify decision type constants match expected values
	assert.Equal(t, DecisionType("TASK_COMPLETED"), DecisionCompleted)
	assert.Equal(t, DecisionType("CONTINUE"), DecisionContinue)
	assert.Equal(t, DecisionType("NEED_HUMAN_HELP"), DecisionNeedHumanHelp)
	assert.Equal(t, DecisionType("GIVE_UP"), DecisionGiveUp)
}

// Test DecisionParser directly
func TestDecisionParser_ParseDecision(t *testing.T) {
	parser := NewDecisionParser()

	output := `Analysis complete.
TASK_COMPLETED
Successfully implemented the feature.`

	decision := parser.ParseDecision(output)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Contains(t, decision.Summary, "Successfully implemented")
}
