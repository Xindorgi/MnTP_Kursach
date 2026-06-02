package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/Xindorgi/MnTP_Kursach/internal/service"
)

// AnalyticsHandler handles GET /api/v1/analytics/:code requests.
type AnalyticsHandler struct {
	urlService *service.URLService
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(urlService *service.URLService) *AnalyticsHandler {
	return &AnalyticsHandler{urlService: urlService}
}

// Handle returns aggregated analytics for a shortened URL.
// Requires a valid management_token query parameter for authorization.
func (h *AnalyticsHandler) Handle(c *fiber.Ctx) error {
	shortCode := c.Params("code")
	if shortCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "short code is required",
		})
	}

	token := c.Query("token")
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "management token is required",
		})
	}

	stats, err := h.urlService.GetAnalytics(c.Context(), shortCode, token)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(stats)
}

