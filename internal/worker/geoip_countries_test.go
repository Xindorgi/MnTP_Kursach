package worker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/v8950/url-shortener/internal/domain"
)

// publicCountryCases uses well-known public IPs. ISO codes depend on the installed GeoLite2 build;
// update expectations only if MaxMind changes allocation for these addresses.
var publicCountryCases = []struct {
	name    string
	ip      string
	country string
}{
	{name: "United States (Google DNS)", ip: "8.8.8.8", country: "US"},
	{name: "Germany", ip: "178.63.41.15", country: "DE"},
	{name: "United Kingdom", ip: "81.2.69.142", country: "GB"},
	{name: "Japan", ip: "202.12.27.33", country: "JP"},
	{name: "France", ip: "90.127.167.255", country: "FR"},
}

func TestEnrichWithGeoIP_PublicCountries(t *testing.T) {
	w := newAnalyticsWorkerWithGeoIP(t)

	for _, tt := range publicCountryCases {
		t.Run(tt.name, func(t *testing.T) {
			event := domain.ClickEvent{IPAddress: tt.ip}
			w.enrichWithGeoIP(&event)
			assert.Equal(t, tt.country, event.Country, "unexpected country for %s", tt.ip)
		})
	}
}

func TestEnrichWithGeoIP_PublicIPWithoutDatabase(t *testing.T) {
	t.Parallel()

	w := &AnalyticsWorker{}
	event := domain.ClickEvent{IPAddress: "8.8.8.8"}
	w.enrichWithGeoIP(&event)
	assert.Empty(t, event.Country)
}

func TestEnrichWithGeoIP_InvalidIPUnchanged(t *testing.T) {
	t.Parallel()

	w := &AnalyticsWorker{}
	event := domain.ClickEvent{IPAddress: "not-an-ip"}
	w.enrichWithGeoIP(&event)
	assert.Empty(t, event.Country)
}

func TestNewAnalyticsWorker_MissingDatabaseStillCreatesWorker(t *testing.T) {
	t.Parallel()

	w, err := NewAnalyticsWorker(nil, filepath.Join("geoip", "nonexistent.mmdb"))
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Nil(t, w.geoIP)
}
