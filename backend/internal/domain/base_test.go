package domain

import (
	"testing"
	"time"
)

func TestBaseModelStruct(t *testing.T) {
	now := time.Now()
	base := BaseModel{
		ID:        1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if base.ID != 1 {
		t.Errorf("expected ID 1, got %d", base.ID)
	}
	if base.CreatedAt != now {
		t.Error("expected CreatedAt to match")
	}
	if base.UpdatedAt != now {
		t.Error("expected UpdatedAt to match")
	}
}

func TestTenantModelStruct(t *testing.T) {
	now := time.Now()
	tenant := TenantModel{
		BaseModel: BaseModel{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		OrganizationID: 100,
	}

	if tenant.ID != 1 {
		t.Errorf("expected ID 1, got %d", tenant.ID)
	}
	if tenant.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", tenant.OrganizationID)
	}
}

func TestTeamScopedModelStruct(t *testing.T) {
	now := time.Now()
	teamID := int64(10)
	teamScoped := TeamScopedModel{
		TenantModel: TenantModel{
			BaseModel: BaseModel{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			OrganizationID: 100,
		},
		TeamID: &teamID,
	}

	if teamScoped.ID != 1 {
		t.Errorf("expected ID 1, got %d", teamScoped.ID)
	}
	if teamScoped.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", teamScoped.OrganizationID)
	}
	if *teamScoped.TeamID != 10 {
		t.Errorf("expected TeamID 10, got %d", *teamScoped.TeamID)
	}
}

func TestTeamScopedModelWithNilTeamID(t *testing.T) {
	teamScoped := TeamScopedModel{
		TenantModel: TenantModel{
			OrganizationID: 100,
		},
		TeamID: nil,
	}

	if teamScoped.TeamID != nil {
		t.Error("expected TeamID to be nil")
	}
}

// --- Benchmark Tests ---

func BenchmarkBaseModelCreation(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		_ = BaseModel{
			ID:        int64(i),
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
}

func BenchmarkTenantModelCreation(b *testing.B) {
	now := time.Now()
	for i := 0; i < b.N; i++ {
		_ = TenantModel{
			BaseModel: BaseModel{
				ID:        int64(i),
				CreatedAt: now,
				UpdatedAt: now,
			},
			OrganizationID: 100,
		}
	}
}
