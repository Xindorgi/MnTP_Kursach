package worker

import (
	"os"
	"path/filepath"
	"testing"
)

// geoIPDatabasePath returns the first existing GeoLite2-City.mmdb path, or "" if none found.
func geoIPDatabasePath() string {
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
	return ""
}

// requireGeoIPDatabase skips the test when GeoLite2-City.mmdb is not available locally.
func requireGeoIPDatabase(t *testing.T) string {
	t.Helper()
	p := geoIPDatabasePath()
	if p == "" {
		t.Skip("GeoLite2-City.mmdb not found — place it in geoip/ or set GEOIP_DB_PATH (see geoip/README.md)")
	}
	return p
}

// newAnalyticsWorkerWithGeoIP opens a worker with a loaded MaxMind database.
func newAnalyticsWorkerWithGeoIP(t *testing.T) *AnalyticsWorker {
	t.Helper()
	dbPath := requireGeoIPDatabase(t)
	w, err := NewAnalyticsWorker(nil, dbPath)
	if err != nil {
		t.Fatalf("NewAnalyticsWorker: %v", err)
	}
	if w.geoIP == nil {
		t.Fatal("expected GeoIP reader to be loaded")
	}
	t.Cleanup(w.Close)
	return w
}
