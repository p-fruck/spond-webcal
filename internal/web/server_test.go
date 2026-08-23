package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

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

	server, err := NewServer(cfg, h, NewMemoryUserTokenStore(), NewMemoryEventSyncStore(), NewMemoryAccessTokenStore())
	if err != nil {
		t.Fatalf("new web server: %v", err)
	}

	return server
}

func newTestServerWithSyncStore(t *testing.T, cfg config.Config, syncStore EventSyncStore) *Server {
	t.Helper()
	return newTestServerWithStores(t, cfg, syncStore, NewMemoryAccessTokenStore())
}

func newTestServerWithStores(t *testing.T, cfg config.Config, syncStore EventSyncStore, accessTokenStore AccessTokenStore) *Server {
	t.Helper()
	if cfg.CookieSecret == "" {
		cfg.CookieSecret = "test-cookie-secret"
	}

	h, err := caldav.NewHandler(caldav.NewMemoryResourceStore(), "test")
	if err != nil {
		t.Fatalf("new caldav handler: %v", err)
	}

	server, err := NewServer(cfg, h, NewMemoryUserTokenStore(), syncStore, accessTokenStore)
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

	_, err = NewServer(config.Config{Addr: ":9090", CookieSecret: ""}, h, NewMemoryUserTokenStore(), NewMemoryEventSyncStore(), NewMemoryAccessTokenStore())
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

	if !strings.Contains(body, "Last sync") || !strings.Contains(body, "Next sync") || !strings.Contains(body, "Sync now") {
		t.Fatalf("expected sync status controls, got %q", body)
	}
}

