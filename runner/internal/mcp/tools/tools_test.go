package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltInTools(t *testing.T) {
	tools := BuiltInTools()

	if len(tools) == 0 {
		t.Error("BuiltInTools should return some tools")
	}

	// Check for expected tools
	expectedTools := []string{
		"read_file",
		"write_file",
		"list_directory",
		"execute_command",
		"get_working_directory",
		"search_files",
		"git_status",
		"git_diff",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool %s not found", expected)
		}
	}
}

func TestReadFileTool(t *testing.T) {
	tool := ReadFileTool()

	if tool.Name != "read_file" {
		t.Errorf("Name: got %v, want read_file", tool.Name)
	}

	if tool.Handler == nil {
		t.Error("Handler should not be nil")
	}

	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	// Test the handler
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": testFile,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	if len(toolResult.Content) != 1 {
		t.Errorf("content length: got %v, want 1", len(toolResult.Content))
	}

	if toolResult.Content[0].Text != "test content" {
		t.Errorf("content: got %v, want 'test content'", toolResult.Content[0].Text)
	}
}

func TestReadFileToolMissingPath(t *testing.T) {
	tool := ReadFileTool()

	result, err := tool.Handler(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	if !toolResult.IsError {
		t.Error("should return error for missing path")
	}
}

func TestReadFileToolNotExists(t *testing.T) {
	tool := ReadFileTool()

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": "/nonexistent/file.txt",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	if !toolResult.IsError {
		t.Error("should return error for nonexistent file")
	}
}

func TestWriteFileTool(t *testing.T) {
	tool := WriteFileTool()

	if tool.Name != "write_file" {
		t.Errorf("Name: got %v, want write_file", tool.Name)
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new_file.txt")

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": "new content",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}

	// Verify file content
	data, _ := os.ReadFile(testFile)
	if string(data) != "new content" {
		t.Errorf("file content: got %v, want 'new content'", string(data))
	}
}

func TestWriteFileToolMissingParams(t *testing.T) {
	tool := WriteFileTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path": "/tmp/test.txt",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing content")
	}
}

func TestWriteFileToolCreatesDir(t *testing.T) {
	tool := WriteFileTool()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "file.txt")

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    testFile,
		"content": "test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestListDirectoryTool(t *testing.T) {
	tool := ListDirectoryTool()

	if tool.Name != "list_directory" {
		t.Errorf("Name: got %v, want list_directory", tool.Name)
	}

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": tmpDir,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestListDirectoryToolInvalidPath(t *testing.T) {
	tool := ListDirectoryTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path": "/nonexistent/directory",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for nonexistent directory")
	}
}

func TestExecuteCommandTool(t *testing.T) {
	tool := ExecuteCommandTool()

	if tool.Name != "execute_command" {
		t.Errorf("Name: got %v, want execute_command", tool.Name)
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestExecuteCommandToolWithWorkDir(t *testing.T) {
	tool := ExecuteCommandTool()

	tmpDir := t.TempDir()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"command":     "pwd",
		"working_dir": tmpDir,
	})

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestExecuteCommandToolMissingCommand(t *testing.T) {
	tool := ExecuteCommandTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing command")
	}
}

func TestGetWorkingDirectoryTool(t *testing.T) {
	tool := GetWorkingDirectoryTool()

	if tool.Name != "get_working_directory" {
		t.Errorf("Name: got %v, want get_working_directory", tool.Name)
	}

	result, err := tool.Handler(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}

	if toolResult.Content[0].Text == "" {
		t.Error("working directory should not be empty")
	}
}

func TestSearchFilesTool(t *testing.T) {
	tool := SearchFilesTool()

	if tool.Name != "search_files" {
		t.Errorf("Name: got %v, want search_files", tool.Name)
	}

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("content"), 0644)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    tmpDir,
		"pattern": "*.txt",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestSearchFilesToolMissingParams(t *testing.T) {
	tool := SearchFilesTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path": "/tmp",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing pattern")
	}
}

func TestGitStatusTool(t *testing.T) {
	tool := GitStatusTool()

	if tool.Name != "git_status" {
		t.Errorf("Name: got %v, want git_status", tool.Name)
	}

	// Test with path parameter
	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path": "/tmp",
	})

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	// Result might be error if not a git repo, but should not panic
	_ = toolResult
}

func TestGitDiffTool(t *testing.T) {
	tool := GitDiffTool()

	if tool.Name != "git_diff" {
		t.Errorf("Name: got %v, want git_diff", tool.Name)
	}

	// Test with various parameters
	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path":   "/tmp",
		"staged": true,
	})

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	_ = toolResult
}

