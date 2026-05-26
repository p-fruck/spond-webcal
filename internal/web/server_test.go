package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.p-fruck.eu/spond-webcal/internal/config"
)

func TestHealthzRouteReturnsOK(t *testing.T) {
	server := NewServer(config.Config{Addr: ":8080"})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["status"] != "ok" {
		t.Fatalf("expected health status ok, got %q", payload["status"])
	}
}

func TestIndexRouteExposesBasicServiceMetadata(t *testing.T) {
	server := NewServer(config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["name"] != "spond-webcal" {
		t.Fatalf("expected service name spond-webcal, got %q", payload["name"])
	}

	if payload["listenAddr"] != ":9090" {
		t.Fatalf("expected listen addr :9090, got %q", payload["listenAddr"])
	}
}