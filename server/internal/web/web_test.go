package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	site := fstest.MapFS{
		".gitkeep":             {},
		"index.html":           {Data: []byte("<html>app</html>")},
		"manifest.webmanifest": {Data: []byte("{}")},
		"_app/immutable/a.js":  {Data: []byte("js")},
		"service-worker.js":    {Data: []byte("sw")},
	}
	tests := []struct {
		name, method, path string
		status             int
		body, contentType  string
		cache              string
	}{
		{"root", "GET", "/", 200, "<html>app</html>", "text/html", "no-cache"},
		{"client route", "GET", "/devices/123", 200, "<html>app</html>", "text/html", "no-cache"},
		{"index by name", "GET", "/index.html", 200, "<html>app</html>", "text/html", "no-cache"},
		{"hashed asset", "GET", "/_app/immutable/a.js", 200, "js", "javascript", "immutable"},
		{"service worker", "GET", "/service-worker.js", 200, "sw", "javascript", "no-cache"},
		{"manifest", "GET", "/manifest.webmanifest", 200, "{}", "application/manifest+json", "no-cache"},
		{"missing asset", "GET", "/_app/immutable/gone.js", 404, "not_found", "application/json", "no-cache"},
		{"dotfile is not served", "GET", "/.gitkeep", 404, "not_found", "", ""},
		{"traversal", "GET", "/../../web.go", 404, "not_found", "", ""},
		{"post", "POST", "/", 404, "not_found", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(site).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			h := rec.Header()
			if rec.Code != tt.status || !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("got %d %q", rec.Code, rec.Body)
			}
			if !strings.Contains(h.Get("Content-Type"), tt.contentType) || !strings.Contains(h.Get("Cache-Control"), tt.cache) {
				t.Errorf("headers = %v", h)
			}
			if tt.method == http.MethodGet && !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Error("page can be framed")
			}
		})
	}
}

func TestNotBuilt(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d", rec.Code)
	}
}
