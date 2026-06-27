package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

var (
	_ types.SettingsStore = &SettingsStore{}

	theSettingsStoreMtx sync.Mutex
	theSettingsStore    *SettingsStore
)

// SettingsStore is the PostgreSQL implementation of SettingsStore
type SettingsStore struct {
	*Client
}

// NewSettingsStore creates the singleton SettingsStore
func NewSettingsStore() (*SettingsStore, error) {
	theSettingsStoreMtx.Lock()
	defer theSettingsStoreMtx.Unlock()

	if theSettingsStore != nil {
		return theSettingsStore, nil
	}
	if err := MustClientInitialized(client); err != nil {
		return nil, err
	}
	theSettingsStore = &SettingsStore{Client: client}

	logrus.Info("SettingsStore created")
	return theSettingsStore, nil
}

// GetAll returns every settings row
func (ss *SettingsStore) GetAll(ctx context.Context) ([]*interfaces.Setting, error) {
	var settings []*interfaces.Setting
	if err := ss.session.GetDB().WithContext(ctx).
		Order("key ASC").
		Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}
	return settings, nil
}

// GetByKey returns a single settings row by its key
func (ss *SettingsStore) GetByKey(ctx context.Context, key string) (*interfaces.Setting, error) {
	s := new(interfaces.Setting)
	err := ss.session.GetDB().WithContext(ctx).Where("key = ?", key).Take(s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get setting %q: %w", key, err)
	}
	return s, nil
}

// Upsert creates or updates a settings row
func (ss *SettingsStore) Upsert(ctx context.Context, key string, value interfaces.SettingsValue) (*interfaces.Setting, error) {
	if key == "" {
		return nil, fmt.Errorf("settings key is required")
	}

	now := time.Now()
	s := &interfaces.Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}

	result := ss.session.GetDB().WithContext(ctx).
		Where("key = ?", key).
		Assign(map[string]interface{}{
			"value":      value,
			"updated_at": now,
		}).
		FirstOrCreate(s)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to upsert setting %q: %w", key, result.Error)
	}

	// Re-read to get the final state
	return ss.GetByKey(ctx, key)
}
