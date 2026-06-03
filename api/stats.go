package api

import "time"

// ============================================================================
// DASHBOARD STATS
// ============================================================================

// StatsKpis holds the headline numbers shown at the top of the dashboard.
type StatsKpis struct {
	// Chiffre d'affaire (commandes non annulées) sur la période + tendance vs
	// période précédente de même durée.
	Revenue      float64 `json:"revenue"`
	RevenuePrev  float64 `json:"revenue_prev"`
	RevenueTrend float64 `json:"revenue_trend"`

	Orders      int64   `json:"orders"`
	OrdersPrev  int64   `json:"orders_prev"`
	OrdersTrend float64 `json:"orders_trend"`

	// Commandes en attente de traitement (status = pending), tous temps confondus.
	PendingOrders int64 `json:"pending_orders"`

	// Panier moyen sur la période.
	AvgOrderValue float64 `json:"avg_order_value"`

	// Ratio de validation : part des commandes traitées (confirmées / expédiées /
	// livrées) parmi celles qui ont quitté l'état "en attente" (traitées + annulées).
	ValidationRate   float64 `json:"validation_rate"`
	CancellationRate float64 `json:"cancellation_rate"`

	NewCustomers int64 `json:"new_customers"`

	// Alertes stock : variants sous le seuil critique et variants en rupture.
	LowStockCount   int64 `json:"low_stock_count"`
	OutOfStockCount int64 `json:"out_of_stock_count"`
}

// StatusCount is an order count + revenue grouped by status.
type StatusCount struct {
	Status  string  `json:"status"`
	Count   int64   `json:"count"`
	Revenue float64 `json:"revenue"`
}

// DayPoint is a single day on the revenue timeline.
type DayPoint struct {
	Day     string  `json:"day"` // YYYY-MM-DD
	Revenue float64 `json:"revenue"`
	Orders  int64   `json:"orders"`
}

// GouvernoratStat aggregates orders & revenue by Tunisian gouvernorat.
type GouvernoratStat struct {
	Gouvernorat string  `json:"gouvernorat"`
	Orders      int64   `json:"orders"`
	Revenue     float64 `json:"revenue"`
}

// StockAlert is a product whose aggregated variant stock is low or out.
type StockAlert struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Stock     int64  `json:"stock"`
}

// ProductPerf is a best-seller line (units sold + revenue over the period).
type ProductPerf struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	ViewCount int64   `json:"view_count"`
	Units     int64   `json:"units"`
	Revenue   float64 `json:"revenue"`
}

// Recommendation flags surfaced for conversion analysis.
const (
	RecoFeature       = "feature"        // peu vu mais bonne conversion → à mettre en Featured
	RecoReviewPricing = "review_pricing" // très vu mais peu vendu → prix/photos à revoir
	RecoOK            = "ok"
)

// ProductConversion is the conversion-rate analysis for a single product.
type ProductConversion struct {
	ProductID      string  `json:"product_id"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	ViewCount      int64   `json:"view_count"`
	Units          int64   `json:"units"`
	ConversionRate float64 `json:"conversion_rate"` // (units / view_count) * 100
	IsFeatured     bool    `json:"is_featured"`
	Price          float64 `json:"price"`
	Recommendation string  `json:"recommendation"`
}

// DeadstockProduct is an aging product with stock but no recent traction.
type DeadstockProduct struct {
	ProductID string    `json:"product_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	ViewCount int64     `json:"view_count"`
	Stock     int64     `json:"stock"`
	CreatedAt time.Time `json:"created_at"`
}

// PaymentMethodStat aggregates orders & revenue by payment method.
type PaymentMethodStat struct {
	Method  string  `json:"method"`
	Count   int64   `json:"count"`
	Revenue float64 `json:"revenue"`
}

// StatsOverview is the full dashboard payload returned in a single request.
type StatsOverview struct {
	Days            int                 `json:"days"`
	GeneratedAt     time.Time           `json:"generated_at"`
	Kpis            StatsKpis           `json:"kpis"`
	OrdersByStatus  []StatusCount       `json:"orders_by_status"`
	RevenueByDay    []DayPoint          `json:"revenue_by_day"`
	TopGouvernorats []GouvernoratStat   `json:"top_gouvernorats"`
	StockAlerts     []StockAlert        `json:"stock_alerts"`
	TopProducts     []ProductPerf       `json:"top_products"`
	Conversion      []ProductConversion `json:"conversion"`
	Deadstock       []DeadstockProduct  `json:"deadstock"`
	PaymentMethods  []PaymentMethodStat `json:"payment_methods"`
}
