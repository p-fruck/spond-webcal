package spond

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.p-fruck.eu/spond-webcal/internal/api"
)

func TestLoginStoresToken(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth2/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
	}))
	defer testServer.Close()

	client, err := New(testServer.URL)
	if err != nil {
		t.Fatalf("create spond client: %v", err)
	}

	if err := client.Login(context.Background(), "me@example.com", "secret"); err != nil {
		t.Fatalf("login: %v", err)
	}

	if client.Token() != "TOKEN123" {
		t.Fatalf("expected token to be stored, got %q", client.Token())
	}
}

func TestFetchEventsRequiresLogin(t *testing.T) {
	client := NewWithAPIClient(nil)

	_, err := client.FetchEvents(context.Background(), 10)
	if err == nil {
		t.Fatal("expected fetch events to fail without authentication")
	}
}

func TestFetchEventsReturnsEventsAndSendsBearer(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
		case "/sponds/":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"EVT1","heading":"Training","startTimestamp":"2026-05-26T18:00:00Z","endTimestamp":"2026-05-26T19:00:00Z"}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()

	client, err := New(testServer.URL)
	if err != nil {
		t.Fatalf("create spond client: %v", err)
	}

	if err := client.Login(context.Background(), "me@example.com", "secret"); err != nil {
		t.Fatalf("login: %v", err)
	}

	events, err := client.FetchEvents(context.Background(), 100)
	if err != nil {
		t.Fatalf("fetch events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Id != "EVT1" {
		t.Fatalf("expected event id EVT1, got %q", events[0].Id)
	}

	if events[0].Heading != "Training" {
		t.Fatalf("expected event heading Training, got %q", events[0].Heading)
	}
}

func TestLoginReturnsErrorOnUnauthorized(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(api.ErrorResponse{Message: strPtr("invalid credentials")})
	}))
	defer testServer.Close()

	client, err := New(testServer.URL)
	if err != nil {
		t.Fatalf("create spond client: %v", err)
	}

	err = client.Login(context.Background(), "me@example.com", "wrong")
	if err == nil {
		t.Fatal("expected login to fail on unauthorized response")
	}
}

func TestFetchProfileRequiresLogin(t *testing.T) {
	client := NewWithAPIClient(nil)

	_, err := client.FetchProfile(context.Background())
	if err == nil {
		t.Fatal("expected fetch profile to fail without authentication")
	}
}

func TestFetchProfileReturnsProfileAndSendsBearer(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
		case "/profile":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"U1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()

	client, err := New(testServer.URL)
	if err != nil {
		t.Fatalf("create spond client: %v", err)
	}

	if err := client.Login(context.Background(), "me@example.com", "secret"); err != nil {
		t.Fatalf("login: %v", err)
	}

	profile, err := client.FetchProfile(context.Background())
	if err != nil {
		t.Fatalf("fetch profile: %v", err)
	}

	if profile.FirstName != "Ada" || profile.LastName != "Lovelace" {
		t.Fatalf("expected Ada Lovelace, got %q %q", profile.FirstName, profile.LastName)
	}
}

func strPtr(value string) *string {
	return &value
}
