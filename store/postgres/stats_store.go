package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/store/types"
)

// primaryImageSubquery resolves a product's display image: the one flagged
// primary, otherwise the first by position. Returns ” when none exists.
const primaryImageSubquery = `COALESCE((
    SELECT pi.url FROM product_images pi
    WHERE pi.product_id = p.id
    ORDER BY pi.is_primary DESC, pi.position ASC
    LIMIT 1
), '')`

var (
	_ types.StatsStore = &StatsStore{}

	theStatsStoreMtx sync.Mutex
	theStatsStore    *StatsStore
)

// StatsStore is the PostgreSQL implementation of StatsStore. It runs read-only
// aggregation queries for the admin dashboard.
type StatsStore struct {
	*Client
}

// NewStatsStore creates the singleton StatsStore
func NewStatsStore() (*StatsStore, error) {
	theStatsStoreMtx.Lock()
	defer theStatsStoreMtx.Unlock()

	if theStatsStore != nil {
		return theStatsStore, nil
	}
	MustClientInitialized(client)
	theStatsStore = &StatsStore{Client: client}

	logrus.Info("StatsStore created")
	return theStatsStore, nil
}

// Kpis computes the headline numbers for the dashboard.
func (ss *StatsStore) Kpis(ctx context.Context, since, prevSince, prevUntil time.Time, lowStockThreshold int) (*api.StatsKpis, error) {
	db := ss.session.GetDB().WithContext(ctx)
	k := &api.StatsKpis{}

	// Revenue + order count, current vs previous window (cancelled excluded).
	type sumRow struct {
		Revenue float64
		Cnt     int64
	}
	var cur, prev sumRow
	if err := db.Raw(
		`SELECT COALESCE(SUM(total_price),0) AS revenue, COUNT(*) AS cnt
		 FROM orders WHERE status <> 'cancelled' AND created_at >= ?`, since,
	).Scan(&cur).Error; err != nil {
		return nil, fmt.Errorf("kpis revenue: %w", err)
	}
	if err := db.Raw(
		`SELECT COALESCE(SUM(total_price),0) AS revenue, COUNT(*) AS cnt
		 FROM orders WHERE status <> 'cancelled' AND created_at >= ? AND created_at < ?`,
		prevSince, prevUntil,
	).Scan(&prev).Error; err != nil {
		return nil, fmt.Errorf("kpis revenue prev: %w", err)
	}
	k.Revenue = cur.Revenue
	k.Orders = cur.Cnt
	k.RevenuePrev = prev.Revenue
	k.OrdersPrev = prev.Cnt
	k.RevenueTrend = pctChange(cur.Revenue, prev.Revenue)
	k.OrdersTrend = pctChange(float64(cur.Cnt), float64(prev.Cnt))
	if cur.Cnt > 0 {
		k.AvgOrderValue = cur.Revenue / float64(cur.Cnt)
	}

	// Pending orders (all time).
	if err := db.Raw(`SELECT COUNT(*) FROM orders WHERE status = 'pending'`).
		Scan(&k.PendingOrders).Error; err != nil {
		return nil, fmt.Errorf("kpis pending: %w", err)
	}

	// Validation ratio over the period: processed vs (processed + cancelled).
	type valRow struct {
		Processed int64
		Cancelled int64
	}
	var v valRow
	if err := db.Raw(
		`SELECT
		   COUNT(*) FILTER (WHERE status IN ('confirmed','shipped','delivered')) AS processed,
		   COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
		 FROM orders WHERE created_at >= ?`, since,
	).Scan(&v).Error; err != nil {
		return nil, fmt.Errorf("kpis validation: %w", err)
	}
	decided := v.Processed + v.Cancelled
	if decided > 0 {
		k.ValidationRate = float64(v.Processed) / float64(decided) * 100
		k.CancellationRate = float64(v.Cancelled) / float64(decided) * 100
	}

	// Delivery success rate (COD): among orders that left the warehouse and
	// reached a final state, the share actually delivered. A cancelled order
	// that had already been shipped counts as a failed delivery (refus client).
	type delRow struct {
		Delivered int64
		Failed    int64
	}
	var d delRow
	if err := db.Raw(
		`SELECT
		   COUNT(*) FILTER (WHERE status = 'delivered') AS delivered,
		   COUNT(*) FILTER (WHERE status = 'cancelled' AND shipped_at IS NOT NULL) AS failed
		 FROM orders WHERE created_at >= ?`, since,
	).Scan(&d).Error; err != nil {
		return nil, fmt.Errorf("kpis delivery: %w", err)
	}
	if dispatched := d.Delivered + d.Failed; dispatched > 0 {
		k.DeliveryRate = float64(d.Delivered) / float64(dispatched) * 100
	}

	// New customers over the period.
	if err := db.Raw(
		`SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= ?`, since,
	).Scan(&k.NewCustomers).Error; err != nil {
		return nil, fmt.Errorf("kpis customers: %w", err)
	}

	// Stock alerts on the authoritative variants table.
	if err := db.Raw(
		`SELECT COUNT(*) FROM product_variants v
		 JOIN products p ON p.id = v.product_id
		 WHERE p.deleted_at IS NULL AND p.is_active = true
		   AND v.stock > 0 AND v.stock <= ?`, lowStockThreshold,
	).Scan(&k.LowStockCount).Error; err != nil {
		return nil, fmt.Errorf("kpis low stock: %w", err)
	}
	if err := db.Raw(
		`SELECT COUNT(*) FROM product_variants v
		 JOIN products p ON p.id = v.product_id
		 WHERE p.deleted_at IS NULL AND p.is_active = true AND v.stock = 0`,
	).Scan(&k.OutOfStockCount).Error; err != nil {
		return nil, fmt.Errorf("kpis out of stock: %w", err)
	}

	return k, nil
}

