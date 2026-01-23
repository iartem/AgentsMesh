package relay

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockDNSProvider implements dns.Provider interface for testing
type MockDNSProvider struct {
	records       map[string]string
	createErr     error
	deleteErr     error
	updateErr     error
	getErr        error
	createCalled  int
	deleteCalled  int
	updateCalled  int
	getCalled     int
}

func NewMockDNSProvider() *MockDNSProvider {
	return &MockDNSProvider{
		records: make(map[string]string),
	}
}

func (m *MockDNSProvider) CreateRecord(ctx context.Context, subdomain, ip string) error {
	m.createCalled++
	if m.createErr != nil {
		return m.createErr
	}
	m.records[subdomain] = ip
	return nil
}

func (m *MockDNSProvider) DeleteRecord(ctx context.Context, subdomain string) error {
	m.deleteCalled++
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.records, subdomain)
	return nil
}

func (m *MockDNSProvider) UpdateRecord(ctx context.Context, subdomain, ip string) error {
	m.updateCalled++
	if m.updateErr != nil {
		return m.updateErr
	}
	m.records[subdomain] = ip
	return nil
}

func (m *MockDNSProvider) GetRecord(ctx context.Context, subdomain string) (string, error) {
	m.getCalled++
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.records[subdomain], nil
}

// newTestDNSService creates a DNSService with mock provider for testing
func newTestDNSService(provider *MockDNSProvider, enabled bool) *DNSService {
	return &DNSService{
		provider:   provider,
		baseDomain: "relay.agentsmesh.cn",
		useHTTPS:   true,
		enabled:    enabled,
		logger:     slog.Default(),
	}
}

