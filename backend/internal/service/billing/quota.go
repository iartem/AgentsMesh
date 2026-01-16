package billing

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
)

// RecordUsage records usage for an organization
func (s *Service) RecordUsage(ctx context.Context, orgID int64, usageType string, quantity float64, metadata billing.UsageMetadata) error {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return err
	}

	record := &billing.UsageRecord{
		OrganizationID: orgID,
		UsageType:      usageType,
		Quantity:       quantity,
		PeriodStart:    sub.CurrentPeriodStart,
		PeriodEnd:      sub.CurrentPeriodEnd,
		Metadata:       metadata,
	}

	return s.db.WithContext(ctx).Create(record).Error
}

// GetUsage returns usage for an organization in current period
func (s *Service) GetUsage(ctx context.Context, orgID int64, usageType string) (float64, error) {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return 0, err
	}

	var total float64
	if err := s.db.WithContext(ctx).Model(&billing.UsageRecord{}).
		Where("organization_id = ? AND usage_type = ? AND period_start >= ? AND period_end <= ?",
			orgID, usageType, sub.CurrentPeriodStart, sub.CurrentPeriodEnd).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

// GetUsageHistory returns usage history for an organization
func (s *Service) GetUsageHistory(ctx context.Context, orgID int64, usageType string, months int) ([]*billing.UsageRecord, error) {
	since := time.Now().AddDate(0, -months, 0)

	var records []*billing.UsageRecord
	query := s.db.WithContext(ctx).Where("organization_id = ? AND period_start >= ?", orgID, since)

	if usageType != "" {
		query = query.Where("usage_type = ?", usageType)
	}

	if err := query.Order("period_start DESC").Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

// CheckQuota checks if organization has quota available
func (s *Service) CheckQuota(ctx context.Context, orgID int64, resource string, requestedAmount int) error {
	sub, err := s.GetSubscription(ctx, orgID)

	var plan *billing.SubscriptionPlan
	if err != nil {
		// No subscription found, use Free plan as default
		if errors.Is(err, ErrSubscriptionNotFound) {
			plan, _ = s.GetPlan(ctx, "free")
			if plan == nil {
				// No Free plan in database, allow by default
				return nil
			}
		} else {
			return err
		}
	} else {
		plan = sub.Plan
		if plan == nil {
			plan, _ = s.GetPlanByID(ctx, sub.PlanID)
		}
	}

	if plan == nil {
		return ErrPlanNotFound
	}

	// Check custom quotas first (only if subscription exists)
	if sub != nil && sub.CustomQuotas != nil {
		if customLimit, ok := sub.CustomQuotas[resource]; ok {
			if limit, ok := customLimit.(float64); ok && int(limit) != -1 {
				current, _ := s.getCurrentResourceCount(ctx, orgID, resource)
				if current+requestedAmount > int(limit) {
					return ErrQuotaExceeded
				}
				return nil
			}
		}
	}

	// Check plan limits
	var limit int
	switch resource {
	case "users":
		limit = plan.MaxUsers
	case "runners":
		limit = plan.MaxRunners
	case "concurrent_pods":
		limit = plan.MaxConcurrentPods
	case "repositories":
		limit = plan.MaxRepositories
	case "pod_minutes":
		limit = plan.IncludedPodMinutes
	default:
		return nil
	}

	// -1 means unlimited
	if limit == -1 {
		return nil
	}

	current, _ := s.getCurrentResourceCount(ctx, orgID, resource)
	if current+requestedAmount > limit {
		return ErrQuotaExceeded
	}

	return nil
}

// CheckSeatAvailability checks if there are available seats to add members
// This is different from CheckQuota - it checks purchased seats vs used seats,
// not plan limits. Use this for member invitations.
func (s *Service) CheckSeatAvailability(ctx context.Context, orgID int64, requestedSeats int) error {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		// No subscription = Free plan with default 1 seat
		sub = &billing.Subscription{
			SeatCount: 1,
		}
	}

	// Count current members (used seats)
	var usedSeats int64
	s.db.WithContext(ctx).Table("organization_members").Where("organization_id = ?", orgID).Count(&usedSeats)

	// Count pending invitations (reserved seats)
	var pendingInvitations int64
	s.db.WithContext(ctx).Table("invitations").
		Where("organization_id = ? AND accepted_at IS NULL AND expires_at > ?", orgID, time.Now()).
		Count(&pendingInvitations)

	// Available seats = purchased seats - used seats - pending invitations
	availableSeats := sub.SeatCount - int(usedSeats) - int(pendingInvitations)

	if availableSeats < requestedSeats {
		return ErrQuotaExceeded
	}

	return nil
}

func (s *Service) getCurrentResourceCount(ctx context.Context, orgID int64, resource string) (int, error) {
	var count int64

	switch resource {
	case "users":
		s.db.WithContext(ctx).Table("organization_members").Where("organization_id = ?", orgID).Count(&count)
	case "runners":
		s.db.WithContext(ctx).Table("runners").Where("organization_id = ?", orgID).Count(&count)
	case "concurrent_pods":
		// Count active pods (running or initializing)
		s.db.WithContext(ctx).Table("pods").
			Where("organization_id = ? AND status IN ?", orgID, []string{"running", "initializing"}).
			Count(&count)
	case "repositories":
		s.db.WithContext(ctx).Table("repositories").Where("organization_id = ?", orgID).Count(&count)
	case "pod_minutes":
		usage, _ := s.GetUsage(ctx, orgID, billing.UsageTypePodMinutes)
		return int(usage), nil
	}

	return int(count), nil
}

// GetCurrentConcurrentPods returns the number of currently active pods for an organization
func (s *Service) GetCurrentConcurrentPods(ctx context.Context, orgID int64) (int, error) {
	return s.getCurrentResourceCount(ctx, orgID, "concurrent_pods")
}

// SetCustomQuota sets a custom quota for an organization
func (s *Service) SetCustomQuota(ctx context.Context, orgID int64, resource string, limit int) error {
	sub, err := s.GetSubscription(ctx, orgID)
	if err != nil {
		return err
	}

	if sub.CustomQuotas == nil {
		sub.CustomQuotas = make(billing.CustomQuotas)
	}

	sub.CustomQuotas[resource] = limit

	return s.db.WithContext(ctx).Save(sub).Error
}
