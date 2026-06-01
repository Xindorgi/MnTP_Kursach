package repository

import (
	"context"

	"github.com/v8950/url-shortener/internal/domain"
)

// URLRepository defines the interface for URL data access.
type URLRepository interface {
	// Insert creates a new URL record and returns it with generated fields.
	Insert(ctx context.Context, longURL string) (*domain.URL, error)

	// FindByShortCode retrieves a URL by its short code.
	FindByShortCode(ctx context.Context, shortCode string) (*domain.URL, error)

	// UpdateShortCode updates the short code for a given URL ID.
	UpdateShortCode(ctx context.Context, id int64, shortCode string) error

	// FindByManagementToken retrieves a URL by its management token.
	FindByManagementToken(ctx context.Context, token string) (*domain.URL, error)
}

// ClickRepository defines the interface for click analytics data access.
type ClickRepository interface {
	// BatchInsert inserts multiple click events in a single batch.
	BatchInsert(ctx context.Context, events []domain.ClickEvent) error

	// GetStats returns aggregated analytics for a given URL ID.
	GetStats(ctx context.Context, urlID int64) (*domain.ClickStats, error)
}

// CacheRepository defines the interface for caching short URL resolutions.
type CacheRepository interface {
	// Get retrieves a long URL from cache by short code.
	Get(ctx context.Context, shortCode string) (string, error)

	// Set stores a long URL in cache with a TTL.
	Set(ctx context.Context, shortCode, longURL string) error

	// Delete removes a cached entry.
	Delete(ctx context.Context, shortCode string) error
}
