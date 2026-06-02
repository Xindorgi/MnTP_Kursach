package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	cacheTTL = 24 * time.Hour // Cache popular URLs for 24 hours
)

// CacheRepository implements repository.CacheRepository using Redis.
type CacheRepository struct {
	client *goredis.Client
}

// NewCacheRepository creates a new Redis cache repository.
func NewCacheRepository(addr, password string, db int) (*CacheRepository, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &CacheRepository{client: client}, nil
}

// Get retrieves a long URL from cache by short code.
func (r *CacheRepository) Get(ctx context.Context, shortCode string) (string, error) {
	longURL, err := r.client.Get(ctx, shortCode).Result()
	if err != nil {
		return "", fmt.Errorf("cache miss: %w", err)
	}
	return longURL, nil
}

// Set stores a long URL in cache with a TTL.
func (r *CacheRepository) Set(ctx context.Context, shortCode, longURL string) error {
	return r.client.Set(ctx, shortCode, longURL, cacheTTL).Err()
}

// Delete removes a cached entry.
func (r *CacheRepository) Delete(ctx context.Context, shortCode string) error {
	return r.client.Del(ctx, shortCode).Err()
}

// Close closes the Redis connection.
func (r *CacheRepository) Close() error {
	return r.client.Close()
}

// In-memory fallback for development without Redis

// InMemoryCacheRepository is an in-memory implementation for development.
type InMemoryCacheRepository struct {
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	longURL   string
	expiresAt time.Time
}

// NewInMemoryCacheRepository creates a new in-memory cache repository.
func NewInMemoryCacheRepository() *InMemoryCacheRepository {
	return &InMemoryCacheRepository{
		cache: make(map[string]cacheEntry),
	}
}

func (r *InMemoryCacheRepository) Get(ctx context.Context, shortCode string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.cache[shortCode]
	if !ok {
		return "", fmt.Errorf("cache miss")
	}
	if time.Now().After(entry.expiresAt) {
		delete(r.cache, shortCode)
		return "", fmt.Errorf("cache expired")
	}
	return entry.longURL, nil
}

func (r *InMemoryCacheRepository) Set(ctx context.Context, shortCode, longURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache[shortCode] = cacheEntry{
		longURL:   longURL,
		expiresAt: time.Now().Add(cacheTTL),
	}
	return nil
}

func (r *InMemoryCacheRepository) Delete(ctx context.Context, shortCode string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.cache, shortCode)
	return nil
}

