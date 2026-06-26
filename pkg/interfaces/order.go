package interfaces

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrderStatus enum
const (
	OrderStatusPending   = "pending"
	OrderStatusConfirmed = "confirmed"
	OrderStatusShipped   = "shipped"
	OrderStatusDelivered = "delivered"
	OrderStatusCancelled = "cancelled"
)

// PaymentMethod enum
const (
	PaymentMethodCash    = "cash"
	PaymentMethodCard    = "card"
	PaymentMethodD17     = "d17"
	PaymentMethodPaymee  = "paymee"
	PaymentMethodKonnect = "konnect"
)

// PaymentStatus enum
const (
	PaymentStatusPending  = "pending"
	PaymentStatusPaid     = "paid"
	PaymentStatusRefunded = "refunded"
	PaymentStatusFailed   = "failed"
)

// ShippingInfo represents the shipping address & contact (denormalized JSONB)
type ShippingInfo struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Phone       string `json:"phone"`
	Email       string `json:"email,omitempty"`
	Gouvernorat string `json:"gouvernorat"`
	Address     string `json:"address"`
	PostalCode  string `json:"postalCode,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// Value implements driver.Valuer for JSONB persistence
func (s ShippingInfo) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements sql.Scanner for JSONB persistence
func (s *ShippingInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for ShippingInfo")
	}
	return json.Unmarshal(bytes, s)
}

// Order represents a customer order (supports guest checkout via nullable UserID)
type Order struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	OrderNumber string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"order_number"`
	UserID      *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`

	Status string `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`

	// Pricing
	Subtotal       float64 `gorm:"type:numeric(10,2);not null" json:"subtotal"`
	ShippingFee    float64 `gorm:"type:numeric(10,2);not null;default:0" json:"shipping_fee"`
	DiscountAmount float64 `gorm:"type:numeric(10,2);not null;default:0" json:"discount_amount"`
	TotalPrice     float64 `gorm:"type:numeric(10,2);not null" json:"total_price"`
	Currency       string  `gorm:"type:varchar(3);not null;default:'TND'" json:"currency"`

	PromoCode *string `gorm:"type:varchar(50)" json:"promo_code,omitempty"`

	PaymentMethod string `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	PaymentStatus string `gorm:"type:varchar(20);not null;default:'pending'" json:"payment_status"`

	ShippingInfo ShippingInfo `gorm:"type:jsonb;not null" json:"shipping_info"`

	TrackingNumber *string    `gorm:"type:varchar(100)" json:"tracking_number,omitempty"`
	ShippedAt      *time.Time `json:"shipped_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CancelledAt    *time.Time `json:"cancelled_at,omitempty"`

	CustomerNotes string `gorm:"type:text" json:"customer_notes,omitempty"`

	Metadata JSONMap `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
}

func (Order) TableName() string { return "orders" }

// BeforeCreate hook
func (o *Order) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	now := time.Now()
	o.CreatedAt = now
	o.UpdatedAt = now
	if o.Currency == "" {
		o.Currency = "TND"
	}
	if o.Status == "" {
		o.Status = OrderStatusPending
	}
	return
}

// OrderItem represents a line item in an order
type OrderItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	OrderID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"order_id"`
	ProductID *uuid.UUID `gorm:"type:uuid" json:"product_id,omitempty"`

	// Snapshot product data
	ProductName  string `gorm:"type:varchar(255);not null" json:"product_name"`
	ProductImage string `gorm:"type:text" json:"product_image,omitempty"`
	ProductSlug  string `gorm:"type:varchar(255)" json:"product_slug,omitempty"`

	Size  string `gorm:"type:varchar(20)" json:"size,omitempty"`
	Color string `gorm:"type:varchar(100)" json:"color,omitempty"`

	UnitPrice float64 `gorm:"type:numeric(10,2);not null" json:"unit_price"`
	Quantity  int     `gorm:"not null" json:"quantity"`
	LineTotal float64 `gorm:"type:numeric(10,2);not null" json:"line_total"`
}

func (OrderItem) TableName() string { return "order_items" }

func (i *OrderItem) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}
