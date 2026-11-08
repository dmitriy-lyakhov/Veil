package server

// Админ-панель в отдельном процессе (контейнер veil-admin, «veil-server admin»).
//
// veil-server по-прежнему решает, кто пришёл из туннеля: запросы /admin от клиентов VPN он проксирует
// в процесс панели (Admin.Mode = container), добавляя адрес клиента в туннеле и общий ключ (файл
// admin.key рядом с сокетом управления). Процесс панели хранит сессии, проверяет вход и роли, а всё,
// что касается сервера, спрашивает у veil-server через сокет управления (пути admin-front/…).
// Без ключа или без панели /admin неотличим от несуществующей страницы.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"veil/internal/config"
)

const (
	adminKeyHeader  = "X-Veil-Admin-Key"
	adminPeerHeader = "X-Veil-Peer-IP"
	adminGenHeader  = "X-Veil-Admin-Gen"
	adminKeyFile    = "admin.key"
	adminFrontCtl   = "admin-front/"
)

// AdminKeyPath — файл общего ключа панели рядом с сокетом управления.
func AdminKeyPath(controlSocket string) string {
	return filepath.Join(filepath.Dir(controlSocket), adminKeyFile)
}

func newAdminKey() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// WriteAdminKey сохраняет ключ, которым veil-server подписывает запросы к отдельной панели.
func (s *Server) WriteAdminKey(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s.adminKey+"\n"), 0o600)
}

// routeAdmin — /admin из туннеля: в этом процессе, в отдельной панели или 404.
func (s *Server) routeAdmin(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	mode, upstream := s.cfg.Admin.Mode, s.cfg.Admin.Upstream
	s.mu.Unlock()
	switch mode {
	case config.AdminOff:
		notFound(w, r)
	case config.AdminContainer:
		s.proxyAdminFront(w, r, upstream)
	default:
		s.serveAdmin(w, r)
	}
}

func (s *Server) proxyAdminFront(w http.ResponseWriter, r *http.Request, upstream string) {
	if upstream == "" {
		upstream = config.DefaultAdminUpstream
	}
	target, err := url.Parse(upstream)
	if err != nil || target.Host == "" {
		notFound(w, r)
		return
	}
	peer := ""
	if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		peer = ap.Addr().Unmap().String()
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host // проверка Origin в панели сравнивает с исходным Host
			for k := range pr.Out.Header {
				if strings.HasPrefix(strings.ToLower(k), "x-veil-") {
					pr.Out.Header.Del(k)
				}
			}
			pr.Out.Header.Set(adminKeyHeader, s.adminKey)
			pr.Out.Header.Set(adminPeerHeader, peer)
		},
		// панель не поднята или недоступна — как будто её нет
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) { notFound(w, r) },
	}
	rp.ServeHTTP(w, r)
}

// SetAdminMode меняет режим веб-панели: "container" (отдельный процесс), "off" (нет, 404) или "builtin".
func (s *Server) SetAdminMode(mode string) error {
	switch mode {
	case config.AdminContainer, config.AdminOff:
	case "builtin", config.AdminBuiltin:
		mode = config.AdminBuiltin
	default:
		return fmt.Errorf("неизвестный режим панели %q", mode)
	}
	s.mu.Lock()
	s.cfg.Admin.Mode = mode
	err := s.saveLocked()
	s.mu.Unlock()
	return err
}

// adminGen — отпечаток пароля панели: при смене пароля сессии отдельной панели становятся недействительными.
func (s *Server) adminGen() string {
	s.mu.Lock()
	h := s.cfg.Admin.PasswordHash
	s.mu.Unlock()
	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:8])
}

