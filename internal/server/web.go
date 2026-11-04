package server

import (
	"context"
	"io"
	"net/http"

	"veil/internal/site"
)

const (
	serverHeader   = "nginx"
	maxContactBody = 8 << 10
	notFoundHTML   = "<html>\r\n<head><title>404 Not Found</title></head>\r\n<body>\r\n<center><h1>404 Not Found</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"
	robotsTXT      = "User-agent: *\nDisallow:\n"
)

type webSite struct {
	site    *site.Site
	index   []byte
	css     []byte
	favicon []byte
	thanks  []byte
}

func newWebSite(s *site.Site) *webSite {
	return &webSite{site: s, index: s.Index(), css: s.Stylesheet(), favicon: s.Favicon(), thanks: s.Thanks()}
}

func (ws *webSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	get := r.Method == http.MethodGet || r.Method == http.MethodHead
	switch {
	case get && (r.URL.Path == "/" || r.URL.Path == "/index.html"):
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Cache-Control", "max-age=600")
		_, _ = w.Write(ws.index)
	case get && r.URL.Path == "/assets/site.css":
		h.Set("Content-Type", "text/css; charset=utf-8")
		h.Set("Cache-Control", "max-age=86400")
		_, _ = w.Write(ws.css)
	case get && r.URL.Path == "/favicon.svg":
		h.Set("Content-Type", "image/svg+xml")
		h.Set("Cache-Control", "max-age=86400")
		_, _ = w.Write(ws.favicon)
	case get && r.URL.Path == "/robots.txt":
		h.Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(robotsTXT))
	case r.Method == http.MethodPost && r.URL.Path == "/contact":
		_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, maxContactBody))
		h.Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(ws.thanks)
	default:
		notFound(w)
	}
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(notFoundHTML))
}

func contextWithCancel(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithCancel(r.Context())
}
