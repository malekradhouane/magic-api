package types

import (
	"context"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ProductStore manages product persistence
type ProductStore interface {
	Create(ctx context.Context, product *interfaces.Product, images []interfaces.ProductImage, sizes []interfaces.ProductSize, colors []interfaces.ProductColor) (*interfaces.Product, error)
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
	DecrementSizeStock(ctx context.Context, productID, size string, quantity int) error
}

// CategoryStore manages category persistence
type CategoryStore interface {
	Create(ctx context.Context, category *interfaces.Category) (*interfaces.Category, error)
	GetByID(ctx context.Context, id string) (*interfaces.Category, error)
	GetBySlug(ctx context.Context, slug string) (*interfaces.Category, error)
	List(ctx context.Context) ([]*interfaces.Category, error)
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

// PromoStore manages promo code persistence
type PromoStore interface {
	Create(ctx context.Context, promo *interfaces.PromoCode) (*interfaces.PromoCode, error)
	GetByCode(ctx context.Context, code string) (*interfaces.PromoCode, error)
	List(ctx context.Context) ([]*interfaces.PromoCode, error)
	IncrementUsage(ctx context.Context, promoID string) error
	RecordUsage(ctx context.Context, usage *interfaces.PromoUsage) error
	CountUserUsage(ctx context.Context, promoID, userID string) (int64, error)
	Delete(ctx context.Context, id string) error
}
