package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"code.p-fruck.eu/spond-webcal/internal/caldav"
	"code.p-fruck.eu/spond-webcal/internal/config"
)

func newTestServer(t *testing.T, cfg config.Config) *Server {
	t.Helper()

	h, err := caldav.NewHandler(caldav.NewMemoryResourceStore(), "test")
	if err != nil {
		t.Fatalf("new caldav handler: %v", err)
	}

	return NewServer(cfg, h)
}

func TestHealthzRouteReturnsOK(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":8080"})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "\"status\":\"ok\"") {
		t.Fatalf("expected health response to contain ok status, got %q", rec.Body.String())
	}
}

func TestIndexRouteRendersSigninPage(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Sign in with Spond") {
		t.Fatalf("expected sign-in page heading, got %q", body)
	}

	if !strings.Contains(body, "action=\"/signin\"") {
		t.Fatalf("expected sign-in form action, got %q", body)
	}
}

func TestSigninShowsAccountNameOnSuccess(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		case "/sponds/":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{
				"id":"EVT1",
				"heading":"Training",
				"startTimestamp":"2099-05-26T18:00:00Z",
				"endTimestamp":"2099-05-26T19:00:00Z",
				"responses":{"acceptedIds":["MEMBER1"]}
			},{
				"id":"EVT2",
				"heading":"Match",
				"startTimestamp":"2000-05-27T18:00:00Z",
				"endTimestamp":"2000-05-27T19:00:00Z",
				"responses":{"declinedIds":["MEMBER1"]}
			}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	form := url.Values{}
	form.Set("email", "ada@example.com")
	form.Set("password", "secret")

	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Signed in successfully") {
		t.Fatalf("expected account page title, got %q", body)
	}

	if !strings.Contains(body, "Ada Lovelace") {
		t.Fatalf("expected account name in response, got %q", body)
	}

	if !strings.Contains(body, "Training") || !strings.Contains(body, "Accepted") {
		t.Fatalf("expected accepted event in response, got %q", body)
	}

	if !strings.Contains(body, "Match") || !strings.Contains(body, "Declined") {
		t.Fatalf("expected declined event in response, got %q", body)
	}

	if !strings.Contains(body, "Past events (1)") {
		t.Fatalf("expected past events disclosure, got %q", body)
	}

	if strings.Index(body, "Past events (1)") > strings.Index(body, "Upcoming events") {
		t.Fatalf("expected past events disclosure above upcoming events, got %q", body)
	}

	if !strings.Contains(body, "class=\"event-start\"") || !strings.Contains(body, "data-start=") {
		t.Fatalf("expected local-time conversion markup in response, got %q", body)
	}
}

func TestSigninRedirectsToIndexOnLoginFailure(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth2/login" {
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	form := url.Values{}
	form.Set("email", "ada@example.com")
	form.Set("password", "wrong")

	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/?error=login" {
		t.Fatalf("expected redirect to /?error=login, got %q", location)
	}
}

func TestSigninMissingFormFieldsRedirects(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/?error=missing" {
		t.Fatalf("expected redirect to /?error=missing, got %q", location)
	}
}

func TestCalDAVEndpointHandlesOptions(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodOptions, "/caldav", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		t.Fatalf("expected 2xx status for OPTIONS, got %d", rec.Code)
	}
}

func TestUserScopedCalDAVEndpointHandlesOptions(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodOptions, "/caldav/alice", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		t.Fatalf("expected 2xx status for user-scoped OPTIONS, got %d", rec.Code)
	}
}

func TestWellKnownCalDAVRedirectsToCalDAV(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/caldav", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/caldav" {
		t.Fatalf("expected redirect location /caldav, got %q", location)
	}
}

func TestIndexRouteShowsErrorMessageWhenRequested(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/?error=login", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if !strings.Contains(string(body), "Login failed") {
		t.Fatalf("expected login error message, got %q", string(body))
	}
}

func TestStaticCSSIsServed(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, ".page-signin") {
		t.Fatalf("expected css body to contain page-signin rules, got %q", body)
	}
}
