package api

import (
	"github.com/malekradhouane/magic/pkg/interfaces"
)

// ============================================================================
// PRODUCTS
// ============================================================================

// ProductFilters represents query filters for product listings
type ProductFilters struct {
	Category    string   `form:"category"`
	Gender      string   `form:"gender"`
	Sizes       []string `form:"sizes"`
	Colors      []string `form:"colors"`
	MinPrice    *float64 `form:"min_price"`
	MaxPrice    *float64 `form:"max_price"`
	IsNew       *bool    `form:"is_new"`
	IsOnSale    *bool    `form:"is_on_sale"`
	IsFeatured  *bool    `form:"is_featured"`
	StockStatus string   `form:"stock_status"` // all | in | low | out | low_or_out
	Sort        string   `form:"sort"`         // relevance | newest | price-asc | price-desc
	Search      string   `form:"search"`
	Limit       int      `form:"limit,default=20"`
	Offset      int      `form:"offset,default=0"`
}

// ProductsResponse is the paginated products response
type ProductsResponse struct {
	Products   []*interfaces.Product `json:"products"`
	TotalCount int64                 `json:"total_count"`
	HasMore    bool                  `json:"has_more"`
	Limit      int                   `json:"limit"`
	Offset     int                   `json:"offset"`
}

// CreateProductRequest is used by admins to create a product
type CreateProductRequest struct {
	Name            string                        `json:"name" valid:"required"`
	SKU             *string                       `json:"sku,omitempty"`
	Description     string                        `json:"description"`
	DescriptionLong string                        `json:"description_long"`
	Entretien       string                        `json:"entretien"`
	Price           float64                       `json:"price" valid:"required"`
	OriginalPrice   *float64                      `json:"original_price"`
	CategoryID      *string                       `json:"category_id"`
	Gender          string                        `json:"gender"`
	IsNew           bool                          `json:"is_new"`
	IsOnSale        bool                          `json:"is_on_sale"`
	IsActive        bool                          `json:"is_active"`
	IsFeatured      bool                          `json:"is_featured"`
	Tags            []string                      `json:"tags"`
	MetaTitle       string                        `json:"meta_title"`
	MetaDescription string                        `json:"meta_description"`
	Images          []CreateProductImageRequest   `json:"images"`
	Sizes           []CreateProductSizeRequest    `json:"sizes"`
	Colors          []CreateProductColorRequest   `json:"colors"`
	Variants        []CreateProductVariantRequest `json:"variants"`
}

type CreateProductImageRequest struct {
	URL       string `json:"url" valid:"required"`
	Alt       string `json:"alt"`
	Position  int    `json:"position"`
	IsPrimary bool   `json:"is_primary"`
	// Color links the image to a specific product color (matches
	// ProductColor.Name). Empty means the image is generic and shown for
	// every color.
	Color string `json:"color"`
}

type CreateProductSizeRequest struct {
	Size     string `json:"size" valid:"required"`
	Stock    int    `json:"stock"`
	Position int    `json:"position"`
}

type CreateProductColorRequest struct {
	Name     string `json:"name" valid:"required"`
	Hex      string `json:"hex" valid:"required"`
	Stock    int    `json:"stock"`
	Position int    `json:"position"`
}

// CreateProductVariantRequest is the authoritative stock unit: stock for a
// specific (size, color) combination.
type CreateProductVariantRequest struct {
	Size     string `json:"size" valid:"required"`
	Color    string `json:"color" valid:"required"`
	Hex      string `json:"hex"`
	Stock    int    `json:"stock"`
	Position int    `json:"position"`
}

// UpdateProductRequest is the full admin form payload (magic-admin always sends all fields).
type UpdateProductRequest struct {
	Name            string                        `json:"name" valid:"required"`
	SKU             string                        `json:"sku"`
	Description     string                        `json:"description"`
	DescriptionLong string                        `json:"description_long"`
	Entretien       string                        `json:"entretien"`
	Price           float64                       `json:"price"`
	OriginalPrice   *float64                      `json:"original_price"`
	CategoryID      string                        `json:"category_id"`
	Gender          string                        `json:"gender"`
	IsNew           bool                          `json:"is_new"`
	IsOnSale        bool                          `json:"is_on_sale"`
	IsActive        bool                          `json:"is_active"`
	IsFeatured      bool                          `json:"is_featured"`
	Tags            []string                      `json:"tags"`
	MetaTitle       string                        `json:"meta_title"`
	MetaDescription string                        `json:"meta_description"`
	Images          []CreateProductImageRequest   `json:"images"`
	Sizes           []CreateProductSizeRequest    `json:"sizes"`
	Colors          []CreateProductColorRequest   `json:"colors"`
	Variants        []CreateProductVariantRequest `json:"variants"`
}

// ============================================================================
// CATEGORIES
// ============================================================================

// CreateCategoryRequest is used by admins to create a category
type CreateCategoryRequest struct {
	Name        string             `json:"name" valid:"required"`
	Description string             `json:"description"`
	ImageURL    string             `json:"image_url"`
	ParentID    *string            `json:"parent_id"`
	Position    int                `json:"position"`
	IsActive    bool               `json:"is_active"`
	Metadata    interfaces.JSONMap `json:"metadata,omitempty"`
}

