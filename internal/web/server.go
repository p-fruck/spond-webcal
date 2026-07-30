package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"code.p-fruck.eu/spond-webcal/internal/api"
	"code.p-fruck.eu/spond-webcal/internal/config"
	"code.p-fruck.eu/spond-webcal/internal/spond"
)

type Server struct {
	e         *echo.Echo
	templates *template.Template
	cfg       config.Config
	userStore UserTokenStore
	syncStore EventSyncStore
}

func NewServer(cfg config.Config, caldavHandler http.Handler, userStore UserTokenStore, syncStore EventSyncStore) (*Server, error) {
	if strings.TrimSpace(cfg.CookieSecret) == "" {
		return nil, fmt.Errorf("SPOND_WEBCAL_COOKIE_SECRET is required")
	}

	if userStore == nil {
		return nil, fmt.Errorf("user token store is required")
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	templates := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	staticSubFS := mustSubFS(staticFS, "static")

	e.Any("/.well-known/caldav", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/caldav")
	})

	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS)))))

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	s := &Server{e: e, templates: templates, cfg: cfg, userStore: userStore, syncStore: syncStore}

	e.GET("/", s.handleEventsPage)
	e.GET("/events.ics", s.handleEventsExport)
	e.GET("/signin", s.handleSigninPage)
	e.POST("/signin", s.handleSigninPost)
	e.GET("/profile", s.handleProfilePage)
	e.POST("/api/events/sync", s.handleEventsSyncNowAPI)
	e.POST("/api/profile/tokens/reveal", s.handleProfileTokensAPI)
	e.POST("/api/profile/tokens/refresh", s.handleProfileTokensRefreshAPI)
	e.GET("/groups/:groupID", s.handleGroupDetailPage)
	e.POST("/logout", s.handleLogoutPost)

	e.Any("/caldav", echo.WrapHandler(caldavHandler))
	e.Any("/caldav/*", echo.WrapHandler(caldavHandler))

	return s, nil
}

func (s *Server) handleEventsPage(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	data, shouldClearSession, err := s.loadFilteredEventsData(c, session)
	if err != nil {
		if shouldClearSession {
			s.clearSession(c)
			return c.Redirect(http.StatusSeeOther, "/signin?error=session")
		}

		return c.String(http.StatusInternalServerError, "failed to load events")
	}

	s.addSyncStatusToAccountPageData(c, session.UserID, &data)

	return s.renderTemplate(c, "account.html", data)
}

func (s *Server) handleEventsSyncNowAPI(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	if s.syncStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "sync store is not configured"})
	}

	accessToken, accessExpires, refreshToken, refreshExpires, err := s.userStore.TokenMetadataByUserID(c.Request().Context(), session.UserID)
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	token, err := chooseTokenForImmediateSync(accessToken, accessExpires, refreshToken, refreshExpires, time.Now().UTC())
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	client, err := spond.New(s.cfg.SpondBaseURL)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to initialize spond client")
	}
	client.SetToken(token)

	events, err := client.FetchEvents(c.Request().Context(), 200)
	if err != nil {
		return c.String(http.StatusBadGateway, "failed to fetch events from spond")
	}

	syncedAt := time.Now().UTC()
	if err := s.syncStore.UpsertUserEvents(c.Request().Context(), session.UserID, events, syncedAt); err != nil {
		return c.String(http.StatusInternalServerError, "failed to cache synced events")
	}

	nextSync := syncedAt.Add(s.effectiveSyncInterval())
	return c.JSON(http.StatusOK, map[string]any{
		"message":    "sync completed",
		"eventCount": len(events),
		"lastSyncAt": syncedAt.Format(time.RFC3339),
		"nextSyncAt": nextSync.Format(time.RFC3339),
	})
}

func (s *Server) handleEventsExport(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	data, shouldClearSession, err := s.loadFilteredEventsData(c, session)
	if err != nil {
		if shouldClearSession {
			s.clearSession(c)
			return c.Redirect(http.StatusSeeOther, "/signin?error=session")
		}

		return c.String(http.StatusInternalServerError, "failed to export events")
	}

	includePast := parseBoolQueryParam(c.QueryParam("includePast"), false)
	events := mergedSortedEvents(data.UpcomingEvents, includePastEvents(data.PastEvents, includePast))
	ics := buildICSCalendar(events, time.Now().UTC())

	c.Response().Header().Set(echo.HeaderContentType, "text/calendar; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=events.ics")
	return c.String(http.StatusOK, ics)
}

