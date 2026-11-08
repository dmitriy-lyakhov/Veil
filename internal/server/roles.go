package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"veil/internal/config"
	"veil/internal/provision"
)

// Роли клиентов. Пустая роль — клиент создан до появления ролей: доступ к панели по паролю, как раньше.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

func normalizeRole(r string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "", RoleUser:
		return RoleUser, nil
	case RoleAdmin:
		return RoleAdmin, nil
	}
	return "", errors.New("роль клиента: user или admin")
}

// peerRole определяет клиента по адресу внутри туннеля и возвращает его роль.
func (s *Server) peerRole(r *http.Request) (role string, isPeer bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	ip = ip.Unmap()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.cfg.Peers {
		for _, a := range []string{p.Address4, p.Address6} {
			if pf, err := netip.ParsePrefix(a); err == nil && pf.Contains(ip) {
				return p.Role, true
			}
		}
	}
	return "", false
}

// SetClientRole задаёт роль клиента.
func (s *Server) SetClientRole(name, role string) error {
	role, err := normalizeRole(role)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.peerIndexLocked(name)
	if i < 0 {
		return errors.New("клиент не найден")
	}
	s.cfg.Peers[i].Role = role
	return s.saveLocked()
}

// SetRelayAdmin разрешает или запрещает узлу, которому выдан ключ каскада, администрировать этот сервер.
func (s *Server) SetRelayAdmin(name string, admin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.cfg.Relays {
		if s.cfg.Relays[i].Name == name {
			s.cfg.Relays[i].Admin = admin
			return s.saveLocked()
		}
	}
	return errors.New("ключ не найден")
}

func (s *Server) relayAdmin(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rl := range s.cfg.Relays {
		if rl.Name == name {
			return rl.Admin
		}
	}
	return false
}

// adminURL — адрес админки внутри туннеля; кладётся в профиль клиента-администратора.
func (s *Server) adminURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return provision.AdminURL(s.cfg)
}

// applyRole выставляет роль выданному клиенту и дописывает в его профиль роль и адрес админки.
func (s *Server) applyRole(prof *config.Profile, name, role string) error {
	role, err := normalizeRole(role)
	if err != nil {
		return err
	}
	if err := s.SetClientRole(name, role); err != nil {
		return err
	}
	if prof != nil {
		prof.Role = role
		if role == RoleAdmin {
			prof.AdminURL = s.adminURL()
		}
	}
	return nil
}

// ---------- администрирование каскада из одной панели ----------

const maxAdminProxy = 1 << 20

// recorder — минимальный ResponseWriter для выполнения запроса API внутри процесса.
type recorder struct {
	code int
	hdr  http.Header
	body bytes.Buffer
}

func (r *recorder) Header() http.Header         { return r.hdr }
func (r *recorder) WriteHeader(c int)           { r.code = c }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }

// runAdminLocal выполняет запрос API админки на этом сервере.
func (s *Server) runAdminLocal(ctx context.Context, method, path string, body []byte) (int, []byte) {
	req, err := http.NewRequestWithContext(ctx, method, adminPrefix+"/api/"+path, bytes.NewReader(body))
	if err != nil {
		return http.StatusBadRequest, []byte(`{"error":"bad request"}`)
	}
	rec := &recorder{code: http.StatusOK, hdr: http.Header{}}
	s.handleAPI(rec, req, path, true)
	return rec.code, rec.body.Bytes()
}

// proxyAdmin передаёт запрос админки узлу каскада по цепочке узлов route (имена узлов друг за другом).
func (s *Server) proxyAdmin(w http.ResponseWriter, r *http.Request, path string, route []string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAdminProxy))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "некорректный запрос"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), relayCallTime)
	defer cancel()
	code, raw, err := s.forwardAdmin(ctx, route, r.Method, path, body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(raw)
}

func (s *Server) forwardAdmin(ctx context.Context, route []string, method, path string, body []byte) (int, []byte, error) {
	if len(route) == 0 {
		return 0, nil, errors.New("не указан узел")
	}
	if len(route) > maxRouteLen {
		return 0, nil, errors.New("слишком длинная цепочка узлов")
	}
	d, err := s.upstreamDialer(route[0], "")
	if err != nil {
		return 0, nil, err
	}
	var resp relayResponse
	req := relayRequest{Op: relayOpAdmin, Route: route[1:], Method: method, Path: path, Body: body}
	if err := d.Call(ctx, relayAPISuffix, req, &resp); err != nil {
		return 0, nil, err
	}
	if resp.Status == 0 {
		resp.Status = http.StatusOK
	}
	return resp.Status, resp.Raw, nil
}

// relayAdminOp выполняет запрос админки по просьбе узла каскада, которому это разрешено ключом.
func (s *Server) relayAdminOp(ctx context.Context, from string, req relayRequest) (relayResponse, error) {
	if !s.relayAdmin(from) {
		return relayResponse{}, errors.New("этому узлу не разрешено администрировать сервер")
	}
	if len(req.Body) > maxAdminProxy {
		return relayResponse{}, errors.New("запрос слишком большой")
	}
	if len(req.Route) > 0 {
		code, raw, err := s.forwardAdmin(ctx, req.Route, req.Method, req.Path, req.Body)
		return relayResponse{Status: code, Raw: raw}, err
	}
	code, raw := s.runAdminLocal(ctx, req.Method, req.Path, req.Body)
	return relayResponse{Status: code, Raw: json.RawMessage(raw)}, nil
}

// CascadeNode — узел каскада, доступный для администрирования из этой панели.
type CascadeNode struct {
	Name  string   `json:"name"`  // пусто — этот сервер
	Route []string `json:"route"` // цепочка имён узлов от этого сервера
}

// CascadeNodes перечисляет этот сервер и все узлы дальше по каскаду (поиск в ширину по ключам «следующих узлов»).
func (s *Server) CascadeNodes(ctx context.Context) []CascadeNode {
	out := []CascadeNode{{Name: "", Route: []string{}}}
	queue := [][]string{{}}
	seen := map[string]bool{}
	for len(queue) > 0 && len(out) < 64 {
		route := queue[0]
		queue = queue[1:]
		names, err := s.Explore(ctx, route)
		if err != nil {
			continue
		}
		for _, n := range names {
			if seen[n] || len(route)+1 > maxRouteLen {
				continue
			}
			seen[n] = true
			next := append(append([]string(nil), route...), n)
			out = append(out, CascadeNode{Name: n, Route: next})
			queue = append(queue, next)
		}
	}
	return out
}

// AdminHost — короткое имя админки внутри туннеля: https://admin
const AdminHost = "admin"

func isAdminHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == AdminHost || host == AdminHost+".veil"
}
