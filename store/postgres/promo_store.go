package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

var (
	_ types.PromoStore = &PromoStore{}

	thePromoStoreMtx sync.Mutex
	thePromoStore    *PromoStore
)

// PromoStore is the PostgreSQL implementation of PromoStore
type PromoStore struct {
	*Client
}

// NewPromoStore creates the singleton PromoStore
func NewPromoStore() (*PromoStore, error) {
	thePromoStoreMtx.Lock()
	defer thePromoStoreMtx.Unlock()

	if thePromoStore != nil {
		return thePromoStore, nil
	}
	if err := MustClientInitialized(client); err != nil {
		return nil, err
	}
	thePromoStore = &PromoStore{Client: client}

	logrus.Info("PromoStore created")
	return thePromoStore, nil
}

// Create persists a new promo code
func (ps *PromoStore) Create(ctx context.Context, promo *interfaces.PromoCode) (*interfaces.PromoCode, error) {
	if promo == nil {
		return nil, fmt.Errorf("promo is nil")
	}
	if promo.ID == uuid.Nil {
		promo.ID = uuid.New()
	}
	promo.Code = strings.ToUpper(promo.Code)

	if err := ps.session.GetDB().WithContext(ctx).Create(promo).Error; err != nil {
		return nil, fmt.Errorf("failed to create promo code: %w", err)
	}
	return promo, nil
}

// GetByCode retrieves a promo code by its code (case-insensitive)
func (ps *PromoStore) GetByCode(ctx context.Context, code string) (*interfaces.PromoCode, error) {
	p := new(interfaces.PromoCode)
	err := ps.session.GetDB().WithContext(ctx).
		Where("UPPER(code) = ?", strings.ToUpper(code)).
		Take(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get promo code: %w", err)
	}
	return p, nil
}

// GetByID retrieves a promo code by its UUID
func (ps *PromoStore) GetByID(ctx context.Context, id string) (*interfaces.PromoCode, error) {
	if id == "" {
		return nil, fmt.Errorf("promo ID is required")
	}
	p := new(interfaces.PromoCode)
	err := ps.session.GetDB().WithContext(ctx).
		Where("id = ?", id).
		Take(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get promo code: %w", err)
	}
	return p, nil
}

// Update applies a partial update to a promo code and returns the fresh row
func (ps *PromoStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.PromoCode, error) {
	if id == "" {
		return nil, fmt.Errorf("promo ID is required")
	}
	if len(fields) == 0 {
		return nil, errs.ErrEmptyUpdate
	}
	if code, ok := fields["code"].(string); ok {
		fields["code"] = strings.ToUpper(code)
	}

	db := ps.session.GetDB().WithContext(ctx)
	if err := db.Model(&interfaces.PromoCode{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("failed to update promo code: %w", err)
	}
	return ps.GetByID(ctx, id)
}

// List returns all active promo codes
func (ps *PromoStore) List(ctx context.Context) ([]*interfaces.PromoCode, error) {
	var promos []*interfaces.PromoCode
	if err := ps.session.GetDB().WithContext(ctx).
		Order("created_at DESC").
		Find(&promos).Error; err != nil {
		return nil, fmt.Errorf("failed to list promo codes: %w", err)
	}
	return promos, nil
}

// IncrementUsage atomically increments usage_count, but only while the global
// usage_limit has not been reached. The WHERE clause makes the check-and-set a
// single statement, so concurrent orders can never push usage_count past the
// configured limit (guards against a check-then-act race). Returns
// errs.ErrPromoUsageLimitReached when the limit was already exhausted.
func (ps *PromoStore) IncrementUsage(ctx context.Context, promoID string) error {
	res := ps.session.GetDB().WithContext(ctx).
		Model(&interfaces.PromoCode{}).
		Where("id = ? AND (usage_limit IS NULL OR usage_count < usage_limit)", promoID).
		Update("usage_count", gorm.Expr("usage_count + 1"))
	if res.Error != nil {
		return fmt.Errorf("failed to increment usage: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrPromoUsageLimitReached
	}
	return nil
}

// RecordUsage stores an audit row of a promo usage
func (ps *PromoStore) RecordUsage(ctx context.Context, usage *interfaces.PromoUsage) error {
	if usage == nil {
		return fmt.Errorf("usage is nil")
	}
	if usage.ID == uuid.Nil {
		usage.ID = uuid.New()
	}
	if err := ps.session.GetDB().WithContext(ctx).Create(usage).Error; err != nil {
		return fmt.Errorf("failed to record promo usage: %w", err)
	}
	return nil
}

// CountUserUsage returns how many times a user has used a promo code
func (ps *PromoStore) CountUserUsage(ctx context.Context, promoID, userID string) (int64, error) {
	var count int64
	err := ps.session.GetDB().WithContext(ctx).
		Model(&interfaces.PromoUsage{}).
		Where("promo_code_id = ? AND user_id = ?", promoID, userID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count user usage: %w", err)
	}
	return count, nil
}

// Delete removes a promo code
func (ps *PromoStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("promo ID is required")
	}
	if err := ps.session.GetDB().WithContext(ctx).
		Where("id = ?", id).
		Delete(&interfaces.PromoCode{}).Error; err != nil {
		return fmt.Errorf("failed to delete promo code: %w", err)
	}
	return nil
}
