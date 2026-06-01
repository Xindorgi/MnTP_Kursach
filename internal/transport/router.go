package transport

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/v8950/url-shortener/internal/transport/handlers"
	"github.com/v8950/url-shortener/internal/transport/middleware"
)

// SetupRoutes configures all application routes and returns the Fiber app.
func SetupRoutes(
	shortenHandler *handlers.ShortenHandler,
	redirectHandler *handlers.RedirectHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	dashboardHandler *handlers.DashboardHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "URL Shortener",
	})

	// Global middlewares
	app.Use(middleware.Logger())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET, POST, OPTIONS",
		AllowHeaders: "Content-Type, Authorization",
	}))

	// Dashboard route (must be before /:code to not be caught by redirect)
	app.Get("/dashboard", dashboardHandler.Handle)

	// API v1 routes
	v1 := app.Group("/api/v1")
	v1.Post("/shorten", shortenHandler.Handle)
	v1.Get("/analytics/:code", analyticsHandler.Handle)

	// Redirect route (must be last to not conflict with API routes)
	app.Get("/:code", redirectHandler.Handle)

	return app
}
