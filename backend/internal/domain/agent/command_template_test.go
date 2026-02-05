package agent

import (
	"testing"
)

// --- Test CommandTemplate Scan/Value ---

func TestCommandTemplate_Scan(t *testing.T) {
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
			input:   []byte(`{"args":[{"args":["--verbose"]}]}`),
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "valid JSON string",
			input:   `{"args":[{"args":["--model","opus"]}]}`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "empty JSON",
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
			var ct CommandTemplate
			err := ct.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("CommandTemplate.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(ct.Args) != tt.wantLen {
				t.Errorf("CommandTemplate.Scan() len = %d, want %d", len(ct.Args), tt.wantLen)
			}
		})
	}
}

func TestCommandTemplate_Value(t *testing.T) {
	tests := []struct {
		name    string
		ct      CommandTemplate
		wantErr bool
	}{
		{
			name:    "empty template",
			ct:      CommandTemplate{},
			wantErr: false,
		},
		{
			name: "template with args",
			ct: CommandTemplate{
				Args: []ArgRule{
					{Args: []string{"--verbose"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ct.Value()

			if (err != nil) != tt.wantErr {
				t.Errorf("CommandTemplate.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got == nil {
				t.Error("CommandTemplate.Value() returned nil without error")
			}
		})
	}
}
