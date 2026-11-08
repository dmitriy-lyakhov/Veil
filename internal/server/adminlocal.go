package server

// Запросы панели, которые обслуживает сам процесс панели (встроенная в veil-server или отдельный
// контейнер veil-admin): свои сессии, блокировки входа, «кто я», протоколы через Docker.
// Пути: /admin/api/admin/… — их не пересылают на другие узлы каскада.

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

const adminLocalPrefix = "admin/"

type sessMeta struct {
	ip   string
	ua   string
	last time.Time
}

// touch отмечает активность сессии панели (адрес и браузер — для страницы «Безопасность»).
func (a *adminState) touch(key [32]byte, ip, ua string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.meta == nil {
		a.meta = map[[32]byte]*sessMeta{}
	}
	m := a.meta[key]
	if m == nil {
		m = &sessMeta{}
		a.meta[key] = m
	}
	m.ip, m.ua, m.last = ip, ua, time.Now()
}

func sessID(key [32]byte) string { return hex.EncodeToString(key[:4]) }

type adminSessionInfo struct {
	ID      string    `json:"id"`
	IP      string    `json:"ip"`
	UA      string    `json:"ua"`
	Last    time.Time `json:"last"`
	Current bool      `json:"current"`
}

func (a *adminState) list(cur [32]byte) []adminSessionInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	out := []adminSessionInfo{}
	for k, exp := range a.sessions {
		if now.After(exp) {
			continue
		}
		info := adminSessionInfo{ID: sessID(k), Current: k == cur}
		if m := a.meta[k]; m != nil {
			info.IP, info.UA, info.Last = m.ip, m.ua, m.last
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Last.After(out[j].Last) })
	return out
}

// dropWhere завершает сессии, для которых match вернул true; возвращает их число.
func (a *adminState) dropWhere(match func(k [32]byte) bool) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for k := range a.sessions {
		if match(k) {
			delete(a.sessions, k)
			delete(a.meta, k)
			n++
		}
	}
	return n
}

type blockedInfo struct {
	IP    string    `json:"ip"`
	Fails int       `json:"fails"`
	Until time.Time `json:"until"`
}

func (a *adminState) blockedList(now time.Time) []blockedInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []blockedInfo{}
	for ip, at := range a.attempts {
		if now.Before(at.until) {
			out = append(out, blockedInfo{IP: ip, Fails: at.fails, Until: at.until})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Until.After(out[j].Until) })
	return out
}

func (a *adminState) unblock(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.attempts[ip]; !ok {
		return false
	}
	delete(a.attempts, ip)
	return true
}

// adminLocal — контекст запроса для локальных путей панели.
type adminLocal struct {
	st       *adminState
	cur      [32]byte // сессия этого запроса (нулевая — вход по роли клиента)
	user, ip string
	roleUser bool
	mode     string
	docker   *dockerClient // nil — Docker панели недоступен
	event    func(kind, text string)
}

// serveAdminLocal обслуживает /admin/api/admin/…; false — путь не локальный.
func serveAdminLocal(w http.ResponseWriter, r *http.Request, api string, l adminLocal) bool {
	if !strings.HasPrefix(api, adminLocalPrefix) {
		return false
	}
	p := strings.TrimPrefix(api, adminLocalPrefix)
	decode := func(v any) bool {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBody)).Decode(v); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный запрос"})
			return false
		}
		return true
	}
	fail := func(err error) { writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()}) }
	ok := func() { writeJSON(w, http.StatusOK, map[string]string{"ok": "1"}) }
	get, post := r.Method == http.MethodGet, r.Method == http.MethodPost
	switch {
	case get && p == "me":
		writeJSON(w, http.StatusOK, map[string]any{"user": l.user, "ip": l.ip, "role_login": l.roleUser, "mode": l.mode})
	case get && p == "sessions":
		writeJSON(w, http.StatusOK, map[string]any{"sessions": l.st.list(l.cur)})
	case post && p == "sessions/kill":
		var req struct {
			ID        string `json:"id"`
			AllOthers bool   `json:"all_others"`
		}
		if !decode(&req) {
			return true
		}
		n := l.st.dropWhere(func(k [32]byte) bool {
			if req.AllOthers {
				return k != l.cur
			}
			return req.ID != "" && sessID(k) == req.ID && k != l.cur
		})
		if n == 0 {
			fail(errors.New("сессия не найдена (свою текущую завершайте кнопкой «Выйти»)"))
			return true
		}
		l.event("login", "Завершено сессий панели: "+itoa(n))
		ok()
	case get && p == "blocked":
		writeJSON(w, http.StatusOK, map[string]any{"blocked": l.st.blockedList(time.Now())})
	case post && p == "unblock":
		var req struct {
			IP string `json:"ip"`
		}
		if !decode(&req) {
			return true
		}
		if !l.st.unblock(req.IP) {
			fail(errors.New("адрес не заблокирован"))
			return true
		}
		l.event("login", "Разблокирован вход с "+req.IP)
		ok()
	case strings.HasPrefix(p, "protocols"):
		l.serveProtocols(w, r, strings.TrimPrefix(strings.TrimPrefix(p, "protocols"), "/"), decode, fail, ok)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return true
}

func (l adminLocal) serveProtocols(w http.ResponseWriter, r *http.Request, p string, decode func(any) bool, fail func(error), ok func()) {
	if l.docker == nil {
		if r.Method == http.MethodGet && p == "" {
			writeJSON(w, http.StatusOK, map[string]any{"docker": false, "containers": []any{}})
			return
		}
		fail(errors.New("панели не дан доступ к Docker: управляйте протоколами из приложения"))
		return
	}
	switch {
	case r.Method == http.MethodGet && p == "":
		list, err := l.docker.containers(r.Context())
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"docker": true, "containers": list})
	case r.Method == http.MethodPost && p == "restart":
		var req struct {
			Name string `json:"name"`
		}
		if !decode(&req) {
			return
		}
		if err := l.docker.restart(r.Context(), req.Name); err != nil {
			fail(err)
			return
		}
		l.event("system", "Перезапущен контейнер "+req.Name)
		ok()
	case r.Method == http.MethodPost && p == "client":
		var req struct {
			Name   string `json:"name"`
			Client string `json:"client"`
		}
		if !decode(&req) {
			return
		}
		text, file, err := l.docker.addClient(r.Context(), req.Name, req.Client)
		if err != nil {
			fail(err)
			return
		}
		l.event("clients", "Клиент «"+req.Client+"» выдан в "+req.Name)
		writeJSON(w, http.StatusOK, map[string]string{"text": text, "file": file})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
