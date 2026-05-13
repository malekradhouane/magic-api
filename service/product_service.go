package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
	"github.com/malekradhouane/magic/utils/text"
)

// ProductService handles product business logic
type ProductService struct {
	store  types.ProductStore
	logger *logrus.Logger
}

// NewProductService constructs a ProductService
func NewProductService(store types.ProductStore, logger *logrus.Logger) *ProductService {
	if logger == nil {
		logger = logrus.New()
	}
	return &ProductService{store: store, logger: logger}
}

// Create persists a product with its variants
func (ps *ProductService) Create(ctx context.Context, req *api.CreateProductRequest) (*interfaces.Product, error) {
	product := &interfaces.Product{
		Slug:            text.Slugify(req.Name),
		Name:            req.Name,
		SKU:             req.SKU,
		Description:     req.Description,
		DescriptionLong: req.DescriptionLong,
		Price:           req.Price,
		OriginalPrice:   req.OriginalPrice,
		Currency:        "TND",
		IsNew:           req.IsNew,
		IsOnSale:        req.IsOnSale,
		IsActive:        req.IsActive,
		IsFeatured:      req.IsFeatured,
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		Tags:            interfaces.StringArray(req.Tags),
	}
	if req.CategoryID != nil && *req.CategoryID != "" {
		cid, err := uuid.Parse(*req.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("invalid category_id: %w", err)
		}
		product.CategoryID = &cid
	}

	images := make([]interfaces.ProductImage, 0, len(req.Images))
	for _, img := range req.Images {
		images = append(images, interfaces.ProductImage{
			URL:       img.URL,
			Alt:       img.Alt,
			Position:  img.Position,
			IsPrimary: img.IsPrimary,
		})
	}

	sizes := make([]interfaces.ProductSize, 0, len(req.Sizes))
	for _, s := range req.Sizes {
		sizes = append(sizes, interfaces.ProductSize{
			Size:     s.Size,
			Stock:    s.Stock,
			Position: s.Position,
		})
	}

	colors := make([]interfaces.ProductColor, 0, len(req.Colors))
	for _, c := range req.Colors {
		colors = append(colors, interfaces.ProductColor{
			Name:     c.Name,
			Hex:      c.Hex,
			Stock:    c.Stock,
			Position: c.Position,
		})
	}

	created, err := ps.store.Create(ctx, product, images, sizes, colors)
	if err != nil {
		ps.logger.WithError(err).Error("Failed to create product")
		return nil, err
	}
	return created, nil
}

// Get returns a product by ID (and increments view count)
func (ps *ProductService) Get(ctx context.Context, id string) (*interfaces.Product, error) {
	product, err := ps.store.GetByID(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	go func() {
		_ = ps.store.IncrementViewCount(context.Background(), id)
	}()
	return product, nil
}

// GetBySlug returns a product by slug (and increments view count)
func (ps *ProductService) GetBySlug(ctx context.Context, slug string) (*interfaces.Product, error) {
	product, err := ps.store.GetBySlug(ctx, slug)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	go func(id string) {
		_ = ps.store.IncrementViewCount(context.Background(), id)
	}(product.ID.String())
	return product, nil
}

// List returns paginated, filtered products
func (ps *ProductService) List(ctx context.Context, filters *api.ProductFilters) (*api.ProductsResponse, error) {
	products, total, err := ps.store.List(ctx, filters)
	if err != nil {
		return nil, err
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	return &api.ProductsResponse{
		Products:   products,
		TotalCount: total,
		HasMore:    int64(filters.Offset+limit) < total,
		Limit:      limit,
		Offset:     filters.Offset,
	}, nil
}

// GetSimilar returns related products
func (ps *ProductService) GetSimilar(ctx context.Context, productID string, limit int) ([]*interfaces.Product, error) {
	return ps.store.GetSimilar(ctx, productID, limit)
}

// Update applies field updates to a product
func (ps *ProductService) Update(ctx context.Context, id string, req *api.UpdateProductRequest) (*interfaces.Product, error) {
	fields := map[string]interface{}{}
	if req.Name != nil {
		fields["name"] = *req.Name
		fields["slug"] = text.Slugify(*req.Name)
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.DescriptionLong != nil {
		fields["description_long"] = *req.DescriptionLong
	}
	if req.Price != nil {
		fields["price"] = *req.Price
	}
	if req.OriginalPrice != nil {
		fields["original_price"] = *req.OriginalPrice
	}
	if req.CategoryID != nil {
		if *req.CategoryID == "" {
			fields["category_id"] = nil
		} else {
			cid, err := uuid.Parse(*req.CategoryID)
			if err != nil {
				return nil, fmt.Errorf("invalid category_id: %w", err)
			}
			fields["category_id"] = cid
		}
	}
	if req.IsNew != nil {
		fields["is_new"] = *req.IsNew
	}
	if req.IsOnSale != nil {
		fields["is_on_sale"] = *req.IsOnSale
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}
	if req.IsFeatured != nil {
		fields["is_featured"] = *req.IsFeatured
	}
	if req.Tags != nil {
		fields["tags"] = interfaces.StringArray(req.Tags)
	}

	if len(fields) == 0 {
		return ps.store.GetByID(ctx, id)
	}
	return ps.store.Update(ctx, id, fields)
}

// Delete soft-deletes a product
func (ps *ProductService) Delete(ctx context.Context, id string) error {
	return ps.store.Delete(ctx, id)
}
