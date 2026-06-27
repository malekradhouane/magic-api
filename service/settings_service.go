package service

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

// SettingsService handles settings business logic
type SettingsService struct {
	store  types.SettingsStore
	logger *logrus.Logger
}

// NewSettingsService constructs a SettingsService
func NewSettingsService(store types.SettingsStore, logger *logrus.Logger) *SettingsService {
	if logger == nil {
		logger = logrus.New()
	}
	return &SettingsService{store: store, logger: logger}
}

// GetAll returns every settings group
func (ss *SettingsService) GetAll(ctx context.Context) ([]*interfaces.Setting, error) {
	settings, err := ss.store.GetAll(ctx)
	if err != nil {
		ss.logger.WithError(err).Error("Failed to get all settings")
		return nil, err
	}
	return settings, nil
}

// GetByKey returns a single settings group by key
func (ss *SettingsService) GetByKey(ctx context.Context, key string) (*interfaces.Setting, error) {
	setting, err := ss.store.GetByKey(ctx, key)
	if err != nil {
		ss.logger.WithError(err).WithField("key", key).Error("Failed to get setting")
		return nil, err
	}
	return setting, nil
}

// Update upserts a single settings group
func (ss *SettingsService) Update(ctx context.Context, key string, value interfaces.SettingsValue) (*interfaces.Setting, error) {
	setting, err := ss.store.Upsert(ctx, key, value)
	if err != nil {
		ss.logger.WithError(err).WithField("key", key).Error("Failed to update setting")
		return nil, err
	}
	return setting, nil
}
