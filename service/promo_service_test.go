package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// PromoService.Create
// ---------------------------------------------------------------------------

func TestPromoService_Create_Success(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	promoID := uuid.New()
	store.On("Create", mock.Anything, mock.AnythingOfType("*interfaces.PromoCode")).
		Return(&interfaces.PromoCode{ID: promoID, Code: "SUMMER10"}, nil)

	got, err := svc.Create(context.Background(), &api.CreatePromoRequest{
		Code:          "SUMMER10",
		DiscountType:  "percentage",
		DiscountValue: 10,
		IsActive:      true,
	})
	require.NoError(t, err)
	assert.Equal(t, "SUMMER10", got.Code)
	store.AssertExpectations(t)
}

func TestPromoService_Create_WithDates(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	starts := "2026-01-01T00:00:00Z"
	expires := "2026-12-31T23:59:59Z"

	store.On("Create", mock.Anything, mock.MatchedBy(func(p *interfaces.PromoCode) bool {
		return p.StartsAt != nil && p.ExpiresAt != nil
	})).Return(&interfaces.PromoCode{ID: uuid.New(), Code: "NY2026"}, nil)

	_, err := svc.Create(context.Background(), &api.CreatePromoRequest{
		Code:          "NY2026",
		DiscountType:  "fixed",
		DiscountValue: 5,
		StartsAt:      &starts,
		ExpiresAt:     &expires,
	})
	require.NoError(t, err)
}

func TestPromoService_Create_InvalidStartsAt(t *testing.T) {
	t.Parallel()
	svc := NewPromoService(new(MockPromoStore), nil)

	bad := "not-a-date"
	_, err := svc.Create(context.Background(), &api.CreatePromoRequest{
		Code:          "BAD",
		DiscountType:  "fixed",
		DiscountValue: 5,
		StartsAt:      &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid starts_at")
}

func TestPromoService_Create_InvalidExpiresAt(t *testing.T) {
	t.Parallel()
	svc := NewPromoService(new(MockPromoStore), nil)

	bad := "not-a-date"
	_, err := svc.Create(context.Background(), &api.CreatePromoRequest{
		Code:          "BAD",
		DiscountType:  "fixed",
		DiscountValue: 5,
		ExpiresAt:     &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid expires_at")
}

// ---------------------------------------------------------------------------
// PromoService.Validate
// ---------------------------------------------------------------------------

func TestPromoService_Validate_ValidCode(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "SUMMER10").
		Return(&interfaces.PromoCode{
			ID:            uuid.New(),
			Code:          "SUMMER10",
			DiscountType:  interfaces.PromoTypePercentage,
			DiscountValue: 10,
			IsActive:      true,
		}, nil)

	got, err := svc.Validate(context.Background(), "SUMMER10", 200.0, "")
	require.NoError(t, err)
	assert.True(t, got.Valid)
	assert.Equal(t, 20.0, got.DiscountAmount) // 10% of 200
}

func TestPromoService_Validate_CodeNotFound(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "INVALID").
		Return((*interfaces.PromoCode)(nil), errs.ErrNoSuchEntity)

	got, err := svc.Validate(context.Background(), "INVALID", 100.0, "")
	require.NoError(t, err) // not an error, just invalid
	assert.False(t, got.Valid)
	assert.Contains(t, got.Message, "invalide")
}

func TestPromoService_Validate_ExpiredCode(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	past := time.Now().Add(-24 * time.Hour)
	store.On("GetByCode", mock.Anything, "EXPIRED").
		Return(&interfaces.PromoCode{
			Code:      "EXPIRED",
			IsActive:  true,
			ExpiresAt: &past,
		}, nil)

	got, err := svc.Validate(context.Background(), "EXPIRED", 100.0, "")
	require.NoError(t, err)
	assert.False(t, got.Valid)
	assert.Contains(t, got.Message, "expiré")
}

func TestPromoService_Validate_InactiveCode(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "DISABLED").
		Return(&interfaces.PromoCode{
			Code:     "DISABLED",
			IsActive: false,
		}, nil)

	got, err := svc.Validate(context.Background(), "DISABLED", 100.0, "")
	require.NoError(t, err)
	assert.False(t, got.Valid)
}

func TestPromoService_Validate_MinOrderNotReached(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "MIN50").
		Return(&interfaces.PromoCode{
			Code:          "MIN50",
			DiscountType:  interfaces.PromoTypeFixed,
			DiscountValue: 10,
			MinOrderTotal: 50,
			IsActive:      true,
		}, nil)

	got, err := svc.Validate(context.Background(), "MIN50", 30.0, "")
	require.NoError(t, err)
	assert.False(t, got.Valid)
	assert.Contains(t, got.Message, "minimum")
}

