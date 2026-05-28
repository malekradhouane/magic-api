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

// ListTree returns active top-level categories with their direct children
// preloaded, ordered by position.
func (cs *CategoryStore) ListTree(ctx context.Context) ([]*interfaces.Category, error) {
	var categories []*interfaces.Category
	if err := cs.session.GetDB().WithContext(ctx).
		Where("parent_id IS NULL AND is_active = ? AND deleted_at IS NULL", true).
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ? AND deleted_at IS NULL", true).
				Order("position ASC, name ASC")
		}).
		Order("position ASC, name ASC").
		Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("failed to list category tree: %w", err)
	}
	return categories, nil
}

// SeedDefaultCategories idempotently ensures the default 2-level product-type
// taxonomy exists (groups -> sub-categories). Safe to run on every startup:
// existing slugs are left untouched thanks to ON CONFLICT DO NOTHING.
func (cs *CategoryStore) SeedDefaultCategories(ctx context.Context) error {
	db := cs.session.GetDB().WithContext(ctx)

	// Top-level groups.
	if err := db.Exec(`
		INSERT INTO categories (slug, name, position, is_active) VALUES
			('vetements',   'Vêtements',   1, TRUE),
			('chaussures',  'Chaussures',  2, TRUE),
			('accessoires', 'Accessoires', 3, TRUE)
		ON CONFLICT (slug) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("failed to seed category groups: %w", err)
	}

	// Sub-categories, linked to their parent group by slug.
	if err := db.Exec(`
		INSERT INTO categories (slug, name, parent_id, position, is_active)
		SELECT child.slug, child.name, parent.id, child.position, TRUE
		FROM (
			VALUES
				('t-shirts',        'T-shirts',             'vetements',    1),
				('chemises',        'Chemises',             'vetements',    2),
				('pulls-gilets',    'Pulls & Gilets',       'vetements',    3),
				('vestes-manteaux', 'Vestes & Manteaux',    'vetements',    4),
				('pantalons',       'Pantalons',            'vetements',    5),
				('jeans',           'Jeans',                'vetements',    6),
				('shorts',          'Shorts',               'vetements',    7),
				('robes',           'Robes',                'vetements',    8),
				('jupes',           'Jupes',                'vetements',    9),
				('baskets',         'Baskets',              'chaussures',   1),
				('bottes',          'Bottes & Bottines',    'chaussures',   2),
				('sandales',        'Sandales',             'chaussures',   3),
				('sacs',            'Sacs',                 'accessoires',  1),
				('ceintures',       'Ceintures',            'accessoires',  2),
				('chapeaux',        'Chapeaux & Casquettes','accessoires',  3),
				('echarpes',        'Écharpes & Foulards',  'accessoires',  4)
		) AS child(slug, name, parent_slug, position)
		JOIN categories parent ON parent.slug = child.parent_slug
		ON CONFLICT (slug) DO NOTHING
	`).Error; err != nil {
		return fmt.Errorf("failed to seed sub-categories: %w", err)
	}

	// Default gender scope for gender-specific categories (metadata.genders).
	// Only applied when the key is absent, so admin edits are never overwritten.
	// Categories without this key are shown for every gender.
	genderScopes := map[string][]string{
		`["femme","enfant"]`: {"robes", "jupes"},
	}
	for genders, slugs := range genderScopes {
		if err := db.Exec(`
			UPDATE categories
			SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{genders}', ?::jsonb)
			WHERE slug IN ? AND NOT jsonb_exists(COALESCE(metadata, '{}'::jsonb), 'genders')
		`, genders, slugs).Error; err != nil {
			return fmt.Errorf("failed to set category gender scope: %w", err)
		}
	}

	return nil
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
