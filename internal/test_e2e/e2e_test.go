// Package test_e2e contains end-to-end integration tests for the URL shortener.
// These tests use in-memory repositories and do not require external dependencies.
package test_e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/v8950/url-shortener/internal/domain"
	"github.com/v8950/url-shortener/internal/repository/postgres"
	"github.com/v8950/url-shortener/internal/repository/redis"
	"github.com/v8950/url-shortener/internal/service"
	"github.com/v8950/url-shortener/internal/transport"
	"github.com/v8950/url-shortener/internal/transport/handlers"
	"github.com/v8950/url-shortener/internal/worker"
)

// setupTestApp creates a fully wired Fiber app with in-memory repositories for testing.
func setupTestApp(t *testing.T) (*fiber.App, context.CancelFunc) {
	t.Helper()

	// Initialize in-memory repositories (no Docker needed)
	urlRepo := postgres.NewInMemoryURLRepository()
	clickRepo := postgres.NewInMemoryClickRepository()
	cacheRepo := redis.NewInMemoryCacheRepository()

	// Initialize analytics worker with in-memory click repo
	analyticsWorker, err := worker.NewAnalyticsWorker(clickRepo, "")
	require.NoError(t, err)

	// Start the analytics worker in background
	ctx, cancel := context.WithCancel(context.Background())
	go analyticsWorker.Start(ctx)

	// Initialize URL service
	urlSvc, err := service.NewURLService(urlRepo, clickRepo, cacheRepo, analyticsWorker.EventsChan(), "http://localhost:8080")
	require.NoError(t, err)

	// Initialize handlers
	shortenHandler := handlers.NewShortenHandler(urlSvc)
	redirectHandler := handlers.NewRedirectHandler(urlSvc)
	analyticsHandler := handlers.NewAnalyticsHandler(urlSvc)
	dashboardHandler := handlers.NewDashboardHandler()
	indexHandler := handlers.NewIndexHandler()

	// Setup Fiber app
	app := transport.SetupRoutes(shortenHandler, redirectHandler, analyticsHandler, dashboardHandler, indexHandler)

	return app, cancel
}

// waitForAnalytics polls the analytics endpoint until expected clicks are recorded or timeout.
func waitForAnalytics(t *testing.T, app *fiber.App, shortCode, token string, expectedClicks int64) *domain.ClickStats {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/analytics/"+shortCode+"?token="+token,
			nil,
		)

		resp, err := app.Test(req, 2000)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK {
			var stats domain.ClickStats
			err = json.NewDecoder(resp.Body).Decode(&stats)
			resp.Body.Close()
			require.NoError(t, err)

			if stats.TotalClicks >= expectedClicks {
				return &stats
			}
		} else {
			resp.Body.Close()
		}

		time.Sleep(200 * time.Millisecond)
	}

	require.FailNowf(t, "timeout", "expected %d clicks but never reached", expectedClicks)
	return nil
}

// TestE2EShortenAndRedirect tests the complete flow:
// Create short URL → Redirect → Analytics verification.
func TestE2EShortenAndRedirect(t *testing.T) {
	app, cancel := setupTestApp(t)
	defer cancel()

	t.Run("Full E2E: Create → Redirect → Analytics", func(t *testing.T) {
		// Step 1: Create a short URL
		longURL := "https://example.com/very/long/url/for/testing"
		reqBody := `{"url":"` + longURL + `"}`

		req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Parse response
		var createResp struct {
			ShortURL        string `json:"short_url"`
			ShortCode       string `json:"short_code"`
			ManagementToken string `json:"management_token"`
		}
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEmpty(t, createResp.ShortCode)
		assert.NotEmpty(t, createResp.ShortURL)
		assert.NotEmpty(t, createResp.ManagementToken)
		t.Logf("Created short code: %s, token: %s", createResp.ShortCode, createResp.ManagementToken)

		// Step 2: Follow the redirect
		req = httptest.NewRequest(http.MethodGet, "/"+createResp.ShortCode, nil)
		req.Header.Set("User-Agent", "test-agent/1.0")
		req.Header.Set("Referer", "https://example.com")

		resp, err = app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
		assert.Equal(t, longURL, resp.Header.Get("Location"))
		resp.Body.Close()

		// Step 3: Follow redirect again (to test cache hit)
		req = httptest.NewRequest(http.MethodGet, "/"+createResp.ShortCode, nil)
		req.Header.Set("User-Agent", "test-agent/2.0")
		req.Header.Set("Referer", "https://other.com")

		resp, err = app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
		assert.Equal(t, longURL, resp.Header.Get("Location"))
		resp.Body.Close()

		// Step 4: Wait for analytics worker to process events (with polling)
		stats := waitForAnalytics(t, app, createResp.ShortCode, createResp.ManagementToken, 2)

		// Verify analytics data
		assert.Equal(t, int64(2), stats.TotalClicks, "Should have 2 recorded clicks")
		t.Logf("Analytics: total_clicks=%d", stats.TotalClicks)
	})

	t.Run("Create with invalid URL returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"url":"not-a-url"}`))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Redirect to non-existent code returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)

		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp.Body.Close()
	})

	t.Run("Analytics with invalid token returns 403", func(t *testing.T) {
		// First create a URL
		req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"url":"https://example.com"}`))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var createResp struct {
			ShortCode string `json:"short_code"`
		}
		err = json.NewDecoder(resp.Body).Decode(&createResp)
		require.NoError(t, err)
		resp.Body.Close()

		// Try to get analytics with wrong token
		req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/"+createResp.ShortCode+"?token=wrong-token", nil)

		resp, err = app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		resp.Body.Close()
	})
}

// TestDashboardPage verifies the dashboard HTML is served and includes the fixed loader.
func TestDashboardPage(t *testing.T) {
	app, cancel := setupTestApp(t)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(bodyBytes)

	assert.Contains(t, body, "Analytics Dashboard")
	assert.Contains(t, body, "async function loadAnalytics()")
	assert.NotContains(t, body, `dispatchEvent(new Event('submit'))`)
}
