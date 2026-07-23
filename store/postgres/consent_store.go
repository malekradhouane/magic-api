package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/store/types"
)

var (
	_ types.ConsentStore = &ConsentStore{}

	theConsentStoreMtx sync.Mutex
	theConsentStore    *ConsentStore
)

// ConsentStore is the PostgreSQL implementation of ConsentStore.
type ConsentStore struct {
	*Client
}

// NewConsentStore creates the singleton ConsentStore.
func NewConsentStore() (*ConsentStore, error) {
	theConsentStoreMtx.Lock()
	defer theConsentStoreMtx.Unlock()

	if theConsentStore != nil {
		return theConsentStore, nil
	}
	if err := MustClientInitialized(client); err != nil {
		return nil, err
	}
	theConsentStore = &ConsentStore{Client: client}

	logrus.Info("ConsentStore created")
	return theConsentStore, nil
}

// Create persists a new consent proof.
func (cs *ConsentStore) Create(ctx context.Context, consent *interfaces.Consent) (*interfaces.Consent, error) {
	if consent == nil {
		return nil, fmt.Errorf("consent is nil")
	}
	if consent.ID == uuid.Nil {
		consent.ID = uuid.New()
	}
	if err := cs.session.GetDB().WithContext(ctx).Create(consent).Error; err != nil {
		return nil, fmt.Errorf("failed to create consent: %w", err)
	}
	return consent, nil
}

// List returns consent proofs, optionally filtered by type, ordered newest first.
func (cs *ConsentStore) List(ctx context.Context, consentType string, limit, offset int) ([]*interfaces.Consent, int64, error) {
	db := cs.session.GetDB().WithContext(ctx).Model(&interfaces.Consent{})
	if consentType != "" {
		db = db.Where("type = ?", consentType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count consents: %w", err)
	}

	var consents []*interfaces.Consent
	if err := db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&consents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list consents: %w", err)
	}
	return consents, total, nil
}