// handleAdminFront — запросы процесса панели через сокет управления.
func (s *Server) handleAdminFront(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set(adminGenHeader, s.adminGen())
	switch {
	case path == "whois":
		var role string
		var isPeer bool
		if ip, err := netip.ParseAddr(r.URL.Query().Get("ip")); err == nil {
			fake := &http.Request{RemoteAddr: netip.AddrPortFrom(ip, 1).String()}
			role, isPeer = s.peerRole(fake)
		}
		writeJSON(w, http.StatusOK, map[string]any{"role": role, "peer": isPeer})
	case path == "auth" && r.Method == http.MethodPost:
		var req struct{ Login, Password string }
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBody)).Decode(&req)
		creds := s.adminCredentials()
		user := creds.User
		if user == "" {
			user = defaultAdminUser
		}
		userOK := subtle.ConstantTimeCompare([]byte(req.Login), []byte(user)) == 1
		hash := []byte(creds.PasswordHash)
		if len(hash) == 0 {
			hash = s.admin.dummy
		}
		passOK := bcrypt.CompareHashAndPassword(hash, []byte(req.Password)) == nil
		writeJSON(w, http.StatusOK, map[string]bool{"ok": userOK && passOK && creds.PasswordHash != ""})
	case path == "event" && r.Method == http.MethodPost:
		var e Event
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBody)).Decode(&e) == nil && e.Text != "" {
			s.Event(e.Kind, e.Actor, e.IP, e.Text)
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
	case path == "login-page":
		msg := ""
		if k := r.URL.Query().Get("err"); k == "too_many" || k == "bad_login" {
			msg = s.web.site.T(k)
		}
		s.renderLogin(w, msg)
	case strings.HasPrefix(path, "api/"):
		api := strings.TrimPrefix(path, "api/")
		if node := r.URL.Query().Get("node"); node != "" {
			s.proxyAdmin(w, r, api, strings.Split(node, ">"))
			return
		}
		s.handleAPI(w, r, api, false)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// ---------- процесс панели ----------

// AdminFront — веб-панель в отдельном процессе; всё о сервере — через сокет управления veil-server.
type AdminFront struct {
	socket  string
	keyFile string
	log     *slog.Logger
	st      *adminState
	hc      *http.Client
	docker  *dockerClient // nil — сокет Docker не смонтирован

	mu   sync.Mutex
	key  string
	gens map[[32]byte]string // сессия → отпечаток пароля при входе
}

func NewAdminFront(socket string, log *slog.Logger) *AdminFront {
	return &AdminFront{
		socket: socket, keyFile: AdminKeyPath(socket), log: log, st: newAdminState(), gens: map[[32]byte]string{}, docker: newDockerClient(),
		hc: &http.Client{Timeout: ctlTimeout, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		}}},
	}
}

// keyOK сверяет ключ запроса с файлом (перечитывая его, если veil-server перезапустился с новым ключом).
func (a *AdminFront) keyOK(got string) bool {
	if got == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for attempt := 0; attempt < 2; attempt++ {
		if a.key != "" && subtle.ConstantTimeCompare([]byte(got), []byte(a.key)) == 1 {
			return true
		}
		b, err := os.ReadFile(a.keyFile)
		if err != nil {
			return false
		}
		a.key = strings.TrimSpace(string(b))
	}
	return false
}

// ctl — запрос к veil-server через сокет управления. Кто действует, передаётся для журнала событий.
func (a *AdminFront) ctl(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://veil/"+adminFrontCtl+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if who, ok := ctx.Value(actorKey{}).([2]string); ok {
		req.Header.Set(actorHeader, who[0])
		req.Header.Set(actorIPHeader, who[1])
	}
	return a.hc.Do(req)
}

type actorKey struct{}

// event — запись в журнал veil-server от имени процесса панели.
func (a *AdminFront) event(kind, actor, ip, text string) {
	b, _ := json.Marshal(Event{Kind: kind, Actor: actor, IP: ip, Text: text})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if resp, err := a.ctl(ctx, http.MethodPost, "event", bytes.NewReader(b)); err == nil {
		resp.Body.Close()
	}
}

func (a *AdminFront) ctlJSON(ctx context.Context, method, path string, in, out any) (string, error) {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	resp, err := a.ctl(ctx, method, path, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("veil-server: %s", resp.Status)
	}
	return resp.Header.Get(adminGenHeader), json.NewDecoder(resp.Body).Decode(out)
}

