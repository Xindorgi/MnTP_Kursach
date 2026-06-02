.PHONY: all build test test-unit test-e2e test-geoip test-cover bench lint sec vulncheck clean

# Build the binary
build:
	go build -ldflags="-s -w" -o url-shortener ./cmd/server

# Run all tests (unit + e2e)
test:
	go test -count=1 -timeout=60s ./...

# Run unit tests only (service + worker + transport + repository)
test-unit:
	go test -count=1 -timeout=30s ./internal/service/... ./internal/worker/... ./internal/transport/... ./internal/repository/...

# Run e2e tests only
test-e2e:
	go test -count=1 -timeout=30s -v ./internal/test_e2e/...

# GeoIP tests (skip automatically if geoip/GeoLite2-City.mmdb is missing)
test-geoip:
	go test -count=1 -timeout=30s -v ./internal/worker/... -run 'GeoIP|PublicCountries'
	go test -count=1 -timeout=30s -v ./internal/test_e2e/... -run GeoIP

# Coverage report for tested packages
test-cover:
	go test -count=1 -cover ./internal/service/... ./internal/worker/... ./internal/transport/... ./internal/repository/... ./internal/test_e2e/...

# Run benchmarks
bench:
	go test -bench=. -benchmem -count=1 -timeout=30s ./internal/service/...

# Run golangci-lint (requires golangci-lint to be installed)
lint:
	golangci-lint run --timeout=5m ./...

# Run gosec security scanner (requires gosec to be installed)
sec:
	gosec -no-fail -fmt sarif -out gosec-results.sarif ./...

# Run govulncheck (requires govulncheck to be installed)
vulncheck:
	govulncheck ./...

# Run all quality checks
qa: lint sec vulncheck test bench

# Clean build artifacts
clean:
	rm -f url-shortener
	rm -f url-shortener.exe
	rm -f gosec-results.sarif