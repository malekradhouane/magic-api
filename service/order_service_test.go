package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/errs"
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ---------------------------------------------------------------------------
// OrderService helpers
// ---------------------------------------------------------------------------

func newTestOrderService(
	orderStore *MockOrderStore,
	productStore *MockProductStore,
	promoStore *MockPromoStore,
	userStore *MockUserStore,
	addressStore *MockAddressStore,
) *OrderService {
	promoSvc := NewPromoService(promoStore, nil)
	addrSvc := NewAddressService(addressStore, nil)
	return NewOrderService(
		orderStore, productStore, promoSvc, userStore, addrSvc,
		nil, "", "", "", nil,
	)
}

// ---------------------------------------------------------------------------
// OrderService.Create
// ---------------------------------------------------------------------------

func TestOrderService_Create_NoItems(t *testing.T) {
	t.Parallel()
	svc := newTestOrderService(
		new(MockOrderStore), new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{Items: nil}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no items")
}

func TestOrderService_Create_InvalidQuantity(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: uuid.New().String(), Quantity: 0},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quantity")
}

func TestOrderService_Create_ProductNotFound(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New().String()
	prodStore.On("GetByID", mock.Anything, prodID).
		Return((*interfaces.Product)(nil), errors.New("not found"))

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID, Quantity: 1, Size: "M", Color: "Noir"},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOrderService_Create_InactiveProduct(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID, Name: "Inactive", IsActive: false}, nil)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 1, Size: "M", Color: "Noir"},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestOrderService_Create_MissingSize(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID, Name: "Prod", IsActive: true}, nil)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 1, Size: "", Color: "Noir"},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "taille requise")
}

func TestOrderService_Create_MissingColor(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{ID: prodID, Name: "Prod", IsActive: true}, nil)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 1, Size: "M", Color: ""},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "couleur requise")
}

func TestOrderService_Create_InsufficientStock(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	svc := newTestOrderService(
		new(MockOrderStore), prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{
			ID: prodID, Name: "Prod", IsActive: true,
			Variants: []interfaces.ProductVariant{
				{Size: "M", Color: "Noir", Stock: 1},
			},
		}, nil)

	_, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 5, Size: "M", Color: "Noir"},
		},
	}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stock insuffisant")
}

func TestOrderService_Create_FreeShipping(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{
			ID: prodID, Name: "Expensive", IsActive: true, Price: 200.0,
			Variants: []interfaces.ProductVariant{
				{Size: "M", Color: "Noir", Stock: 10},
			},
		}, nil)
	prodStore.On("DecrementVariantStock", mock.Anything, prodID.String(), "M", "Noir", 1).Return(nil)

	createdOrder := &interfaces.Order{
		ID:          uuid.New(),
		OrderNumber: "AFR-TEST",
		TotalPrice:  200.0,
		ShippingFee: 0,
	}
	orderStore.On("Create", mock.Anything, mock.MatchedBy(func(o *interfaces.Order) bool {
		return o.ShippingFee == 0 // free shipping when subtotal >= 150
	}), mock.Anything).Return(createdOrder, nil)

	got, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 1, Size: "M", Color: "Noir"},
		},
		ShippingInfo: interfaces.ShippingInfo{
			FirstName: "John", LastName: "Doe",
			Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue",
		},
	}, "")
	require.NoError(t, err)
	assert.Equal(t, float64(0), got.ShippingFee)
}

func TestOrderService_Create_WithShippingFee(t *testing.T) {
	t.Parallel()
	prodStore := new(MockProductStore)
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, prodStore,
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	prodID := uuid.New()
	prodStore.On("GetByID", mock.Anything, prodID.String()).
		Return(&interfaces.Product{
			ID: prodID, Name: "Cheap", IsActive: true, Price: 50.0,
			Variants: []interfaces.ProductVariant{
				{Size: "M", Color: "Noir", Stock: 10},
			},
		}, nil)
	prodStore.On("DecrementVariantStock", mock.Anything, prodID.String(), "M", "Noir", 1).Return(nil)

	orderStore.On("Create", mock.Anything, mock.MatchedBy(func(o *interfaces.Order) bool {
		return o.ShippingFee == StandardShippingFee
	}), mock.Anything).Return(&interfaces.Order{
		ID: uuid.New(), OrderNumber: "AFR-TEST", ShippingFee: StandardShippingFee,
	}, nil)

	got, err := svc.Create(context.Background(), &api.CreateOrderRequest{
		Items: []api.CreateOrderItemRequest{
			{ProductID: prodID.String(), Quantity: 1, Size: "M", Color: "Noir"},
		},
		ShippingInfo: interfaces.ShippingInfo{
			FirstName: "John", LastName: "Doe",
			Phone: "+21622334455", Gouvernorat: "Tunis", Address: "Rue",
		},
	}, "")
	require.NoError(t, err)
	assert.Equal(t, StandardShippingFee, got.ShippingFee)
}