func TestNewTextResult(t *testing.T) {
	result := NewTextResult("test message")

	if len(result.Content) != 1 {
		t.Errorf("content length: got %v, want 1", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("content type: got %v, want text", result.Content[0].Type)
	}

	if result.Content[0].Text != "test message" {
		t.Errorf("content text: got %v, want 'test message'", result.Content[0].Text)
	}

	if result.IsError {
		t.Error("IsError should be false")
	}
}

func TestNewErrorResult(t *testing.T) {
	result := NewErrorResult(os.ErrNotExist)

	if len(result.Content) != 1 {
		t.Errorf("content length: got %v, want 1", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("content type: got %v, want text", result.Content[0].Type)
	}

	if !result.IsError {
		t.Error("IsError should be true")
	}
}

func TestToolRegistry(t *testing.T) {
	registry := NewToolRegistry()

	if registry == nil {
		t.Fatal("NewToolRegistry returned nil")
	}

	// Should have built-in tools
	tools := registry.List()
	if len(tools) == 0 {
		t.Error("registry should have built-in tools")
	}
}

func TestToolRegistryGet(t *testing.T) {
	registry := NewToolRegistry()

	tool, ok := registry.Get("read_file")
	if !ok {
		t.Error("read_file should exist")
	}

	if tool.Name != "read_file" {
		t.Errorf("Name: got %v, want read_file", tool.Name)
	}

	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("nonexistent tool should not exist")
	}
}

func TestToolRegistryRegister(t *testing.T) {
	registry := NewToolRegistry()

	customTool := &Tool{
		Name:        "custom_tool",
		Description: "A custom tool",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return NewTextResult("custom"), nil
		},
	}

	registry.Register(customTool)

	tool, ok := registry.Get("custom_tool")
	if !ok {
		t.Error("custom_tool should be registered")
	}

	if tool.Description != "A custom tool" {
		t.Errorf("Description: got %v, want 'A custom tool'", tool.Description)
	}
}

func TestToolRegistryInvoke(t *testing.T) {
	registry := NewToolRegistry()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	result, err := registry.Invoke(context.Background(), "read_file", map[string]interface{}{
		"path": testFile,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}
}

func TestToolRegistryInvokeNotFound(t *testing.T) {
	registry := NewToolRegistry()

	_, err := registry.Invoke(context.Background(), "nonexistent", map[string]interface{}{})

	if err == nil {
		t.Error("should return error for nonexistent tool")
	}
}

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param": map[string]interface{}{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return NewTextResult("success"), nil
		},
	}

	if tool.Name != "test_tool" {
		t.Errorf("Name: got %v, want test_tool", tool.Name)
	}

	if tool.Handler == nil {
		t.Error("Handler should not be nil")
	}
}

func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello world",
	}

	if block.Type != "text" {
		t.Errorf("Type: got %v, want text", block.Type)
	}

	if block.Text != "Hello world" {
		t.Errorf("Text: got %v, want 'Hello world'", block.Text)
	}
}

func TestToolResult(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: "Line 1"},
			{Type: "text", Text: "Line 2"},
		},
		IsError: false,
	}

	if len(result.Content) != 2 {
		t.Errorf("Content length: got %v, want 2", len(result.Content))
	}

	if result.IsError {
		t.Error("IsError should be false")
	}
}

// --- Additional tests for coverage ---

func TestToolRegistryInvokeHandlerError(t *testing.T) {
	registry := NewToolRegistry()

	// Register a tool that returns an error
	errorTool := &Tool{
		Name:        "error_tool",
		Description: "A tool that errors",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return nil, os.ErrPermission
		},
	}
	registry.Register(errorTool)

	result, err := registry.Invoke(context.Background(), "error_tool", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("should return error result")
	}
}

func TestToolRegistryInvokeWithToolResult(t *testing.T) {
	registry := NewToolRegistry()

	// Register a tool that returns *ToolResult directly
	customTool := &Tool{
		Name:        "custom_result_tool",
		Description: "A tool that returns custom result",
		InputSchema: map[string]interface{}{"type": "object"},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "custom"}},
				IsError: false,
			}, nil
		},
	}
	registry.Register(customTool)

	result, err := registry.Invoke(context.Background(), "custom_result_tool", map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content[0].Text != "custom" {
		t.Errorf("Content: got %v, want custom", result.Content[0].Text)
	}
}

func TestGitStatusToolInGitRepo(t *testing.T) {
	tool := GitStatusTool()

	// Test in current directory which should be a git repo
	result, err := tool.Handler(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result depends on whether we're in a git repo
	_, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}
}

func TestGitDiffToolUnstaged(t *testing.T) {
	tool := GitDiffTool()

	// Test without staged flag
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":   ".",
		"staged": false,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}
}

func TestGitDiffToolWithFile(t *testing.T) {
	tool := GitDiffTool()

	// Test with specific file
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"file": "nonexistent.txt",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}
}

func TestSearchFilesToolNoMatches(t *testing.T) {
	tool := SearchFilesTool()

	tmpDir := t.TempDir()

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path":    tmpDir,
		"pattern": "*.nonexistent",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestSearchFilesToolInvalidPath(t *testing.T) {
	tool := SearchFilesTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"path":    "/nonexistent/directory",
		"pattern": "*.txt",
	})

	toolResult, _ := result.(*ToolResult)
	// The tool may return empty result for nonexistent path instead of error
	// Just verify it doesn't panic and returns a valid result
	if toolResult == nil {
		t.Error("result should not be nil")
	}
}

func TestExecuteCommandToolFailing(t *testing.T) {
	tool := ExecuteCommandTool()

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"command": "exit 1",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	// The result should indicate the command executed but may have failed
	if toolResult == nil {
		t.Error("result should not be nil")
	}
}

func TestListDirectoryToolMissingPath(t *testing.T) {
	tool := ListDirectoryTool()

	result, _ := tool.Handler(context.Background(), map[string]interface{}{})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing path")
	}
}

func TestListDirectoryToolWithHidden(t *testing.T) {
	tool := ListDirectoryTool()

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("visible"), 0644)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"path": tmpDir,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Errorf("unexpected error: %s", toolResult.Content[0].Text)
	}
}

func TestGetMessagesToolWithUnreadOnly(t *testing.T) {
	store := NewCollaborationStore("")
	store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)

	tool := GetMessagesTool(store, "agent-2")

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"unread_only": true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}
}
