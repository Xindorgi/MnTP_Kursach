package clientip

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestFromRequest_UsesFiberIPWhenPresent(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		ProxyHeader: fiber.HeaderXForwardedFor,
	})
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = FromRequest(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set(fiber.HeaderXForwardedFor, "203.0.113.1")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "203.0.113.1", got)
}

func TestFromRequest_FallsBackToRemoteAddrWhenFiberIPMissing(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"10.0.0.0/8"},
	})
	var got string
	app.Get("/", func(c *fiber.Ctx) error {
		got = FromRequest(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	// Untrusted client with no X-Forwarded-For → c.IP() is empty, RemoteAddr is 0.0.0.0 in httptest.
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, got)
}
