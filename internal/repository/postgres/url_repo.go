package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/v8950/url-shortener/internal/domain"
)

// URLRepository implements repository.URLRepository using PostgreSQL.
type URLRepository struct {
	pool *pgxpool.Pool
}

// NewURLRepository creates a new PostgreSQL URL repository.
func NewURLRepository(connString string) (*URLRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &URLRepository{pool: pool}, nil
}

// Pool returns the underlying connection pool.
func (r *URLRepository) Pool() *pgxpool.Pool {
	return r.pool
}

// Insert creates a new URL record.
func (r *URLRepository) Insert(ctx context.Context, longURL string) (*domain.URL, error) {
	url := &domain.URL{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO urls (long_url) VALUES ($1)
		 RETURNING id, long_url, short_code, management_token::text, created_at, updated_at`,
		longURL,
	).Scan(&url.ID, &url.LongURL, &url.ShortCode, &url.ManagementToken, &url.CreatedAt, &url.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert URL: %w", err)
	}
	return url, nil
}

// FindByShortCode retrieves a URL by its short code.
func (r *URLRepository) FindByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	url := &domain.URL{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, long_url, short_code, management_token::text, created_at, updated_at
		 FROM urls WHERE short_code = $1`,
		shortCode,
	).Scan(&url.ID, &url.LongURL, &url.ShortCode, &url.ManagementToken, &url.CreatedAt, &url.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("URL not found: %w", err)
	}
	return url, nil
}

// UpdateShortCode updates the short code for a URL.
func (r *URLRepository) UpdateShortCode(ctx context.Context, id int64, shortCode string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE urls SET short_code = $1, updated_at = NOW() WHERE id = $2`,
		shortCode, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update short code: %w", err)
	}
	return nil
}

// FindByManagementToken retrieves a URL by its management token.
func (r *URLRepository) FindByManagementToken(ctx context.Context, token string) (*domain.URL, error) {
	url := &domain.URL{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, long_url, short_code, management_token::text, created_at, updated_at
		 FROM urls WHERE management_token = $1`,
		token,
	).Scan(&url.ID, &url.LongURL, &url.ShortCode, &url.ManagementToken, &url.CreatedAt, &url.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("URL not found: %w", err)
	}
	return url, nil
}

// Close closes the connection pool.
func (r *URLRepository) Close() {
	r.pool.Close()
}

// In-memory fallback for development without PostgreSQL

// InMemoryURLRepository is an in-memory implementation for development.
type InMemoryURLRepository struct {
	mu     sync.RWMutex
	urls   []*domain.URL
	nextID int64
}

// NewInMemoryURLRepository creates a new in-memory URL repository.
func NewInMemoryURLRepository() *InMemoryURLRepository {
	return &InMemoryURLRepository{
		urls:   make([]*domain.URL, 0),
		nextID: 1,
	}
}

func (r *InMemoryURLRepository) Insert(ctx context.Context, longURL string) (*domain.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	url := &domain.URL{
		ID:              r.nextID,
		LongURL:         longURL,
		ManagementToken: fmt.Sprintf("token-%d", r.nextID),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	r.nextID++
	r.urls = append(r.urls, url)
	return url, nil
}

func (r *InMemoryURLRepository) FindByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.urls {
		if u.ShortCode != nil && *u.ShortCode == shortCode {
			return u, nil
		}
	}
	return nil, fmt.Errorf("URL not found")
}

func (r *InMemoryURLRepository) UpdateShortCode(ctx context.Context, id int64, shortCode string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.urls {
		if u.ID == id {
			u.ShortCode = &shortCode
			u.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("URL not found")
}

func (r *InMemoryURLRepository) FindByManagementToken(ctx context.Context, token string) (*domain.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.urls {
		if u.ManagementToken == token {
			return u, nil
		}
	}
	return nil, fmt.Errorf("URL not found")
}
