package channel

import (
	"testing"

	"github.com/lib/pq"
)

// --- Test PodBinding Permission Methods ---

func TestPodBindingCanObserve(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		grantedScopes []string
		expected      bool
	}{
		{"active with read", BindingStatusActive, []string{BindingScopeTerminalRead}, true},
		{"active without read", BindingStatusActive, []string{BindingScopeTerminalWrite}, false},
		{"pending with read", BindingStatusPending, []string{BindingScopeTerminalRead}, false},
		{"active with no scopes", BindingStatusActive, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &PodBinding{
				Status:        tt.status,
				GrantedScopes: pq.StringArray(tt.grantedScopes),
			}
			if pb.CanObserve() != tt.expected {
				t.Errorf("expected CanObserve() = %v, got %v", tt.expected, pb.CanObserve())
			}
		})
	}
}

func TestPodBindingCanControl(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		grantedScopes []string
		expected      bool
	}{
		{"active with write", BindingStatusActive, []string{BindingScopeTerminalWrite}, true},
		{"active without write", BindingStatusActive, []string{BindingScopeTerminalRead}, false},
		{"pending with write", BindingStatusPending, []string{BindingScopeTerminalWrite}, false},
		{"active with both", BindingStatusActive, []string{BindingScopeTerminalRead, BindingScopeTerminalWrite}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &PodBinding{
				Status:        tt.status,
				GrantedScopes: pq.StringArray(tt.grantedScopes),
			}
			if pb.CanControl() != tt.expected {
				t.Errorf("expected CanControl() = %v, got %v", tt.expected, pb.CanControl())
			}
		})
	}
}
