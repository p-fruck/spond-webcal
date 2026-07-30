package config

import (
	"os"
	"time"
)

const (
	defaultAddr         = "localhost:8080"
	defaultDBURL        = "file:./app.db"
	defaultLogLevel     = "info"
	defaultSpondBaseURL = "https://api.spond.com/core/v1"
	defaultSyncInterval = 5 * time.Minute
	defaultSyncTimeout  = 20 * time.Second
)

type Config struct {
	Addr         string
	DBURL        string
	CookieSecret string
	LogLevel     string
	SpondBaseURL string
	SyncInterval time.Duration
	SyncTimeout  time.Duration
}

func Load() Config {
	return Config{
		Addr:         getenv("SPOND_WEBCAL_ADDR", defaultAddr),
		DBURL:        getenv("SPOND_WEBCAL_DB_URL", defaultDBURL),
		CookieSecret: os.Getenv("SPOND_WEBCAL_COOKIE_SECRET"),
		LogLevel:     getenv("SPOND_WEBCAL_LOG_LEVEL", defaultLogLevel),
		SpondBaseURL: getenv("SPOND_WEBCAL_SPOND_BASE_URL", defaultSpondBaseURL),
		SyncInterval: getDuration("SPOND_WEBCAL_SYNC_INTERVAL", defaultSyncInterval),
		SyncTimeout:  getDuration("SPOND_WEBCAL_SYNC_TIMEOUT", defaultSyncTimeout),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}

	return duration
}
