package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/v8950/url-shortener/internal/service"
	"github.com/v8950/url-shortener/internal/transport/clientip"
)

// RedirectHandler handles GET /:code requests.
type RedirectHandler struct {
	urlService *service.URLService
}

// NewRedirectHandler creates a new RedirectHandler.
func NewRedirectHandler(urlService *service.URLService) *RedirectHandler {
	return &RedirectHandler{urlService: urlService}
}

// Handle processes the redirect request.
func (h *RedirectHandler) Handle(c *fiber.Ctx) error {
	shortCode := c.Params("code")
	if shortCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "short code is required",
		})
	}

	url, err := h.urlService.ResolveURL(c.Context(), shortCode)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "URL not found",
		})
	}

	// Record click event asynchronously (non-blocking)
	h.urlService.RecordClick(
		c.Context(),
		shortCode,
		clientip.FromRequest(c),
		c.Get("User-Agent"),
		c.Get("Referer"),
	)

	// Redirect with 301 Moved Permanently
	return c.Redirect(url.LongURL, fiber.StatusMovedPermanently)
}
