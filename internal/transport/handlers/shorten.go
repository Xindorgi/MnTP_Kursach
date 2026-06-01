package handlers

import (
	"net/url"

	"github.com/gofiber/fiber/v2"

	"github.com/v8950/url-shortener/internal/service"
)

// ShortenHandler handles POST /api/v1/shorten requests.
type ShortenHandler struct {
	urlService *service.URLService
}

// NewShortenHandler creates a new ShortenHandler.
func NewShortenHandler(urlService *service.URLService) *ShortenHandler {
	return &ShortenHandler{urlService: urlService}
}

// shortenRequest represents the incoming request body.
type shortenRequest struct {
	URL string `json:"url" xml:"url" form:"url"`
}

// shortenResponse represents the response for a created short URL.
type shortenResponse struct {
	ShortURL        string `json:"short_url"`
	ShortCode       string `json:"short_code"`
	ManagementToken string `json:"management_token"`
}

// Handle processes the shorten request.
func (h *ShortenHandler) Handle(c *fiber.Ctx) error {
	var req shortenRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url is required",
		})
	}

	// Validate URL format
	parsedURL, err := url.ParseRequestURI(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid URL format. Must be a valid http or https URL",
		})
	}

	created, err := h.urlService.CreateShortURL(c.Context(), req.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create short URL",
		})
	}

	baseURL := h.urlService.BaseURL() // We'll need to expose this

	return c.Status(fiber.StatusCreated).JSON(shortenResponse{
		ShortURL:        baseURL + "/" + created.ShortCode,
		ShortCode:       created.ShortCode,
		ManagementToken: created.ManagementToken,
	})
}
