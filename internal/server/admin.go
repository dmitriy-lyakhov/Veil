package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"veil/internal/config"
)

const (
	adminPrefix      = "/admin"
	adminCookie      = "sid"
	adminSessionTTL  = 12 * time.Hour
	adminCSRFHeader  = "X-Panel-Request"
	loginMaxFails    = 5
	loginWindow      = 15 * time.Minute
	loginFailDelay   = 400 * time.Millisecond
	maxAdminBody     = 16 << 10
	defaultAdminUser = "admin"
)

type loginAttempts struct {
	fails int
	first time.Time
	until time.Time
}

type adminState struct {
	dummy    []byte
	mu       sync.Mutex
	sessions map[[32]byte]time.Time
	attempts map[string]*loginAttempts
}

func newAdminState() *adminState {
	var junk [16]byte
	_, _ = rand.Read(junk[:])
	dummy, _ := bcrypt.GenerateFromPassword(junk[:], bcrypt.DefaultCost)
	return &adminState{dummy: dummy, sessions: map[[32]byte]time.Time{}, attempts: map[string]*loginAttempts{}}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *adminState) blocked(ip string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	at := a.attempts[ip]
	return at != nil && now.Before(at.until)
}

func (a *adminState) recordFailure(ip string, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, v := range a.attempts {
		if now.Sub(v.first) > loginWindow && now.After(v.until) {
			delete(a.attempts, k)
		}
	}
	at := a.attempts[ip]
	if at == nil || now.Sub(at.first) > loginWindow {
		at = &loginAttempts{first: now}
		a.attempts[ip] = at
	}
	at.fails++
	if at.fails >= loginMaxFails {
		at.until = now.Add(loginWindow)
	}
}

func (a *adminState) newSession(now time.Time) string {
	var tok [32]byte
	_, _ = rand.Read(tok[:])
	a.mu.Lock()
	defer a.mu.Unlock()
	for k, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, k)
		}
	}
	a.sessions[sha256.Sum256(tok[:])] = now.Add(adminSessionTTL)
	return hex.EncodeToString(tok[:])
}

func (a *adminState) valid(r *http.Request) ([32]byte, bool) {
	c, err := r.Cookie(adminCookie)
	if err != nil {
		return [32]byte{}, false
	}
	raw, err := hex.DecodeString(c.Value)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	key := sha256.Sum256(raw)
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[key]
	return key, ok && time.Now().Before(exp)
}

func (a *adminState) dropAll() {
	a.mu.Lock()
	clear(a.sessions)
	a.mu.Unlock()
}

func (a *adminState) drop(key [32]byte) {
	a.mu.Lock()
	delete(a.sessions, key)
	a.mu.Unlock()
}

func (s *Server) adminCredentials() config.Admin {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Admin
}

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == adminPrefix+"/login" && r.Method == http.MethodPost {
		s.adminLogin(w, r)
		return
	}
	sessKey, authed := s.admin.valid(r)
	if !authed {
		if path == adminPrefix && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			s.renderLogin(w, "")
			return
		}
		notFound(w)
		return
	}
	if r.Method == http.MethodPost && !sameOriginRequest(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch {
	case path == adminPrefix && r.Method == http.MethodGet:
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:")
		_, _ = w.Write([]byte(panelHTML))
	case strings.HasPrefix(path, adminPrefix+"/api/"):
		s.handleAPI(w, r, strings.TrimPrefix(path, adminPrefix+"/api/"), false)
	case path == adminPrefix+"/logout" && r.Method == http.MethodPost:
		s.admin.drop(sessKey)
		http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: adminPrefix, MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
	default:
		notFound(w)
	}
}

func sameOriginRequest(r *http.Request) bool {
	if r.Header.Get(adminCSRFHeader) != "1" {
		return false
	}
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "https://"+r.Host
}

func (s *Server) renderLogin(w http.ResponseWriter, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.web.site.Login(errMsg))
}

func (s *Server) adminLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	now := time.Now()
	if s.admin.blocked(ip, now) {
		s.renderLogin(w, s.web.site.T("too_many"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminBody)
	if err := r.ParseForm(); err != nil {
		s.renderLogin(w, s.web.site.T("bad_login"))
		return
	}
	creds := s.adminCredentials()
	user := creds.User
	if user == "" {
		user = defaultAdminUser
	}
	userOK := subtle.ConstantTimeCompare([]byte(r.PostFormValue("login")), []byte(user)) == 1
	hash := []byte(creds.PasswordHash)
	if len(hash) == 0 {
		hash = s.admin.dummy
	}
	passOK := bcrypt.CompareHashAndPassword(hash, []byte(r.PostFormValue("password"))) == nil
	if !userOK || !passOK || creds.PasswordHash == "" {
		s.admin.recordFailure(ip, now)
		s.log.Warn("admin login failed", "ip", ip)
		time.Sleep(loginFailDelay)
		s.renderLogin(w, s.web.site.T("bad_login"))
		return
	}
	tok := s.admin.newSession(now)
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: tok, Path: adminPrefix, MaxAge: int(adminSessionTTL.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	s.log.Info("admin login", "ip", ip)
	http.Redirect(w, r, adminPrefix, http.StatusSeeOther)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
