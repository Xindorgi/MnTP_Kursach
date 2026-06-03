package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/Xindorgi/MnTP_Kursach/internal/service"
	"github.com/Xindorgi/MnTP_Kursach/internal/transport/clientip"
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

	// Clone strings from Fiber/FastHTTP before sending to channel.
	// FastHTTP uses zero-copy string interning — strings point to internal buffers
	// that get reused on the next request. Without cloning, the async RecordClick
	// may read corrupted data when the worker processes the event later.
	userAgent := strings.Clone(c.Get("User-Agent"))
	referer := strings.Clone(c.Get("Referer"))
	clientIP := strings.Clone(clientip.FromRequest(c))

	// Record click event asynchronously (non-blocking)
	h.urlService.RecordClick(
		c.Context(),
		shortCode,
		clientIP,
		userAgent,
		referer,
	)

	// Redirect with 301 Moved Permanently
	return c.Redirect(url.LongURL, fiber.StatusMovedPermanently)
}
