package service

import (
	"context"
	"fmt"

	"github.com/sqids/sqids-go"

	"github.com/v8950/url-shortener/internal/domain"
	"github.com/v8950/url-shortener/internal/repository"
)

// URLService handles business logic for URL shortening.
type URLService struct {
	urlRepo   repository.URLRepository
	cacheRepo repository.CacheRepository
	sqids     *sqids.Sqids
	baseURL   string
}

// NewURLService creates a new URLService with dependency injection.
func NewURLService(
	urlRepo repository.URLRepository,
	cacheRepo repository.CacheRepository,
	baseURL string,
) (*URLService, error) {
	s, err := sqids.New(sqids.Options{
		MinLength: 6,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize sqids: %w", err)
	}

	return &URLService{
		urlRepo:   urlRepo,
		cacheRepo: cacheRepo,
		sqids:     s,
		baseURL:   baseURL,
	}, nil
}

// CreateShortURL creates a new shortened URL and returns it.
// Flow: Insert long URL into DB → get serial ID → sqids.Encode(id) → update short_code → cache it.
func (s *URLService) CreateShortURL(ctx context.Context, longURL string) (*domain.URL, error) {
	// 1. Insert the long URL, get back the record with ID and management token
	url, err := s.urlRepo.Insert(ctx, longURL)
	if err != nil {
		return nil, fmt.Errorf("failed to insert URL: %w", err)
	}

	// 2. Generate short code from the serial ID
	shortCode, err := s.sqids.Encode([]uint64{uint64(url.ID)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode ID: %w", err)
	}

	// 3. Update the record with the short code
	if err := s.urlRepo.UpdateShortCode(ctx, url.ID, shortCode); err != nil {
		return nil, fmt.Errorf("failed to update short code: %w", err)
	}
	url.ShortCode = shortCode

	// 4. Cache the result
	if err := s.cacheRepo.Set(ctx, shortCode, longURL); err != nil {
		// Non-fatal: cache failure should not break the request
		// Log would go here in production
	}

	return url, nil
}

// ResolveURL resolves a short code to its original long URL.
// Checks cache first, then falls back to database.
func (s *URLService) ResolveURL(ctx context.Context, shortCode string) (*domain.URL, error) {
	// 1. Try cache first
	longURL, err := s.cacheRepo.Get(ctx, shortCode)
	if err == nil && longURL != "" {
		// Cache hit — return minimal URL object
		return &domain.URL{
			ShortCode: shortCode,
			LongURL:   longURL,
		}, nil
	}

	// 2. Cache miss — query database
	url, err := s.urlRepo.FindByShortCode(ctx, shortCode)
	if err != nil {
		return nil, fmt.Errorf("URL not found: %w", err)
	}

	// 3. Populate cache asynchronously (best-effort)
	if cacheErr := s.cacheRepo.Set(ctx, shortCode, url.LongURL); cacheErr != nil {
		// Non-fatal
	}

	return url, nil
}

// GetAnalytics returns aggregated analytics for a given short code.
// Validates ownership via management token.
func (s *URLService) GetAnalytics(ctx context.Context, shortCode, token string) (*domain.ClickStats, error) {
	url, err := s.urlRepo.FindByShortCode(ctx, shortCode)
	if err != nil {
		return nil, fmt.Errorf("URL not found: %w", err)
	}

	if url.ManagementToken != token {
		return nil, fmt.Errorf("invalid management token")
	}

	// Analytics will be fetched via ClickRepository (implemented later)
	// For now return empty stats
	return &domain.ClickStats{}, nil
}

// GetURLByShortCode retrieves a URL by its short code (for internal use).
func (s *URLService) GetURLByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	return s.urlRepo.FindByShortCode(ctx, shortCode)
}
