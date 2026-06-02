package worker

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Xindorgi/MnTP_Kursach/internal/domain"
)

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ip    string
		local bool
	}{
		{name: "loopback ipv4", ip: "127.0.0.1", local: true},
		{name: "loopback ipv6", ip: "::1", local: true},
		{name: "docker bridge", ip: "172.18.0.1", local: true},
		{name: "lan", ip: "192.168.1.10", local: true},
		{name: "public google dns", ip: "8.8.8.8", local: false},
		{name: "public cloudflare", ip: "1.1.1.1", local: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.local, isPrivateIP(net.ParseIP(tt.ip)))
		})
	}
}

func TestEnrichWithGeoIP_PrivateIPMarkedLocal(t *testing.T) {
	t.Parallel()

	worker := &AnalyticsWorker{}
	event := domain.ClickEvent{IPAddress: "172.18.0.1"}

	worker.enrichWithGeoIP(&event)

	assert.Equal(t, CountryLocal, event.Country)
}

func TestEnrichWithGeoIP_NoDatabaseLeavesPublicIPUnset(t *testing.T) {
	t.Parallel()

	worker := &AnalyticsWorker{}
	event := domain.ClickEvent{IPAddress: "8.8.8.8"}

	worker.enrichWithGeoIP(&event)

	assert.Empty(t, event.Country)
}

func TestEnrichWithGeoIP_EmptyIPMarkedLocal(t *testing.T) {
	t.Parallel()

	worker := &AnalyticsWorker{}
	event := domain.ClickEvent{}

	worker.enrichWithGeoIP(&event)

	assert.Equal(t, CountryLocal, event.Country)
}
