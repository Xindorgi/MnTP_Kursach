package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/v8950/url-shortener/internal/config"
	"github.com/v8950/url-shortener/internal/migrator"
	"github.com/v8950/url-shortener/internal/repository"
	"github.com/v8950/url-shortener/internal/repository/postgres"
	"github.com/v8950/url-shortener/internal/repository/redis"
	"github.com/v8950/url-shortener/internal/service"
	"github.com/v8950/url-shortener/internal/transport"
	"github.com/v8950/url-shortener/internal/transport/handlers"
	"github.com/v8950/url-shortener/internal/worker"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize repositories
	var urlRepo repository.URLRepository
	var clickRepo repository.ClickRepository
	var cacheRepo repository.CacheRepository

	// Try to connect to PostgreSQL
	pgRepo, err := postgres.NewURLRepository(cfg.PostgresConnString())
	if err != nil {
		log.Printf("WARNING: PostgreSQL not available, using in-memory fallback: %v", err)
		// Fallback to in-memory repository for development
		urlRepo = postgres.NewInMemoryURLRepository()
		clickRepo = postgres.NewInMemoryClickRepository()
	} else {
		urlRepo = pgRepo
		clickRepo = postgres.NewClickRepositoryFromPool(pgRepo.Pool())

		// Run database migrations
		if err := migrator.RunUp(context.Background(), pgRepo.Pool(), "migrations"); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	}

	// Try to connect to Redis
	redisCache, err := redis.NewCacheRepository(cfg.RedisAddr(), cfg.RedisPassword, 0)
	if err != nil {
		log.Printf("WARNING: Redis not available, using in-memory cache fallback: %v", err)
		cacheRepo = redis.NewInMemoryCacheRepository()
	} else {
		cacheRepo = redisCache
	}

	// Initialize Analytics Worker
	analyticsWorker, err := worker.NewAnalyticsWorker(clickRepo, cfg.GeoIPDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize analytics worker: %v", err)
	}

	// Initialize services
	urlSvc, err := service.NewURLService(urlRepo, clickRepo, cacheRepo, analyticsWorker.EventsChan(), cfg.BaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize URL service: %v", err)
	}

	// Initialize handlers
	shortenHandler := handlers.NewShortenHandler(urlSvc)
	redirectHandler := handlers.NewRedirectHandler(urlSvc)
	analyticsHandler := handlers.NewAnalyticsHandler(urlSvc)
	dashboardHandler := handlers.NewDashboardHandler()
	indexHandler := handlers.NewIndexHandler()

	// Setup routes
	app := transport.SetupRoutes(shortenHandler, redirectHandler, analyticsHandler, dashboardHandler, indexHandler)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start analytics worker
	go analyticsWorker.Start(ctx)

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

	// Shutdown server first, then close worker
	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	analyticsWorker.Close()
	log.Println("Server stopped gracefully")
}
