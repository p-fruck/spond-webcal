package config

import "os"

const (
	defaultAddr     = ":8080"
	defaultDBURL    = "file:./app.db"
	defaultLogLevel = "debug"
)

type Config struct {
	Addr         string
	DBURL        string
	CookieSecret string
	LogLevel     string
}

func Load() Config {
	return Config{
		Addr:         getenv("SPOND_WEBCAL_ADDR", defaultAddr),
		DBURL:        getenv("SPOND_WEBCAL_DB_URL", defaultDBURL),
		CookieSecret: os.Getenv("SPOND_WEBCAL_COOKIE_SECRET"),
		LogLevel:     getenv("SPOND_WEBCAL_LOG_LEVEL", defaultLogLevel),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}