package test_e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Xindorgi/MnTP_Kursach/internal/domain"
	"github.com/Xindorgi/MnTP_Kursach/internal/worker"
)

func requireGeoIPDatabaseE2E(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("GEOIP_DB_PATH"),
		filepath.Join("geoip", "GeoLite2-City.mmdb"),
		filepath.Join("..", "..", "geoip", "GeoLite2-City.mmdb"),
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("GeoLite2-City.mmdb not found — place it in geoip/ or set GEOIP_DB_PATH (see geoip/README.md)")
	return ""
}

// TestE2E_GeoIPCountries verifies the full HTTP path: X-Forwarded-For → GeoIP → analytics API.
func TestE2E_GeoIPCountries(t *testing.T) {
	dbPath := requireGeoIPDatabaseE2E(t)
	app, cancel := setupTestApp(t, withGeoIPDatabase(dbPath))
	defer cancel()

	longURL := "https://example.com/geo-test"
	createResp := createShortURL(t, app, longURL)

	clicks := []struct {
		forwardedFor string
		country      string
	}{
		{forwardedFor: "8.8.8.8", country: "US"},
		{forwardedFor: "178.63.41.15", country: "DE"},
		{forwardedFor: "81.2.69.142", country: "GB"},
		{forwardedFor: "202.12.27.33", country: "JP"},
	}

	for _, click := range clicks {
		req := httptest.NewRequest(http.MethodGet, "/"+createResp.ShortCode, nil)
		req.Header.Set(fiber.HeaderXForwardedFor, click.forwardedFor)
		req.Header.Set("User-Agent", "geo-e2e-test/1.0")

		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
		resp.Body.Close()
	}

	stats := waitForAnalytics(t, app, createResp.ShortCode, createResp.ManagementToken, int64(len(clicks)))
	assert.Equal(t, int64(len(clicks)), stats.TotalClicks)

	for _, click := range clicks {
		assert.Equal(t, int64(1), statsCountryClicks(stats, click.country),
			"expected 1 click from %s (X-Forwarded-For: %s)", click.country, click.forwardedFor)
	}
}

// TestE2E_GeoIPLocalNetwork labels private client IPs as LOCAL when GeoIP is enabled.
func TestE2E_GeoIPLocalNetwork(t *testing.T) {
	dbPath := requireGeoIPDatabaseE2E(t)
	app, cancel := setupTestApp(t, withGeoIPDatabase(dbPath))
	defer cancel()

	createResp := createShortURL(t, app, "https://example.com/local-geo")

	req := httptest.NewRequest(http.MethodGet, "/"+createResp.ShortCode, nil)
	req.Header.Set(fiber.HeaderXForwardedFor, "192.168.1.10")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	resp.Body.Close()

	stats := waitForAnalytics(t, app, createResp.ShortCode, createResp.ManagementToken, 1)
	assert.Equal(t, int64(1), statsCountryClicks(stats, worker.CountryLocal))
}

type shortenCreateResponse struct {
	ShortCode       string `json:"short_code"`
	ManagementToken string `json:"management_token"`
}

func createShortURL(t *testing.T, app *fiber.App, longURL string) shortenCreateResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"url":"`+longURL+`"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var out shortenCreateResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	resp.Body.Close()
	require.NotEmpty(t, out.ShortCode)
	require.NotEmpty(t, out.ManagementToken)
	return out
}

func statsCountryClicks(stats *domain.ClickStats, country string) int64 {
	for _, c := range stats.TopCountries {
		if c.Country == country {
			return c.Count
		}
	}
	return 0
}

