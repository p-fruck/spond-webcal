package web

import (
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
}

func NewServer(cfg config.Config, caldavHandler http.Handler) *Server {
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

	e.GET("/", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		data := map[string]string{
			"Error": "",
		}

		if c.QueryParam("error") != "" {
			data["Error"] = "Login failed. Please check your credentials and try again."
		}

		return templates.ExecuteTemplate(c.Response(), "signin.html", data)
	})

	e.POST("/signin", func(c echo.Context) error {
		email := strings.TrimSpace(c.FormValue("email"))
		password := c.FormValue("password")
		if email == "" || password == "" {
			return c.Redirect(http.StatusSeeOther, "/?error=missing")
		}

		client, err := spond.New(cfg.SpondBaseURL)
		if err != nil {
			return c.String(http.StatusInternalServerError, "failed to initialize spond client")
		}

		if err := client.Login(c.Request().Context(), email, password); err != nil {
			return c.Redirect(http.StatusSeeOther, "/?error=login")
		}

		profile, err := client.FetchProfile(c.Request().Context())
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/?error=profile")
		}

		groups, err := client.FetchGroups(c.Request().Context())
		if err != nil {
			groups = nil
		}

		events, err := client.FetchEvents(c.Request().Context(), 100)
		if err != nil {
			events = nil
		}

		fullName := fullNameFromProfile(profile.FirstName, profile.LastName, email)

		profileEmail := email
		if profile.Email != nil {
			profileEmail = string(*profile.Email)
		}

		actorIDs := responseActorIDs(profile, profileEmail, groups)
		upcomingEvents, pastEvents := buildEventViewData(events, actorIDs, time.Now().UTC())

		data := accountPageData{
			Name:           fullName,
			Email:          profileEmail,
			UpcomingEvents: upcomingEvents,
			PastEvents:     pastEvents,
		}

		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		return templates.ExecuteTemplate(c.Response(), "account.html", data)
	})

	e.Any("/caldav", echo.WrapHandler(caldavHandler))
	e.Any("/caldav/*", echo.WrapHandler(caldavHandler))

	return &Server{e: e, templates: templates}
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