// OrdersByStatus returns counts + revenue grouped by status over the period.
func (ss *StatsStore) OrdersByStatus(ctx context.Context, since time.Time) ([]api.StatusCount, error) {
	var rows []api.StatusCount
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT status,
		        COUNT(*) AS count,
		        COALESCE(SUM(total_price),0) AS revenue
		 FROM orders
		 WHERE created_at >= ?
		 GROUP BY status
		 ORDER BY count DESC`, since,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("orders by status: %w", err)
	}
	return rows, nil
}

// RevenueByDay returns the daily revenue timeline (cancelled excluded). Gaps are
// filled by the service layer.
func (ss *StatsStore) RevenueByDay(ctx context.Context, since time.Time) ([]api.DayPoint, error) {
	var rows []api.DayPoint
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS day,
		        COALESCE(SUM(total_price),0) AS revenue,
		        COUNT(*) AS orders
		 FROM orders
		 WHERE status <> 'cancelled' AND created_at >= ?
		 GROUP BY 1
		 ORDER BY 1`, since,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("revenue by day: %w", err)
	}
	return rows, nil
}

// TopGouvernorats returns the best gouvernorats by revenue over the period.
func (ss *StatsStore) TopGouvernorats(ctx context.Context, since time.Time, limit int) ([]api.GouvernoratStat, error) {
	var rows []api.GouvernoratStat
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT COALESCE(NULLIF(shipping_info->>'gouvernorat',''), 'Inconnu') AS gouvernorat,
		        COUNT(*) AS orders,
		        COALESCE(SUM(total_price),0) AS revenue
		 FROM orders
		 WHERE status <> 'cancelled' AND created_at >= ?
		 GROUP BY 1
		 ORDER BY revenue DESC
		 LIMIT ?`, since, limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("top gouvernorats: %w", err)
	}
	return rows, nil
}

// StockAlerts lists active products whose aggregated variant stock is at or
// below the critical threshold (includes out-of-stock).
func (ss *StatsStore) StockAlerts(ctx context.Context, threshold, limit int) ([]api.StockAlert, error) {
	var rows []api.StockAlert
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT p.id AS product_id, p.name, p.slug,
		        `+primaryImageSubquery+` AS image,
		        COALESCE(SUM(v.stock),0) AS stock
		 FROM products p
		 JOIN product_variants v ON v.product_id = p.id
		 WHERE p.deleted_at IS NULL AND p.is_active = true
		 GROUP BY p.id, p.name, p.slug
		 HAVING COALESCE(SUM(v.stock),0) <= ?
		 ORDER BY stock ASC, p.name ASC
		 LIMIT ?`, threshold, limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("stock alerts: %w", err)
	}
	return rows, nil
}

