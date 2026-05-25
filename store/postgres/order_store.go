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
	_ types.OrderStore = &OrderStore{}

	theOrderStoreMtx sync.Mutex
	theOrderStore    *OrderStore
)

// OrderStore is the PostgreSQL implementation of OrderStore
type OrderStore struct {
	*Client
}

// NewOrderStore creates the singleton OrderStore
func NewOrderStore() (*OrderStore, error) {
	theOrderStoreMtx.Lock()
	defer theOrderStoreMtx.Unlock()

	if theOrderStore != nil {
		return theOrderStore, nil
	}
	MustClientInitialized(client)
	theOrderStore = &OrderStore{Client: client}

	logrus.Info("OrderStore created")
	return theOrderStore, nil
}

// Create persists an order with its items in a single transaction
func (os *OrderStore) Create(ctx context.Context, order *interfaces.Order, items []interfaces.OrderItem) (*interfaces.Order, error) {
	if order == nil {
		return nil, fmt.Errorf("order is nil")
	}
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}

	err := withTransaction(os.session.GetDB().WithContext(ctx), func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("failed to create order: %w", err)
		}

		for i := range items {
			items[i].OrderID = order.ID
			if items[i].ID == uuid.Nil {
				items[i].ID = uuid.New()
			}
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return fmt.Errorf("failed to create order items: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return os.GetByID(ctx, order.ID.String())
}

// GetByID returns an order with items preloaded (id must be a valid UUID string).
func (os *OrderStore) GetByID(ctx context.Context, id string) (*interfaces.Order, error) {
	id = strings.TrimSpace(id)
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errs.ErrNoSuchEntity
	}
	o := new(interfaces.Order)
	err = os.session.GetDB().WithContext(ctx).
		Preload("Items").
		Where("id = ?", uid).
		Take(o).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return o, nil
}

// GetByOrderNumber returns an order by its human-readable number
func (os *OrderStore) GetByOrderNumber(ctx context.Context, orderNumber string) (*interfaces.Order, error) {
	o := new(interfaces.Order)
	err := os.session.GetDB().WithContext(ctx).
		Preload("Items").
		Where("order_number = ?", orderNumber).
		Take(o).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get order by number: %w", err)
	}
	return o, nil
}

// GetByPhone retrieves an order using its ID + phone (used for guest checkout lookups)
func (os *OrderStore) GetByPhone(ctx context.Context, orderID, phone string) (*interfaces.Order, error) {
	o := new(interfaces.Order)
	err := os.session.GetDB().WithContext(ctx).
		Preload("Items").
		Where("id = ? AND shipping_info->>'phone' = ?", orderID, phone).
		Take(o).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, fmt.Errorf("failed to get order by phone: %w", err)
	}
	return o, nil
}

// ListByUserID returns orders for a given user
func (os *OrderStore) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*interfaces.Order, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	db := os.session.GetDB().WithContext(ctx).
		Model(&interfaces.Order{}).
		Where("user_id = ?", userID)

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	var orders []*interfaces.Order
	if err := db.
		Preload("Items").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list user orders: %w", err)
	}
	return orders, totalCount, nil
}

// List returns paginated, filtered orders (admin)
func (os *OrderStore) List(ctx context.Context, filters *api.ListOrdersFilters) ([]*interfaces.Order, int64, error) {
	if filters == nil {
		filters = &api.ListOrdersFilters{Limit: 50, Offset: 0}
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}

	db := os.session.GetDB().WithContext(ctx).Model(&interfaces.Order{})

	if filters.Status != "" {
		db = db.Where("status = ?", filters.Status)
	}
	if filters.PaymentStatus != "" {
		db = db.Where("payment_status = ?", filters.PaymentStatus)
	}
	if filters.PaymentMethod != "" {
		db = db.Where("payment_method = ?", filters.PaymentMethod)
	}
	if filters.UserID != "" {
		db = db.Where("user_id = ?", filters.UserID)
	}
	if filters.Phone != "" {
		like := "%" + filters.Phone + "%"
		db = db.Where("shipping_info->>'phone' ILIKE ?", like)
	}
	if filters.Search != "" {
		like := "%" + filters.Search + "%"
		db = db.Where(
			"order_number ILIKE ? OR shipping_info->>'firstName' ILIKE ? OR shipping_info->>'lastName' ILIKE ? OR shipping_info->>'phone' ILIKE ?",
			like, like, like, like,
		)
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count orders: %w", err)
	}

	var orders []*interfaces.Order
	if err := db.
		Preload("Items").
		Order("created_at DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list orders: %w", err)
	}
	return orders, totalCount, nil
}

// UpdateStatus applies status / payment / tracking changes
func (os *OrderStore) UpdateStatus(ctx context.Context, id string, fields map[string]interface{}) (*interfaces.Order, error) {
	if id == "" {
		return nil, fmt.Errorf("order ID is required")
	}
	if len(fields) == 0 {
		return nil, errs.ErrEmptyUpdate
	}

	db := os.session.GetDB().WithContext(ctx)
	if err := db.Model(&interfaces.Order{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	return os.GetByID(ctx, id)
}
