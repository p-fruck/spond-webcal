package web

import "embed"

//go:embed templates/*.html static/*.css
var staticFS embed.FS

//go:embed templates/*.html
var templatesFS embed.FS
