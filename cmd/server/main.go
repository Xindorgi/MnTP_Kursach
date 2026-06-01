package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/v8950/url-shortener/internal/config"
	"github.com/v8950/url-shortener/internal/repository"
	"github.com/v8950/url-shortener/internal/repository/postgres"
	"github.com/v8950/url-shortener/internal/repository/redis"
	"github.com/v8950/url-shortener/internal/service"
	"github.com/v8950/url-shortener/internal/transport"
	"github.com/v8950/url-shortener/internal/transport/handlers"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize repositories
	var urlRepo repository.URLRepository
	var cacheRepo repository.CacheRepository

	// Try to connect to PostgreSQL
	pgRepo, err := postgres.NewURLRepository(cfg.PostgresConnString())
	if err != nil {
		log.Printf("WARNING: PostgreSQL not available, using in-memory fallback: %v", err)
		// Fallback to in-memory repository for development
		urlRepo = postgres.NewInMemoryURLRepository()
	} else {
		urlRepo = pgRepo
	}

	// Try to connect to Redis
	redisCache, err := redis.NewCacheRepository(cfg.RedisAddr(), cfg.RedisPassword, 0)
	if err != nil {
		log.Printf("WARNING: Redis not available, using in-memory cache fallback: %v", err)
		cacheRepo = redis.NewInMemoryCacheRepository()
	} else {
		cacheRepo = redisCache
	}

	// Initialize services
	urlSvc, err := service.NewURLService(urlRepo, cacheRepo, cfg.BaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize URL service: %v", err)
	}

	// Initialize handlers
	shortenHandler := handlers.NewShortenHandler(urlSvc)
	redirectHandler := handlers.NewRedirectHandler(urlSvc)

	// Setup routes
	app := transport.SetupRoutes(shortenHandler, redirectHandler)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s", cfg.AppAddr())
		if err := app.Listen(cfg.AppAddr()); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down server...")

	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped gracefully")
}
