package web

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"code.p-fruck.eu/spond-webcal/internal/config"
)

type Server struct {
	e *echo.Echo
}

func NewServer(cfg config.Config) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"name":      "spond-webcal",
			"status":    "starting",
			"listenAddr": cfg.Addr,
		})
	})

	return &Server{e: e}
}

func (s *Server) Echo() *echo.Echo {
	return s.e
}

func (s *Server) Start(addr string) error {
	return s.e.Start(addr)
}