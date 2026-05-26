package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"code.p-fruck.eu/spond-webcal/internal/caldav"
	"code.p-fruck.eu/spond-webcal/internal/config"
)

type Server struct {
	e *echo.Echo
}

func NewServer(cfg config.Config) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Any("/.well-known/caldav", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/caldav")
	})

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"name":       "spond-webcal",
			"status":     "starting",
			"listenAddr": cfg.Addr,
		})
	})

	caldavHandler := caldav.NewHandler()
	e.Any("/caldav", echo.WrapHandler(caldavHandler))
	e.Any("/caldav/*", echo.WrapHandler(caldavHandler))

	return &Server{e: e}
}

func (s *Server) Echo() *echo.Echo {
	return s.e
}

func (s *Server) Start(addr string) error {
	return s.e.Start(addr)
}
