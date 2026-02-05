package agent

import (
	"testing"
)

// --- Test FilesTemplate Scan/Value ---

func TestFilesTemplate_Scan(t *testing.T) {
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
			input:   []byte(`[{"path_template":"/tmp/test.txt","content_template":"hello"}]`),
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "valid JSON string",
			input:   `[{"path_template":"/tmp/a.txt"},{"path_template":"/tmp/b.txt"}]`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "empty JSON array",
			input:   []byte(`[]`),
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
			input:   []byte(`[invalid]`),
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FilesTemplate
			err := ft.Scan(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("FilesTemplate.Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(ft) != tt.wantLen {
				t.Errorf("FilesTemplate.Scan() len = %d, want %d", len(ft), tt.wantLen)
			}
		})
	}
}

func TestFilesTemplate_Value(t *testing.T) {
	tests := []struct {
		name    string
		ft      FilesTemplate
		wantNil bool
		wantErr bool
	}{
		{
			name:    "nil template",
			ft:      nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "empty template",
			ft:      FilesTemplate{},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "template with files",
			ft: FilesTemplate{
				{PathTemplate: "/tmp/test.txt", ContentTemplate: "hello"},
			},
			wantNil: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ft.Value()

			if (err != nil) != tt.wantErr {
				t.Errorf("FilesTemplate.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && (got == nil) != tt.wantNil {
				t.Errorf("FilesTemplate.Value() = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}
