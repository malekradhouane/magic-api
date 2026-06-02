package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/sse"
)

// Event names pushed over the SSE stream.
const (
	EventNewOrder = "new_order"
)

// NewOrderNotification is the JSON payload sent to admins when a customer
// places an order. It is intentionally lightweight (no full order body) so the
// admin UI can show a toast/badge and link to the order detail page.
type NewOrderNotification struct {
	Type         string    `json:"type"`
	OrderID      string    `json:"order_id"`
	OrderNumber  string    `json:"order_number"`
	CustomerName string    `json:"customer_name"`
	Total        float64   `json:"total"`
	Currency     string    `json:"currency"`
	ItemCount    int       `json:"item_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// NotificationService turns domain events into SSE messages on the hub.
type NotificationService struct {
	hub    *sse.Hub
	logger *logrus.Logger
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(hub *sse.Hub, logger *logrus.Logger) *NotificationService {
	if logger == nil {
		logger = logrus.New()
	}
	return &NotificationService{hub: hub, logger: logger}
}

// NotifyNewOrder broadcasts a "new_order" event to every connected admin.
// It never blocks the caller and swallows errors (notifications are best-effort).
func (ns *NotificationService) NotifyNewOrder(o *interfaces.Order) {
	if ns == nil || ns.hub == nil || o == nil {
		return
	}

	customer := strings.TrimSpace(o.ShippingInfo.FirstName + " " + o.ShippingInfo.LastName)
	if customer == "" {
		customer = o.ShippingInfo.Phone
	}

	payload := NewOrderNotification{
		Type:         EventNewOrder,
		OrderID:      o.ID.String(),
		OrderNumber:  o.OrderNumber,
		CustomerName: customer,
		Total:        o.TotalPrice,
		Currency:     o.Currency,
		ItemCount:    len(o.Items),
		CreatedAt:    o.CreatedAt,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		ns.logger.WithError(err).Warn("failed to marshal new order notification")
		return
	}

	ns.hub.Broadcast(sse.Message{Event: EventNewOrder, Data: data})
}
