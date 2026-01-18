package runner

import (
	"testing"
	"time"
)

// --- Test RegistrationToken ---

func TestRegistrationTokenTableName(t *testing.T) {
	token := RegistrationToken{}
	if token.TableName() != "runner_registration_tokens" {
		t.Errorf("expected TableName 'runner_registration_tokens', got %s", token.TableName())
	}
}

func TestRegistrationTokenStruct(t *testing.T) {
	maxUses := 10
	expiresAt := time.Now().Add(24 * time.Hour)
	createdByID := int64(50)

	token := RegistrationToken{
		ID:             1,
		OrganizationID: 100,
		TokenHash:      "hash123",
		Description:    "Test token",
		CreatedByID:    &createdByID,
		IsActive:       true,
		MaxUses:        &maxUses,
		UsedCount:      5,
		ExpiresAt:      &expiresAt,
		CreatedAt:      time.Now(),
	}

	if token.ID != 1 {
		t.Errorf("expected ID 1, got %d", token.ID)
	}
	if token.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", token.OrganizationID)
	}
	if !token.IsActive {
		t.Error("expected IsActive true")
	}
	if *token.MaxUses != 10 {
		t.Errorf("expected MaxUses 10, got %d", *token.MaxUses)
	}
}
