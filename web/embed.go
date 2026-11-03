// Package web holds the single-page frontend, compiled into the binary so
// that the runtime image needs nothing but the binary itself.
package web

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed index.html app.js style.css favicon.svg
var assets embed.FS

// Handler serves the embedded frontend. index.html is served uncached so a
// new image is picked up immediately; the assets next to it have stable names
// and no content hash, so they are only cached briefly.
func Handler() http.Handler {
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=300")
		}
		files.ServeHTTP(w, r)
	})
}
