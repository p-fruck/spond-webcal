package caldav

import (
	"net/http"

	"golang.org/x/net/webdav"
)

func NewHandler() http.Handler {
	return &webdav.Handler{
		Prefix:     "/caldav",
		FileSystem: webdav.NewMemFS(),
		LockSystem: webdav.NewMemLS(),
	}
}