// UpdateCategoryRequest is used by admins to update a category
type UpdateCategoryRequest struct {
	Name        *string             `json:"name,omitempty"`
	Description *string             `json:"description,omitempty"`
	ImageURL    *string             `json:"image_url,omitempty"`
	ParentID    *string             `json:"parent_id,omitempty"`
	Position    *int                `json:"position,omitempty"`
	IsActive    *bool               `json:"is_active,omitempty"`
	Metadata    *interfaces.JSONMap `json:"metadata,omitempty"`
}

// ============================================================================
// ORDERS
// ============================================================================

// CreateOrderRequest is the request payload from the frontend (cart → order)
type CreateOrderRequest struct {
	Items         []CreateOrderItemRequest `json:"items" valid:"required"`
	ShippingInfo  interfaces.ShippingInfo  `json:"shipping_info" valid:"required"`
	PaymentMethod string                   `json:"payment_method"`
	PromoCode     string                   `json:"promo_code,omitempty"`
	CustomerNotes string                   `json:"customer_notes,omitempty"`
}

// CreateOrderItemRequest is a line item in CreateOrderRequest
type CreateOrderItemRequest struct {
	ProductID string `json:"product_id" valid:"required"`
	Size      string `json:"size,omitempty"`
	Color     string `json:"color,omitempty"`
	Quantity  int    `json:"quantity" valid:"required"`
}

// OrderResponse is returned after order creation
type OrderResponse struct {
	Success bool              `json:"success"`
	OrderID string            `json:"order_id"`
	Order   *interfaces.Order `json:"order"`
}

// UpdateOrderStatusRequest is used by admins
type UpdateOrderStatusRequest struct {
	Status         *string `json:"status,omitempty"`
	PaymentStatus  *string `json:"payment_status,omitempty"`
	TrackingNumber *string `json:"tracking_number,omitempty"`
}

// ListOrdersFilters for admin order listing
type ListOrdersFilters struct {
	Status        string `form:"status"`
	PaymentStatus string `form:"payment_status"`
	PaymentMethod string `form:"payment_method"`
	UserID        string `form:"user_id"`
	Phone         string `form:"phone"`
	Search        string `form:"search"`
	Limit         int    `form:"limit,default=50"`
	Offset        int    `form:"offset,default=0"`
}

// OrdersResponse paginated orders response
type OrdersResponse struct {
	Orders     []*interfaces.Order `json:"orders"`
	TotalCount int64               `json:"total_count"`
	HasMore    bool                `json:"has_more"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

// ============================================================================
// ADDRESSES
// ============================================================================

// CreateAddressRequest is used to add a saved address to the user profile.
type CreateAddressRequest struct {
	Label       string `json:"label"`
	FirstName   string `json:"firstName" valid:"required"`
	LastName    string `json:"lastName" valid:"required"`
	Phone       string `json:"phone" valid:"required"`
	Gouvernorat string `json:"gouvernorat" valid:"required"`
	Address     string `json:"address" valid:"required"`
	PostalCode  string `json:"postalCode,omitempty"`
	IsDefault   bool   `json:"isDefault"`
}

// UpdateAddressRequest is used to update a saved address.
type UpdateAddressRequest struct {
	Label       *string `json:"label,omitempty"`
	FirstName   *string `json:"firstName,omitempty"`
	LastName    *string `json:"lastName,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	Gouvernorat *string `json:"gouvernorat,omitempty"`
	Address     *string `json:"address,omitempty"`
	PostalCode  *string `json:"postalCode,omitempty"`
	IsDefault   *bool   `json:"isDefault,omitempty"`
}

// AddressesResponse lists addresses for the current user.
type AddressesResponse struct {
	Addresses []*interfaces.Address `json:"addresses"`
}

// ============================================================================
// PROMO CODES
// ============================================================================

// ValidatePromoRequest is used by frontend to validate a promo code at checkout
type ValidatePromoRequest struct {
	Code     string  `json:"code" valid:"required"`
	Subtotal float64 `json:"subtotal" valid:"required"`
}

// ValidatePromoResponse returns whether the promo is valid + computed discount
type ValidatePromoResponse struct {
	Valid          bool    `json:"valid"`
	Code           string  `json:"code,omitempty"`
	DiscountAmount float64 `json:"discount_amount"`
	DiscountType   string  `json:"discount_type,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// CreatePromoRequest used by admins to create a promo code
type CreatePromoRequest struct {
	Code          string   `json:"code" valid:"required"`
	Description   string   `json:"description"`
	DiscountType  string   `json:"discount_type" valid:"required"` // percentage | fixed
	DiscountValue float64  `json:"discount_value" valid:"required"`
	MinOrderTotal float64  `json:"min_order_total"`
	MaxDiscount   *float64 `json:"max_discount,omitempty"`
	UsageLimit    *int     `json:"usage_limit,omitempty"`
	PerUserLimit  *int     `json:"per_user_limit,omitempty"`
	StartsAt      *string  `json:"starts_at,omitempty"`
	ExpiresAt     *string  `json:"expires_at,omitempty"`
	IsActive      bool     `json:"is_active"`
}

// ============================================================================
// SETTINGS
// ============================================================================

// UpdateSettingRequest is the payload for PUT /admin/settings/:key
type UpdateSettingRequest struct {
	Value map[string]interface{} `json:"value" binding:"required"`
}

// ============================================================================
// PRODUCT IMPORT
// ============================================================================

// ImportProductsRequest is used to import products from a CSV file
type ImportProductsRequest struct {
	File   string `form:"file" binding:"required"` // CSV file path or identifier
	Gender string `form:"gender"`                  // Optional gender for logging
}
