package service

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
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
	// DefaultFreeShippingThreshold: livraison gratuite au-delà (fallback)
	DefaultFreeShippingThreshold = 150.0
	// DefaultStandardShippingFee in TND (fallback)
	DefaultStandardShippingFee = 8.0
	// OrderNumberPrefix: AFR-YYYYMMDD-XXXX
	OrderNumberPrefix = "AFR"
)

// OrderNotifier is notified when an order is created so it can push real-time
// notifications (e.g. SSE to the admin dashboard). It is optional.
type OrderNotifier interface {
	NotifyNewOrder(o *interfaces.Order)
}

// OrderService handles order business logic
type OrderService struct {
	orderStore      types.OrderStore
	productStore    types.ProductStore
	promoService    *PromoService
	userStore       types.UserStore
	addressService  *AddressService
	settingsService *SettingsService
	mailer          mailer.Mailer
	mailFromName    string
	mailFromEmail   string
	frontendURL     string
	logger          *logrus.Logger
	notifier        OrderNotifier
}

// SetNotifier wires an optional real-time notifier (kept out of the constructor
// to avoid breaking existing call sites).
func (os *OrderService) SetNotifier(n OrderNotifier) {
	os.notifier = n
}

// NewOrderService constructs an OrderService
func NewOrderService(
	orderStore types.OrderStore,
	productStore types.ProductStore,
	promoService *PromoService,
	userStore types.UserStore,
	addressService *AddressService,
	settingsService *SettingsService,
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
		orderStore:      orderStore,
		productStore:    productStore,
		promoService:    promoService,
		userStore:       userStore,
		addressService:  addressService,
		settingsService: settingsService,
		mailer:          m,
		mailFromName:    mailFromName,
		mailFromEmail:   mailFromEmail,
		frontendURL:     frontendURL,
		logger:          logger,
	}
}

// Create builds and persists an order from a CreateOrderRequest.
// userID is the empty string for a guest checkout.
//
// The flow is split into helpers to keep this orchestrator readable: build
// line items → compute totals → assemble order → reserve stock → persist →
// post-creation side-effects.
func (os *OrderService) Create(ctx context.Context, req *api.CreateOrderRequest, userID string) (*interfaces.Order, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("order has no items")
	}

	items, subtotal, err := os.buildOrderItems(ctx, req.Items)
	if err != nil {
		return nil, err
	}

	shippingFee := os.computeShippingFee(ctx, subtotal)

	discount, appliedPromo, promoCodeStr, err := os.applyPromoCode(ctx, req.PromoCode, subtotal, userID)
	if err != nil {
		return nil, err
	}

	shippingInfo := req.ShippingInfo
	os.enrichShippingEmail(ctx, &shippingInfo, userID)

	order := buildOrder(req, items, subtotal, shippingFee, discount, promoCodeStr, shippingInfo, userID)

	if err := os.reserveStockForItems(ctx, items); err != nil {
		return nil, err
	}
	order.Metadata = interfaces.JSONMap{orderMetaStockReserved: true}

	created, err := os.orderStore.Create(ctx, order, items)
	if err != nil {
		os.logger.WithError(err).Error("Failed to persist order")
		if relErr := os.releaseStockForItems(ctx, items); relErr != nil {
			os.logger.WithError(relErr).Error("Failed to release reserved stock after order persistence failure")
		}
		return nil, err
	}

	os.afterOrderCreated(ctx, created, items, shippingInfo, userID, appliedPromo, discount)
	return created, nil
}

// buildOrderItems validates every cart line, snapshots product data into
// OrderItems and returns the running subtotal.
func (os *OrderService) buildOrderItems(ctx context.Context, lines []api.CreateOrderItemRequest) ([]interfaces.OrderItem, float64, error) {
	items := make([]interfaces.OrderItem, 0, len(lines))
	var subtotal float64

	for _, lineReq := range lines {
		item, err := os.buildOrderItem(ctx, lineReq)
		if err != nil {
			return nil, 0, err
		}
		subtotal += item.LineTotal
		items = append(items, item)
	}
	return items, subtotal, nil
}

