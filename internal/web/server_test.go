package web

import (
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
	if cfg.CookieSecret == "" {
		cfg.CookieSecret = "test-cookie-secret"
	}

	h, err := caldav.NewHandler(caldav.NewMemoryResourceStore(), "test")
	if err != nil {
		t.Fatalf("new caldav handler: %v", err)
	}

	server, err := NewServer(cfg, h)
	if err != nil {
		t.Fatalf("new web server: %v", err)
	}

	return server
}

func TestNewServerRequiresCookieSecret(t *testing.T) {
	h, err := caldav.NewHandler(caldav.NewMemoryResourceStore(), "test")
	if err != nil {
		t.Fatalf("new caldav handler: %v", err)
	}

	_, err = NewServer(config.Config{Addr: ":9090", CookieSecret: ""}, h)
	if err == nil {
		t.Fatal("expected NewServer to require cookie secret")
	}
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

func TestIndexRouteRedirectsToSigninWithoutSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestSigninPageRenders(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/signin", nil)
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

func TestSigninSetsSessionAndRedirects(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
		case "/profile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
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

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/" {
		t.Fatalf("expected redirect to /, got %q", location)
	}

	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected session cookie to be set")
	}
}

func TestIndexRendersEventsForAuthenticatedSession(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
		case "/profile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		case "/sponds/":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"EVT1","heading":"Training","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Your events") {
		t.Fatalf("expected events page, got %q", body)
	}

	if !strings.Contains(body, "Training") || !strings.Contains(body, "Accepted") {
		t.Fatalf("expected rendered event status, got %q", body)
	}

	if !strings.Contains(body, "href=\"/profile\"") {
		t.Fatalf("expected navbar profile link, got %q", body)
	}

	if !strings.Contains(body, "action=\"/logout\"") {
		t.Fatalf("expected logout form, got %q", body)
	}
}

func TestProfileRouteRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestProfileRouteRendersForAuthenticatedSession(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123"}}`))
		case "/profile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Profile") || !strings.Contains(body, "Ada Lovelace") {
		t.Fatalf("expected profile content, got %q", body)
	}

	if !strings.Contains(body, "PROFILE1") {
		t.Fatalf("expected profile id in content, got %q", body)
	}

	if !strings.Contains(body, "Groups connected") {
		t.Fatalf("expected groups section in content, got %q", body)
	}
}

func TestSigninRedirectsOnLoginFailure(t *testing.T) {
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

	if location := rec.Header().Get("Location"); location != "/signin?error=login" {
		t.Fatalf("expected redirect to /signin?error=login, got %q", location)
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

	if location := rec.Header().Get("Location"); location != "/signin?error=missing" {
		t.Fatalf("expected redirect to /signin?error=missing, got %q", location)
	}
}

func TestLogoutClearsSessionAndRedirectsToSignin(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}

	var found bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge < 0 {
			found = true
		}
	}

	if !found {
		t.Fatal("expected logout to clear session cookie")
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

func TestSigninRouteShowsErrorMessageWhenRequested(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/signin?error=login", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Login failed") {
		t.Fatalf("expected login error message, got %q", body)
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

func signinAndGetSessionCookie(t *testing.T, server *Server) *http.Cookie {
	t.Helper()

	form := url.Values{}
	form.Set("email", "ada@example.com")
	form.Set("password", "secret")

	req := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected sign-in redirect status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}

	t.Fatal("expected session cookie after signin")
	return nil
}
