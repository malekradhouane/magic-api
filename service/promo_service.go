package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

// PromoService handles promo code business logic
type PromoService struct {
	store  types.PromoStore
	logger *logrus.Logger
}

// NewPromoService constructs a PromoService
func NewPromoService(store types.PromoStore, logger *logrus.Logger) *PromoService {
	if logger == nil {
		logger = logrus.New()
	}
	return &PromoService{store: store, logger: logger}
}

// Create persists a new promo code
func (ps *PromoService) Create(ctx context.Context, req *api.CreatePromoRequest) (*interfaces.PromoCode, error) {
	if err := validateDiscount(req.DiscountType, req.DiscountValue); err != nil {
		return nil, err
	}
	if req.MinOrderTotal < 0 {
		return nil, fmt.Errorf("min_order_total cannot be negative")
	}
	if req.MaxDiscount != nil && *req.MaxDiscount < 0 {
		return nil, fmt.Errorf("max_discount cannot be negative")
	}

	promo := &interfaces.PromoCode{
		Code:          req.Code,
		Description:   req.Description,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		MinOrderTotal: req.MinOrderTotal,
		MaxDiscount:   req.MaxDiscount,
		UsageLimit:    req.UsageLimit,
		PerUserLimit:  req.PerUserLimit,
		IsActive:      req.IsActive,
	}

	if req.StartsAt != nil && *req.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, *req.StartsAt)
		if err != nil {
			return nil, fmt.Errorf("invalid starts_at format (expected RFC3339): %w", err)
		}
		promo.StartsAt = &t
	}
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("invalid expires_at format (expected RFC3339): %w", err)
		}
		promo.ExpiresAt = &t
	}

	if promo.StartsAt != nil && promo.ExpiresAt != nil && promo.ExpiresAt.Before(*promo.StartsAt) {
		return nil, fmt.Errorf("expires_at cannot be before starts_at")
	}

	return ps.store.Create(ctx, promo)
}

// validateDiscount enforces the business rules on the discount type/value pair.
// Percentages must sit within (0, 100]; fixed amounts must be strictly positive.
func validateDiscount(discountType string, value float64) error {
	switch discountType {
	case interfaces.PromoTypePercentage:
		if value <= 0 || value > 100 {
			return fmt.Errorf("percentage discount must be between 0 and 100")
		}
	case interfaces.PromoTypeFixed:
		if value <= 0 {
			return fmt.Errorf("fixed discount must be greater than 0")
		}
	default:
		return fmt.Errorf("invalid discount_type: must be %q or %q",
			interfaces.PromoTypePercentage, interfaces.PromoTypeFixed)
	}
	return nil
}

// Update applies a partial update to an existing promo code (admin).
func (ps *PromoService) Update(ctx context.Context, id string, req *api.UpdatePromoRequest) (*interfaces.PromoCode, error) {
	// Resolve the effective discount type/value so cross-field validation runs
	// against the final state, not just the delta.
	current, err := ps.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fields := map[string]interface{}{}

	effType := current.DiscountType
	effValue := current.DiscountValue
	if req.DiscountType != nil {
		effType = *req.DiscountType
	}
	if req.DiscountValue != nil {
		effValue = *req.DiscountValue
	}
	if req.DiscountType != nil || req.DiscountValue != nil {
		if err := validateDiscount(effType, effValue); err != nil {
			return nil, err
		}
		fields["discount_type"] = effType
		fields["discount_value"] = effValue
	}

	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return nil, fmt.Errorf("code cannot be empty")
		}
		fields["code"] = code
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.MinOrderTotal != nil {
		if *req.MinOrderTotal < 0 {
			return nil, fmt.Errorf("min_order_total cannot be negative")
		}
		fields["min_order_total"] = *req.MinOrderTotal
	}
	if req.MaxDiscount != nil {
		if *req.MaxDiscount < 0 {
			return nil, fmt.Errorf("max_discount cannot be negative")
		}
		fields["max_discount"] = *req.MaxDiscount
	}
	if req.UsageLimit != nil {
		fields["usage_limit"] = *req.UsageLimit
	}
	if req.PerUserLimit != nil {
		fields["per_user_limit"] = *req.PerUserLimit
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}

	var startsAt, expiresAt *time.Time
	if req.StartsAt != nil {
		if *req.StartsAt == "" {
			fields["starts_at"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.StartsAt)
			if err != nil {
				return nil, fmt.Errorf("invalid starts_at format (expected RFC3339): %w", err)
			}
			fields["starts_at"] = t
			startsAt = &t
		}
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			fields["expires_at"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				return nil, fmt.Errorf("invalid expires_at format (expected RFC3339): %w", err)
			}
			fields["expires_at"] = t
			expiresAt = &t
		}
	}
	if startsAt == nil {
		startsAt = current.StartsAt
	}
	if expiresAt == nil {
		expiresAt = current.ExpiresAt
	}
	if startsAt != nil && expiresAt != nil && expiresAt.Before(*startsAt) {
		return nil, fmt.Errorf("expires_at cannot be before starts_at")
	}

	return ps.store.Update(ctx, id, fields)
}

