package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRedirect(t *testing.T) {
	srv := HTTPRedirectServer()
	cases := []struct{ host, uri, want string }{
		{"203.0.113.7", "/admin?x=1", "https://203.0.113.7/admin?x=1"},
		{"203.0.113.7:80", "/", "https://203.0.113.7/"},
		{"example.com", "/a/b", "https://example.com/a/b"},
		{"[2001:db8::7]:80", "/", "https://[2001:db8::7]/"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.uri, nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently {
			t.Fatalf("%s: code %d", c.host, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Fatalf("%s: location %q, want %q", c.host, got, c.want)
		}
		if rec.Header().Get("Server") != "nginx" || !strings.Contains(rec.Body.String(), "301 Moved Permanently") {
			t.Fatalf("%s: not nginx-like: %v %q", c.host, rec.Header(), rec.Body.String())
		}
	}
}
