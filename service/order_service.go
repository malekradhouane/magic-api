package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/mailer"
	"github.com/malekradhouane/magic/store/types"
)

const (
	// FreeShippingThreshold: livraison gratuite au-delà
	FreeShippingThreshold = 150.0
	// StandardShippingFee in TND
	StandardShippingFee = 8.0
	// OrderNumberPrefix: AFR-YYYYMMDD-XXXX
	OrderNumberPrefix = "AFR"
)

// OrderService handles order business logic
type OrderService struct {
	orderStore     types.OrderStore
	productStore   types.ProductStore
	promoService   *PromoService
	userStore      types.UserStore
	addressService *AddressService
	mailer         mailer.Mailer
	mailFromName   string
	mailFromEmail  string
	frontendURL    string
	logger         *logrus.Logger
}

// NewOrderService constructs an OrderService
func NewOrderService(
	orderStore types.OrderStore,
	productStore types.ProductStore,
	promoService *PromoService,
	userStore types.UserStore,
	addressService *AddressService,
	m mailer.Mailer,
	mailFromName, mailFromEmail, frontendURL string,
	logger *logrus.Logger,
) *OrderService {
	if logger == nil {
		logger = logrus.New()
	}
	if mailFromName == "" {
		mailFromName = "Magic"
	}
	if mailFromEmail == "" {
		mailFromEmail = "noreply@magic.fr"
	}
	return &OrderService{
		orderStore:     orderStore,
		productStore:   productStore,
		promoService:   promoService,
		userStore:      userStore,
		addressService: addressService,
		mailer:         m,
		mailFromName:   mailFromName,
		mailFromEmail:  mailFromEmail,
		frontendURL:    frontendURL,
		logger:         logger,
	}
}

// Create builds and persists an order from a CreateOrderRequest
// userID is empty string for guest checkout
func (os *OrderService) Create(ctx context.Context, req *api.CreateOrderRequest, userID string) (*interfaces.Order, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("order has no items")
	}

	// 1. Validate items & build line items + compute subtotal
	subtotal := 0.0
	items := make([]interfaces.OrderItem, 0, len(req.Items))

	for _, lineReq := range req.Items {
		if lineReq.Quantity <= 0 {
			return nil, fmt.Errorf("invalid quantity for product %s", lineReq.ProductID)
		}

		product, err := os.productStore.GetByID(ctx, lineReq.ProductID)
		if err != nil {
			os.logger.WithError(err).
				WithField("product_id", lineReq.ProductID).
				Error("Failed to load product for order")
			return nil, fmt.Errorf("product %s not found", lineReq.ProductID)
		}
		if !product.IsActive {
			return nil, fmt.Errorf("product %s is not available", product.Name)
		}

		// Stock check on size
		if lineReq.Size != "" {
			ok := false
			for _, ps := range product.Sizes {
				if ps.Size == lineReq.Size && ps.Stock >= lineReq.Quantity {
					ok = true
					break
				}
			}
			if !ok {
				return nil, fmt.Errorf("insufficient stock for product %s size %s", product.Name, lineReq.Size)
			}
		}

		lineTotal := product.Price * float64(lineReq.Quantity)
		subtotal += lineTotal

		// Snapshot main image
		var mainImage string
		for _, img := range product.Images {
			if img.IsPrimary || mainImage == "" {
				mainImage = img.URL
				if img.IsPrimary {
					break
				}
			}
		}

		productID := product.ID
		items = append(items, interfaces.OrderItem{
			ProductID:    &productID,
			ProductName:  product.Name,
			ProductImage: mainImage,
			ProductSlug:  product.Slug,
			Size:         lineReq.Size,
			Color:        lineReq.Color,
			UnitPrice:    product.Price,
			Quantity:     lineReq.Quantity,
			LineTotal:    lineTotal,
		})
	}

	// 2. Compute shipping fee
	shippingFee := StandardShippingFee
	if subtotal >= FreeShippingThreshold {
		shippingFee = 0
	}

	// 3. Apply promo code (if any)
	discount := 0.0
	var appliedPromo *interfaces.PromoCode
	var promoCodeStr *string
	if req.PromoCode != "" {
		validation, err := os.promoService.Validate(ctx, req.PromoCode, subtotal, userID)
		if err != nil {
			return nil, err
		}
		if !validation.Valid {
			return nil, fmt.Errorf("invalid promo code: %s", validation.Message)
		}
		discount = validation.DiscountAmount
		// Reload the promo for usage tracking
		promo, err := os.promoService.GetByCode(ctx, req.PromoCode)
		if err == nil {
			appliedPromo = promo
			pc := promo.Code
			promoCodeStr = &pc
		}
	}

	totalPrice := subtotal + shippingFee - discount
	if totalPrice < 0 {
		totalPrice = 0
	}

	// 4. Build order
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = interfaces.PaymentMethodCash
	}

	shippingInfo := req.ShippingInfo
	os.enrichShippingEmail(ctx, &shippingInfo, userID)

	order := &interfaces.Order{
		OrderNumber:    generateOrderNumber(),
		Status:         interfaces.OrderStatusPending,
		Subtotal:       subtotal,
		ShippingFee:    shippingFee,
		DiscountAmount: discount,
		TotalPrice:     totalPrice,
		Currency:       "TND",
		PromoCode:      promoCodeStr,
		PaymentMethod:  paymentMethod,
		PaymentStatus:  interfaces.PaymentStatusPending,
		ShippingInfo:   shippingInfo,
		CustomerNotes:  req.CustomerNotes,
	}
	if userID != "" {
		uid, err := uuid.Parse(userID)
		if err == nil {
			order.UserID = &uid
		}
	}

	// 5. Reserve stock (commande en attente = stock bloqué)
	for _, item := range items {
		if item.ProductID == nil || item.Size == "" {
			return nil, fmt.Errorf("taille requise pour réserver le stock du produit %s", item.ProductName)
		}
	}
	if err := os.reserveStockForItems(ctx, items); err != nil {
		return nil, err
	}
	order.Metadata = interfaces.JSONMap{orderMetaStockReserved: true}

	// 6. Persist
	created, err := os.orderStore.Create(ctx, order, items)
	if err != nil {
		os.logger.WithError(err).Error("Failed to persist order")
		_ = os.releaseStockForItems(ctx, items)
		return nil, err
	}

	if appliedPromo != nil {
		if err := os.promoService.MarkUsed(ctx, appliedPromo, userID, created.ID.String(), discount); err != nil {
			os.logger.WithError(err).Warn("Failed to record promo usage")
		}
	}

	os.logger.WithFields(logrus.Fields{
		"order_id":     created.ID,
		"order_number": created.OrderNumber,
		"total":        created.TotalPrice,
		"items":        len(items),
	}).Info("Order created")

	if userID != "" && os.addressService != nil {
		if err := os.addressService.SaveFromShipping(ctx, userID, shippingInfo); err != nil {
			os.logger.WithError(err).Warn("Failed to save shipping address to user profile")
		}
	}

	os.sendOrderConfirmationEmailAsync(created)

	return created, nil
}

