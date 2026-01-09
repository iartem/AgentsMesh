package invitation

import (
	"time"

	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

// NewRepository creates a new invitation repository
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(invitation *Invitation) error {
	return r.db.Create(invitation).Error
}

func (r *repository) GetByToken(token string) (*Invitation, error) {
	var inv Invitation
	err := r.db.Where("token = ?", token).First(&inv).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *repository) GetByID(id int64) (*Invitation, error) {
	var inv Invitation
	err := r.db.First(&inv, id).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *repository) GetByOrgAndEmail(orgID int64, email string) (*Invitation, error) {
	var inv Invitation
	err := r.db.Where("organization_id = ? AND email = ? AND accepted_at IS NULL", orgID, email).First(&inv).Error
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *repository) ListByOrganization(orgID int64) ([]*Invitation, error) {
	var invitations []*Invitation
	err := r.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *repository) ListPendingByEmail(email string) ([]*Invitation, error) {
	var invitations []*Invitation
	err := r.db.Where("email = ? AND accepted_at IS NULL AND expires_at > ?", email, time.Now()).
		Order("created_at DESC").Find(&invitations).Error
	if err != nil {
		return nil, err
	}
	return invitations, nil
}

func (r *repository) Update(invitation *Invitation) error {
	return r.db.Save(invitation).Error
}

func (r *repository) Delete(id int64) error {
	return r.db.Delete(&Invitation{}, id).Error
}

func (r *repository) DeleteExpired() error {
	return r.db.Where("expires_at < ? AND accepted_at IS NULL", time.Now()).Delete(&Invitation{}).Error
}
