package service

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/malekradhouane/magic/api"
	"github.com/malekradhouane/magic/store/types"
)

const (
	// LowStockThreshold: un variant à ce niveau ou en dessous déclenche une alerte.
	LowStockThreshold = 5
	// deadstockWindowDays: fenêtre d'inactivité avant de qualifier un produit de "dormant".
	deadstockWindowDays = 30
	// conversionMinViews: on ignore les produits trop peu vus pour un taux fiable.
	conversionMinViews = 20
	// conversionListLimit: nombre de produits analysés pour la conversion.
	conversionListLimit = 30
)

// StatsService builds the admin dashboard overview.
type StatsService struct {
	store  types.StatsStore
	logger *logrus.Logger
}

// NewStatsService constructs a StatsService
func NewStatsService(store types.StatsStore, logger *logrus.Logger) *StatsService {
	if logger == nil {
		logger = logrus.New()
	}
	return &StatsService{store: store, logger: logger}
}

// Overview assembles the full dashboard payload for the last `days` days.
func (s *StatsService) Overview(ctx context.Context, days int) (*api.StatsOverview, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	now := time.Now()
	since := now.AddDate(0, 0, -days)
	prevSince := since.AddDate(0, 0, -days)

	out := &api.StatsOverview{
		Days:        days,
		GeneratedAt: now,
	}

	// All sections are independent reads → fetch them concurrently.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		k, err := s.store.Kpis(gctx, since, prevSince, since, LowStockThreshold)
		if err != nil {
			return err
		}
		out.Kpis = *k
		return nil
	})
	g.Go(func() error {
		// 7 points (today + 6 previous days) for the KPI sparklines.
		sp, err := s.store.Sparklines(gctx, now.AddDate(0, 0, -6))
		if err != nil {
			return err
		}
		out.Sparklines = *sp
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.OrdersByStatus(gctx, since)
		if err != nil {
			return err
		}
		out.OrdersByStatus = rows
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.RevenueByDay(gctx, since)
		if err != nil {
			return err
		}
		out.RevenueByDay = fillDayGaps(rows, since, now)
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.TopGouvernorats(gctx, since, 10)
		if err != nil {
			return err
		}
		out.TopGouvernorats = rows
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.StockAlerts(gctx, LowStockThreshold, 20)
		if err != nil {
			return err
		}
		out.StockAlerts = rows
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.TopProducts(gctx, since, 10)
		if err != nil {
			return err
		}
		out.TopProducts = rows
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.Conversion(gctx, conversionMinViews, conversionListLimit)
		if err != nil {
			return err
		}
		out.Conversion = annotateConversion(rows)
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.Deadstock(gctx, deadstockWindowDays, 15)
		if err != nil {
			return err
		}
		out.Deadstock = rows
		return nil
	})
	g.Go(func() error {
		rows, err := s.store.PaymentMethods(gctx, since)
		if err != nil {
			return err
		}
		out.PaymentMethods = rows
		return nil
	})

	if err := g.Wait(); err != nil {
		s.logger.WithError(err).Error("failed to build dashboard overview")
		return nil, err
	}

	return out, nil
}

// fillDayGaps returns one point per day in [since, now], inserting zeros where
// no orders were recorded so the timeline renders without holes.
func fillDayGaps(rows []api.DayPoint, since, now time.Time) []api.DayPoint {
	byDay := make(map[string]api.DayPoint, len(rows))
	for _, r := range rows {
		byDay[r.Day] = r
	}

	start := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	out := make([]api.DayPoint, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if p, ok := byDay[key]; ok {
			out = append(out, p)
		} else {
			out = append(out, api.DayPoint{Day: key})
		}
	}
	return out
}

// annotateConversion flags each product with an actionable recommendation:
//   - very visible but barely sold  → review_pricing (prix trop haut / photos)
//   - rarely seen but converts well → feature (à mettre en avant sur la home)
func annotateConversion(rows []api.ProductConversion) []api.ProductConversion {
	if len(rows) == 0 {
		return rows
	}

	var totalViews int64
	for _, r := range rows {
		totalViews += r.ViewCount
	}
	avgViews := float64(totalViews) / float64(len(rows))

	const (
		lowConversion  = 2.0  // %
		highConversion = 10.0 // %
	)

	for i := range rows {
		r := &rows[i]
		switch {
		case float64(r.ViewCount) >= avgViews && r.ConversionRate < lowConversion:
			r.Recommendation = api.RecoReviewPricing
		case r.ConversionRate >= highConversion && float64(r.ViewCount) < avgViews && !r.IsFeatured:
			r.Recommendation = api.RecoFeature
		default:
			r.Recommendation = api.RecoOK
		}
	}
	return rows
}
