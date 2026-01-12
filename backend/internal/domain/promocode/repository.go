package promocode

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// Repository defines the interface for promo code data access
type Repository interface {
	// PromoCode CRUD
	Create(ctx context.Context, code *PromoCode) error
	GetByID(ctx context.Context, id int64) (*PromoCode, error)
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
	List(ctx context.Context, filter *ListFilter) ([]*PromoCode, int64, error)
	Update(ctx context.Context, code *PromoCode) error
	Delete(ctx context.Context, id int64) error
	IncrementUsedCount(ctx context.Context, id int64) error

	// Redemption
	CreateRedemption(ctx context.Context, redemption *Redemption) error
	GetRedemptionsByOrg(ctx context.Context, orgID int64) ([]*Redemption, error)
	CountOrgRedemptionsForCode(ctx context.Context, orgID int64, codeID int64) (int64, error)
}

// repository implements Repository interface
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new promo code repository
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Create creates a new promo code
func (r *repository) Create(ctx context.Context, code *PromoCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

// GetByID gets a promo code by ID
func (r *repository) GetByID(ctx context.Context, id int64) (*PromoCode, error) {
	var code PromoCode
	if err := r.db.WithContext(ctx).First(&code, id).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

// GetByCode gets a promo code by code string
func (r *repository) GetByCode(ctx context.Context, code string) (*PromoCode, error) {
	var promoCode PromoCode
	if err := r.db.WithContext(ctx).Where("code = ?", strings.ToUpper(code)).First(&promoCode).Error; err != nil {
		return nil, err
	}
	return &promoCode, nil
}

// List lists promo codes with filtering
func (r *repository) List(ctx context.Context, filter *ListFilter) ([]*PromoCode, int64, error) {
	query := r.db.WithContext(ctx).Model(&PromoCode{})

	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.PlanName != nil {
		query = query.Where("plan_name = ?", *filter.PlanName)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.Search != nil && *filter.Search != "" {
		search := "%" + *filter.Search + "%"
		query = query.Where("code ILIKE ? OR name ILIKE ?", search, search)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.PageSize

	var codes []*PromoCode
	if err := query.Order("created_at DESC").Offset(offset).Limit(filter.PageSize).Find(&codes).Error; err != nil {
		return nil, 0, err
	}

	return codes, total, nil
}

// Update updates a promo code
func (r *repository) Update(ctx context.Context, code *PromoCode) error {
	return r.db.WithContext(ctx).Save(code).Error
}

// Delete deletes a promo code by ID
func (r *repository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&PromoCode{}, id).Error
}

// IncrementUsedCount increments the used count of a promo code
func (r *repository) IncrementUsedCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&PromoCode{}).
		Where("id = ?", id).
		Update("used_count", gorm.Expr("used_count + 1")).Error
}

// CreateRedemption creates a new redemption record
func (r *repository) CreateRedemption(ctx context.Context, redemption *Redemption) error {
	return r.db.WithContext(ctx).Create(redemption).Error
}

// GetRedemptionsByOrg gets all redemptions for an organization
func (r *repository) GetRedemptionsByOrg(ctx context.Context, orgID int64) ([]*Redemption, error) {
	var redemptions []*Redemption
	if err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Preload("PromoCode").
		Order("created_at DESC").
		Find(&redemptions).Error; err != nil {
		return nil, err
	}
	return redemptions, nil
}

// CountOrgRedemptionsForCode counts redemptions for a specific org and code
func (r *repository) CountOrgRedemptionsForCode(ctx context.Context, orgID int64, codeID int64) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Redemption{}).
		Where("organization_id = ? AND promo_code_id = ?", orgID, codeID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
