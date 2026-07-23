package service

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// MockCategoryStore
// ---------------------------------------------------------------------------

type MockCategoryStore struct{ mock.Mock }

func (m *MockCategoryStore) Create(ctx context.Context, category *interfaces.Category) (*interfaces.Category, error) {
	args := m.Called(ctx, category)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) GetByID(ctx context.Context, id string) (*interfaces.Category, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) GetBySlug(ctx context.Context, slug string) (*interfaces.Category, error) {
	args := m.Called(ctx, slug)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) List(ctx context.Context) ([]*interfaces.Category, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) ListTree(ctx context.Context) ([]*interfaces.Category, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) SeedDefaultCategories(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *MockCategoryStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Category, error) {
	args := m.Called(ctx, id, fields)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockCategoryStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// MockProductStore
// ---------------------------------------------------------------------------

type MockProductStore struct{ mock.Mock }

func (m *MockProductStore) Create(ctx context.Context, product *interfaces.Product, images []interfaces.ProductImage, sizes []interfaces.ProductSize, colors []interfaces.ProductColor, variants []interfaces.ProductVariant) (*interfaces.Product, error) {
	args := m.Called(ctx, product, images, sizes, colors, variants)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Product), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockProductStore) GetByID(ctx context.Context, id string) (*interfaces.Product, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Product), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockProductStore) GetBySlug(ctx context.Context, slug string) (*interfaces.Product, error) {
	args := m.Called(ctx, slug)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Product), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockProductStore) List(ctx context.Context, filters *api.ProductFilters) ([]*interfaces.Product, int64, error) {
	args := m.Called(ctx, filters)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Product), args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}
func (m *MockProductStore) GetSimilar(ctx context.Context, productID string, limit int) ([]*interfaces.Product, error) {
	args := m.Called(ctx, productID, limit)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Product), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockProductStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Product, error) {
	args := m.Called(ctx, id, fields)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Product), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockProductStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockProductStore) IncrementViewCount(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockProductStore) UpsertImages(ctx context.Context, productID string, images []interfaces.ProductImage) error {
	args := m.Called(ctx, productID, images)
	return args.Error(0)
}
func (m *MockProductStore) UpsertSizes(ctx context.Context, productID string, sizes []interfaces.ProductSize) error {
	args := m.Called(ctx, productID, sizes)
	return args.Error(0)
}
func (m *MockProductStore) UpsertColors(ctx context.Context, productID string, colors []interfaces.ProductColor) error {
	args := m.Called(ctx, productID, colors)
	return args.Error(0)
}
func (m *MockProductStore) UpsertVariants(ctx context.Context, productID string, variants []interfaces.ProductVariant) error {
	args := m.Called(ctx, productID, variants)
	return args.Error(0)
}
func (m *MockProductStore) DecrementVariantStock(ctx context.Context, productID, size, color string, quantity int) error {
	args := m.Called(ctx, productID, size, color, quantity)
	return args.Error(0)
}
func (m *MockProductStore) IncrementVariantStock(ctx context.Context, productID, size, color string, quantity int) error {
	args := m.Called(ctx, productID, size, color, quantity)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// MockOrderStore
// ---------------------------------------------------------------------------

type MockOrderStore struct{ mock.Mock }

func (m *MockOrderStore) Create(ctx context.Context, order *interfaces.Order, items []interfaces.OrderItem) (*interfaces.Order, error) {
	args := m.Called(ctx, order, items)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Order), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOrderStore) GetByID(ctx context.Context, id string) (*interfaces.Order, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Order), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOrderStore) GetByOrderNumber(ctx context.Context, orderNumber string) (*interfaces.Order, error) {
	args := m.Called(ctx, orderNumber)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Order), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOrderStore) GetByPhone(ctx context.Context, orderID, phone string) (*interfaces.Order, error) {
	args := m.Called(ctx, orderID, phone)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Order), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockOrderStore) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*interfaces.Order, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Order), args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}
func (m *MockOrderStore) List(ctx context.Context, filters *api.ListOrdersFilters) ([]*interfaces.Order, int64, error) {
	args := m.Called(ctx, filters)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Order), args.Get(1).(int64), args.Error(2)
	}
	return nil, args.Get(1).(int64), args.Error(2)
}
func (m *MockOrderStore) UpdateStatus(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Order, error) {
	args := m.Called(ctx, id, fields)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Order), args.Error(1)
	}
	return nil, args.Error(1)
}

// ---------------------------------------------------------------------------
// MockAddressStore
// ---------------------------------------------------------------------------

type MockAddressStore struct{ mock.Mock }

