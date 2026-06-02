package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	AppPort int
	AppEnv  string
	BaseURL string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	RedisHost     string
	RedisPort     int
	RedisPassword string

	GeoIPDBPath string
}

// PostgresDSN returns the PostgreSQL connection string.
func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.PostgresUser, c.PostgresPassword,
		c.PostgresHost, c.PostgresPort,
		c.PostgresDB, c.PostgresSSLMode,
	)
}

// PostgresConnString returns a connection string for pgx pool.
func (c *Config) PostgresConnString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.PostgresHost, c.PostgresPort,
		c.PostgresUser, c.PostgresPassword,
		c.PostgresDB, c.PostgresSSLMode,
	)
}

// RedisAddr returns the Redis address.
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.RedisHost, c.RedisPort)
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	return &Config{
		AppPort: getEnvInt("APP_PORT", 8080),
		AppEnv:  getEnv("APP_ENV", "development"),
		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnvInt("POSTGRES_PORT", 5432),
		PostgresUser:     getEnv("POSTGRES_USER", "urlshortener"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "urlshortener"),
		PostgresDB:       getEnv("POSTGRES_DB", "urlshortener"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("REDIS_PORT", 6379),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		GeoIPDBPath: getEnv("GEOIP_DB_PATH", "./geoip/GeoLite2-City.mmdb"),
	}
}

// AppAddr returns the address the server should listen on.
func (c *Config) AppAddr() string {
	return fmt.Sprintf(":%d", c.AppPort)
}

// ReadTimeout for the HTTP server.
func (c *Config) ReadTimeout() time.Duration {
	return 10 * time.Second
}

// WriteTimeout for the HTTP server.
func (c *Config) WriteTimeout() time.Duration {
	return 10 * time.Second
}

// IdleTimeout for the HTTP server.
func (c *Config) IdleTimeout() time.Duration {
	return 60 * time.Second
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