// buildOrderItem validates a single cart line and turns it into an OrderItem.
func (os *OrderService) buildOrderItem(ctx context.Context, lineReq api.CreateOrderItemRequest) (interfaces.OrderItem, error) {
	if lineReq.Quantity <= 0 {
		return interfaces.OrderItem{}, fmt.Errorf("invalid quantity for product %s", lineReq.ProductID)
	}

	product, err := os.productStore.GetByID(ctx, lineReq.ProductID)
	if err != nil {
		os.logger.WithError(err).
			WithField("product_id", lineReq.ProductID).
			Error("Failed to load product for order")
		return interfaces.OrderItem{}, fmt.Errorf("product %s not found", lineReq.ProductID)
	}
	if !product.IsActive {
		return interfaces.OrderItem{}, fmt.Errorf("product %s is not available", product.Name)
	}
	if lineReq.Size == "" {
		return interfaces.OrderItem{}, fmt.Errorf("taille requise pour le produit %s", product.Name)
	}
	if lineReq.Color == "" {
		return interfaces.OrderItem{}, fmt.Errorf("couleur requise pour le produit %s", product.Name)
	}
	if !hasVariantStock(product, lineReq.Size, lineReq.Color, lineReq.Quantity) {
		return interfaces.OrderItem{}, fmt.Errorf(
			"stock insuffisant pour %s (%s / %s)",
			product.Name, lineReq.Size, lineReq.Color,
		)
	}

	lineTotal := product.Price * float64(lineReq.Quantity)
	productID := product.ID
	return interfaces.OrderItem{
		ProductID:    &productID,
		ProductName:  product.Name,
		ProductImage: pickMainImage(product.Images),
		ProductSlug:  product.Slug,
		Size:         lineReq.Size,
		Color:        lineReq.Color,
		UnitPrice:    product.Price,
		Quantity:     lineReq.Quantity,
		LineTotal:    lineTotal,
	}, nil
}

// pickMainImage returns the URL of the primary image, falling back to the
// first one when no flag is set.
func pickMainImage(images []interfaces.ProductImage) string {
	var fallback string
	for _, img := range images {
		if img.IsPrimary {
			return img.URL
		}
		if fallback == "" {
			fallback = img.URL
		}
	}
	return fallback
}

// notifDefaults holds the resolved notification flags for a single request.
type notifDefaults struct {
	orderEmailEnabled bool
	notifyNewOrder    bool
	notifyLowStock    bool
	lowStockThreshold int
}

// getNotificationSettings reads the "notifications" settings row, falling back
// to safe defaults (all enabled) if the row is missing or malformed.
func (os *OrderService) getNotificationSettings(ctx context.Context) notifDefaults {
	d := notifDefaults{
		orderEmailEnabled: true,
		notifyNewOrder:    true,
		notifyLowStock:    true,
		lowStockThreshold: 5,
	}
	if os.settingsService == nil {
		return d
	}
	s, err := os.settingsService.GetByKey(ctx, "notifications")
	if err != nil || s == nil {
		return d
	}
	if v, ok := s.Value["order_email_enabled"]; ok {
		if b, ok := v.(bool); ok {
			d.orderEmailEnabled = b
		}
	}
	if v, ok := s.Value["notify_new_order"]; ok {
		if b, ok := v.(bool); ok {
			d.notifyNewOrder = b
		}
	}
	if v, ok := s.Value["notify_low_stock"]; ok {
		if b, ok := v.(bool); ok {
			d.notifyLowStock = b
		}
	}
	if v, ok := s.Value["low_stock_threshold"]; ok {
		if f, ok := v.(float64); ok && f > 0 {
			d.lowStockThreshold = int(f)
		}
	}
	return d
}

// computeShippingFee reads shipping settings from the DB, falling back to
// hardcoded defaults if the settings row is missing or malformed.
func (os *OrderService) computeShippingFee(ctx context.Context, subtotal float64) float64 {
	threshold := DefaultFreeShippingThreshold
	fee := DefaultStandardShippingFee

	if os.settingsService != nil {
		if s, err := os.settingsService.GetByKey(ctx, "shipping"); err == nil && s != nil {
			if v, ok := s.Value["free_shipping_threshold"]; ok {
				if f, ok := v.(float64); ok {
					threshold = f
				}
			}
			if v, ok := s.Value["default_shipping_cost"]; ok {
				if f, ok := v.(float64); ok {
					fee = f
				}
			}
		}
	}

	if threshold > 0 && subtotal >= threshold {
		return 0
	}
	return fee
}

// applyPromoCode validates the promo code (if any) and returns the discount,
// the promo to record for usage tracking and a pointer to the code string for
// storage on the order. An empty code yields zero discount and no error.
func (os *OrderService) applyPromoCode(
	ctx context.Context, code string, subtotal float64, userID string,
) (float64, *interfaces.PromoCode, *string, error) {
	if code == "" {
		return 0, nil, nil, nil
	}
	validation, err := os.promoService.Validate(ctx, code, subtotal, userID)
	if err != nil {
		return 0, nil, nil, err
	}
	if !validation.Valid {
		return 0, nil, nil, fmt.Errorf("invalid promo code: %s", validation.Message)
	}

	promo, err := os.promoService.GetByCode(ctx, code)
	if err != nil {
		// We still apply the discount but skip usage tracking when the reload fails.
		return validation.DiscountAmount, nil, nil, nil
	}
	pc := promo.Code
	return validation.DiscountAmount, promo, &pc, nil
}