func (s *Server) loadFilteredEventsData(c echo.Context, session authSession) (accountPageData, bool, error) {
	client, shouldClearSession, err := s.spondClientForSession(c, session)
	if err != nil {
		return accountPageData{}, shouldClearSession, err
	}

	groups, err := client.FetchGroups(c.Request().Context())
	if err != nil {
		return accountPageData{}, true, err
	}

	selectedGroupIDsList := parseCompactFilterValues(c.QueryParams(), "groups", "group")
	selectedGroupIDs := normalizeFilterValues(selectedGroupIDsList)
	selectedStatuses := normalizeStatusFilterValues(parseCompactFilterValues(c.QueryParams(), "status"))

	var events []api.Event
	if len(selectedGroupIDsList) > 0 {
		events, err = client.FetchEventsForGroups(c.Request().Context(), 100, selectedGroupIDsList)
	} else {
		events, err = client.FetchEvents(c.Request().Context(), 100)
	}
	if err != nil {
		return accountPageData{}, true, err
	}

	groupNamesByID := buildGroupNamesByID(groups)
	subGroupParentByID := buildSubGroupParentByID(groups)
	upcomingEvents, pastEvents := buildEventViewData(events, session.ActorIDs, groupNamesByID, subGroupParentByID, time.Now().UTC())

	groupFilter := selectedGroupIDs
	if len(selectedGroupIDsList) > 0 {
		groupFilter = nil
	}

	upcomingEvents = filterEventViewData(upcomingEvents, groupFilter, selectedStatuses)
	pastEvents = filterEventViewData(pastEvents, groupFilter, selectedStatuses)

	data := accountPageData{
		Name:           session.Name,
		Email:          session.Email,
		GroupFilters:   buildGroupFilterViewData(groups, selectedGroupIDs),
		StatusFilters:  buildStatusFilterViewData(selectedStatuses),
		UpcomingEvents: upcomingEvents,
		PastEvents:     pastEvents,
	}

	return data, false, nil
}

func (s *Server) handleSigninPage(c echo.Context) error {
	if _, ok := s.readSession(c); ok {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	errorKey := c.QueryParam("error")
	errorMsg := ""
	switch errorKey {
	case "missing":
		errorMsg = "Please enter your email and password."
	case "profile":
		errorMsg = "Logged in, but failed to load your profile. Please try again."
	case "session":
		errorMsg = "Your session expired. Please sign in again."
	case "login", "":
		if errorKey != "" {
			errorMsg = "Login failed. Please check your credentials and try again."
		}
	default:
		errorMsg = "Something went wrong. Please sign in again."
	}

	data := map[string]string{"Error": errorMsg}
	return s.renderTemplate(c, "signin.html", data)
}

func (s *Server) handleSigninPost(c echo.Context) error {
	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")
	if email == "" || password == "" {
		return c.Redirect(http.StatusSeeOther, "/signin?error=missing")
	}

	client, err := spond.New(s.cfg.SpondBaseURL)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to initialize spond client")
	}

	if err := client.Login(c.Request().Context(), email, password); err != nil {
		return c.Redirect(http.StatusSeeOther, "/signin?error=login")
	}

	profile, err := client.FetchProfile(c.Request().Context())
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/signin?error=profile")
	}

	groups, err := client.FetchGroups(c.Request().Context())
	if err != nil {
		groups = nil
	}

	profileEmail := email
	if profile.Email != nil {
		profileEmail = string(*profile.Email)
	}

	userID, err := s.userStore.UpsertUserToken(
		c.Request().Context(),
		profile.Id,
		profileEmail,
		client.Token(),
		client.TokenExpiresAt(),
		client.RefreshToken(),
		client.RefreshTokenExpiresAt(),
	)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to persist user session")
	}

	session := authSession{
		UserID:     userID,
		Name:       fullNameFromProfile(profile.FirstName, profile.LastName, email),
		Email:      profileEmail,
		ProfileID:  profile.Id,
		GroupCount: len(groups),
		ActorIDs:   responseActorIDs(profile, profileEmail, groups),
	}

	if err := s.writeSession(c, session); err != nil {
		return c.String(http.StatusInternalServerError, "failed to create session")
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) handleProfilePage(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	client, shouldClearSession, err := s.spondClientForSession(c, session)
	if err != nil {
		if shouldClearSession {
			s.clearSession(c)
			return c.Redirect(http.StatusSeeOther, "/signin?error=session")
		}

		return c.String(http.StatusInternalServerError, "failed to initialize spond client")
	}

	groups, err := client.FetchGroups(c.Request().Context())
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	data := profilePageData{
		Name:       session.Name,
		Email:      session.Email,
		ProfileID:  session.ProfileID,
		GroupCount: len(groups),
		ActorIDs:   session.ActorIDs,
		Groups:     buildGroupSummaryData(groups),
	}

	return s.renderTemplate(c, "profile.html", data)
}

