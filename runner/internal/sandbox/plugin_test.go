package sandbox

import (
	"testing"
)

func TestGetStringConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "existing string key",
			config:   map[string]interface{}{"name": "test"},
			key:      "name",
			expected: "test",
		},
		{
			name:     "missing key",
			config:   map[string]interface{}{"other": "value"},
			key:      "name",
			expected: "",
		},
		{
			name:     "nil config",
			config:   nil,
			key:      "name",
			expected: "",
		},
		{
			name:     "non-string value",
			config:   map[string]interface{}{"count": 123},
			key:      "count",
			expected: "",
		},
		{
			name:     "empty string",
			config:   map[string]interface{}{"name": ""},
			key:      "name",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringConfig(tt.config, tt.key)
			if result != tt.expected {
				t.Errorf("GetStringConfig() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetIntConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		key      string
		expected int
	}{
		{
			name:     "int value",
			config:   map[string]interface{}{"count": 42},
			key:      "count",
			expected: 42,
		},
		{
			name:     "int64 value",
			config:   map[string]interface{}{"count": int64(100)},
			key:      "count",
			expected: 100,
		},
		{
			name:     "float64 value",
			config:   map[string]interface{}{"count": float64(3.14)},
			key:      "count",
			expected: 3,
		},
		{
			name:     "missing key",
			config:   map[string]interface{}{"other": 1},
			key:      "count",
			expected: 0,
		},
		{
			name:     "string value",
			config:   map[string]interface{}{"count": "not a number"},
			key:      "count",
			expected: 0,
		},
		{
			name:     "nil config",
			config:   nil,
			key:      "count",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetIntConfig(tt.config, tt.key)
			if result != tt.expected {
				t.Errorf("GetIntConfig() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestGetMapConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		key      string
		expected map[string]interface{}
	}{
		{
			name: "existing map",
			config: map[string]interface{}{
				"env_vars": map[string]interface{}{"KEY": "value"},
			},
			key:      "env_vars",
			expected: map[string]interface{}{"KEY": "value"},
		},
		{
			name:     "missing key",
			config:   map[string]interface{}{"other": "value"},
			key:      "env_vars",
			expected: nil,
		},
		{
			name:     "non-map value",
			config:   map[string]interface{}{"env_vars": "not a map"},
			key:      "env_vars",
			expected: nil,
		},
		{
			name:     "nil config",
			config:   nil,
			key:      "env_vars",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMapConfig(tt.config, tt.key)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("GetMapConfig() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("GetMapConfig() = nil, want %v", tt.expected)
				}
			}
		})
	}
}
