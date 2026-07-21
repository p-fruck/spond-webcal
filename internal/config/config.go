package config

import "os"

const (
	defaultAddr         = "localhost:8080"
	defaultDBURL        = "file:./app.db"
	defaultLogLevel     = "info"
	defaultSpondBaseURL = "https://api.spond.com/core/v1"
)

type Config struct {
	Addr         string
	DBURL        string
	CookieSecret string
	LogLevel     string
	SpondBaseURL string
}

func Load() Config {
	return Config{
		Addr:         getenv("SPOND_WEBCAL_ADDR", defaultAddr),
		DBURL:        getenv("SPOND_WEBCAL_DB_URL", defaultDBURL),
		CookieSecret: os.Getenv("SPOND_WEBCAL_COOKIE_SECRET"),
		LogLevel:     getenv("SPOND_WEBCAL_LOG_LEVEL", defaultLogLevel),
		SpondBaseURL: getenv("SPOND_WEBCAL_SPOND_BASE_URL", defaultSpondBaseURL),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
