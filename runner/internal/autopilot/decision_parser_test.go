package autopilot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecisionParser_NewDecisionParser(t *testing.T) {
	dp := NewDecisionParser()
	assert.NotNil(t, dp)
}

func TestDecisionParser_ParseDecision_AllTypes(t *testing.T) {
	dp := NewDecisionParser()

	tests := []struct {
		name         string
		input        string
		expectedType DecisionType
	}{
		{
			name:         "TASK_COMPLETED",
			input:        "Analysis done.\nTASK_COMPLETED\nAll tasks finished.",
			expectedType: DecisionCompleted,
		},
		{
			name:         "CONTINUE",
			input:        "Making progress.\nCONTINUE\nMoving to next step.",
			expectedType: DecisionContinue,
		},
		{
			name:         "NEED_HUMAN_HELP",
			input:        "Encountered issue.\nNEED_HUMAN_HELP\nNeed credentials.",
			expectedType: DecisionNeedHumanHelp,
		},
		{
			name:         "GIVE_UP",
			input:        "Cannot proceed.\nGIVE_UP\nTask impossible.",
			expectedType: DecisionGiveUp,
		},
		{
			name:         "No marker defaults to CONTINUE",
			input:        "Just some text without any marker.",
			expectedType: DecisionContinue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := dp.ParseDecision(tt.input)
			assert.Equal(t, tt.expectedType, decision.Type)
		})
	}
}

func TestDecisionParser_ParseDecision_WithFilesChanged(t *testing.T) {
	dp := NewDecisionParser()

	input := `Task done.
TASK_COMPLETED
{"files_changed": ["main.go", "utils.go", "test.go"]}
Summary here.`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Len(t, decision.FilesChanged, 3)
	assert.Contains(t, decision.FilesChanged, "main.go")
	assert.Contains(t, decision.FilesChanged, "utils.go")
	assert.Contains(t, decision.FilesChanged, "test.go")
}

func TestDecisionParser_ParseDecision_JSONOutput(t *testing.T) {
	dp := NewDecisionParser()

	// Claude Code JSON output format
	input := `{"result": "Analysis done.\nTASK_COMPLETED\nAll finished.", "session_id": "abc-123"}`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionCompleted, decision.Type)
}

func TestExtractResultFromJSON_Valid(t *testing.T) {
	input := `{"result": "This is the result text.", "session_id": "123"}`
	result := ExtractResultFromJSON(input)
	assert.Equal(t, "This is the result text.", result)
}

func TestExtractResultFromJSON_Empty(t *testing.T) {
	input := `{"result": "", "session_id": "123"}`
	result := ExtractResultFromJSON(input)
	assert.Empty(t, result)
}

func TestExtractResultFromJSON_NoResult(t *testing.T) {
	input := `{"session_id": "123"}`
	result := ExtractResultFromJSON(input)
	assert.Empty(t, result)
}

func TestExtractResultFromJSON_InvalidJSON(t *testing.T) {
	input := `not json at all`
	result := ExtractResultFromJSON(input)
	assert.Empty(t, result)
}

