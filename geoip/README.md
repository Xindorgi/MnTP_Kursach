# GeoLite2 database

Download **GeoLite2-City.mmdb** from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) and place it in this directory:

```
geoip/GeoLite2-City.mmdb
```

The file is gitignored (license/size). Docker Compose mounts this folder into the app container at `/app/geoip`.

## Verify GeoIP is loaded

After restart, check app logs:

```bash
docker compose -p url-shortener logs app | findstr GeoIP
```

Expected: `GeoIP database loaded from /app/geoip/GeoLite2-City.mmdb`

If you upgraded from an older schema, apply the latest migration (needed for `LOCAL` country label):

```bash
docker exec -i url-shortener-db psql -U urlshortener -d urlshortener < migrations/000003_expand_country_column.up.sql
```

If you see `Failed to open GeoIP database`, the `.mmdb` file is missing or the path is wrong.

## Test with a public IP (local dev)

Browser clicks on `localhost` appear as **Local network** — Docker sees a private bridge IP, not your public address. Older clicks recorded before this fix may still show **Unknown** in the dashboard.

Simulate a real client IP:

```bash
curl -L -H "X-Forwarded-For: 8.8.8.8" "http://localhost:8080/YOUR_CODE"
```

Then refresh the dashboard — country should show **United States** (for 8.8.8.8).
