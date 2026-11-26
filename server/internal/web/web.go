// Package web serves the dashboard that is built into the binary. The build
// (web/build) is copied into dist/ before compiling; see server/Dockerfile.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the embedded dashboard.
func Handler() http.Handler {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // the directory is embedded above
	}
	return handler(root)
}

// handler serves root as a single-page app: unknown page paths get index.html.
func handler(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpx.Error(w, http.StatusNotFound, "not_found", "There's nothing at this address.", "")
			return
		}
		h := w.Header()
		// The page's own CSP is a <meta> tag; framing can only be refused here.
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cache-Control", "no-cache")

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		info, err := fs.Stat(root, name)
		isFile := err == nil && !info.IsDir() && !strings.HasPrefix(path.Base(name), ".")
		switch {
		case isFile && name != "index.html":
			if strings.HasPrefix(name, "_app/immutable/") {
				h.Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			if path.Ext(name) == ".webmanifest" {
				h.Set("Content-Type", "application/manifest+json")
			}
			files.ServeHTTP(w, r)
		case isFile || path.Ext(name) == "":
			// ServeFileFS would redirect "index.html", so serve the bytes.
			index, err := fs.ReadFile(root, "index.html")
			if err != nil {
				httpx.Error(w, http.StatusNotFound, "not_found", "The dashboard isn't part of this build.", "")
				return
			}
			h.Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(index)
		default:
			// A missing asset must not come back as HTML.
			httpx.Error(w, http.StatusNotFound, "not_found", "There's nothing at this address.", "")
		}
	})
}