// Validate checks a code & returns the discount amount that would apply
func (ps *PromoService) Validate(ctx context.Context, code string, subtotal float64, userID string) (*api.ValidatePromoResponse, error) {
	promo, err := ps.store.GetByCode(ctx, code)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return &api.ValidatePromoResponse{
				Valid:   false,
				Message: "Code promo invalide",
			}, nil
		}
		return nil, err
	}

	if !promo.IsValid() {
		return &api.ValidatePromoResponse{
			Valid:   false,
			Code:    promo.Code,
			Message: "Code promo expiré ou non disponible",
		}, nil
	}

	if subtotal < promo.MinOrderTotal {
		return &api.ValidatePromoResponse{
			Valid: false,
			Code:  promo.Code,
			Message: fmt.Sprintf(
				"Montant minimum de commande non atteint (%.2f TND)",
				promo.MinOrderTotal,
			),
		}, nil
	}

	if userID != "" && promo.PerUserLimit != nil {
		usedCount, err := ps.store.CountUserUsage(ctx, promo.ID.String(), userID)
		if err != nil {
			return nil, err
		}
		if usedCount >= int64(*promo.PerUserLimit) {
			return &api.ValidatePromoResponse{
				Valid:   false,
				Code:    promo.Code,
				Message: "Limite d'utilisation par utilisateur atteinte",
			}, nil
		}
	}

	discount := promo.CalculateDiscount(subtotal)

	return &api.ValidatePromoResponse{
		Valid:          true,
		Code:           promo.Code,
		DiscountAmount: discount,
		DiscountType:   promo.DiscountType,
	}, nil
}

// GetByCode returns a promo code (used by OrderService when applying at checkout)
func (ps *PromoService) GetByCode(ctx context.Context, code string) (*interfaces.PromoCode, error) {
	return ps.store.GetByCode(ctx, code)
}

// MarkUsed records a promo usage and increments the counter
func (ps *PromoService) MarkUsed(ctx context.Context, promo *interfaces.PromoCode, userID, orderID string, discount float64) error {
	usage := &interfaces.PromoUsage{
		PromoCodeID:    promo.ID,
		DiscountAmount: discount,
	}
	if userID != "" {
		uid, err := parseUUIDOpt(userID)
		if err != nil {
			return err
		}
		usage.UserID = uid
	}
	if orderID != "" {
		oid, err := parseUUIDOpt(orderID)
		if err != nil {
			return err
		}
		usage.OrderID = oid
	}

	if err := ps.store.RecordUsage(ctx, usage); err != nil {
		return err
	}
	return ps.store.IncrementUsage(ctx, promo.ID.String())
}

// List returns all promo codes (admin)
func (ps *PromoService) List(ctx context.Context) ([]*interfaces.PromoCode, error) {
	return ps.store.List(ctx)
}

// Delete removes a promo code
func (ps *PromoService) Delete(ctx context.Context, id string) error {
	return ps.store.Delete(ctx, id)
}
