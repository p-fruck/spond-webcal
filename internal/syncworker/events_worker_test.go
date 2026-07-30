package syncworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"code.p-fruck.eu/spond-webcal/internal/api"
	"code.p-fruck.eu/spond-webcal/internal/db"
)

type fakeUserTokenReader struct {
	users []db.UserTokenRecord
	err   error
}

func (f fakeUserTokenReader) ActiveUsersWithToken(_ context.Context) ([]db.UserTokenRecord, error) {
	return f.users, f.err
}

type fakeEventCacheWriter struct {
	mu      sync.Mutex
	writes  int
	events  []api.Event
	userIDs []uint
}

func (f *fakeEventCacheWriter) UpsertUserEvents(_ context.Context, userID uint, events []api.Event, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++
	f.userIDs = append(f.userIDs, userID)
	f.events = append(f.events, events...)
	return nil
}

func TestEventsWorkerRunOnceCachesEvents(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sponds/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
			t.Fatalf("expected bearer token, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"EVT1","heading":"Training","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z"}]`))
	}))
	defer apiServer.Close()

	reader := fakeUserTokenReader{users: []db.UserTokenRecord{{UserID: 42, ProfileID: "P1", SpondToken: "TOKEN123"}}}
	cache := &fakeEventCacheWriter{}

	worker, err := NewEventsWorker(reader, cache, apiServer.URL, time.Minute, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	if cache.writes != 1 {
		t.Fatalf("expected 1 cache write, got %d", cache.writes)
	}

	if len(cache.events) != 1 || cache.events[0].Id != "EVT1" {
		t.Fatalf("expected EVT1 cached, got %+v", cache.events)
	}
}

func TestEventsWorkerRunOnceHonorsTimeout(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer apiServer.Close()

	reader := fakeUserTokenReader{users: []db.UserTokenRecord{{UserID: 42, ProfileID: "P1", SpondToken: "TOKEN123"}}}
	cache := &fakeEventCacheWriter{}

	worker, err := NewEventsWorker(reader, cache, apiServer.URL, time.Minute, 20*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once should not fail hard for single-user timeout: %v", err)
	}

	if cache.writes != 0 {
		t.Fatalf("expected no cache writes when request times out, got %d", cache.writes)
	}
}

func TestChooseTokenForSyncPrefersAccessTokenWhenValid(t *testing.T) {
	now := time.Now().UTC()
	accessExpiry := now.Add(2 * time.Minute)
	refreshExpiry := now.Add(10 * time.Minute)

	token, err := chooseTokenForSync(db.UserTokenRecord{
		SpondToken:          "ACCESS",
		TokenExpires:        &accessExpiry,
		RefreshToken:        "REFRESH",
		RefreshTokenExpires: &refreshExpiry,
	}, now)
	if err != nil {
		t.Fatalf("choose token: %v", err)
	}

	if token != "ACCESS" {
		t.Fatalf("expected ACCESS token, got %q", token)
	}
}

func TestChooseTokenForSyncFallsBackToRefresh(t *testing.T) {
	now := time.Now().UTC()
	accessExpiry := now.Add(-1 * time.Minute)
	refreshExpiry := now.Add(10 * time.Minute)

	token, err := chooseTokenForSync(db.UserTokenRecord{
		SpondToken:          "ACCESS",
		TokenExpires:        &accessExpiry,
		RefreshToken:        "REFRESH",
		RefreshTokenExpires: &refreshExpiry,
	}, now)
	if err != nil {
		t.Fatalf("choose token: %v", err)
	}

	if token != "REFRESH" {
		t.Fatalf("expected REFRESH token, got %q", token)
	}
}

func TestChooseTokenForSyncErrorsWhenAllExpired(t *testing.T) {
	now := time.Now().UTC()
	accessExpiry := now.Add(-2 * time.Minute)
	refreshExpiry := now.Add(-1 * time.Minute)

	_, err := chooseTokenForSync(db.UserTokenRecord{
		SpondToken:          "ACCESS",
		TokenExpires:        &accessExpiry,
		RefreshToken:        "REFRESH",
		RefreshTokenExpires: &refreshExpiry,
	}, now)
	if err == nil {
		t.Fatal("expected error when all tokens are expired")
	}
}