func TestIndexFiltersEventsByGroupAndStatus(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch r.URL.Query().Get("groupId") {
			case "G1":
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Accepted in Team A","groupId":"G1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
					{"id":"EVT2","heading":"Declined in Team A","groupId":"G1","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"declinedIds":["MEMBER1"]}}
				]`))
			case "G2":
				_, _ = w.Write([]byte(`[
					{"id":"EVT3","heading":"Accepted in Team B","groupId":"G2","startTimestamp":"2099-05-28T18:00:00Z","endTimestamp":"2099-05-28T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			default:
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Accepted in Team A","groupId":"G1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
					{"id":"EVT2","heading":"Declined in Team A","groupId":"G1","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"declinedIds":["MEMBER1"]}},
					{"id":"EVT3","heading":"Accepted in Team B","groupId":"G2","startTimestamp":"2099-05-28T18:00:00Z","endTimestamp":"2099-05-28T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/?groups=G1&status=0,2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Accepted in Team A") {
		t.Fatalf("expected filtered accepted event from selected group, got %q", body)
	}

	if strings.Contains(body, "Declined in Team A") {
		t.Fatalf("did not expect declined event when filtering by accepted/unanswered, got %q", body)
	}

	if strings.Contains(body, "Accepted in Team B") {
		t.Fatalf("did not expect events from non-selected groups, got %q", body)
	}
}

func TestEventsSyncNowRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodPost, "/api/events/sync", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestEventsSyncNowRunsAndReturnsSchedule(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"EVT1","heading":"Training","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z"}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServerWithSyncStore(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL, SyncInterval: 3 * time.Minute}, NewMemoryEventSyncStore())
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/events/sync", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "sync completed") || !strings.Contains(body, "eventCount") || !strings.Contains(body, "nextSyncAt") {
		t.Fatalf("expected sync payload fields, got %q", body)
	}
}

func TestCreateAccessTokenRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodPost, "/api/profile/access-tokens", strings.NewReader(`{"category":"ical","groups":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestCreateAccessTokenAndScopedExport(t *testing.T) {
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
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			if got := r.Header.Get("Authorization"); got != "Bearer TOKEN123" {
				t.Fatalf("expected bearer token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":"EVT1","heading":"Accepted in Team A","groupId":"G1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
				{"id":"EVT2","heading":"Declined in Team A","groupId":"G1","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"declinedIds":["MEMBER1"]}},
				{"id":"EVT3","heading":"Accepted in Team B","groupId":"G2","startTimestamp":"2099-05-28T18:00:00Z","endTimestamp":"2099-05-28T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
				{"id":"EVT4","heading":"Past Accepted in Team A","groupId":"G1","startTimestamp":"2001-05-28T18:00:00Z","endTimestamp":"2001-05-28T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
			]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServerWithStores(
		t,
		config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL},
		NewMemoryEventSyncStore(),
		NewMemoryAccessTokenStore(),
	)
	cookie := signinAndGetSessionCookie(t, server)

	createBody := `{"category":"ical","groups":[{"groupId":"G1","statuses":["0"],"includePast":true}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/profile/access-tokens", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, createRec.Code, createRec.Body.String())
	}

	var payload struct {
		Token     string `json:"token"`
		ExportURL string `json:"exportURL"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal create token payload: %v", err)
	}
	if payload.Token == "" || payload.ExportURL == "" {
		t.Fatalf("expected token and export url, got %+v", payload)
	}

	exportReq := httptest.NewRequest(http.MethodGet, payload.ExportURL, nil)
	exportRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, exportRec.Code, exportRec.Body.String())
	}

	ics := exportRec.Body.String()
	if !strings.Contains(ics, "SUMMARY:Accepted in Team A") {
		t.Fatalf("expected scoped accepted event from Team A, got %q", ics)
	}

	if !strings.Contains(ics, "SUMMARY:Past Accepted in Team A") {
		t.Fatalf("expected past event when includePast is true, got %q", ics)
	}

	if strings.Contains(ics, "SUMMARY:Declined in Team A") {
		t.Fatalf("did not expect declined events for accepted-only scope, got %q", ics)
	}

	if strings.Contains(ics, "SUMMARY:Accepted in Team B") {
		t.Fatalf("did not expect events from non-scoped group, got %q", ics)
	}
}

func TestAccessTokenExportIncludesEventsWhenGroupIDIsOmittedByAPI(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch r.URL.Query().Get("groupId") {
			case "G1":
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Accepted in Team A","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			case "G2":
				_, _ = w.Write([]byte(`[
					{"id":"EVT2","heading":"Accepted in Team B","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			default:
				_, _ = w.Write([]byte(`[]`))
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServerWithStores(
		t,
		config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL},
		NewMemoryEventSyncStore(),
		NewMemoryAccessTokenStore(),
	)
	cookie := signinAndGetSessionCookie(t, server)

	createBody := `{"category":"ical","groups":[{"groupId":"G1","statuses":["accepted"],"includePast":false}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/profile/access-tokens", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, createRec.Code, createRec.Body.String())
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal create token payload: %v", err)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/events.ics?access_token="+url.QueryEscape(payload.Token), nil)
	exportRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, exportRec.Code, exportRec.Body.String())
	}

	ics := exportRec.Body.String()
	if !strings.Contains(ics, "SUMMARY:Accepted in Team A") {
		t.Fatalf("expected accepted event from selected group despite missing groupId in API payload, got %q", ics)
	}
	if strings.Contains(ics, "SUMMARY:Accepted in Team B") {
		t.Fatalf("did not expect event from unselected group, got %q", ics)
	}
}

func TestEventsExportRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/events.ics", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestEventsExportReturnsICSWithAppliedFilters(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch r.URL.Query().Get("groupId") {
			case "G1":
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Accepted in Team A","groupId":"G1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
					{"id":"EVT2","heading":"Declined in Team A","groupId":"G1","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"declinedIds":["MEMBER1"]}}
				]`))
			case "G2":
				_, _ = w.Write([]byte(`[
					{"id":"EVT3","heading":"Accepted in Team B","groupId":"G2","startTimestamp":"2099-05-28T18:00:00Z","endTimestamp":"2099-05-28T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			default:
				_, _ = w.Write([]byte(`[]`))
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/events.ics?groups=G1&status=0,2", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/calendar") {
		t.Fatalf("expected text/calendar content type, got %q", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VCALENDAR") || !strings.Contains(body, "BEGIN:VEVENT") {
		t.Fatalf("expected ICS payload, got %q", body)
	}

	if !strings.Contains(body, "SUMMARY:Accepted in Team A") {
		t.Fatalf("expected accepted Team A event in export, got %q", body)
	}

	if strings.Contains(body, "SUMMARY:Declined in Team A") {
		t.Fatalf("did not expect declined Team A event in export, got %q", body)
	}

	if strings.Contains(body, "SUMMARY:Accepted in Team B") {
		t.Fatalf("did not expect Team B event in export, got %q", body)
	}
}

func TestEventsExportIncludePastFlag(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":"EVT-UPCOMING","heading":"Upcoming event","groupId":"G1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
				{"id":"EVT-PAST","heading":"Past event","groupId":"G1","startTimestamp":"2001-05-26T18:00:00Z","endTimestamp":"2001-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
			]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	reqDefault := httptest.NewRequest(http.MethodGet, "/events.ics", nil)
	reqDefault.AddCookie(cookie)
	recDefault := httptest.NewRecorder()
	server.Echo().ServeHTTP(recDefault, reqDefault)

	if recDefault.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recDefault.Code)
	}

	defaultBody := recDefault.Body.String()
	if !strings.Contains(defaultBody, "SUMMARY:Upcoming event") {
		t.Fatalf("expected upcoming event in default export, got %q", defaultBody)
	}

	if strings.Contains(defaultBody, "SUMMARY:Past event") {
		t.Fatalf("did not expect past event in default export, got %q", defaultBody)
	}

	reqWithPast := httptest.NewRequest(http.MethodGet, "/events.ics?includePast=true", nil)
	reqWithPast.AddCookie(cookie)
	recWithPast := httptest.NewRecorder()
	server.Echo().ServeHTTP(recWithPast, reqWithPast)

	if recWithPast.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recWithPast.Code)
	}

	withPastBody := recWithPast.Body.String()
	if !strings.Contains(withPastBody, "SUMMARY:Past event") {
		t.Fatalf("expected past event when includePast=true, got %q", withPastBody)
	}
}

func TestIndexFiltersEventsByGroupWhenEventUsesSubGroupID(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}],"subGroups":[{"id":"SG1","name":"Squad 1"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}],"subGroups":[{"id":"SG2","name":"Squad 2"}]}
			]`))
		case "/sponds/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch r.URL.Query().Get("groupId") {
			case "G1":
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Subgroup event in Team A","subGroupId":"SG1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			case "G2":
				_, _ = w.Write([]byte(`[
					{"id":"EVT2","heading":"Subgroup event in Team B","subGroupId":"SG2","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			default:
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Subgroup event in Team A","subGroupId":"SG1","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}},
					{"id":"EVT2","heading":"Subgroup event in Team B","subGroupId":"SG2","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/?group=G1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Subgroup event in Team A") {
		t.Fatalf("expected subgroup event in selected parent group, got %q", body)
	}

	if strings.Contains(body, "Subgroup event in Team B") {
		t.Fatalf("did not expect subgroup event from non-selected group, got %q", body)
	}
}

func TestIndexFiltersEventsByGroupWhenEventsDoNotExposeGroupIDs(t *testing.T) {
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
			_, _ = w.Write([]byte(`[
				{"id":"G1","name":"Team A","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]},
				{"id":"G2","name":"Team B","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}
			]`))
		case "/sponds/":
			groupID := r.URL.Query().Get("groupId")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			switch groupID {
			case "G1":
				_, _ = w.Write([]byte(`[
					{"id":"EVT1","heading":"Group A event no id fields","startTimestamp":"2099-05-26T18:00:00Z","endTimestamp":"2099-05-26T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			case "G2":
				_, _ = w.Write([]byte(`[
					{"id":"EVT2","heading":"Group B event no id fields","startTimestamp":"2099-05-27T18:00:00Z","endTimestamp":"2099-05-27T19:00:00Z","responses":{"acceptedIds":["MEMBER1"]}}
				]`))
			default:
				_, _ = w.Write([]byte(`[]`))
			}
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/?group=G1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Group A event no id fields") {
		t.Fatalf("expected group-scoped event for selected group, got %q", body)
	}

	if strings.Contains(body, "Group B event no id fields") {
		t.Fatalf("did not expect events for unselected group, got %q", body)
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
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

	if !strings.Contains(body, "Team Alpha") || !strings.Contains(body, "href=\"/groups/G1\"") {
		t.Fatalf("expected group link in profile page, got %q", body)
	}

	if !strings.Contains(body, "Show token state") || !strings.Contains(body, "Refresh now") {
		t.Fatalf("expected advanced token controls in profile page, got %q", body)
	}

	if !strings.Contains(body, "Access tokens") || !strings.Contains(body, "Create token") {
		t.Fatalf("expected access token section in profile page, got %q", body)
	}
}

func TestAccessTokensPageRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/profile/access-tokens", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/profile" {
		t.Fatalf("expected redirect to /profile, got %q", location)
	}
}

func TestAccessTokenCreatePageRendersGroups(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/profile/access-tokens/new", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Create iCal access token") || !strings.Contains(body, "Team Alpha") {
		t.Fatalf("expected token create page and group rules, got %q", body)
	}

	if !strings.Contains(body, "value=\"accepted\"") || !strings.Contains(body, "Include past events") {
		t.Fatalf("expected status and past-event checkboxes, got %q", body)
	}
}

func TestAccessTokenCreatePostRedirectsAndShowsCreatedToken(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	form := url.Values{}
	form.Add("group", "G1")
	form.Add("status__G1", "accepted")
	form.Add("status__G1", "unanswered")
	form.Set("past__G1", "1")

	req := httptest.NewRequest(http.MethodPost, "/profile/access-tokens/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/profile?createdToken=") {
		t.Fatalf("expected redirect with created token, got %q", location)
	}

	listReq := httptest.NewRequest(http.MethodGet, location, nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRec.Code)
	}

	body := listRec.Body.String()
	if !strings.Contains(body, "Token created. This is shown only once.") || !strings.Contains(body, "Group G1") {
		t.Fatalf("expected created token and rule summary in list page, got %q", body)
	}
}

func TestAccessTokenDeleteFromProfile(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	form := url.Values{}
	form.Add("group", "G1")
	form.Add("status__G1", "accepted")
	req := httptest.NewRequest(http.MethodPost, "/profile/access-tokens/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	memoryStore, ok := server.accessTokens.(*MemoryAccessTokenStore)
	if !ok {
		t.Fatal("expected memory access token store")
	}
	records, err := memoryStore.ListAccessTokensByUserID(req.Context(), 1)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one token, got %d", len(records))
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/profile/access-tokens/"+strconv.FormatUint(uint64(records[0].ID), 10)+"/delete", nil)
	deleteReq.AddCookie(cookie)
	deleteRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, deleteRec.Code)
	}

	remaining, err := memoryStore.ListAccessTokensByUserID(req.Context(), 1)
	if err != nil {
		t.Fatalf("list remaining tokens: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected zero tokens after delete, got %d", len(remaining))
	}
}

func TestExpiredAccessTokenIsRejectedAndDeletedOnExport(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	expiresAt := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	createBody := `{"category":"ical","expiresAt":"` + expiresAt + `","groups":[{"groupId":"G1","statuses":["accepted"],"includePast":false}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/profile/access-tokens", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, createRec.Code)
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/events.ics?access_token="+url.QueryEscape(payload.Token), nil)
	exportRec := httptest.NewRecorder()
	server.Echo().ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, exportRec.Code)
	}

	memoryStore, ok := server.accessTokens.(*MemoryAccessTokenStore)
	if !ok {
		t.Fatal("expected memory access token store")
	}
	remaining, err := memoryStore.ListAccessTokensByUserID(exportReq.Context(), 1)
	if err != nil {
		t.Fatalf("list remaining tokens: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected expired token to be removed, got %d token(s)", len(remaining))
	}
}

func TestProfileTokensAPIRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodPost, "/api/profile/tokens/reveal", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestProfileTokensAPIReturnsStoredTokens(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123","expiration":"2099-01-01T00:00:00Z"},"refreshToken":{"token":"REFRESH123","expiration":"2099-02-01T00:00:00Z"}}`))
		case "/profile":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/profile/tokens/reveal", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "TOKEN123") || !strings.Contains(body, "REFRESH123") {
		t.Fatalf("expected access/refresh tokens in payload, got %q", body)
	}
}

