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
	_ types.AddressStore = &AddressStore{}

	theAddressStoreMtx sync.Mutex
	theAddressStore    *AddressStore
)

// AddressStore is the PostgreSQL implementation of AddressStore.
type AddressStore struct {
	*Client
}

// NewAddressStore creates the singleton AddressStore.
func NewAddressStore() (*AddressStore, error) {
	theAddressStoreMtx.Lock()
	defer theAddressStoreMtx.Unlock()

	if theAddressStore != nil {
		return theAddressStore, nil
	}
	if err := MustClientInitialized(client); err != nil {
		return nil, err
	}
	theAddressStore = &AddressStore{Client: client}

	logrus.Info("AddressStore created")
	return theAddressStore, nil
}

// ListByUserID returns all addresses for a user.
func (as *AddressStore) ListByUserID(ctx context.Context, userID string) ([]*interfaces.Address, error) {
	var addresses []*interfaces.Address
	err := as.session.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_default DESC, created_at DESC").
		Find(&addresses).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses: %w", err)
	}
	return addresses, nil
}

// GetByID returns an address if it belongs to the given user.
func (as *AddressStore) GetByID(ctx context.Context, userID, id string) (*interfaces.Address, error) {
	a := new(interfaces.Address)
	err := as.session.GetDB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Take(a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get address: %w", err)
	}
	return a, nil
}

// FindMatching looks for an existing address with the same delivery details.
func (as *AddressStore) FindMatching(
	ctx context.Context,
	userID, phone, gouvernorat, addressLine string,
) (*interfaces.Address, error) {
	a := new(interfaces.Address)
	err := as.session.GetDB().WithContext(ctx).
		Where(
			"user_id = ? AND phone = ? AND gouvernorat = ? AND address = ?",
			userID, phone, gouvernorat, addressLine,
		).
		Take(a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to find matching address: %w", err)
	}
	return a, nil
}

// Create persists a new address for a user.
func (as *AddressStore) Create(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error) {
	if addr == nil {
		return nil, fmt.Errorf("address is nil")
	}
	if addr.ID == uuid.Nil {
		addr.ID = uuid.New()
	}

	err := withTransaction(as.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if addr.IsDefault {
			if err := clearDefaultAddress(tx, addr.UserID.String()); err != nil {
				return err
			}
		} else {
			var count int64
			if err := tx.Model(&interfaces.Address{}).
				Where("user_id = ?", addr.UserID).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				addr.IsDefault = true
			}
		}
		return tx.Create(addr).Error
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}
	return as.GetByID(ctx, addr.UserID.String(), addr.ID.String())
}

// Update updates an address owned by the user.
func (as *AddressStore) Update(ctx context.Context, addr *interfaces.Address) (*interfaces.Address, error) {
	if addr == nil {
		return nil, fmt.Errorf("address is nil")
	}

	err := withTransaction(as.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		var existing interfaces.Address
		if err := tx.Where("id = ? AND user_id = ?", addr.ID, addr.UserID).Take(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrNoSuchEntity
			}
			return err
		}
		if addr.IsDefault && !existing.IsDefault {
			if err := clearDefaultAddress(tx, addr.UserID.String()); err != nil {
				return err
			}
		}
		return tx.Model(&existing).Updates(map[string]interface{}{
			"label":       addr.Label,
			"first_name":  addr.FirstName,
			"last_name":   addr.LastName,
			"phone":       addr.Phone,
			"gouvernorat": addr.Gouvernorat,
			"address":     addr.Address,
			"postal_code": addr.PostalCode,
			"is_default":  addr.IsDefault,
			"updated_at":  gorm.Expr("now()"),
		}).Error
	})
	if err != nil {
		if errors.Is(err, errs.ErrNoSuchEntity) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to update address: %w", err)
	}
	return as.GetByID(ctx, addr.UserID.String(), addr.ID.String())
}

// SetDefault marks an address as the user's default.
func (as *AddressStore) SetDefault(ctx context.Context, userID, id string) error {
	return withTransaction(as.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		var existing interfaces.Address
		if err := tx.Where("id = ? AND user_id = ?", id, userID).Take(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrNoSuchEntity
			}
			return err
		}
		if err := clearDefaultAddress(tx, userID); err != nil {
			return err
		}
		return tx.Model(&existing).Update("is_default", true).Error
	})
}

// Delete removes an address owned by the user.
func (as *AddressStore) Delete(ctx context.Context, userID, id string) error {
	return withTransaction(as.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		var existing interfaces.Address
		if err := tx.Where("id = ? AND user_id = ?", id, userID).Take(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrNoSuchEntity
			}
			return err
		}
		if err := tx.Delete(&existing).Error; err != nil {
			return err
		}
		if existing.IsDefault {
			var next interfaces.Address
			if err := tx.Where("user_id = ?", userID).
				Order("created_at DESC").
				First(&next).Error; err == nil {
				return tx.Model(&next).Update("is_default", true).Error
			}
		}
		return nil
	})
}

func clearDefaultAddress(tx *gorm.DB, userID string) error {
	return tx.Model(&interfaces.Address{}).
		Where("user_id = ? AND is_default = ?", userID, true).
		Update("is_default", false).Error
}