// TopProducts returns best-sellers by revenue over the period.
func (ss *StatsStore) TopProducts(ctx context.Context, since time.Time, limit int) ([]api.ProductPerf, error) {
	var rows []api.ProductPerf
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT p.id AS product_id, p.name, p.slug, p.view_count,
		        `+primaryImageSubquery+` AS image,
		        COALESCE(SUM(oi.quantity),0) AS units,
		        COALESCE(SUM(oi.line_total),0) AS revenue
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 JOIN products p ON p.id = oi.product_id
		 WHERE o.status <> 'cancelled' AND o.created_at >= ?
		 GROUP BY p.id, p.name, p.slug, p.view_count
		 ORDER BY revenue DESC
		 LIMIT ?`, since, limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("top products: %w", err)
	}
	return rows, nil
}

// Conversion returns lifetime conversion data for active products that have at
// least minViews views. Units are summed from non-cancelled order items.
func (ss *StatsStore) Conversion(ctx context.Context, minViews, limit int) ([]api.ProductConversion, error) {
	var rows []api.ProductConversion
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT p.id AS product_id, p.name, p.slug, p.view_count,
		        p.is_featured, p.price,
		        COALESCE(s.units,0) AS units,
		        CASE WHEN p.view_count > 0
		             THEN (COALESCE(s.units,0)::float / p.view_count) * 100
		             ELSE 0 END AS conversion_rate
		 FROM products p
		 LEFT JOIN (
		     SELECT oi.product_id, SUM(oi.quantity) AS units
		     FROM order_items oi
		     JOIN orders o ON o.id = oi.order_id AND o.status <> 'cancelled'
		     GROUP BY oi.product_id
		 ) s ON s.product_id = p.id
		 WHERE p.deleted_at IS NULL AND p.is_active = true AND p.view_count >= ?
		 ORDER BY p.view_count DESC
		 LIMIT ?`, minViews, limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("conversion: %w", err)
	}
	return rows, nil
}

