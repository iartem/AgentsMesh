// Package sandbox provides a plugin-based sandbox architecture for session environments.
package sandbox

import "context"

// Plugin defines the interface for sandbox plugins.
// Plugins are executed in order based on their Order() value (lower = earlier).
type Plugin interface {
	// Name returns the plugin name (for logging and debugging).
	Name() string

	// Order returns the execution order (lower numbers execute first).
	Order() int

	// Setup is called when creating the sandbox.
	// It can modify the sandbox's WorkDir, EnvVars, LaunchArgs, and Metadata.
	Setup(ctx context.Context, sb *Sandbox, config map[string]interface{}) error

	// Teardown is called when cleaning up the sandbox (executed in reverse order).
	Teardown(sb *Sandbox) error
}

// GetStringConfig safely retrieves a string value from config map.
func GetStringConfig(config map[string]interface{}, key string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetIntConfig safely retrieves an int value from config map.
func GetIntConfig(config map[string]interface{}, key string) int {
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetMapConfig safely retrieves a map value from config map.
func GetMapConfig(config map[string]interface{}, key string) map[string]interface{} {
	if v, ok := config[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}