// enrichShippingEmail copies the account email when checkout did not include one.
func (os *OrderService) enrichShippingEmail(ctx context.Context, shipping *interfaces.ShippingInfo, userID string) {
	if shipping == nil || strings.TrimSpace(shipping.Email) != "" {
		return
	}
	if userID == "" || os.userStore == nil {
		return
	}
	user, err := os.userStore.Get(ctx, userID)
	if err != nil || user == nil {
		return
	}
	if e := strings.TrimSpace(user.Email); e != "" {
		shipping.Email = e
	}
}

// Get returns an order by ID
func (os *OrderService) Get(ctx context.Context, id string) (*interfaces.Order, error) {
	o, err := os.orderStore.GetByID(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	return o, nil
}

// GetForAdmin returns an order by UUID or by human-readable order_number (admin).
func (os *OrderService) GetForAdmin(ctx context.Context, idOrNumber string) (*interfaces.Order, error) {
	key := strings.TrimSpace(idOrNumber)
	if key == "" {
		return nil, errs.ErrNoSuchEntity
	}
	if _, err := uuid.Parse(key); err == nil {
		return os.Get(ctx, key)
	}
	o, err := os.orderStore.GetByOrderNumber(ctx, key)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}
	return o, nil
}

// GetForCustomer returns an order with phone validation (for guest checkout)
func (os *OrderService) GetForCustomer(ctx context.Context, id, phone string) (*interfaces.Order, error) {
	if phone == "" {
		return nil, fmt.Errorf("phone is required to access guest orders")
	}
	return os.orderStore.GetByPhone(ctx, id, phone)
}

