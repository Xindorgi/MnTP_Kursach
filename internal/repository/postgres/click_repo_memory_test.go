package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Xindorgi/MnTP_Kursach/internal/domain"
)

func TestInMemoryClickRepository_GetStats_AggregatesCountries(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryClickRepository()
	ctx := context.Background()
	now := time.Now()

	events := []domain.ClickEvent{
		{URLID: 1, Country: "US", ClickedAt: now},
		{URLID: 1, Country: "US", ClickedAt: now},
		{URLID: 1, Country: "DE", ClickedAt: now},
		{URLID: 1, Country: "LOCAL", ClickedAt: now},
		{URLID: 1, Country: "", ClickedAt: now}, // empty → Unknown
		{URLID: 2, Country: "JP", ClickedAt: now},
	}
	require.NoError(t, repo.BatchInsert(ctx, events))

	stats, err := repo.GetStats(ctx, 1)
	require.NoError(t, err)

	assert.Equal(t, int64(5), stats.TotalClicks)
	assert.Equal(t, int64(2), countryClicks(stats, "US"))
	assert.Equal(t, int64(1), countryClicks(stats, "DE"))
	assert.Equal(t, int64(1), countryClicks(stats, "LOCAL"))
	assert.Equal(t, int64(1), countryClicks(stats, "Unknown"))
}

func countryClicks(stats *domain.ClickStats, country string) int64 {
	for _, c := range stats.TopCountries {
		if c.Country == country {
			return c.Count
		}
	}
	return 0
}

