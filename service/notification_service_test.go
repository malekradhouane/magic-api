package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/malekradhouane/magic/pkg/interfaces"
	"github.com/malekradhouane/magic/pkg/sse"
)

// ---------------------------------------------------------------------------
// NotificationService.NotifyNewOrder
// ---------------------------------------------------------------------------

func TestNotificationService_NotifyNewOrder_NilService(t *testing.T) {
	t.Parallel()

	// Should not panic when service is nil.
	var ns *NotificationService
	ns.NotifyNewOrder(&interfaces.Order{})
}

func TestNotificationService_NotifyNewOrder_NilHub(t *testing.T) {
	t.Parallel()

	ns := &NotificationService{hub: nil}
	ns.NotifyNewOrder(&interfaces.Order{}) // should not panic
}

func TestNotificationService_NotifyNewOrder_NilOrder(t *testing.T) {
	t.Parallel()

	hub := sse.NewHub()
	ns := NewNotificationService(hub, nil)
	ns.NotifyNewOrder(nil) // should not panic
}

func TestNotificationService_NotifyNewOrder_Broadcasts(t *testing.T) {
	t.Parallel()

	hub := sse.NewHub()
	ns := NewNotificationService(hub, nil)

	// Subscribe a client.
	_, ch := hub.Subscribe()

	orderID := uuid.New()
	ns.NotifyNewOrder(&interfaces.Order{
		ID:          orderID,
		OrderNumber: "AFR-20260616-1234",
		TotalPrice:  120.50,
		Currency:    "TND",
		CreatedAt:   time.Now(),
		ShippingInfo: interfaces.ShippingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Phone:     "+21622334455",
		},
		Items: []interfaces.OrderItem{{}, {}},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, EventNewOrder, msg.Event)
		assert.Contains(t, string(msg.Data), orderID.String())
		assert.Contains(t, string(msg.Data), "AFR-20260616-1234")
		assert.Contains(t, string(msg.Data), "John Doe")
	default:
		t.Fatal("expected a message on the SSE channel")
	}
}

func TestNotificationService_NotifyNewOrder_FallbackCustomerName(t *testing.T) {
	t.Parallel()

	hub := sse.NewHub()
	ns := NewNotificationService(hub, nil)

	_, ch := hub.Subscribe()

	ns.NotifyNewOrder(&interfaces.Order{
		ID:          uuid.New(),
		OrderNumber: "AFR-TEST",
		Currency:    "TND",
		ShippingInfo: interfaces.ShippingInfo{
			Phone: "+21622334455",
			// FirstName and LastName are empty.
		},
	})

	select {
	case msg := <-ch:
		// Customer name should fall back to phone.
		assert.Contains(t, string(msg.Data), "+21622334455")
	default:
		t.Fatal("expected a message on the SSE channel")
	}
}
