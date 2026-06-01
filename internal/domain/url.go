package domain

import (
	"time"
)

// URL represents a shortened URL entity.
type URL struct {
	ID              int64     `json:"id"`
	LongURL         string    `json:"long_url"`
	ShortCode       string    `json:"short_code"`
	ManagementToken string    `json:"management_token"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