func (a *AdminFront) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.keyOK(r.Header.Get(adminKeyHeader)) {
		notFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	ctx := r.Context()
	ip := r.Header.Get(adminPeerHeader)
	path := strings.TrimSuffix(r.URL.Path, "/")
	var who struct {
		Role string `json:"role"`
		Peer bool   `json:"peer"`
	}
	gen, err := a.ctlJSON(ctx, http.MethodGet, "whois?ip="+url.QueryEscape(ip), nil, &who)
	if err != nil {
		a.log.Warn("veil-server unavailable", "err", err)
		notFound(w, r)
		return
	}
	if who.Peer && who.Role == RoleUser {
		notFound(w, r)
		return
	}
	if path == adminPrefix+"/login" && r.Method == http.MethodPost {
		a.login(w, r, ip)
		return
	}
	sessKey, authed := a.st.valid(r)
	if authed {
		a.mu.Lock()
		authed = a.gens[sessKey] == gen
		a.mu.Unlock()
	}
	if who.Peer && who.Role == RoleAdmin {
		authed = true
	}
	if !authed {
		if path == adminPrefix && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			a.loginPage(w, r, "")
			return
		}
		notFound(w, r)
		return
	}
	if r.Method == http.MethodPost && !sameOriginRequest(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	roleLogin := sessKey == [32]byte{}
	actor := defaultAdminUser
	if roleLogin {
		actor = "клиент-администратор"
	} else {
		a.st.touch(sessKey, ip, r.UserAgent())
	}
	ctx = context.WithValue(ctx, actorKey{}, [2]string{actor, ip})
	switch {
	case path == adminPrefix && r.Method == http.MethodGet:
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		_, _ = w.Write([]byte(panelHTML))
	case strings.HasPrefix(path, adminPrefix+"/fonts/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		serveAdminFont(w, strings.TrimPrefix(path, adminPrefix+"/fonts/"))
	case strings.HasPrefix(path, adminPrefix+"/api/"):
		api := strings.TrimPrefix(path, adminPrefix+"/api/")
		if serveAdminLocal(w, r, api, adminLocal{st: a.st, cur: sessKey, user: actor, ip: ip, roleUser: roleLogin, mode: "container", docker: a.docker,
			event: func(kind, text string) { a.event(kind, actor, ip, text) }}) {
			return
		}
		q := ""
		if r.URL.RawQuery != "" {
			q = "?" + r.URL.RawQuery
		}
		resp, err := a.ctl(ctx, r.Method, "api/"+api+q, http.MaxBytesReader(w, r.Body, maxAdminProxy))
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "сервер не отвечает"})
			return
		}
		defer resp.Body.Close()
		h.Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	case path == adminPrefix+"/logout" && r.Method == http.MethodPost:
		a.st.drop(sessKey)
		http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: adminPrefix, MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
	default:
		notFound(w, r)
	}
}

func (a *AdminFront) loginPage(w http.ResponseWriter, r *http.Request, errKey string) {
	resp, err := a.ctl(r.Context(), http.MethodGet, "login-page?err="+errKey, nil)
	if err != nil {
		notFound(w, r)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, resp.Body)
}

func (a *AdminFront) login(w http.ResponseWriter, r *http.Request, ip string) {
	now := time.Now()
	if a.st.blocked(ip, now) {
		a.loginPage(w, r, "too_many")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminBody)
	if err := r.ParseForm(); err != nil {
		a.loginPage(w, r, "bad_login")
		return
	}
	var res struct {
		OK bool `json:"ok"`
	}
	gen, err := a.ctlJSON(r.Context(), http.MethodPost, "auth", map[string]string{"login": r.PostFormValue("login"), "password": r.PostFormValue("password")}, &res)
	if err != nil || !res.OK {
		a.st.recordFailure(ip, now)
		a.log.Warn("admin login failed", "ip", ip)
		a.event("login", "—", ip, "Неудачный вход в панель")
		if a.st.blocked(ip, now) {
			a.event("login", "—", ip, "Вход с адреса заблокирован на 15 минут")
		}
		time.Sleep(loginFailDelay)
		a.loginPage(w, r, "bad_login")
		return
	}
	tok := a.st.newSession(now)
	if raw, err := hex.DecodeString(tok); err == nil {
		a.mu.Lock()
		a.gens[sha256.Sum256(raw)] = gen
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: tok, Path: adminPrefix, MaxAge: int(adminSessionTTL.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	a.log.Info("admin login", "ip", ip)
	if raw, err := hex.DecodeString(tok); err == nil {
		a.st.touch(sha256.Sum256(raw), ip, r.UserAgent())
	}
	a.event("login", defaultAdminUser, ip, "Вход в панель")
	http.Redirect(w, r, adminPrefix, http.StatusSeeOther)
}
