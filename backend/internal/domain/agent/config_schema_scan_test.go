package agent

import (
	"testing"
)

// --- Test ConfigSchema Scan/Value ---

func TestConfigSchema_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantLen int
		wantErr bool
	}{
		{
			name:    "nil value",
			input:   nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "valid JSON bytes",
			input:   []byte(`{"fields":[{"name":"model","type":"select"}]}`),
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "valid JSON string",
			input:   `{"fields":[{"name":"model","type":"select"},{"name":"perm","type":"string"}]}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "empty JSON object",
			input:   []byte(`{}`),
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   123,
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{invalid}`),
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cs ConfigSchema
			err := cs.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigSchema.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(cs.Fields) != tt.wantLen {
				t.Errorf("ConfigSchema.Scan() len = %d, want %d", len(cs.Fields), tt.wantLen)
			}
		})
	}
}

func TestConfigSchema_Value(t *testing.T) {
	tests := []struct {
		name    string
		cs      ConfigSchema
		wantErr bool
	}{
		{
			name:    "empty schema",
			cs:      ConfigSchema{},
			wantErr: false,
		},
		{
			name: "schema with fields",
			cs: ConfigSchema{
				Fields: []ConfigField{
					{Name: "model", Type: "select"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cs.Value()

			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigSchema.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got == nil {
				t.Error("ConfigSchema.Value() returned nil without error")
			}
		})
	}
}