func TestPromoService_Validate_PerUserLimitReached(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	promoID := uuid.New()
	limit := 1
	store.On("GetByCode", mock.Anything, "ONCE").
		Return(&interfaces.PromoCode{
			ID:            promoID,
			Code:          "ONCE",
			DiscountType:  interfaces.PromoTypeFixed,
			DiscountValue: 10,
			PerUserLimit:  &limit,
			IsActive:      true,
		}, nil)
	store.On("CountUserUsage", mock.Anything, promoID.String(), "user-1").
		Return(int64(1), nil)

	got, err := svc.Validate(context.Background(), "ONCE", 100.0, "user-1")
	require.NoError(t, err)
	assert.False(t, got.Valid)
	assert.Contains(t, got.Message, "utilisateur")
}

func TestPromoService_Validate_StoreError(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "ERR").
		Return((*interfaces.PromoCode)(nil), errors.New("db error"))

	_, err := svc.Validate(context.Background(), "ERR", 100.0, "")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// PromoService.MarkUsed
// ---------------------------------------------------------------------------

func TestPromoService_MarkUsed_Success(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	promoID := uuid.New()
	promo := &interfaces.PromoCode{ID: promoID, Code: "USED"}
	userID := uuid.New().String()
	orderID := uuid.New().String()

	store.On("RecordUsage", mock.Anything, mock.AnythingOfType("*interfaces.PromoUsage")).Return(nil)
	store.On("IncrementUsage", mock.Anything, promoID.String()).Return(nil)

	err := svc.MarkUsed(context.Background(), promo, userID, orderID, 15.0)
	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestPromoService_MarkUsed_EmptyUserAndOrder(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	promoID := uuid.New()
	promo := &interfaces.PromoCode{ID: promoID}

	store.On("RecordUsage", mock.Anything, mock.AnythingOfType("*interfaces.PromoUsage")).Return(nil)
	store.On("IncrementUsage", mock.Anything, promoID.String()).Return(nil)

	err := svc.MarkUsed(context.Background(), promo, "", "", 10.0)
	require.NoError(t, err)
}

func TestPromoService_MarkUsed_InvalidUserID(t *testing.T) {
	t.Parallel()
	svc := NewPromoService(new(MockPromoStore), nil)

	promo := &interfaces.PromoCode{ID: uuid.New()}
	err := svc.MarkUsed(context.Background(), promo, "bad-uuid", "", 10.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}

// ---------------------------------------------------------------------------
// PromoService.List
// ---------------------------------------------------------------------------

func TestPromoService_List_Success(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("List", mock.Anything).Return([]*interfaces.PromoCode{
		{Code: "A"}, {Code: "B"},
	}, nil)

	got, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

// ---------------------------------------------------------------------------
// PromoService.Delete
// ---------------------------------------------------------------------------

func TestPromoService_Delete_Success(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("Delete", mock.Anything, "id-1").Return(nil)

	err := svc.Delete(context.Background(), "id-1")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// PromoService.GetByCode
// ---------------------------------------------------------------------------

func TestPromoService_GetByCode_Success(t *testing.T) {
	t.Parallel()
	store := new(MockPromoStore)
	svc := NewPromoService(store, nil)

	store.On("GetByCode", mock.Anything, "CODE1").
		Return(&interfaces.PromoCode{Code: "CODE1"}, nil)

	got, err := svc.GetByCode(context.Background(), "CODE1")
	require.NoError(t, err)
	assert.Equal(t, "CODE1", got.Code)
}
