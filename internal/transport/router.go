package transport

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/v8950/url-shortener/internal/transport/handlers"
	"github.com/v8950/url-shortener/internal/transport/middleware"
)

func productionFiberConfig() fiber.Config {
	return fiber.Config{
		AppName: "URL Shortener",
		// Trust X-Forwarded-For from Docker bridge / reverse proxies so GeoIP sees the real client IP.
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies: []string{
			"127.0.0.1/8",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"::1/128",
		},
	}
}

func testFiberConfig() fiber.Config {
	cfg := productionFiberConfig()
	// httptest has no trusted reverse proxy; accept X-Forwarded-For as-is.
	cfg.EnableTrustedProxyCheck = false
	return cfg
}

func mountRoutes(
	app *fiber.App,
	shortenHandler *handlers.ShortenHandler,
	redirectHandler *handlers.RedirectHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	dashboardHandler *handlers.DashboardHandler,
	indexHandler *handlers.IndexHandler,
) {
	app.Use(middleware.Logger())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, OPTIONS",
		AllowHeaders: "Content-Type, Authorization",
	}))

	app.Get("/", indexHandler.Handle)
	app.Get("/dashboard", dashboardHandler.Handle)

	v1 := app.Group("/api/v1")
	v1.Post("/shorten", shortenHandler.Handle)
	v1.Get("/analytics/:code", analyticsHandler.Handle)

	app.Get("/:code", redirectHandler.Handle)
}

// SetupRoutes configures all application routes and returns the Fiber app.
func SetupRoutes(
	shortenHandler *handlers.ShortenHandler,
	redirectHandler *handlers.RedirectHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	dashboardHandler *handlers.DashboardHandler,
	indexHandler *handlers.IndexHandler,
) *fiber.App {
	app := fiber.New(productionFiberConfig())
	mountRoutes(app, shortenHandler, redirectHandler, analyticsHandler, dashboardHandler, indexHandler)
	return app
}

// SetupRoutesForTest is like SetupRoutes but trusts X-Forwarded-For in httptest (no reverse proxy).
func SetupRoutesForTest(
	shortenHandler *handlers.ShortenHandler,
	redirectHandler *handlers.RedirectHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	dashboardHandler *handlers.DashboardHandler,
	indexHandler *handlers.IndexHandler,
) *fiber.App {
	app := fiber.New(testFiberConfig())
	mountRoutes(app, shortenHandler, redirectHandler, analyticsHandler, dashboardHandler, indexHandler)
	return app
}