// ListByUser returns orders for a given authenticated user
func (os *OrderService) ListByUser(ctx context.Context, userID string, limit, offset int) (*api.OrdersResponse, error) {
	orders, total, err := os.orderStore.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return &api.OrdersResponse{
		Orders:     orders,
		TotalCount: total,
		HasMore:    int64(offset+limit) < total,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// List returns paginated orders (admin)
func (os *OrderService) List(ctx context.Context, filters *api.ListOrdersFilters) (*api.OrdersResponse, error) {
	orders, total, err := os.orderStore.List(ctx, filters)
	if err != nil {
		return nil, err
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	return &api.OrdersResponse{
		Orders:     orders,
		TotalCount: total,
		HasMore:    int64(filters.Offset+limit) < total,
		Limit:      limit,
		Offset:     filters.Offset,
	}, nil
}

// UpdateStatus changes order status / payment / tracking
func (os *OrderService) UpdateStatus(ctx context.Context, id string, req *api.UpdateOrderStatusRequest) (*interfaces.Order, error) {
	previous, err := os.orderStore.GetByID(ctx, id)
	if err != nil {
		if errs.IsNoSuchEntityError(err) {
			return nil, errs.ErrNoSuchEntity
		}
		return nil, err
	}

	fields := map[string]interface{}{}
	now := time.Now()
	notifyShipped := false

	if req.Status != nil {
		switch *req.Status {
		case interfaces.OrderStatusPending,
			interfaces.OrderStatusConfirmed,
			interfaces.OrderStatusShipped,
			interfaces.OrderStatusDelivered,
			interfaces.OrderStatusCancelled:
			fields["status"] = *req.Status
		default:
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}

		if *req.Status == interfaces.OrderStatusShipped && previous.Status != interfaces.OrderStatusShipped {
			notifyShipped = true
		}

		switch *req.Status {
		case interfaces.OrderStatusShipped:
			fields["shipped_at"] = now
		case interfaces.OrderStatusDelivered:
			fields["delivered_at"] = now
		case interfaces.OrderStatusCancelled:
			fields["cancelled_at"] = now
		}
	}

	if req.PaymentStatus != nil {
		switch *req.PaymentStatus {
		case interfaces.PaymentStatusPending,
			interfaces.PaymentStatusPaid,
			interfaces.PaymentStatusRefunded,
			interfaces.PaymentStatusFailed:
			fields["payment_status"] = *req.PaymentStatus
		default:
			return nil, fmt.Errorf("invalid payment status: %s", *req.PaymentStatus)
		}
	}

	if req.TrackingNumber != nil {
		fields["tracking_number"] = *req.TrackingNumber
	}

	// Annulation admin : remettre le stock réservé
	if req.Status != nil &&
		*req.Status == interfaces.OrderStatusCancelled &&
		previous.Status != interfaces.OrderStatusCancelled {
		if orderStockWasReserved(previous) && !orderStockWasReleased(previous) {
			if err := os.releaseStockForItems(ctx, previous.Items); err != nil {
				return nil, fmt.Errorf("impossible de remettre le stock en inventaire: %w", err)
			}
			fields["metadata"] = orderMetadataMarkReleased(previous.Metadata)
		}
	}

	if len(fields) == 0 {
		return previous, nil
	}

	updated, err := os.orderStore.UpdateStatus(ctx, id, fields)
	if err != nil {
		return nil, err
	}

	if notifyShipped {
		os.sendOrderShippedEmail(ctx, updated)
	}

	return updated, nil
}

const (
	orderMetaStockReserved = "stock_reserved"
	orderMetaStockReleased = "stock_released"
)

func orderStockWasReserved(o *interfaces.Order) bool {
	return o.Metadata.Bool(orderMetaStockReserved)
}

func orderStockWasReleased(o *interfaces.Order) bool {
	return o.Metadata.Bool(orderMetaStockReleased)
}

func orderMetadataMarkReleased(meta interfaces.JSONMap) interfaces.JSONMap {
	m := meta.Clone()
	m[orderMetaStockReleased] = true
	return m
}

func (os *OrderService) reserveStockForItems(ctx context.Context, items []interfaces.OrderItem) error {
	reserved := make([]interfaces.OrderItem, 0, len(items))
	for _, item := range items {
		if item.ProductID == nil || item.Size == "" || item.Color == "" {
			continue
		}
		if err := os.productStore.DecrementVariantStock(
			ctx,
			item.ProductID.String(),
			item.Size,
			item.Color,
			item.Quantity,
		); err != nil {
			_ = os.releaseStockForItems(ctx, reserved)
			return err
		}
		reserved = append(reserved, item)
	}
	return nil
}

func (os *OrderService) releaseStockForItems(ctx context.Context, items []interfaces.OrderItem) error {
	var firstErr error
	for _, item := range items {
		if item.ProductID == nil || item.Size == "" || item.Color == "" {
			continue
		}
		if err := os.productStore.IncrementVariantStock(
			ctx,
			item.ProductID.String(),
			item.Size,
			item.Color,
			item.Quantity,
		); err != nil {
			os.logger.WithError(err).WithFields(logrus.Fields{
				"product_id": item.ProductID.String(),
				"size":       item.Size,
				"color":      item.Color,
				"quantity":   item.Quantity,
			}).Error("Failed to restore stock on order cancellation")
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// generateOrderNumber returns a human-readable order number: AFR-YYYYMMDD-XXXX
func generateOrderNumber() string {
	now := time.Now()
	suffix := rand.Intn(9000) + 1000
	return strings.ToUpper(fmt.Sprintf("%s-%s-%d", OrderNumberPrefix, now.Format("20060102"), suffix))
}
