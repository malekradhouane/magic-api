package types

import (
	"context"
	"time"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// StatsStore exposes read-only aggregation queries for the admin dashboard.
type StatsStore interface {
	Kpis(ctx context.Context, since, prevSince, prevUntil time.Time, lowStockThreshold int) (*api.StatsKpis, error)
	Sparklines(ctx context.Context, since time.Time) (*api.StatsSparklines, error)
	OrdersByStatus(ctx context.Context, since time.Time) ([]api.StatusCount, error)
	RevenueByDay(ctx context.Context, since time.Time) ([]api.DayPoint, error)
	TopGouvernorats(ctx context.Context, since time.Time, limit int) ([]api.GouvernoratStat, error)
	StockAlerts(ctx context.Context, threshold, limit int) ([]api.StockAlert, error)
	TopProducts(ctx context.Context, since time.Time, limit int) ([]api.ProductPerf, error)
	Conversion(ctx context.Context, minViews, limit int) ([]api.ProductConversion, error)
	Deadstock(ctx context.Context, days, limit int) ([]api.DeadstockProduct, error)
	PaymentMethods(ctx context.Context, since time.Time) ([]api.PaymentMethodStat, error)
}

// ProductStore manages product persistence
type ProductStore interface {
	Create(ctx context.Context, product *interfaces.Product, images []interfaces.ProductImage, sizes []interfaces.ProductSize, colors []interfaces.ProductColor, variants []interfaces.ProductVariant) (*interfaces.Product, error)
	GetByID(ctx context.Context, id string) (*interfaces.Product, error)
	GetBySlug(ctx context.Context, slug string) (*interfaces.Product, error)
	List(ctx context.Context, filters *api.ProductFilters) ([]*interfaces.Product, int64, error)
	GetSimilar(ctx context.Context, productID string, limit int) ([]*interfaces.Product, error)
	Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Product, error)
	Delete(ctx context.Context, id string) error
	IncrementViewCount(ctx context.Context, id string) error

	// Variants management
	UpsertImages(ctx context.Context, productID string, images []interfaces.ProductImage) error
	UpsertSizes(ctx context.Context, productID string, sizes []interfaces.ProductSize) error
	UpsertColors(ctx context.Context, productID string, colors []interfaces.ProductColor) error
	UpsertVariants(ctx context.Context, productID string, variants []interfaces.ProductVariant) error
	DecrementVariantStock(ctx context.Context, productID, size, color string, quantity int) error
	IncrementVariantStock(ctx context.Context, productID, size, color string, quantity int) error
}

// CategoryStore manages category persistence
type CategoryStore interface {
	Create(ctx context.Context, category *interfaces.Category) (*interfaces.Category, error)
	GetByID(ctx context.Context, id string) (*interfaces.Category, error)
	GetBySlug(ctx context.Context, slug string) (*interfaces.Category, error)
	List(ctx context.Context) ([]*interfaces.Category, error)
	ListTree(ctx context.Context) ([]*interfaces.Category, error)
	SeedDefaultCategories(ctx context.Context) error
	Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Category, error)
	Delete(ctx context.Context, id string) error
}

// OrderStore manages order persistence
type OrderStore interface {
	Create(ctx context.Context, order *interfaces.Order, items []interfaces.OrderItem) (*interfaces.Order, error)
	GetByID(ctx context.Context, id string) (*interfaces.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*interfaces.Order, error)
	GetByPhone(ctx context.Context, orderID, phone string) (*interfaces.Order, error)
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*interfaces.Order, int64, error)
	List(ctx context.Context, filters *api.ListOrdersFilters) ([]*interfaces.Order, int64, error)
	UpdateStatus(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Order, error)
}

// AddressStore manages user shipping addresses
type AddressStore interface {
	ListByUserID(ctx context.Context, userID string) ([]*interfaces.Address, error)
	GetByID(ctx context.Context, userID, id string) (*interfaces.Address, error)
	FindMatching(ctx context.Context, userID, phone, gouvernorat, addressLine string) (*interfaces.Address, error)
	Create(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error)
	Update(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error)
	SetDefault(ctx context.Context, userID, id string) error
	Delete(ctx context.Context, userID, id string) error
}

// SettingsStore manages application settings (key-value JSONB rows)
type SettingsStore interface {
	GetAll(ctx context.Context) ([]*interfaces.Setting, error)
	GetByKey(ctx context.Context, key string) (*interfaces.Setting, error)
	Upsert(ctx context.Context, key string, value interfaces.SettingsValue) (*interfaces.Setting, error)
}

// PromoStore manages promo code persistence
type PromoStore interface {
	Create(ctx context.Context, promo *interfaces.PromoCode) (*interfaces.PromoCode, error)
	GetByCode(ctx context.Context, code string) (*interfaces.PromoCode, error)
	GetByID(ctx context.Context, id string) (*interfaces.PromoCode, error)
	List(ctx context.Context) ([]*interfaces.PromoCode, error)
	Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.PromoCode, error)
	IncrementUsage(ctx context.Context, promoID string) error
	RecordUsage(ctx context.Context, usage *interfaces.PromoUsage) error
	CountUserUsage(ctx context.Context, promoID, userID string) (int64, error)
	Delete(ctx context.Context, id string) error
}
