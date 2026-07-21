package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"code.p-fruck.eu/spond-webcal/internal/config"
	"code.p-fruck.eu/spond-webcal/internal/spond"
)

type Server struct {
	e         *echo.Echo
	templates *template.Template
	cfg       config.Config
}

func NewServer(cfg config.Config, caldavHandler http.Handler) (*Server, error) {
	if strings.TrimSpace(cfg.CookieSecret) == "" {
		return nil, fmt.Errorf("SPOND_WEBCAL_COOKIE_SECRET is required")
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

	s := &Server{e: e, templates: templates, cfg: cfg}

	e.GET("/", s.handleEventsPage)
	e.GET("/signin", s.handleSigninPage)
	e.POST("/signin", s.handleSigninPost)
	e.GET("/profile", s.handleProfilePage)
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

	client, err := spond.New(s.cfg.SpondBaseURL)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to initialize spond client")
	}
	client.SetToken(session.Token)

	events, err := client.FetchEvents(c.Request().Context(), 100)
	if err != nil {
		s.clearSession(c)
		return c.Redirect(http.StatusSeeOther, "/signin?error=session")
	}

	upcomingEvents, pastEvents := buildEventViewData(events, session.ActorIDs, time.Now().UTC())
	data := accountPageData{
		Name:           session.Name,
		Email:          session.Email,
		UpcomingEvents: upcomingEvents,
		PastEvents:     pastEvents,
	}

	return s.renderTemplate(c, "account.html", data)
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

	session := authSession{
		Token:      client.Token(),
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

	data := profilePageData{Name: session.Name, Email: session.Email}
	data.ProfileID = session.ProfileID
	data.GroupCount = session.GroupCount
	data.ActorIDs = session.ActorIDs
	return s.renderTemplate(c, "profile.html", data)
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
