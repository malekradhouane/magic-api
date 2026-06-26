package interfaces

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PromoDiscountType enum
const (
	PromoTypePercentage = "percentage"
	PromoTypeFixed      = "fixed"
)

// PromoCode represents a discount code
type PromoCode struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	Code        string `gorm:"type:varchar(50);uniqueIndex;not null" json:"code"`
	Description string `gorm:"type:text" json:"description,omitempty"`

	DiscountType  string  `gorm:"type:varchar(20);not null" json:"discount_type"` // 'percentage' | 'fixed'
	DiscountValue float64 `gorm:"type:numeric(10,2);not null" json:"discount_value"`

	MinOrderTotal float64  `gorm:"type:numeric(10,2);default:0" json:"min_order_total"`
	MaxDiscount   *float64 `gorm:"type:numeric(10,2)" json:"max_discount,omitempty"`

	UsageLimit   *int `json:"usage_limit,omitempty"`
	UsageCount   int  `gorm:"not null;default:0" json:"usage_count"`
	PerUserLimit *int `gorm:"default:1" json:"per_user_limit,omitempty"`

	StartsAt  *time.Time `json:"starts_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsActive  bool       `gorm:"not null;default:true" json:"is_active"`
}

func (PromoCode) TableName() string { return "promo_codes" }

// BeforeCreate hook
func (p *PromoCode) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return
}

// IsValid returns true if the promo code is currently usable
func (p *PromoCode) IsValid() bool {
	if !p.IsActive {
		return false
	}
	now := time.Now()
	if p.StartsAt != nil && now.Before(*p.StartsAt) {
		return false
	}
	if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
		return false
	}
	if p.UsageLimit != nil && p.UsageCount >= *p.UsageLimit {
		return false
	}
	return true
}

// CalculateDiscount returns the discount amount for a given subtotal
func (p *PromoCode) CalculateDiscount(subtotal float64) float64 {
	if subtotal < p.MinOrderTotal {
		return 0
	}

	var discount float64
	switch p.DiscountType {
	case PromoTypePercentage:
		discount = subtotal * (p.DiscountValue / 100)
	case PromoTypeFixed:
		discount = p.DiscountValue
	}

	if p.MaxDiscount != nil && discount > *p.MaxDiscount {
		discount = *p.MaxDiscount
	}
	if discount > subtotal {
		discount = subtotal
	}
	return discount
}

// PromoUsage tracks individual uses of a promo code
type PromoUsage struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
	PromoCodeID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"promo_code_id"`
	UserID         *uuid.UUID `gorm:"type:uuid" json:"user_id,omitempty"`
	OrderID        *uuid.UUID `gorm:"type:uuid" json:"order_id,omitempty"`
	DiscountAmount float64    `gorm:"type:numeric(10,2);not null" json:"discount_amount"`
}

func (PromoUsage) TableName() string { return "promo_usages" }

func (u *PromoUsage) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