func TestSanitizeRelayName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "us-east-1",
			expected: "us-east-1",
		},
		{
			name:     "uppercase converted",
			input:    "US-East-1",
			expected: "us-east-1",
		},
		{
			name:     "underscores converted to hyphens",
			input:    "us_east_1",
			expected: "us-east-1",
		},
		{
			name:     "dots converted to hyphens",
			input:    "relay.us.east",
			expected: "relay-us-east",
		},
		{
			name:     "multiple consecutive hyphens collapsed",
			input:    "us--east--1",
			expected: "us-east-1",
		},
		{
			name:     "leading hyphen removed",
			input:    "-us-east-1",
			expected: "us-east-1",
		},
		{
			name:     "trailing hyphen removed",
			input:    "us-east-1-",
			expected: "us-east-1",
		},
		{
			name:     "special characters removed",
			input:    "us@east#1!",
			expected: "useast1",
		},
		{
			name:     "mixed alphanumeric",
			input:    "relay123abc",
			expected: "relay123abc",
		},
		{
			name:     "long name truncated to 63 chars",
			input:    "this-is-a-very-long-relay-name-that-exceeds-the-63-character-limit-for-dns-labels",
			expected: "this-is-a-very-long-relay-name-that-exceeds-the-63-character-li",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRelayName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDNSServiceGenerateRelayDomain(t *testing.T) {
	svc := &DNSService{
		baseDomain: "relay.agentsmesh.cn",
		useHTTPS:   true,
		enabled:    true,
	}

	tests := []struct {
		name       string
		relayName  string
		expected   string
	}{
		{
			name:      "simple name",
			relayName: "us-east-1",
			expected:  "us-east-1.relay.agentsmesh.cn",
		},
		{
			name:      "uppercase converted",
			relayName: "US-West-2",
			expected:  "us-west-2.relay.agentsmesh.cn",
		},
		{
			name:      "with underscores",
			relayName: "ap_south_1",
			expected:  "ap-south-1.relay.agentsmesh.cn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.GenerateRelayDomain(tt.relayName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDNSServiceGenerateRelayURL(t *testing.T) {
	tests := []struct {
		name       string
		useHTTPS   bool
		relayName  string
		expected   string
	}{
		{
			name:      "HTTPS enabled",
			useHTTPS:  true,
			relayName: "us-east-1",
			expected:  "wss://us-east-1.relay.agentsmesh.cn",
		},
		{
			name:      "HTTPS disabled",
			useHTTPS:  false,
			relayName: "us-east-1",
			expected:  "ws://us-east-1.relay.agentsmesh.cn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &DNSService{
				baseDomain: "relay.agentsmesh.cn",
				useHTTPS:   tt.useHTTPS,
				enabled:    true,
			}
			result := svc.GenerateRelayURL(tt.relayName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDNSServiceIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{
			name:     "enabled",
			enabled:  true,
			expected: true,
		},
		{
			name:     "disabled",
			enabled:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &DNSService{
				enabled: tt.enabled,
			}
			assert.Equal(t, tt.expected, svc.IsEnabled())
		})
	}
}

// === Tests for DNS CRUD operations with mock provider ===

func TestDNSService_CreateRecord(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.CreateRecord(ctx, "us-east-1", "192.168.1.1")

	assert.NoError(t, err)
	assert.Equal(t, 1, mockProvider.createCalled)
	assert.Equal(t, "192.168.1.1", mockProvider.records["us-east-1.relay.agentsmesh.cn"])
}

func TestDNSService_CreateRecord_Disabled(t *testing.T) {
	svc := newTestDNSService(nil, false)

	ctx := context.Background()
	err := svc.CreateRecord(ctx, "us-east-1", "192.168.1.1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DNS service is not enabled")
}

func TestDNSService_CreateRecord_ProviderError(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.createErr = errors.New("provider error")
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.CreateRecord(ctx, "us-east-1", "192.168.1.1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create DNS record")
}

func TestDNSService_DeleteRecord(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.records["us-east-1.relay.agentsmesh.cn"] = "192.168.1.1"
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.DeleteRecord(ctx, "us-east-1")

	assert.NoError(t, err)
	assert.Equal(t, 1, mockProvider.deleteCalled)
	assert.Empty(t, mockProvider.records["us-east-1.relay.agentsmesh.cn"])
}

func TestDNSService_DeleteRecord_Disabled(t *testing.T) {
	svc := newTestDNSService(nil, false)

	ctx := context.Background()
	err := svc.DeleteRecord(ctx, "us-east-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DNS service is not enabled")
}

func TestDNSService_DeleteRecord_ProviderError(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.deleteErr = errors.New("provider error")
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.DeleteRecord(ctx, "us-east-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete DNS record")
}

func TestDNSService_UpdateRecord(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.records["us-east-1.relay.agentsmesh.cn"] = "192.168.1.1"
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.UpdateRecord(ctx, "us-east-1", "192.168.1.2")

	assert.NoError(t, err)
	assert.Equal(t, 1, mockProvider.updateCalled)
	assert.Equal(t, "192.168.1.2", mockProvider.records["us-east-1.relay.agentsmesh.cn"])
}

func TestDNSService_UpdateRecord_Disabled(t *testing.T) {
	svc := newTestDNSService(nil, false)

	ctx := context.Background()
	err := svc.UpdateRecord(ctx, "us-east-1", "192.168.1.2")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DNS service is not enabled")
}

func TestDNSService_UpdateRecord_ProviderError(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.updateErr = errors.New("provider error")
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	err := svc.UpdateRecord(ctx, "us-east-1", "192.168.1.2")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update DNS record")
}

func TestDNSService_GetRecord(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.records["us-east-1.relay.agentsmesh.cn"] = "192.168.1.1"
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	ip, err := svc.GetRecord(ctx, "us-east-1")

	assert.NoError(t, err)
	assert.Equal(t, "192.168.1.1", ip)
	assert.Equal(t, 1, mockProvider.getCalled)
}

func TestDNSService_GetRecord_NotFound(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	ip, err := svc.GetRecord(ctx, "non-existent")

	assert.NoError(t, err)
	assert.Empty(t, ip)
}

func TestDNSService_GetRecord_Disabled(t *testing.T) {
	svc := newTestDNSService(nil, false)

	ctx := context.Background()
	ip, err := svc.GetRecord(ctx, "us-east-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DNS service is not enabled")
	assert.Empty(t, ip)
}

func TestDNSService_GetRecord_ProviderError(t *testing.T) {
	mockProvider := NewMockDNSProvider()
	mockProvider.getErr = errors.New("provider error")
	svc := newTestDNSService(mockProvider, true)

	ctx := context.Background()
	ip, err := svc.GetRecord(ctx, "us-east-1")

	assert.Error(t, err)
	assert.Empty(t, ip)
}

// Test NewDNSService with disabled configuration
func TestNewDNSService_Disabled(t *testing.T) {
	// Create a disabled service directly
	svc := newTestDNSService(nil, false)

	assert.False(t, svc.IsEnabled())
}
