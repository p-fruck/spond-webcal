package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("SPOND_WEBCAL_ADDR", "")
	t.Setenv("SPOND_WEBCAL_DB_URL", "")
	t.Setenv("SPOND_WEBCAL_COOKIE_SECRET", "dev-secret")
	t.Setenv("SPOND_WEBCAL_LOG_LEVEL", "")
	t.Setenv("SPOND_WEBCAL_SPOND_BASE_URL", "")
	t.Setenv("SPOND_WEBCAL_SYNC_INTERVAL", "")
	t.Setenv("SPOND_WEBCAL_SYNC_TIMEOUT", "")

	config := Load()

	if config.Addr != defaultAddr {
		t.Fatalf("expected default addr %q, got %q", defaultAddr, config.Addr)
	}

	if config.DBURL != defaultDBURL {
		t.Fatalf("expected default db url %q, got %q", defaultDBURL, config.DBURL)
	}

	if config.CookieSecret != "dev-secret" {
		t.Fatalf("expected cookie secret dev-secret, got %q", config.CookieSecret)
	}

	if config.LogLevel != defaultLogLevel {
		t.Fatalf("expected default log level %q, got %q", defaultLogLevel, config.LogLevel)
	}

	if config.SpondBaseURL != defaultSpondBaseURL {
		t.Fatalf("expected default spond base url %q, got %q", defaultSpondBaseURL, config.SpondBaseURL)
	}

	if config.SyncInterval != defaultSyncInterval {
		t.Fatalf("expected default sync interval %s, got %s", defaultSyncInterval, config.SyncInterval)
	}

	if config.SyncTimeout != defaultSyncTimeout {
		t.Fatalf("expected default sync timeout %s, got %s", defaultSyncTimeout, config.SyncTimeout)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("SPOND_WEBCAL_ADDR", ":9090")
	t.Setenv("SPOND_WEBCAL_DB_URL", "postgres://example")
	t.Setenv("SPOND_WEBCAL_COOKIE_SECRET", "secret")
	t.Setenv("SPOND_WEBCAL_LOG_LEVEL", "info")
	t.Setenv("SPOND_WEBCAL_SPOND_BASE_URL", "http://localhost:9999")
	t.Setenv("SPOND_WEBCAL_SYNC_INTERVAL", "2m")
	t.Setenv("SPOND_WEBCAL_SYNC_TIMEOUT", "15s")

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

	if config.SpondBaseURL != "http://localhost:9999" {
		t.Fatalf("expected overridden spond base url, got %q", config.SpondBaseURL)
	}

	if config.SyncInterval != 2*time.Minute {
		t.Fatalf("expected overridden sync interval 2m, got %s", config.SyncInterval)
	}

	if config.SyncTimeout != 15*time.Second {
		t.Fatalf("expected overridden sync timeout 15s, got %s", config.SyncTimeout)
	}
}

func TestLoadFallsBackForInvalidDuration(t *testing.T) {
	t.Setenv("SPOND_WEBCAL_SYNC_INTERVAL", "bad")
	t.Setenv("SPOND_WEBCAL_SYNC_TIMEOUT", "0s")

	config := Load()

	if config.SyncInterval != defaultSyncInterval {
		t.Fatalf("expected default sync interval on invalid value, got %s", config.SyncInterval)
	}

	if config.SyncTimeout != defaultSyncTimeout {
		t.Fatalf("expected default sync timeout on invalid value, got %s", config.SyncTimeout)
	}
}