// Deadstock lists active products older than `days` days that still hold stock
// but recorded no sales in the same window. Ordered by lowest visibility first.
func (ss *StatsStore) Deadstock(ctx context.Context, days, limit int) ([]api.DeadstockProduct, error) {
	cutoff := fmt.Sprintf("%d days", days)
	var rows []api.DeadstockProduct
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT p.id AS product_id, p.name, p.slug, p.view_count, p.created_at,
		        `+primaryImageSubquery+` AS image,
		        COALESCE(SUM(v.stock),0) AS stock
		 FROM products p
		 LEFT JOIN product_variants v ON v.product_id = p.id
		 WHERE p.deleted_at IS NULL AND p.is_active = true
		   AND p.created_at <= now() - ?::interval
		   AND NOT EXISTS (
		       SELECT 1 FROM order_items oi
		       JOIN orders o ON o.id = oi.order_id
		       WHERE oi.product_id = p.id
		         AND o.status <> 'cancelled'
		         AND o.created_at >= now() - ?::interval
		   )
		 GROUP BY p.id, p.name, p.slug, p.view_count, p.created_at
		 HAVING COALESCE(SUM(v.stock),0) > 0
		 ORDER BY p.view_count ASC, stock DESC
		 LIMIT ?`, cutoff, cutoff, limit,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("deadstock: %w", err)
	}
	return rows, nil
}

// PaymentMethods returns the payment-method breakdown over the period.
func (ss *StatsStore) PaymentMethods(ctx context.Context, since time.Time) ([]api.PaymentMethodStat, error) {
	var rows []api.PaymentMethodStat
	err := ss.session.GetDB().WithContext(ctx).Raw(
		`SELECT payment_method AS method,
		        COUNT(*) AS count,
		        COALESCE(SUM(total_price),0) AS revenue
		 FROM orders
		 WHERE status <> 'cancelled' AND created_at >= ?
		 GROUP BY payment_method
		 ORDER BY count DESC`, since,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("payment methods: %w", err)
	}
	return rows, nil
}

// Sparklines returns one point per day in [since, now] for the KPI mini-charts.
// generate_series guarantees a value for every day (zero-filled gaps).
func (ss *StatsStore) Sparklines(ctx context.Context, since time.Time) (*api.StatsSparklines, error) {
	db := ss.session.GetDB().WithContext(ctx)
	now := time.Now()

	type oRow struct {
		Day       string
		Orders    int64
		Processed int64
		Delivered int64
	}
	var orows []oRow
	if err := db.Raw(
		`SELECT to_char(d::date, 'YYYY-MM-DD') AS day,
		        COALESCE(o.orders, 0) AS orders,
		        COALESCE(o.processed, 0) AS processed,
		        COALESCE(o.delivered, 0) AS delivered
		 FROM generate_series(?::date, ?::date, '1 day') d
		 LEFT JOIN (
		     SELECT date_trunc('day', created_at)::date AS dd,
		            COUNT(*) AS orders,
		            COUNT(*) FILTER (WHERE status IN ('confirmed','shipped','delivered')) AS processed,
		            COUNT(*) FILTER (WHERE status = 'delivered') AS delivered
		     FROM orders WHERE created_at >= ?::date
		     GROUP BY 1
		 ) o ON o.dd = d::date
		 ORDER BY day`, since, now, since,
	).Scan(&orows).Error; err != nil {
		return nil, fmt.Errorf("sparklines orders: %w", err)
	}

	type cRow struct {
		Day string
		Cnt int64
	}
	var crows []cRow
	if err := db.Raw(
		`SELECT to_char(d::date, 'YYYY-MM-DD') AS day, COALESCE(c.cnt, 0) AS cnt
		 FROM generate_series(?::date, ?::date, '1 day') d
		 LEFT JOIN (
		     SELECT date_trunc('day', created_at)::date AS dd, COUNT(*) AS cnt
		     FROM users WHERE deleted_at IS NULL AND created_at >= ?::date
		     GROUP BY 1
		 ) c ON c.dd = d::date
		 ORDER BY day`, since, now, since,
	).Scan(&crows).Error; err != nil {
		return nil, fmt.Errorf("sparklines customers: %w", err)
	}

	sp := &api.StatsSparklines{
		Orders:       make([]float64, 0, len(orows)),
		Processed:    make([]float64, 0, len(orows)),
		Delivered:    make([]float64, 0, len(orows)),
		NewCustomers: make([]float64, 0, len(crows)),
	}
	for _, r := range orows {
		sp.Orders = append(sp.Orders, float64(r.Orders))
		sp.Processed = append(sp.Processed, float64(r.Processed))
		sp.Delivered = append(sp.Delivered, float64(r.Delivered))
	}
	for _, r := range crows {
		sp.NewCustomers = append(sp.NewCustomers, float64(r.Cnt))
	}
	return sp, nil
}

// pctChange returns the percentage change from prev to cur, rounded to 1 dp.
func pctChange(cur, prev float64) float64 {
	if prev == 0 {
		if cur == 0 {
			return 0
		}
		return 100
	}
	return roundTo((cur-prev)/prev*100, 1)
}

func roundTo(v float64, dp int) float64 {
	p := 1.0
	for i := 0; i < dp; i++ {
		p *= 10
	}
	if v >= 0 {
		return float64(int64(v*p+0.5)) / p
	}
	return float64(int64(v*p-0.5)) / p
}