func TestProfileTokensRefreshPromotesRefreshToken(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth2/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accessToken":{"token":"TOKEN123","expiration":"2001-01-01T00:00:00Z"},"refreshToken":{"token":"REFRESH123","expiration":"2099-02-01T00:00:00Z"}}`))
		case "/profile":
			if got := r.Header.Get("Authorization"); got != "Bearer REFRESH123" && got != "Bearer TOKEN123" {
				t.Fatalf("unexpected bearer token %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PROFILE1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}`))
		case "/groups/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodPost, "/api/profile/tokens/refresh", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "refresh token promoted") {
		t.Fatalf("expected promotion message, got %q", body)
	}

	if !strings.Contains(body, "REFRESH123") {
		t.Fatalf("expected promoted access token to be refresh token, got %q", body)
	}
}

func TestGroupDetailRouteRequiresSession(t *testing.T) {
	server := newTestServer(t, config.Config{Addr: ":9090"})
	req := httptest.NewRequest(http.MethodGet, "/groups/G1", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/signin" {
		t.Fatalf("expected redirect to /signin, got %q", location)
	}
}

func TestGroupDetailRouteRendersForAuthenticatedSession(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"id":"G1","name":"Team Alpha","members":[{"id":"MEMBER1","firstName":"Ada","lastName":"Lovelace","email":"ada@example.com"}],"subGroups":[{"id":"SG1","name":"Sub A"}]}]`))
		default:
			t.Fatalf("unexpected API path: %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	server := newTestServer(t, config.Config{Addr: ":9090", SpondBaseURL: apiServer.URL})
	cookie := signinAndGetSessionCookie(t, server)

	req := httptest.NewRequest(http.MethodGet, "/groups/G1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Team Alpha") {
		t.Fatalf("expected group name in detail page, got %q", body)
	}

	if !strings.Contains(body, "MEMBER1") {
		t.Fatalf("expected member id in detail page, got %q", body)
	}
}

func TestGroupDetailRouteReturnsNotFoundWhenMissing(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/groups/DOES_NOT_EXIST", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
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
