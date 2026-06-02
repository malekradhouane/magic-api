package interfaces

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Product represents a product (prêt-à-porter)
type Product struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:now()" json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`

	// Identification
	Slug string  `gorm:"type:varchar(255);uniqueIndex;not null" json:"slug"`
	Name string  `gorm:"type:varchar(255);not null" json:"name"`
	SKU  *string `gorm:"type:varchar(100);uniqueIndex" json:"sku,omitempty"`

	// Description
	Description     string `gorm:"type:text" json:"description"`
	DescriptionLong string `gorm:"type:text" json:"description_long,omitempty"`
	Entretien       string `gorm:"type:text" json:"entretien,omitempty"`

	// Pricing
	Price           float64 `gorm:"type:numeric(10,2);not null" json:"price"`
	OriginalPrice   *float64 `gorm:"type:numeric(10,2)" json:"original_price,omitempty"`
	DiscountPercent int      `gorm:"->" json:"discount_percent"` // computed column, read-only
	Currency        string   `gorm:"type:varchar(3);not null;default:'TND'" json:"currency"`

	// Categorization
	CategoryID *uuid.UUID `gorm:"type:uuid" json:"category_id"`
	Category   *Category  `gorm:"foreignKey:CategoryID" json:"category,omitempty"`

	// Gender / audience dimension (homme | femme | enfant | unisexe), independent
	// from the product-type category tree.
	Gender string `gorm:"type:varchar(20)" json:"gender,omitempty"`

	// Status flags
	IsNew      bool `gorm:"not null;default:false" json:"is_new"`
	IsOnSale   bool `gorm:"not null;default:false" json:"is_on_sale"`
	IsActive   bool `gorm:"not null;default:true" json:"is_active"`
	IsFeatured bool `gorm:"not null;default:false" json:"is_featured"`

	// Stats
	ViewCount int `gorm:"not null;default:0" json:"view_count"`
	SaleCount int `gorm:"not null;default:0" json:"sale_count"`

	// SEO
	MetaTitle       string `gorm:"type:varchar(255)" json:"meta_title,omitempty"`
	MetaDescription string `gorm:"type:text" json:"meta_description,omitempty"`

	// Tags
	Tags StringArray `gorm:"type:text[]" json:"tags"`

	// Metadata
	Metadata JSONMap `gorm:"type:jsonb" json:"metadata,omitempty"`

	// Relations (loaded on demand)
	Images   []ProductImage   `gorm:"foreignKey:ProductID" json:"images,omitempty"`
	Sizes    []ProductSize    `gorm:"foreignKey:ProductID" json:"sizes,omitempty"`
	Colors   []ProductColor   `gorm:"foreignKey:ProductID" json:"colors,omitempty"`
	Variants []ProductVariant `gorm:"foreignKey:ProductID" json:"variants,omitempty"`
}

// TableName overrides default GORM naming
func (Product) TableName() string { return "products" }

// BeforeCreate hook
func (p *Product) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Currency == "" {
		p.Currency = "TND"
	}
	return
}

// ProductImage represents a product photo
type ProductImage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	URL       string    `gorm:"type:text;not null" json:"url"`
	Alt       string    `gorm:"type:varchar(255)" json:"alt"`
	Position  int       `gorm:"not null;default:0" json:"position"`
	IsPrimary bool      `gorm:"not null;default:false" json:"is_primary"`
}

func (ProductImage) TableName() string { return "product_images" }

func (i *ProductImage) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}

// ProductSize represents available sizes with stock
type ProductSize struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Size      string    `gorm:"type:varchar(20);not null" json:"size"`
	Stock     int       `gorm:"not null;default:0" json:"stock"`
	Position  int       `gorm:"not null;default:0" json:"position"`
}

func (ProductSize) TableName() string { return "product_sizes" }

func (s *ProductSize) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}

// ProductColor represents available colors with stock
type ProductColor struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	Hex       string    `gorm:"type:varchar(7);not null" json:"hex"`
	Stock     int       `gorm:"not null;default:0" json:"stock"`
	Position  int       `gorm:"not null;default:0" json:"position"`
}

func (ProductColor) TableName() string { return "product_colors" }

func (c *ProductColor) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// ProductVariant represents stock for a specific (size, color) combination.
// This is the authoritative inventory unit (e.g. size M + color Noir = 20).
type ProductVariant struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Size      string    `gorm:"type:varchar(20);not null" json:"size"`
	Color     string    `gorm:"type:varchar(100);not null" json:"color"`
	Hex       string    `gorm:"type:varchar(7);not null;default:'#000000'" json:"hex"`
	Stock     int       `gorm:"not null;default:0" json:"stock"`
	Position  int       `gorm:"not null;default:0" json:"position"`
}

func (ProductVariant) TableName() string { return "product_variants" }

func (v *ProductVariant) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return
}