// buildOrder assembles the *interfaces.Order from the validated request and
// pre-computed pricing. UserID is parsed defensively (guest checkouts pass "").
func buildOrder(
	req *api.CreateOrderRequest,
	items []interfaces.OrderItem,
	subtotal, shippingFee, discount float64,
	promoCodeStr *string,
	shippingInfo interfaces.ShippingInfo,
	userID string,
) *interfaces.Order {
	totalPrice := subtotal + shippingFee - discount
	if totalPrice < 0 {
		totalPrice = 0
	}

	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = interfaces.PaymentMethodCash
	}

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
		if uid, err := uuid.Parse(userID); err == nil {
			order.UserID = &uid
		}
	}
	// Items are stored alongside the order by the OrderStore; copying here
	// makes the link explicit for future serialization paths.
	order.Items = items
	return order
}

// afterOrderCreated runs every best-effort side effect that should not block
// the response to the customer: promo usage tracking, structured log entry,
// address book persistence, transactional email and real-time admin push.
func (os *OrderService) afterOrderCreated(
	ctx context.Context,
	created *interfaces.Order,
	items []interfaces.OrderItem,
	shippingInfo interfaces.ShippingInfo,
	userID string,
	appliedPromo *interfaces.PromoCode,
	discount float64,
) {
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

	// Check notification settings before sending email / SSE push.
	notifSettings := os.getNotificationSettings(ctx)
	if notifSettings.orderEmailEnabled {
		os.sendOrderConfirmationEmailAsync(created)
	}
	if notifSettings.notifyNewOrder && os.notifier != nil {
		os.notifier.NotifyNewOrder(created)
	}
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

	// Expédition : déduire le stock si une ancienne commande n'avait pas encore été réservée
	if req.Status != nil &&
		*req.Status == interfaces.OrderStatusShipped &&
		previous.Status != interfaces.OrderStatusShipped &&
		!orderStockWasReserved(previous) {
		if err := os.reserveStockForItems(ctx, previous.Items); err != nil {
			return nil, fmt.Errorf("impossible de déduire le stock à l'expédition: %w", err)
		}
		fields["metadata"] = orderMetadataMarkReserved(previous.Metadata)
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

func orderMetadataMarkReserved(meta interfaces.JSONMap) interfaces.JSONMap {
	m := meta.Clone()
	m[orderMetaStockReserved] = true
	return m
}

func hasVariantStock(product *interfaces.Product, size, color string, quantity int) bool {
	for _, v := range product.Variants {
		if v.Size == size && v.Color == color {
			return v.Stock >= quantity
		}
	}
	return false
}

func (os *OrderService) reserveStockForItems(ctx context.Context, items []interfaces.OrderItem) error {
	reserved := make([]interfaces.OrderItem, 0, len(items))
	for _, item := range items {
		if item.ProductID == nil || item.Size == "" || item.Color == "" {
			return fmt.Errorf(
				"taille et couleur requises pour réserver le stock du produit %s",
				item.ProductName,
			)
		}
		if err := os.productStore.DecrementVariantStock(
			ctx,
			item.ProductID.String(),
			item.Size,
			item.Color,
			item.Quantity,
		); err != nil {
			if relErr := os.releaseStockForItems(ctx, reserved); relErr != nil {
				os.logger.WithError(relErr).Error("Failed to release partial stock reservation")
			}
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

// generateOrderNumber returns a human-readable order number:
// AFR-YYYYMMDD-XXXXXX. The 6-digit suffix is drawn from crypto/rand so it
// is not predictable by a guest who already knows a valid number; this
// protects the guest-lookup endpoint against enumeration.
func generateOrderNumber() string {
	now := time.Now()
	const maxSuffix = 1_000_000 // 6 digits
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		// crypto/rand should never fail; fall back to nanoseconds to keep
		// uniqueness if it ever does.
		return strings.ToUpper(fmt.Sprintf("%s-%s-%06d",
			OrderNumberPrefix, now.Format("20060102"), now.UnixNano()%maxSuffix))
	}
	suffix := binary.BigEndian.Uint64(buf[:]) % maxSuffix
	return strings.ToUpper(fmt.Sprintf("%s-%s-%06d",
		OrderNumberPrefix, now.Format("20060102"), suffix))
}