// ---------------------------------------------------------------------------
// OrderService.Get
// ---------------------------------------------------------------------------

func TestOrderService_Get_Success(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	orderStore.On("GetByID", mock.Anything, orderID.String()).
		Return(&interfaces.Order{ID: orderID}, nil)

	got, err := svc.Get(context.Background(), orderID.String())
	require.NoError(t, err)
	assert.Equal(t, orderID, got.ID)
}

func TestOrderService_Get_NotFound(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("GetByID", mock.Anything, "missing").
		Return((*interfaces.Order)(nil), errs.ErrNoSuchEntity)

	_, err := svc.Get(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// OrderService.GetForAdmin
// ---------------------------------------------------------------------------

func TestOrderService_GetForAdmin_ByUUID(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	orderStore.On("GetByID", mock.Anything, orderID.String()).
		Return(&interfaces.Order{ID: orderID}, nil)

	got, err := svc.GetForAdmin(context.Background(), orderID.String())
	require.NoError(t, err)
	assert.Equal(t, orderID, got.ID)
}

func TestOrderService_GetForAdmin_ByOrderNumber(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("GetByOrderNumber", mock.Anything, "AFR-20260101-1234").
		Return(&interfaces.Order{OrderNumber: "AFR-20260101-1234"}, nil)

	got, err := svc.GetForAdmin(context.Background(), "AFR-20260101-1234")
	require.NoError(t, err)
	assert.Equal(t, "AFR-20260101-1234", got.OrderNumber)
}

func TestOrderService_GetForAdmin_Empty(t *testing.T) {
	t.Parallel()
	svc := newTestOrderService(
		new(MockOrderStore), new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	_, err := svc.GetForAdmin(context.Background(), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

// ---------------------------------------------------------------------------
// OrderService.GetForCustomer
// ---------------------------------------------------------------------------

func TestOrderService_GetForCustomer_MissingPhone(t *testing.T) {
	t.Parallel()
	svc := newTestOrderService(
		new(MockOrderStore), new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	_, err := svc.GetForCustomer(context.Background(), "id", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phone is required")
}

func TestOrderService_GetForCustomer_Success(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("GetByPhone", mock.Anything, "id-1", "+21622334455").
		Return(&interfaces.Order{ID: uuid.New()}, nil)

	got, err := svc.GetForCustomer(context.Background(), "id-1", "+21622334455")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

// ---------------------------------------------------------------------------
// OrderService.ListByUser
// ---------------------------------------------------------------------------

func TestOrderService_ListByUser_Success(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("ListByUserID", mock.Anything, "u1", 10, 0).
		Return([]*interfaces.Order{{ID: uuid.New()}}, int64(1), nil)

	got, err := svc.ListByUser(context.Background(), "u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, got.Orders, 1)
	assert.Equal(t, int64(1), got.TotalCount)
	assert.False(t, got.HasMore)
}

func TestOrderService_ListByUser_DefaultLimit(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("ListByUserID", mock.Anything, "u1", 0, 0).
		Return([]*interfaces.Order{}, int64(0), nil)

	got, err := svc.ListByUser(context.Background(), "u1", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 20, got.Limit)
}

// ---------------------------------------------------------------------------
// OrderService.List (admin)
// ---------------------------------------------------------------------------

func TestOrderService_List_Success(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	filters := &api.ListOrdersFilters{Limit: 50, Offset: 0}
	orderStore.On("List", mock.Anything, filters).
		Return([]*interfaces.Order{{ID: uuid.New()}}, int64(1), nil)

	got, err := svc.List(context.Background(), filters)
	require.NoError(t, err)
	assert.Len(t, got.Orders, 1)
}

// ---------------------------------------------------------------------------
// OrderService.UpdateStatus
// ---------------------------------------------------------------------------

func TestOrderService_UpdateStatus_Success(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	confirmed := interfaces.OrderStatusConfirmed
	orderStore.On("GetByID", mock.Anything, orderID.String()).
		Return(&interfaces.Order{ID: orderID, Status: interfaces.OrderStatusPending}, nil)
	orderStore.On("UpdateStatus", mock.Anything, orderID.String(), mock.Anything).
		Return(&interfaces.Order{ID: orderID, Status: interfaces.OrderStatusConfirmed}, nil)

	got, err := svc.UpdateStatus(context.Background(), orderID.String(), &api.UpdateOrderStatusRequest{
		Status: &confirmed,
	})
	require.NoError(t, err)
	assert.Equal(t, interfaces.OrderStatusConfirmed, got.Status)
}

func TestOrderService_UpdateStatus_InvalidStatus(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	bad := "invalid_status"
	orderStore.On("GetByID", mock.Anything, orderID.String()).
		Return(&interfaces.Order{ID: orderID, Status: interfaces.OrderStatusPending}, nil)

	_, err := svc.UpdateStatus(context.Background(), orderID.String(), &api.UpdateOrderStatusRequest{
		Status: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestOrderService_UpdateStatus_InvalidPaymentStatus(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	bad := "bad_payment"
	orderStore.On("GetByID", mock.Anything, orderID.String()).
		Return(&interfaces.Order{ID: orderID, Status: interfaces.OrderStatusPending}, nil)

	_, err := svc.UpdateStatus(context.Background(), orderID.String(), &api.UpdateOrderStatusRequest{
		PaymentStatus: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment status")
}

func TestOrderService_UpdateStatus_NotFound(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderStore.On("GetByID", mock.Anything, "missing").
		Return((*interfaces.Order)(nil), errs.ErrNoSuchEntity)

	_, err := svc.UpdateStatus(context.Background(), "missing", &api.UpdateOrderStatusRequest{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrNoSuchEntity))
}

func TestOrderService_UpdateStatus_NoChanges(t *testing.T) {
	t.Parallel()
	orderStore := new(MockOrderStore)
	svc := newTestOrderService(
		orderStore, new(MockProductStore),
		new(MockPromoStore), new(MockUserStore), new(MockAddressStore),
	)

	orderID := uuid.New()
	existing := &interfaces.Order{ID: orderID, Status: interfaces.OrderStatusPending}
	orderStore.On("GetByID", mock.Anything, orderID.String()).Return(existing, nil)

	got, err := svc.UpdateStatus(context.Background(), orderID.String(), &api.UpdateOrderStatusRequest{})
	require.NoError(t, err)
	assert.Equal(t, existing, got)
}

// ---------------------------------------------------------------------------
// hasVariantStock
// ---------------------------------------------------------------------------

func TestHasVariantStock(t *testing.T) {
	t.Parallel()

	product := &interfaces.Product{
		Variants: []interfaces.ProductVariant{
			{Size: "M", Color: "Noir", Stock: 5},
			{Size: "L", Color: "Blanc", Stock: 0},
		},
	}

	tests := []struct {
		name     string
		size     string
		color    string
		quantity int
		want     bool
	}{
		{"sufficient stock", "M", "Noir", 3, true},
		{"exact stock", "M", "Noir", 5, true},
		{"insufficient stock", "M", "Noir", 10, false},
		{"zero stock", "L", "Blanc", 1, false},
		{"variant not found", "XL", "Noir", 1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hasVariantStock(product, tc.size, tc.color, tc.quantity)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// orderMetadata helpers
// ---------------------------------------------------------------------------

func TestOrderMetadataHelpers(t *testing.T) {
	t.Parallel()

	t.Run("orderStockWasReserved", func(t *testing.T) {
		o := &interfaces.Order{Metadata: interfaces.JSONMap{"stock_reserved": true}}
		assert.True(t, orderStockWasReserved(o))

		o2 := &interfaces.Order{Metadata: interfaces.JSONMap{}}
		assert.False(t, orderStockWasReserved(o2))
	})

	t.Run("orderStockWasReleased", func(t *testing.T) {
		o := &interfaces.Order{Metadata: interfaces.JSONMap{"stock_released": true}}
		assert.True(t, orderStockWasReleased(o))
	})

	t.Run("orderMetadataMarkReserved", func(t *testing.T) {
		m := orderMetadataMarkReserved(interfaces.JSONMap{"existing": "value"})
		assert.True(t, m.Bool("stock_reserved"))
		assert.Equal(t, "value", m["existing"])
	})

	t.Run("orderMetadataMarkReleased", func(t *testing.T) {
		m := orderMetadataMarkReleased(interfaces.JSONMap{})
		assert.True(t, m.Bool("stock_released"))
	})

	t.Run("nil metadata", func(t *testing.T) {
		m := orderMetadataMarkReserved(nil)
		assert.True(t, m.Bool("stock_reserved"))
	})
}

// ---------------------------------------------------------------------------
// formatMoney
// ---------------------------------------------------------------------------

func TestFormatMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		amount   float64
		currency string
		want     string
	}{
		{89.99, "TND", "89.99 TND"},
		{0, "TND", "0.00 TND"},
		{100, "", "100.00 TND"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatMoney(tc.amount, tc.currency))
		})
	}
}

// ---------------------------------------------------------------------------
// formatPaymentMethod
// ---------------------------------------------------------------------------

func TestFormatPaymentMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		want   string
	}{
		{interfaces.PaymentMethodCash, "Paiement à la livraison (espèces)"},
		{interfaces.PaymentMethodCard, "Carte bancaire"},
		{interfaces.PaymentMethodD17, "D17"},
		{interfaces.PaymentMethodPaymee, "Paymee"},
		{interfaces.PaymentMethodKonnect, "Konnect"},
		{"unknown", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, formatPaymentMethod(tc.method))
		})
	}
}
