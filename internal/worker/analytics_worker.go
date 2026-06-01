package worker

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/oschwald/geoip2-golang"

	"github.com/v8950/url-shortener/internal/domain"
	"github.com/v8950/url-shortener/internal/repository"
)

const (
	// ClickEventChanSize is the buffer size for the click events channel.
	ClickEventChanSize = 1000
	// BatchSize is the number of events to accumulate before flushing to DB.
	BatchSize = 50
	// FlushInterval is the maximum time to wait before flushing a partial batch.
	FlushInterval = 1 * time.Second
)

// AnalyticsWorker processes click events asynchronously.
// It reads from a channel, performs GeoIP lookup, and batch-inserts into PostgreSQL.
type AnalyticsWorker struct {
	clickRepo repository.ClickRepository
	geoIP     *geoip2.Reader
	events    chan domain.ClickEvent
}

// NewAnalyticsWorker creates a new AnalyticsWorker.
// geoIPDBPath is the path to the GeoLite2-City.mmdb file. If empty, GeoIP is disabled.
func NewAnalyticsWorker(
	clickRepo repository.ClickRepository,
	geoIPDBPath string,
) (*AnalyticsWorker, error) {
	var geoIP *geoip2.Reader

	if geoIPDBPath != "" {
		var err error
		geoIP, err = geoip2.Open(geoIPDBPath)
		if err != nil {
			log.Printf("WARNING: Failed to open GeoIP database at %s: %v. GeoIP disabled.", geoIPDBPath, err)
		} else {
			log.Printf("GeoIP database loaded from %s", geoIPDBPath)
		}
	}

	return &AnalyticsWorker{
		clickRepo: clickRepo,
		geoIP:     geoIP,
		events:    make(chan domain.ClickEvent, ClickEventChanSize),
	}, nil
}

// EventsChan returns the channel for pushing click events.
func (w *AnalyticsWorker) EventsChan() chan<- domain.ClickEvent {
	return w.events
}

// Start begins the worker loop. Should be run as a goroutine.
func (w *AnalyticsWorker) Start(ctx context.Context) {
	log.Println("Analytics worker started")

	batch := make([]domain.ClickEvent, 0, BatchSize)
	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining events before shutting down
			if len(batch) > 0 {
				w.flush(batch)
			}
			log.Println("Analytics worker stopped")
			return

		case event := <-w.events:
			// Perform GeoIP lookup
			if w.geoIP != nil {
				w.enrichWithGeoIP(&event)
			}

			batch = append(batch, event)

			if len(batch) >= BatchSize {
				w.flush(batch)
				batch = make([]domain.ClickEvent, 0, BatchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = make([]domain.ClickEvent, 0, BatchSize)
			}
		}
	}
}

// enrichWithGeoIP looks up the IP address and fills in country and city.
func (w *AnalyticsWorker) enrichWithGeoIP(event *domain.ClickEvent) {
	if event.IPAddress == "" {
		return
	}

	ip := net.ParseIP(event.IPAddress)
	if ip == nil {
		return
	}

	// Skip private IPs
	if isPrivateIP(ip) {
		return
	}

	record, err := w.geoIP.City(ip)
	if err != nil {
		return
	}

	if record.Country.IsoCode != "" {
		event.Country = record.Country.IsoCode
	}
	if record.City.Names != nil {
		if name, ok := record.City.Names["en"]; ok {
			event.City = name
		}
	}
}

// flush writes a batch of click events to the database.
func (w *AnalyticsWorker) flush(batch []domain.ClickEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.clickRepo.BatchInsert(ctx, batch); err != nil {
		log.Printf("ERROR: Failed to batch insert %d click events: %v", len(batch), err)
		return
	}
	log.Printf("Flushed %d click events to database", len(batch))
}

// isPrivateIP checks if an IP is private (RFC 1918, RFC 6598, loopback, etc.).
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}
	return false
}

// Close closes the GeoIP database reader.
func (w *AnalyticsWorker) Close() {
	if w.geoIP != nil {
		w.geoIP.Close()
	}
}
