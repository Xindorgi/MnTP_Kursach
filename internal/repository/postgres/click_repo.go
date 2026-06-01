package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/v8950/url-shortener/internal/domain"
)

// ClickRepository implements repository.ClickRepository using PostgreSQL.
type ClickRepository struct {
	pool *pgxpool.Pool
}

// NewClickRepositoryFromPool creates a ClickRepository from an existing pool.
func NewClickRepositoryFromPool(pool *pgxpool.Pool) *ClickRepository {
	return &ClickRepository{pool: pool}
}

// BatchInsert inserts multiple click events in a single batch.
func (r *ClickRepository) BatchInsert(ctx context.Context, events []domain.ClickEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, event := range events {
		batch.Queue(
			`INSERT INTO url_clicks (url_id, ip_address, user_agent, referer, country, city, clicked_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			event.URLID, event.IPAddress, event.UserAgent, event.Referer,
			event.Country, event.City, event.ClickedAt,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range events {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert click event: %w", err)
		}
	}

	return nil
}

// GetStats returns aggregated analytics for a given URL ID.
func (r *ClickRepository) GetStats(ctx context.Context, urlID int64) (*domain.ClickStats, error) {
	stats := &domain.ClickStats{}

	// Total clicks
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM url_clicks WHERE url_id = $1`, urlID,
	).Scan(&stats.TotalClicks)
	if err != nil {
		return nil, fmt.Errorf("failed to get total clicks: %w", err)
	}

	// Daily clicks (last 30 days)
	rows, err := r.pool.Query(ctx,
		`SELECT DATE(clicked_at)::text as date, COUNT(*) as count
		 FROM url_clicks
		 WHERE url_id = $1 AND clicked_at >= NOW() - INTERVAL '30 days'
		 GROUP BY DATE(clicked_at)
		 ORDER BY date DESC`,
		urlID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily clicks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d domain.DailyClickCount
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return nil, fmt.Errorf("failed to scan daily click: %w", err)
		}
		stats.DailyClicks = append(stats.DailyClicks, d)
	}

	// Top countries
	rows, err = r.pool.Query(ctx,
		`SELECT COALESCE(country, 'Unknown') as country, COUNT(*) as count
		 FROM url_clicks
		 WHERE url_id = $1
		 GROUP BY country
		 ORDER BY count DESC
		 LIMIT 10`,
		urlID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get top countries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c domain.CountryCount
		if err := rows.Scan(&c.Country, &c.Count); err != nil {
			return nil, fmt.Errorf("failed to scan country count: %w", err)
		}
		stats.TopCountries = append(stats.TopCountries, c)
	}

	// Top referrers
	rows, err = r.pool.Query(ctx,
		`SELECT COALESCE(referer, 'Direct') as referer, COUNT(*) as count
		 FROM url_clicks
		 WHERE url_id = $1
		 GROUP BY referer
		 ORDER BY count DESC
		 LIMIT 10`,
		urlID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get top referrers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r domain.ReferrerCount
		if err := rows.Scan(&r.Referrer, &r.Count); err != nil {
			return nil, fmt.Errorf("failed to scan referrer count: %w", err)
		}
		stats.TopReferrers = append(stats.TopReferrers, r)
	}

	return stats, nil
}

// In-memory fallback for development

// InMemoryClickRepository is an in-memory implementation for development.
type InMemoryClickRepository struct {
	mu     sync.RWMutex
	clicks []domain.ClickEvent
}

// NewInMemoryClickRepository creates a new in-memory click repository.
func NewInMemoryClickRepository() *InMemoryClickRepository {
	return &InMemoryClickRepository{
		clicks: make([]domain.ClickEvent, 0),
	}
}

func (r *InMemoryClickRepository) BatchInsert(ctx context.Context, events []domain.ClickEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clicks = append(r.clicks, events...)
	return nil
}

func (r *InMemoryClickRepository) GetStats(ctx context.Context, urlID int64) (*domain.ClickStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &domain.ClickStats{}
	countryCount := make(map[string]int64)
	referrerCount := make(map[string]int64)
	dayCount := make(map[string]int64)

	for _, c := range r.clicks {
		if c.URLID != urlID {
			continue
		}
		stats.TotalClicks++

		country := c.Country
		if country == "" {
			country = "Unknown"
		}
		countryCount[country]++

		referrer := c.Referer
		if referrer == "" {
			referrer = "Direct"
		}
		referrerCount[referrer]++

		day := c.ClickedAt.Format("2006-01-02")
		dayCount[day]++
	}

	for country, count := range countryCount {
		stats.TopCountries = append(stats.TopCountries, domain.CountryCount{
			Country: country, Count: count,
		})
	}
	for referrer, count := range referrerCount {
		stats.TopReferrers = append(stats.TopReferrers, domain.ReferrerCount{
			Referrer: referrer, Count: count,
		})
	}
	for date, count := range dayCount {
		stats.DailyClicks = append(stats.DailyClicks, domain.DailyClickCount{
			Date: date, Count: count,
		})
	}

	return stats, nil
}
