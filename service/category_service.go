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

// CategoryService handles category business logic
type CategoryService struct {
	store  types.CategoryStore
	logger *logrus.Logger
}

// NewCategoryService constructs a CategoryService
func NewCategoryService(store types.CategoryStore, logger *logrus.Logger) *CategoryService {
	if logger == nil {
		logger = logrus.New()
	}
	return &CategoryService{store: store, logger: logger}
}

// Create creates a category
func (cs *CategoryService) Create(ctx context.Context, req *api.CreateCategoryRequest) (*interfaces.Category, error) {
	cat := &interfaces.Category{
		Slug:        text.Slugify(req.Name),
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		Position:    req.Position,
		IsActive:    req.IsActive,
		Metadata:    req.Metadata,
	}
	if req.ParentID != nil && *req.ParentID != "" {
		parentID, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent_id: %w", err)
		}
		cat.ParentID = &parentID
	}

	created, err := cs.store.Create(ctx, cat)
	if err != nil {
		cs.logger.WithError(err).Error("Failed to create category")
		return nil, err
	}
	return created, nil
}

// Get returns a category by ID
func (cs *CategoryService) Get(ctx context.Context, id string) (*interfaces.Category, error) {
	c, err := cs.store.GetByID(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	return c, nil
}

// GetBySlug returns a category by slug
func (cs *CategoryService) GetBySlug(ctx context.Context, slug string) (*interfaces.Category, error) {
	c, err := cs.store.GetBySlug(ctx, slug)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	return c, nil
}

// List returns all active categories
func (cs *CategoryService) List(ctx context.Context) ([]*interfaces.Category, error) {
	return cs.store.List(ctx)
}

// ListTree returns active top-level categories with their sub-categories nested.
func (cs *CategoryService) ListTree(ctx context.Context) ([]*interfaces.Category, error) {
	return cs.store.ListTree(ctx)
}

// SeedDefaults idempotently ensures the default category taxonomy exists.
func (cs *CategoryService) SeedDefaults(ctx context.Context) error {
	return cs.store.SeedDefaultCategories(ctx)
}

// Update applies the requested updates to a category
func (cs *CategoryService) Update(ctx context.Context, id string, req *api.UpdateCategoryRequest) (*interfaces.Category, error) {
	fields := map[string]interface{}{}
	if req.Name != nil {
		fields["name"] = *req.Name
		fields["slug"] = text.Slugify(*req.Name)
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.ImageURL != nil {
		fields["image_url"] = *req.ImageURL
	}
	if req.Position != nil {
		fields["position"] = *req.Position
	}
	if req.IsActive != nil {
		fields["is_active"] = *req.IsActive
	}
	if req.ParentID != nil {
		if *req.ParentID == "" {
			fields["parent_id"] = nil
		} else {
			pid, err := uuid.Parse(*req.ParentID)
			if err != nil {
				return nil, fmt.Errorf("invalid parent_id: %w", err)
			}
			fields["parent_id"] = pid
		}
	}
	if req.Metadata != nil {
		fields["metadata"] = *req.Metadata
	}
	return cs.store.Update(ctx, id, fields)
}

// Delete removes a category
func (cs *CategoryService) Delete(ctx context.Context, id string) error {
	return cs.store.Delete(ctx, id)
}
