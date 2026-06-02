package clientip

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// FromRequest returns the best-effort client IP for analytics.
// Fiber may return an empty string when trusted-proxy checks fail even though
// the TCP remote address is available (common with Docker Desktop on Windows).
func FromRequest(c *fiber.Ctx) string {
	if ip := strings.TrimSpace(c.IP()); ip != "" {
		return ip
	}

	addr := c.Context().RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return strings.TrimSpace(addr)
	}
	return host
}