func TestFindDecisionMarker_CaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected DecisionType
	}{
		{"TASK_COMPLETED", DecisionCompleted},
		{"task_completed", DecisionCompleted},
		{"Task_Completed", DecisionCompleted},
		{"CONTINUE", DecisionContinue},
		{"continue", DecisionContinue},
		{"NEED_HUMAN_HELP", DecisionNeedHumanHelp},
		{"need_human_help", DecisionNeedHumanHelp},
		{"GIVE_UP", DecisionGiveUp},
		{"give_up", DecisionGiveUp},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FindDecisionMarker(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindDecisionMarker_NoMarker(t *testing.T) {
	result := FindDecisionMarker("No marker here")
	assert.Empty(t, result)
}

func TestFindDecisionMarker_MarkerInMiddle(t *testing.T) {
	// Marker must be at start of line
	input := "Line 1\n  TASK_COMPLETED\nLine 3"
	result := FindDecisionMarker(input)
	assert.Equal(t, DecisionCompleted, result)
}

func TestExtractSummary_AfterMarker(t *testing.T) {
	input := "Prefix\nTASK_COMPLETED\nThis is the summary line.\nAnother line."
	result := ExtractSummary(input)
	assert.Contains(t, result, "This is the summary")
}

func TestExtractSummary_NoMarker(t *testing.T) {
	input := "Line 1\nLine 2\nLine 3"
	result := ExtractSummary(input)
	// Should return last 3 lines
	assert.Contains(t, result, "Line 1")
	assert.Contains(t, result, "Line 2")
	assert.Contains(t, result, "Line 3")
}

func TestExtractSummary_EmptyLines(t *testing.T) {
	input := "\n\n\nActual content"
	result := ExtractSummary(input)
	assert.Equal(t, "Actual content", result)
}

func TestExtractSummary_SkipsJSON(t *testing.T) {
	input := "Summary here\n{\"key\": \"value\"}\nMore summary"
	result := ExtractSummary(input)
	assert.NotContains(t, result, "{")
}

func TestExtractSummary_Truncation(t *testing.T) {
	// Create a very long line
	longLine := ""
	for i := 0; i < 50; i++ {
		longLine += "word "
	}

	input := "TASK_COMPLETED\n" + longLine

	result := ExtractSummary(input)
	assert.LessOrEqual(t, len(result), 203) // 200 + "..."
	if len(result) > 200 {
		assert.True(t, result[len(result)-3:] == "...")
	}
}

func TestExtractJSONBlock_Simple(t *testing.T) {
	input := `Some text {"key": "value"} more text`
	result := ExtractJSONBlock(input)
	assert.NotNil(t, result)
	assert.Equal(t, "value", result["key"])
}

func TestExtractJSONBlock_Nested(t *testing.T) {
	input := `Text {"outer": {"inner": 123}} end`
	result := ExtractJSONBlock(input)
	assert.NotNil(t, result)
	inner, ok := result["outer"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(123), inner["inner"])
}

func TestExtractJSONBlock_NoJSON(t *testing.T) {
	input := "No JSON here"
	result := ExtractJSONBlock(input)
	assert.Nil(t, result)
}

func TestExtractJSONBlock_Incomplete(t *testing.T) {
	input := `{"key": "value"`
	result := ExtractJSONBlock(input)
	assert.Nil(t, result)
}

func TestExtractJSONBlock_Invalid(t *testing.T) {
	input := `{invalid json}`
	result := ExtractJSONBlock(input)
	assert.Nil(t, result)
}

func TestExtractJSONBlock_FilesChanged(t *testing.T) {
	input := `{"files_changed": ["a.go", "b.go"]}`
	result := ExtractJSONBlock(input)
	assert.NotNil(t, result)
	files, ok := result["files_changed"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, files, 2)
}

func TestExtractSessionID_JSON(t *testing.T) {
	input := `{"session_id": "abc-123-def", "result": "ok"}`
	result := ExtractSessionID(input)
	assert.Equal(t, "abc-123-def", result)
}

func TestExtractSessionID_Embedded(t *testing.T) {
	input := `Some text "session_id": "xyz-789" more text`
	result := ExtractSessionID(input)
	assert.Equal(t, "xyz-789", result)
}

func TestExtractSessionID_NoSessionID(t *testing.T) {
	input := `{"result": "ok"}`
	result := ExtractSessionID(input)
	assert.Empty(t, result)
}

func TestExtractSessionID_EmptyValue(t *testing.T) {
	input := `{"session_id": "", "result": "ok"}`
	result := ExtractSessionID(input)
	assert.Empty(t, result)
}

func TestDecisionType_String(t *testing.T) {
	assert.Equal(t, "TASK_COMPLETED", string(DecisionCompleted))
	assert.Equal(t, "CONTINUE", string(DecisionContinue))
	assert.Equal(t, "NEED_HUMAN_HELP", string(DecisionNeedHumanHelp))
	assert.Equal(t, "GIVE_UP", string(DecisionGiveUp))
}

// Tests for structured JSON decision parsing

func TestDecisionParser_ParseStructuredDecision_Continue(t *testing.T) {
	dp := NewDecisionParser()

	input := `{
		"decision": {
			"type": "continue",
			"confidence": 0.9,
			"reasoning": "任务正在进展中，Pod 已完成文件创建"
		},
		"progress": {
			"summary": "已完成 2/5 个子任务",
			"completed": ["创建项目结构", "编写主文件"],
			"remaining": ["添加测试", "更新文档"]
		},
		"action": {
			"type": "send_input",
			"content": "请继续完成测试文件",
			"reason": "引导 Pod 进入下一步"
		}
	}`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionContinue, decision.Type)
	assert.Equal(t, 0.9, decision.Confidence)
	assert.Contains(t, decision.Reasoning, "任务正在进展中")

	// Progress
	assert.NotNil(t, decision.Progress)
	assert.Equal(t, "已完成 2/5 个子任务", decision.Progress.Summary)
	assert.Len(t, decision.Progress.CompletedSteps, 2)
	assert.Len(t, decision.Progress.RemainingSteps, 2)

	// Action
	assert.NotNil(t, decision.Action)
	assert.Equal(t, "send_input", decision.Action.Type)
	assert.Equal(t, "请继续完成测试文件", decision.Action.Content)
}

func TestDecisionParser_ParseStructuredDecision_Completed(t *testing.T) {
	dp := NewDecisionParser()

	input := `{
		"decision": {
			"type": "completed",
			"confidence": 1.0,
			"reasoning": "所有子任务已完成，测试通过"
		},
		"progress": {
			"summary": "任务 100% 完成",
			"completed": ["步骤1", "步骤2", "步骤3"],
			"remaining": [],
			"percent": 100
		},
		"action": {
			"type": "none",
			"content": "",
			"reason": "任务已完成"
		}
	}`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Equal(t, 1.0, decision.Confidence)
	assert.NotNil(t, decision.Progress)
	assert.Equal(t, 100, decision.Progress.Percent)
	assert.Empty(t, decision.Progress.RemainingSteps)
}

func TestDecisionParser_ParseStructuredDecision_NeedHelp(t *testing.T) {
	dp := NewDecisionParser()

	input := `{
		"decision": {
			"type": "need_help",
			"confidence": 0.8,
			"reasoning": "Pod 遇到权限错误"
		},
		"help_request": {
			"reason": "无法安装依赖包",
			"context": "正在执行任务 '添加数据处理功能'",
			"terminal_excerpt": "npm ERR! EACCES",
			"suggestions": [
				{"action": "approve", "label": "批准：以 sudo 权限重试"},
				{"action": "skip", "label": "跳过：不安装此依赖"}
			]
		}
	}`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionNeedHumanHelp, decision.Type)
	assert.NotNil(t, decision.HelpRequest)
	assert.Equal(t, "无法安装依赖包", decision.HelpRequest.Reason)
	assert.Contains(t, decision.HelpRequest.TerminalExcerpt, "npm ERR!")
	assert.Len(t, decision.HelpRequest.Suggestions, 2)
	assert.Equal(t, "approve", decision.HelpRequest.Suggestions[0].Action)
}