func (s *Server) handleProfileTokensAPI(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	tokens, err := s.tokenPayloadForUser(c, session.UserID)
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	return c.JSON(http.StatusOK, tokens)
}

func (s *Server) handleProfileTokensRefreshAPI(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	accessToken, _, refreshToken, _, err := s.userStore.TokenMetadataByUserID(c.Request().Context(), session.UserID)
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no refresh token is stored"})
	}

	if err := s.userStore.PromoteRefreshTokenByUserID(c.Request().Context(), session.UserID); err != nil {
		return c.String(http.StatusInternalServerError, "failed to promote refresh token")
	}

	tokens, err := s.tokenPayloadForUser(c, session.UserID)
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	tokens["message"] = "refresh token promoted to active access token"
	tokens["previousAccessToken"] = accessToken
	return c.JSON(http.StatusOK, tokens)
}

func (s *Server) handleGroupDetailPage(c echo.Context) error {
	session, ok := s.readSession(c)
	if !ok {
		return c.Redirect(http.StatusSeeOther, "/signin")
	}

	groupID := strings.TrimSpace(c.Param("groupID"))
	if groupID == "" {
		return c.String(http.StatusNotFound, "group not found")
	}

	client, shouldClearSession, err := s.spondClientForSession(c, session)
	if err != nil {
		if shouldClearSession {
			s.clearSession(c)
			return c.Redirect(http.StatusSeeOther, "/signin?error=session")
		}

		return c.String(http.StatusInternalServerError, "failed to initialize spond client")
	}

	groups, err := client.FetchGroups(c.Request().Context())
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	group, found := findGroupByID(groups, groupID)
	if !found {
		return c.String(http.StatusNotFound, groupNotFoundError(groupID))
	}

	data := groupDetailPageData{Group: buildGroupDetailData(group)}
	return s.renderTemplate(c, "group.html", data)
}

func (s *Server) handleLogoutPost(c echo.Context) error {
	s.clearSession(c)
	return c.Redirect(http.StatusSeeOther, "/signin")
}

func (s *Server) renderTemplate(c echo.Context, name string, data any) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return s.templates.ExecuteTemplate(c.Response(), name, data)
}

func (s *Server) Echo() *echo.Echo {
	return s.e
}

func (s *Server) Start(addr string) error {
	return s.e.Start(addr)
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}

	return sub
}

func normalizeFilterList(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		if _, exists := seen[trimmed]; exists {
			continue
		}

		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}

	return items
}

func parseCompactFilterValues(queryParams map[string][]string, names ...string) []string {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, queryParams[name]...)
	}

	values := make([]string, 0, len(parts))
	for _, part := range parts {
		for _, split := range strings.Split(part, ",") {
			trimmed := strings.TrimSpace(split)
			if trimmed == "" {
				continue
			}
			values = append(values, trimmed)
		}
	}

	return normalizeFilterList(values)
}

