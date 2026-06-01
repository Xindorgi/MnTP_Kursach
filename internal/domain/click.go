package domain

import "time"

// ClickEvent represents a single click/redirect event for analytics.
type ClickEvent struct {
	URLID     int64     `json:"url_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Country   string    `json:"country"`
	City      string    `json:"city"`
	ClickedAt time.Time `json:"clicked_at"`
}

// ClickStats holds aggregated analytics data for a shortened URL.
type ClickStats struct {
	TotalClicks  int64             `json:"total_clicks"`
	DailyClicks  []DailyClickCount `json:"daily_clicks"`
	TopCountries []CountryCount    `json:"top_countries"`
	TopReferrers []ReferrerCount   `json:"top_referrers"`
}

// DailyClickCount represents click count for a specific day.
type DailyClickCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// CountryCount represents click count from a specific country.
type CountryCount struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

// ReferrerCount represents click count from a specific referrer.
type ReferrerCount struct {
	Referrer string `json:"referrer"`
	Count    int64  `json:"count"`
}
