package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

var (
	_ types.CategoryStore = &CategoryStore{}

	theCategoryStoreMtx sync.Mutex
	theCategoryStore    *CategoryStore
)

// CategoryStore is the PostgreSQL implementation of CategoryStore
type CategoryStore struct {
	*Client
}

// NewCategoryStore creates the singleton CategoryStore
func NewCategoryStore() (*CategoryStore, error) {
	theCategoryStoreMtx.Lock()
	defer theCategoryStoreMtx.Unlock()

	if theCategoryStore != nil {
		return theCategoryStore, nil
	}
	MustClientInitialized(client)
	theCategoryStore = &CategoryStore{Client: client}

	logrus.Info("CategoryStore created")
	return theCategoryStore, nil
}

// Create persists a new category
func (cs *CategoryStore) Create(ctx context.Context, category *interfaces.Category) (*interfaces.Category, error) {
	if category == nil {
		return nil, fmt.Errorf("category is nil")
	}
	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}
	if err := cs.session.GetDB().WithContext(ctx).Create(category).Error; err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return category, nil
}

// GetByID retrieves a category by its UUID
func (cs *CategoryStore) GetByID(ctx context.Context, id string) (*interfaces.Category, error) {
	c := new(interfaces.Category)
	err := cs.session.GetDB().WithContext(ctx).Where("id = ?", id).Take(c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return c, nil
}

// GetBySlug retrieves a category by its slug
func (cs *CategoryStore) GetBySlug(ctx context.Context, slug string) (*interfaces.Category, error) {
	c := new(interfaces.Category)
	err := cs.session.GetDB().WithContext(ctx).Where("slug = ?", slug).Take(c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get category by slug: %w", err)
	}
	return c, nil
}

// List returns active categories ordered by position
func (cs *CategoryStore) List(ctx context.Context) ([]*interfaces.Category, error) {
	var categories []*interfaces.Category
	if err := cs.session.GetDB().WithContext(ctx).
		Where("is_active = ? AND deleted_at IS NULL", true).
		Order("position ASC, name ASC").
		Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	return categories, nil
}

// Update applies the given fields to a category
func (cs *CategoryStore) Update(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Category, error) {
	if id == "" {
		return nil, fmt.Errorf("category ID is required")
	}
	if len(fields) == 0 {
		return nil, errs.ErrEmptyUpdate
	}

	db := cs.session.GetDB().WithContext(ctx)
	if err := db.Model(&interfaces.Category{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	return cs.GetByID(ctx, id)
}

// Delete soft-deletes a category
func (cs *CategoryStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("category ID is required")
	}
	if err := cs.session.GetDB().WithContext(ctx).
		Where("id = ?", id).
		Delete(&interfaces.Category{}).Error; err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}
