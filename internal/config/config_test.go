package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("SPOND_WEBCAL_ADDR", "")
	t.Setenv("SPOND_WEBCAL_DB_URL", "")
	t.Setenv("SPOND_WEBCAL_COOKIE_SECRET", "")
	t.Setenv("SPOND_WEBCAL_LOG_LEVEL", "")

	config := Load()

	if config.Addr != defaultAddr {
		t.Fatalf("expected default addr %q, got %q", defaultAddr, config.Addr)
	}

	if config.DBURL != defaultDBURL {
		t.Fatalf("expected default db url %q, got %q", defaultDBURL, config.DBURL)
	}

	if config.CookieSecret != "" {
		t.Fatalf("expected empty cookie secret, got %q", config.CookieSecret)
	}

	if config.LogLevel != defaultLogLevel {
		t.Fatalf("expected default log level %q, got %q", defaultLogLevel, config.LogLevel)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("SPOND_WEBCAL_ADDR", ":9090")
	t.Setenv("SPOND_WEBCAL_DB_URL", "postgres://example")
	t.Setenv("SPOND_WEBCAL_COOKIE_SECRET", "secret")
	t.Setenv("SPOND_WEBCAL_LOG_LEVEL", "info")

	config := Load()

	if config.Addr != ":9090" {
		t.Fatalf("expected overridden addr, got %q", config.Addr)
	}

	if config.DBURL != "postgres://example" {
		t.Fatalf("expected overridden db url, got %q", config.DBURL)
	}

	if config.CookieSecret != "secret" {
		t.Fatalf("expected overridden cookie secret, got %q", config.CookieSecret)
	}

	if config.LogLevel != "info" {
		t.Fatalf("expected overridden log level, got %q", config.LogLevel)
	}
}