func parseBoolQueryParam(value string, defaultValue bool) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return defaultValue
	}

	switch trimmed {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func (s *Server) addSyncStatusToAccountPageData(c echo.Context, userID uint, data *accountPageData) {
	data.SyncEnabled = s.syncStore != nil
	if !data.SyncEnabled {
		data.LastSyncLabel = "Sync unavailable"
		data.NextSyncLabel = "Sync unavailable"
		return
	}

	lastSyncedAt, err := s.syncStore.LastSyncedAtByUserID(c.Request().Context(), userID)
	if err != nil {
		data.LastSyncLabel = "Sync status unavailable"
		data.NextSyncLabel = "Sync status unavailable"
		return
	}

	if lastSyncedAt == nil {
		nextSync := time.Now().UTC().Add(s.effectiveSyncInterval())
		data.LastSyncLabel = "Not synced yet"
		data.NextSyncISO = nextSync.Format(time.RFC3339)
		data.NextSyncLabel = nextSync.Format("2006-01-02 15:04 UTC")
		return
	}

	data.LastSyncISO = lastSyncedAt.UTC().Format(time.RFC3339)
	data.LastSyncLabel = lastSyncedAt.UTC().Format("2006-01-02 15:04 UTC")

	nextSync := lastSyncedAt.UTC().Add(s.effectiveSyncInterval())
	data.NextSyncISO = nextSync.Format(time.RFC3339)
	data.NextSyncLabel = nextSync.Format("2006-01-02 15:04 UTC")
}

func (s *Server) effectiveSyncInterval() time.Duration {
	if s.cfg.SyncInterval > 0 {
		return s.cfg.SyncInterval
	}

	return 5 * time.Minute
}

func chooseTokenForImmediateSync(accessToken string, accessExpires *time.Time, refreshToken string, refreshExpires *time.Time, now time.Time) (string, error) {
	const expirySkew = 30 * time.Second

	accessToken = strings.TrimSpace(accessToken)
	refreshToken = strings.TrimSpace(refreshToken)

	accessValid := accessToken != "" && (accessExpires == nil || accessExpires.After(now.Add(expirySkew)))
	if accessValid {
		return accessToken, nil
	}

	refreshValid := refreshToken != "" && (refreshExpires == nil || refreshExpires.After(now.Add(expirySkew)))
	if refreshValid {
		return refreshToken, nil
	}

	if accessToken == "" && refreshToken == "" {
		return "", fmt.Errorf("no usable spond token is stored")
	}

	if accessExpires != nil && accessExpires.Before(now.Add(expirySkew)) {
		if refreshExpires != nil && refreshExpires.Before(now.Add(expirySkew)) {
			return "", fmt.Errorf("access and refresh tokens are expired")
		}

		if refreshToken == "" {
			return "", fmt.Errorf("access token expired and no refresh token stored")
		}
	}

	if accessToken != "" {
		return accessToken, nil
	}

	return "", fmt.Errorf("no usable spond token is available")
}

func (s *Server) spondClientForSession(c echo.Context, session authSession) (*spond.Client, bool, error) {
	if session.UserID == 0 {
		return nil, true, fmt.Errorf("missing user id in session")
	}

	token, err := s.userStore.TokenByUserID(c.Request().Context(), session.UserID)
	if err != nil {
		return nil, true, fmt.Errorf("load user token: %w", err)
	}

	client, err := spond.New(s.cfg.SpondBaseURL)
	if err != nil {
		return nil, false, err
	}
	client.SetToken(token)

	return client, false, nil
}

func (s *Server) tokenPayloadForUser(c echo.Context, userID uint) (map[string]any, error) {
	accessToken, accessExpires, refreshToken, refreshExpires, err := s.userStore.TokenMetadataByUserID(c.Request().Context(), userID)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"accessToken":           accessToken,
		"refreshToken":          refreshToken,
		"accessTokenExpiresAt":  nil,
		"refreshTokenExpiresAt": nil,
	}

	if accessExpires != nil {
		payload["accessTokenExpiresAt"] = accessExpires.UTC().Format(time.RFC3339)
	}

	if refreshExpires != nil {
		payload["refreshTokenExpiresAt"] = refreshExpires.UTC().Format(time.RFC3339)
	}

	return payload, nil
}