func TestDecisionParser_ParseStructuredDecision_GiveUp(t *testing.T) {
	dp := NewDecisionParser()

	input := `{
		"decision": {
			"type": "give_up",
			"confidence": 0.7,
			"reasoning": "无法完成任务，架构不兼容"
		}
	}`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionGiveUp, decision.Type)
	assert.Contains(t, decision.Reasoning, "架构不兼容")
}

func TestDecisionParser_FallbackToKeyword(t *testing.T) {
	dp := NewDecisionParser()

	// Non-structured output should fall back to keyword parsing
	input := `观察终端输出后，任务已完成。
TASK_COMPLETED
所有功能已实现并测试通过。`

	decision := dp.ParseDecision(input)

	assert.Equal(t, DecisionCompleted, decision.Type)
	assert.Contains(t, decision.Summary, "所有功能已实现")
}

func TestMapDecisionType(t *testing.T) {
	tests := []struct {
		input    string
		expected DecisionType
	}{
		{"completed", DecisionCompleted},
		{"COMPLETED", DecisionCompleted},
		{"task_completed", DecisionCompleted},
		{"continue", DecisionContinue},
		{"CONTINUE", DecisionContinue},
		{"need_help", DecisionNeedHumanHelp},
		{"NEED_HUMAN_HELP", DecisionNeedHumanHelp},
		{"give_up", DecisionGiveUp},
		{"giveup", DecisionGiveUp},
		{"unknown", DecisionContinue}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapDecisionType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateSummary(t *testing.T) {
	short := "Short text"
	assert.Equal(t, short, truncateSummary(short, 200))

	long := ""
	for i := 0; i < 50; i++ {
		long += "word "
	}
	result := truncateSummary(long, 50)
	assert.Equal(t, 53, len(result)) // 50 + "..."
	assert.True(t, result[len(result)-3:] == "...")
}
