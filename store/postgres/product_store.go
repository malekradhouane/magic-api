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

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

var (
	_ types.ProductStore = &ProductStore{}

	theProductStoreMtx sync.Mutex
	theProductStore    *ProductStore
)

// ProductStore is the PostgreSQL implementation of ProductStore
type ProductStore struct {
	*Client
}

// NewProductStore creates the singleton ProductStore
func NewProductStore() (*ProductStore, error) {
	theProductStoreMtx.Lock()
	defer theProductStoreMtx.Unlock()

	if theProductStore != nil {
		return theProductStore, nil
	}
	MustClientInitialized(client)
	theProductStore = &ProductStore{Client: client}

	logrus.Info("ProductStore created")
	return theProductStore, nil
}

// Create persists a product with its variants in a single transaction
func (ps *ProductStore) Create(ctx context.Context, product *interfaces.Product, images []interfaces.ProductImage, sizes []interfaces.ProductSize, colors []interfaces.ProductColor) (*interfaces.Product, error) {
	if product == nil {
		return nil, fmt.Errorf("product is nil")
	}
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}

	err := withTransaction(ps.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Create(product).Error; err != nil {
			return fmt.Errorf("failed to create product: %w", err)
		}

		for i := range images {
			images[i].ProductID = product.ID
			if images[i].ID == uuid.Nil {
				images[i].ID = uuid.New()
			}
		}
		if len(images) > 0 {
			if err := tx.Create(&images).Error; err != nil {
				return fmt.Errorf("failed to create product images: %w", err)
			}
		}

		for i := range sizes {
			sizes[i].ProductID = product.ID
			if sizes[i].ID == uuid.Nil {
				sizes[i].ID = uuid.New()
			}
		}
		if len(sizes) > 0 {
			if err := tx.Create(&sizes).Error; err != nil {
				return fmt.Errorf("failed to create product sizes: %w", err)
			}
		}

		for i := range colors {
			colors[i].ProductID = product.ID
			if colors[i].ID == uuid.Nil {
				colors[i].ID = uuid.New()
			}
		}
		if len(colors) > 0 {
			if err := tx.Create(&colors).Error; err != nil {
				return fmt.Errorf("failed to create product colors: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ps.GetByID(ctx, product.ID.String())
}

// GetByID returns a product with all variants preloaded
func (ps *ProductStore) GetByID(ctx context.Context, id string) (*interfaces.Product, error) {
	p := new(interfaces.Product)
	err := ps.session.GetDB().WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Sizes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Colors", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Category").
		Where("id = ?", id).
		Take(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	return p, nil
}

// GetBySlug returns a product by slug with all variants preloaded
func (ps *ProductStore) GetBySlug(ctx context.Context, slug string) (*interfaces.Product, error) {
	p := new(interfaces.Product)
	err := ps.session.GetDB().WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Sizes", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Colors", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Category").
		Where("slug = ?", slug).
		Take(p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get product by slug: %w", err)
	}
	return p, nil
}

// List returns paginated, filtered products
func (ps *ProductStore) List(ctx context.Context, filters *api.ProductFilters) ([]*interfaces.Product, int64, error) {
	if filters == nil {
		filters = &api.ProductFilters{Limit: 20, Offset: 0}
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}

	db := ps.session.GetDB().WithContext(ctx).Model(&interfaces.Product{}).
		Where("products.is_active = ? AND products.deleted_at IS NULL", true)

	// Category filter (by slug)
	if filters.Category != "" && filters.Category != "all" {
		switch filters.Category {
		case "new":
			db = db.Where("is_new = ?", true)
		case "soldes", "sale":
			db = db.Where("is_on_sale = ?", true)
		default:
			db = db.Joins("LEFT JOIN categories c ON c.id = products.category_id").
				Where("c.slug = ?", filters.Category)
		}
	}

	if filters.IsNew != nil {
		db = db.Where("is_new = ?", *filters.IsNew)
	}
	if filters.IsOnSale != nil {
		db = db.Where("is_on_sale = ?", *filters.IsOnSale)
	}
	if filters.IsFeatured != nil {
		db = db.Where("is_featured = ?", *filters.IsFeatured)
	}

	if filters.MinPrice != nil {
		db = db.Where("price >= ?", *filters.MinPrice)
	}
	if filters.MaxPrice != nil {
		db = db.Where("price <= ?", *filters.MaxPrice)
	}

	// Size / color filters via subquery
	if len(filters.Sizes) > 0 {
		db = db.Where("EXISTS (SELECT 1 FROM product_sizes ps WHERE ps.product_id = products.id AND ps.size IN ? AND ps.stock > 0)", filters.Sizes)
	}
	if len(filters.Colors) > 0 {
		db = db.Where("EXISTS (SELECT 1 FROM product_colors pc WHERE pc.product_id = products.id AND pc.name IN ? AND pc.stock > 0)", filters.Colors)
	}

	if filters.Search != "" {
		searchPattern := "%" + strings.ToLower(filters.Search) + "%"
		db = db.Where(
			"LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR full_text_search @@ plainto_tsquery('simple', ?)",
			searchPattern, searchPattern, filters.Search,
		)
	}

	// Count total
	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// Sorting
	switch filters.Sort {
	case "newest":
		db = db.Order("products.created_at DESC")
	case "price-asc":
		db = db.Order("products.price ASC")
	case "price-desc":
		db = db.Order("products.price DESC")
	default: // relevance
		db = db.Order("products.is_featured DESC, products.created_at DESC")
	}

	var products []*interfaces.Product
	err := db.
		Preload("Images", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Preload("Sizes", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Preload("Colors", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Preload("Category").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&products).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list products: %w", err)
	}
	return products, totalCount, nil
}

// GetSimilar returns products in the same category (excluding the current one)
func (ps *ProductStore) GetSimilar(ctx context.Context, productID string, limit int) ([]*interfaces.Product, error) {
	if limit <= 0 {
		limit = 4
	}

	current, err := ps.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}

	var products []*interfaces.Product
	q := ps.session.GetDB().WithContext(ctx).Model(&interfaces.Product{}).
		Where("id <> ? AND is_active = ? AND deleted_at IS NULL", productID, true)

	if current.CategoryID != nil {
		q = q.Where("category_id = ?", *current.CategoryID)
	}

	if err := q.
		Preload("Images", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Preload("Colors", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Order("created_at DESC").
		Limit(limit).
		Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to get similar products: %w", err)
	}
	return products, nil
}

// Update applies the given fields to a product
func (ps *ProductStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Product, error) {
	if id == "" {
		return nil, fmt.Errorf("product ID is required")
	}
	if len(fields) == 0 {
		return nil, errs.ErrEmptyUpdate
	}

	db := ps.session.GetDB().WithContext(ctx)
	if err := db.Model(&interfaces.Product{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	return ps.GetByID(ctx, id)
}

// Delete soft-deletes a product
func (ps *ProductStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("product ID is required")
	}
	if err := ps.session.GetDB().WithContext(ctx).
		Where("id = ?", id).
		Delete(&interfaces.Product{}).Error; err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	return nil
}

// IncrementViewCount increments the view counter
func (ps *ProductStore) IncrementViewCount(ctx context.Context, id string) error {
	return ps.session.GetDB().WithContext(ctx).
		Model(&interfaces.Product{}).
		Where("id = ?", id).
		Update("view_count", gorm.Expr("view_count + 1")).Error
}

// UpsertImages replaces all images for a product
func (ps *ProductStore) UpsertImages(ctx context.Context, productID string, images []interfaces.ProductImage) error {
	return withTransaction(ps.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", productID).Delete(&interfaces.ProductImage{}).Error; err != nil {
			return fmt.Errorf("failed to clear images: %w", err)
		}
		if len(images) == 0 {
			return nil
		}
		pid, err := uuid.Parse(productID)
		if err != nil {
			return fmt.Errorf("invalid product ID: %w", err)
		}
		for i := range images {
			images[i].ProductID = pid
		}
		return tx.Create(&images).Error
	})
}

// UpsertSizes replaces all sizes for a product
func (ps *ProductStore) UpsertSizes(ctx context.Context, productID string, sizes []interfaces.ProductSize) error {
	return withTransaction(ps.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", productID).Delete(&interfaces.ProductSize{}).Error; err != nil {
			return fmt.Errorf("failed to clear sizes: %w", err)
		}
		if len(sizes) == 0 {
			return nil
		}
		pid, err := uuid.Parse(productID)
		if err != nil {
			return fmt.Errorf("invalid product ID: %w", err)
		}
		for i := range sizes {
			sizes[i].ProductID = pid
		}
		return tx.Create(&sizes).Error
	})
}

// UpsertColors replaces all colors for a product
func (ps *ProductStore) UpsertColors(ctx context.Context, productID string, colors []interfaces.ProductColor) error {
	return withTransaction(ps.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", productID).Delete(&interfaces.ProductColor{}).Error; err != nil {
			return fmt.Errorf("failed to clear colors: %w", err)
		}
		if len(colors) == 0 {
			return nil
		}
		pid, err := uuid.Parse(productID)
		if err != nil {
			return fmt.Errorf("invalid product ID: %w", err)
		}
		for i := range colors {
			colors[i].ProductID = pid
		}
		return tx.Create(&colors).Error
	})
}

// DecrementSizeStock atomically decrements stock for a given (product, size)
func (ps *ProductStore) DecrementSizeStock(ctx context.Context, productID, size string, quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("invalid quantity: %d", quantity)
	}
	res := ps.session.GetDB().WithContext(ctx).
		Model(&interfaces.ProductSize{}).
		Where("product_id = ? AND size = ? AND stock >= ?", productID, size, quantity).
		Update("stock", gorm.Expr("stock - ?", quantity))
	if res.Error != nil {
		return fmt.Errorf("failed to decrement size stock: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("insufficient stock for product %s size %s", productID, size)
	}
	return nil
}

// IncrementSizeStock restores stock for a given (product, size) — e.g. order cancellation
func (ps *ProductStore) IncrementSizeStock(ctx context.Context, productID, size string, quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("invalid quantity: %d", quantity)
	}
	res := ps.session.GetDB().WithContext(ctx).
		Model(&interfaces.ProductSize{}).
		Where("product_id = ? AND size = ?", productID, size).
		Update("stock", gorm.Expr("stock + ?", quantity))
	if res.Error != nil {
		return fmt.Errorf("failed to increment size stock: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("size variant not found for product %s size %s", productID, size)
	}
	return nil
}
