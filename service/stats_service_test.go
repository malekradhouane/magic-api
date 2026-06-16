package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/malekradhouane/magic/api"
)

// ---------------------------------------------------------------------------
// fillDayGaps
// ---------------------------------------------------------------------------

func TestFillDayGaps_NoGaps(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	rows := []api.DayPoint{
		{Day: "2026-06-01", Revenue: 100, Orders: 2},
		{Day: "2026-06-02", Revenue: 200, Orders: 3},
		{Day: "2026-06-03", Revenue: 50, Orders: 1},
	}

	got := fillDayGaps(rows, since, now)
	assert.Len(t, got, 3)
	assert.Equal(t, float64(100), got[0].Revenue)
	assert.Equal(t, float64(200), got[1].Revenue)
	assert.Equal(t, float64(50), got[2].Revenue)
}

func TestFillDayGaps_WithGaps(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	rows := []api.DayPoint{
		{Day: "2026-06-01", Revenue: 100, Orders: 2},
		{Day: "2026-06-03", Revenue: 50, Orders: 1},
		{Day: "2026-06-05", Revenue: 75, Orders: 1},
	}

	got := fillDayGaps(rows, since, now)
	assert.Len(t, got, 5)
	// Missing days should have zero values.
	assert.Equal(t, "2026-06-02", got[1].Day)
	assert.Equal(t, float64(0), got[1].Revenue)
	assert.Equal(t, "2026-06-04", got[3].Day)
	assert.Equal(t, float64(0), got[3].Revenue)
}

func TestFillDayGaps_Empty(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)

	got := fillDayGaps(nil, since, now)
	assert.Len(t, got, 3) // 3 zero-value days
	for _, p := range got {
		assert.Equal(t, float64(0), p.Revenue)
	}
}

// ---------------------------------------------------------------------------
// annotateConversion
// ---------------------------------------------------------------------------

func TestAnnotateConversion_Empty(t *testing.T) {
	t.Parallel()

	got := annotateConversion(nil)
	assert.Nil(t, got)
}

func TestAnnotateConversion_ReviewPricing(t *testing.T) {
	t.Parallel()

	// Product with high views but low conversion -> review_pricing.
	rows := []api.ProductConversion{
		{Name: "Popular", ViewCount: 1000, Units: 5, ConversionRate: 0.5, IsFeatured: false},
	}

	got := annotateConversion(rows)
	assert.Equal(t, api.RecoReviewPricing, got[0].Recommendation)
}

func TestAnnotateConversion_Feature(t *testing.T) {
	t.Parallel()

	// Product with low views but high conversion and not featured -> feature.
	rows := []api.ProductConversion{
		{Name: "Hidden", ViewCount: 50, Units: 10, ConversionRate: 20.0, IsFeatured: false},
		{Name: "Normal", ViewCount: 500, Units: 25, ConversionRate: 5.0, IsFeatured: false},
	}

	got := annotateConversion(rows)
	// avg views = (50+500)/2 = 275. "Hidden" has 50 < 275, conversion 20% > 10% -> feature
	assert.Equal(t, api.RecoFeature, got[0].Recommendation)
}

func TestAnnotateConversion_OK(t *testing.T) {
	t.Parallel()

	rows := []api.ProductConversion{
		{Name: "Normal", ViewCount: 100, Units: 5, ConversionRate: 5.0, IsFeatured: false},
	}

	got := annotateConversion(rows)
	assert.Equal(t, api.RecoOK, got[0].Recommendation)
}

func TestAnnotateConversion_FeaturedNotRecommended(t *testing.T) {
	t.Parallel()

	// Already featured, high conversion, low views -> should NOT get "feature" recommendation.
	rows := []api.ProductConversion{
		{Name: "Featured", ViewCount: 10, Units: 5, ConversionRate: 50.0, IsFeatured: true},
		{Name: "Other", ViewCount: 500, Units: 25, ConversionRate: 5.0, IsFeatured: false},
	}

	got := annotateConversion(rows)
	// "Featured" has IsFeatured=true, so should not get "feature" even with good conversion.
	assert.NotEqual(t, api.RecoFeature, got[0].Recommendation)
}
