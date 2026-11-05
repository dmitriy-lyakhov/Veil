package server

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"veil/internal/site"
)

const (
	serverHeader   = "nginx"
	maxContactBody = 8 << 10
	notFoundHTML   = "<html>\r\n<head><title>404 Not Found</title></head>\r\n<body>\r\n<center><h1>404 Not Found</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"
	robotsTXT      = "User-agent: *\nDisallow:\n"
)

type webSite struct {
	site     *site.Site
	index    []byte
	css      []byte
	favicon  []byte
	thanks   []byte
	cover    atomic.Pointer[site.Tree] // сайт из репозитория; nil — отдаётся сгенерированный
	reserved func(string) bool         // пути, которые сайт из репозитория занять не может
}

func newWebSite(s *site.Site) *webSite {
	return &webSite{site: s, index: s.Index(), css: s.Stylesheet(), favicon: s.Favicon(), thanks: s.Thanks()}
}

func (ws *webSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if t := ws.cover.Load(); t != nil && (ws.reserved == nil || !ws.reserved(r.URL.Path)) {
		ws.serveCover(t, w, r)
		return
	}
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
		notFound(w, r)
	}
}

// msiePadding — блок, который nginx добавляет к страницам ошибок для MSIE и Chrome
// (msie_padding on), чтобы они не подменяли страницу своей «дружелюбной».
const msiePadding = "<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n" +
	"<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n" +
	"<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n" +
	"<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n" +
	"<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n" +
	"<!-- a padding to disable MSIE and Chrome friendly error page -->\r\n"

func notFound(w http.ResponseWriter, r *http.Request) {
	body := notFoundHTML
	if ua := r.UserAgent(); strings.Contains(ua, "Chrome/") || strings.Contains(ua, "MSIE ") {
		body += msiePadding
	}
	h := w.Header()
	h.Set("Content-Type", "text/html")
	if r.ProtoMajor == 1 {
		h.Set("Connection", "keep-alive") // nginx всегда сообщает его для HTTP/1.1
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(body))
}

func contextWithCancel(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithCancel(r.Context())
}

const movedHTML = "<html>\r\n<head><title>301 Moved Permanently</title></head>\r\n<body>\r\n<center><h1>301 Moved Permanently</h1></center>\r\n<hr><center>nginx</center>\r\n</body>\r\n</html>\r\n"

// HTTPRedirectServer — порт 80: как у обычного сайта, всё перенаправляется на https
// (nginx: return 301 https://$host$request_uri). Без него порт 80 отвечает отказом
// в соединении, чего у настоящих сайтов почти не бывает.
func HTTPRedirectServer() *http.Server {
	return &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
				host = "[" + host + "]" // IPv6-литерал
			}
			if host == "" {
				w.Header().Set("Server", serverHeader)
				notFound(w, r)
				return
			}
			h := w.Header()
			h.Set("Server", serverHeader)
			h.Set("Content-Type", "text/html")
			h.Set("Location", "https://"+host+r.URL.RequestURI())
			if r.ProtoMajor == 1 {
				h.Set("Connection", "keep-alive")
			}
			w.WriteHeader(http.StatusMovedPermanently)
			if r.Method != http.MethodHead {
				_, _ = w.Write([]byte(movedHTML))
			}
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
}

// serveCover отдаёт сайт из репозитория: файл из снимка, иначе — тот же 404, что и у сгенерированного сайта.
func (ws *webSite) serveCover(t *site.Tree, w http.ResponseWriter, r *http.Request) {
	get := r.Method == http.MethodGet || r.Method == http.MethodHead
	if t.Serve(w, r) {
		return
	}
	switch {
	case get && r.URL.Path == "/robots.txt":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(robotsTXT))
	case get && t.IsDir(r.URL.Path): // как nginx: каталог без «/» — перенаправление на каталог со «/»
		h := w.Header()
		h.Set("Content-Type", "text/html")
		h.Set("Location", "https://"+r.Host+r.URL.EscapedPath()+"/"+func() string {
			if r.URL.RawQuery != "" {
				return "?" + r.URL.RawQuery
			}
			return ""
		}())
		if r.ProtoMajor == 1 {
			h.Set("Connection", "keep-alive")
		}
		w.WriteHeader(http.StatusMovedPermanently)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte(movedHTML))
		}
	default:
		notFound(w, r)
	}
}