func (m *MockAddressStore) ListByUserID(ctx context.Context, userID string) ([]*interfaces.Address, error) {
	args := m.Called(ctx, userID)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.Address), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAddressStore) GetByID(ctx context.Context, userID, id string) (*interfaces.Address, error) {
	args := m.Called(ctx, userID, id)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Address), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAddressStore) FindMatching(ctx context.Context, userID, phone, gouvernorat, addressLine string) (*interfaces.Address, error) {
	args := m.Called(ctx, userID, phone, gouvernorat, addressLine)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Address), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAddressStore) Create(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error) {
	args := m.Called(ctx, addr)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Address), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAddressStore) Update(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error) {
	args := m.Called(ctx, addr)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.Address), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockAddressStore) SetDefault(ctx context.Context, userID, id string) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}
func (m *MockAddressStore) Delete(ctx context.Context, userID, id string) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// MockPromoStore
// ---------------------------------------------------------------------------

type MockPromoStore struct{ mock.Mock }

func (m *MockPromoStore) Create(ctx context.Context, promo *interfaces.PromoCode) (*interfaces.PromoCode, error) {
	args := m.Called(ctx, promo)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.PromoCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPromoStore) GetByCode(ctx context.Context, code string) (*interfaces.PromoCode, error) {
	args := m.Called(ctx, code)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.PromoCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPromoStore) GetByID(ctx context.Context, id string) (*interfaces.PromoCode, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.PromoCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPromoStore) List(ctx context.Context) ([]*interfaces.PromoCode, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]*interfaces.PromoCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPromoStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.PromoCode, error) {
	args := m.Called(ctx, id, fields)
	if v := args.Get(0); v != nil {
		return v.(*interfaces.PromoCode), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockPromoStore) IncrementUsage(ctx context.Context, promoID string) error {
	args := m.Called(ctx, promoID)
	return args.Error(0)
}
func (m *MockPromoStore) RecordUsage(ctx context.Context, usage *interfaces.PromoUsage) error {
	args := m.Called(ctx, usage)
	return args.Error(0)
}
func (m *MockPromoStore) CountUserUsage(ctx context.Context, promoID, userID string) (int64, error) {
	args := m.Called(ctx, promoID, userID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockPromoStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---------------------------------------------------------------------------
// MockStatsStore
// ---------------------------------------------------------------------------

type MockStatsStore struct{ mock.Mock }

func (m *MockStatsStore) Kpis(ctx context.Context, since, prevSince, prevUntil time.Time, lowStockThreshold int) (*api.StatsKpis, error) {
	args := m.Called(ctx, since, prevSince, prevUntil, lowStockThreshold)
	if v := args.Get(0); v != nil {
		return v.(*api.StatsKpis), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) Sparklines(ctx context.Context, since time.Time) (*api.StatsSparklines, error) {
	args := m.Called(ctx, since)
	if v := args.Get(0); v != nil {
		return v.(*api.StatsSparklines), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) OrdersByStatus(ctx context.Context, since time.Time) ([]api.StatusCount, error) {
	args := m.Called(ctx, since)
	if v := args.Get(0); v != nil {
		return v.([]api.StatusCount), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) RevenueByDay(ctx context.Context, since time.Time) ([]api.DayPoint, error) {
	args := m.Called(ctx, since)
	if v := args.Get(0); v != nil {
		return v.([]api.DayPoint), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) TopGouvernorats(ctx context.Context, since time.Time, limit int) ([]api.GouvernoratStat, error) {
	args := m.Called(ctx, since, limit)
	if v := args.Get(0); v != nil {
		return v.([]api.GouvernoratStat), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) StockAlerts(ctx context.Context, threshold, limit int) ([]api.StockAlert, error) {
	args := m.Called(ctx, threshold, limit)
	if v := args.Get(0); v != nil {
		return v.([]api.StockAlert), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) TopProducts(ctx context.Context, since time.Time, limit int) ([]api.ProductPerf, error) {
	args := m.Called(ctx, since, limit)
	if v := args.Get(0); v != nil {
		return v.([]api.ProductPerf), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) Conversion(ctx context.Context, minViews, limit int) ([]api.ProductConversion, error) {
	args := m.Called(ctx, minViews, limit)
	if v := args.Get(0); v != nil {
		return v.([]api.ProductConversion), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) Deadstock(ctx context.Context, days, limit int) ([]api.DeadstockProduct, error) {
	args := m.Called(ctx, days, limit)
	if v := args.Get(0); v != nil {
		return v.([]api.DeadstockProduct), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockStatsStore) PaymentMethods(ctx context.Context, since time.Time) ([]api.PaymentMethodStat, error) {
	args := m.Called(ctx, since)
	if v := args.Get(0); v != nil {
		return v.([]api.PaymentMethodStat), args.Error(1)
	}
	return nil, args.Error(1)
}

// ---------------------------------------------------------------------------
// MockMailer
// ---------------------------------------------------------------------------

type MockMailer struct{ mock.Mock }

func (m *MockMailer) Send(ctx context.Context, fromName, fromEmail, toName, toEmail, subject, textPart, htmlPart string) error {
	args := m.Called(ctx, fromName, fromEmail, toName, toEmail, subject, textPart, htmlPart)
	return args.Error(0)
}
