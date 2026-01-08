// Package tools provides built-in MCP tools for collaboration and session management.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool represents a built-in tool that can be invoked.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler is a function that handles tool invocations.
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// ToolResult represents the result of a tool invocation.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewTextResult creates a text result.
func NewTextResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// NewErrorResult creates an error result.
func NewErrorResult(err error) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: err.Error()}},
		IsError: true,
	}
}

// BuiltInTools returns all built-in tools.
func BuiltInTools() []*Tool {
	return []*Tool{
		ReadFileTool(),
		WriteFileTool(),
		ListDirectoryTool(),
		ExecuteCommandTool(),
		GetWorkingDirectoryTool(),
		SearchFilesTool(),
		GitStatusTool(),
		GitDiffTool(),
	}
}

// ReadFileTool creates a tool for reading files.
func ReadFileTool() *Tool {
	return &Tool{
		Name:        "read_file",
		Description: "Read the contents of a file at the specified path",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("path must be a string")), nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return NewErrorResult(err), nil
			}

			return NewTextResult(string(data)), nil
		},
	}
}

// WriteFileTool creates a tool for writing files.
func WriteFileTool() *Tool {
	return &Tool{
		Name:        "write_file",
		Description: "Write content to a file at the specified path",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("path must be a string")), nil
			}

			content, ok := args["content"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("content must be a string")), nil
			}

			// Ensure directory exists
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return NewErrorResult(err), nil
			}

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return NewErrorResult(err), nil
			}

			return NewTextResult(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)), nil
		},
	}
}

// ListDirectoryTool creates a tool for listing directory contents.
func ListDirectoryTool() *Tool {
	return &Tool{
		Name:        "list_directory",
		Description: "List the contents of a directory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the directory to list",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("path must be a string")), nil
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return NewErrorResult(err), nil
			}

			var lines []string
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					continue
				}

				prefix := "-"
				if entry.IsDir() {
					prefix = "d"
				}

				lines = append(lines, fmt.Sprintf("%s %10d %s %s",
					prefix,
					info.Size(),
					info.ModTime().Format("Jan 02 15:04"),
					entry.Name(),
				))
			}

			return NewTextResult(strings.Join(lines, "\n")), nil
		},
	}
}

// ExecuteCommandTool creates a tool for executing shell commands.
func ExecuteCommandTool() *Tool {
	return &Tool{
		Name:        "execute_command",
		Description: "Execute a shell command and return the output",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"working_dir": map[string]interface{}{
					"type":        "string",
					"description": "The working directory for the command (optional)",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in seconds (default: 30)",
				},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			command, ok := args["command"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("command must be a string")), nil
			}

			timeout := 30 * time.Second
			if t, ok := args["timeout"].(float64); ok {
				timeout = time.Duration(t) * time.Second
			}

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

			if wd, ok := args["working_dir"].(string); ok && wd != "" {
				cmd.Dir = wd
			}

			output, err := cmd.CombinedOutput()
			if err != nil {
				return NewTextResult(fmt.Sprintf("Error: %v\nOutput: %s", err, string(output))), nil
			}

			return NewTextResult(string(output)), nil
		},
	}
}

// GetWorkingDirectoryTool creates a tool for getting the current working directory.
func GetWorkingDirectoryTool() *Tool {
	return &Tool{
		Name:        "get_working_directory",
		Description: "Get the current working directory",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			wd, err := os.Getwd()
			if err != nil {
				return NewErrorResult(err), nil
			}
			return NewTextResult(wd), nil
		},
	}
}

// SearchFilesTool creates a tool for searching files by pattern.
func SearchFilesTool() *Tool {
	return &Tool{
		Name:        "search_files",
		Description: "Search for files matching a pattern in a directory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory to search in",
				},
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The glob pattern to match (e.g., *.go, **/*.ts)",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default: 100)",
				},
			},
			"required": []string{"path", "pattern"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			path, ok := args["path"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("path must be a string")), nil
			}

			pattern, ok := args["pattern"].(string)
			if !ok {
				return NewErrorResult(fmt.Errorf("pattern must be a string")), nil
			}

			maxResults := 100
			if m, ok := args["max_results"].(float64); ok {
				maxResults = int(m)
			}

			var matches []string
			err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip errors
				}

				if len(matches) >= maxResults {
					return filepath.SkipAll
				}

				matched, _ := filepath.Match(pattern, filepath.Base(filePath))
				if matched {
					relPath, _ := filepath.Rel(path, filePath)
					matches = append(matches, relPath)
				}

				return nil
			})

			if err != nil {
				return NewErrorResult(err), nil
			}

			return NewTextResult(strings.Join(matches, "\n")), nil
		},
	}
}

// GitStatusTool creates a tool for getting git status.
func GitStatusTool() *Tool {
	return &Tool{
		Name:        "git_status",
		Description: "Get the git status of the current repository",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the git repository (optional, uses current directory)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")

			if path, ok := args["path"].(string); ok && path != "" {
				cmd.Dir = path
			}

			output, err := cmd.CombinedOutput()
			if err != nil {
				return NewTextResult(fmt.Sprintf("Error: %v\nOutput: %s", err, string(output))), nil
			}

			if len(output) == 0 {
				return NewTextResult("Working tree clean"), nil
			}

			return NewTextResult(string(output)), nil
		},
	}
}

// GitDiffTool creates a tool for getting git diff.
func GitDiffTool() *Tool {
	return &Tool{
		Name:        "git_diff",
		Description: "Get the git diff of changes",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the git repository (optional)",
				},
				"staged": map[string]interface{}{
					"type":        "boolean",
					"description": "Show staged changes only (default: false)",
				},
				"file": map[string]interface{}{
					"type":        "string",
					"description": "Specific file to diff (optional)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			cmdArgs := []string{"diff"}

			if staged, ok := args["staged"].(bool); ok && staged {
				cmdArgs = append(cmdArgs, "--staged")
			}

			if file, ok := args["file"].(string); ok && file != "" {
				cmdArgs = append(cmdArgs, "--", file)
			}

			cmd := exec.CommandContext(ctx, "git", cmdArgs...)

			if path, ok := args["path"].(string); ok && path != "" {
				cmd.Dir = path
			}

			output, err := cmd.CombinedOutput()
			if err != nil {
				return NewTextResult(fmt.Sprintf("Error: %v\nOutput: %s", err, string(output))), nil
			}

			if len(output) == 0 {
				return NewTextResult("No changes"), nil
			}

			return NewTextResult(string(output)), nil
		},
	}
}

// ToolRegistry manages tool registration and lookup.
type ToolRegistry struct {
	tools map[string]*Tool
}

// NewToolRegistry creates a new tool registry with built-in tools.
func NewToolRegistry() *ToolRegistry {
	registry := &ToolRegistry{
		tools: make(map[string]*Tool),
	}

	// Register built-in tools
	for _, tool := range BuiltInTools() {
		registry.Register(tool)
	}

	return registry
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool *Tool) {
	r.tools[tool.Name] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (*Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
func (r *ToolRegistry) List() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Invoke invokes a tool by name with arguments.
func (r *ToolRegistry) Invoke(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return NewErrorResult(err), nil
	}

	if tr, ok := result.(*ToolResult); ok {
		return tr, nil
	}

	// Convert other results to JSON text
	data, err := json.Marshal(result)
	if err != nil {
		return NewErrorResult(err), nil
	}

	return NewTextResult(string(data)), nil
}
