package service

import (
	"context"
	"fmt"
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

	return ps.store.Create(ctx, promo)
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
