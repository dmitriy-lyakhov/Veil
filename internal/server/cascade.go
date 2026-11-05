package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/wgkey"
)

const (
	relayQueue      = 1024
	relayCallTime   = 30 * time.Second
	maxRelayBody    = 64 << 10
	maxRouteLen     = 32
	relayOpRegister = "register"
	relayOpRemove   = "remove"
	relayOpExplore  = "explore"
	relayOpPing     = "ping"
	relayOpLatency  = "latency"
	relayOpAdmin    = "admin"
)

type relayCounter struct {
	rx, tx atomic.Uint64
	last   atomic.Int64
}

func (c *relayCounter) touch() { c.last.Store(time.Now().Unix()) }

type hop struct {
	name    string
	profile *config.Profile
}

type relayRequest struct {
	Op        string   `json:"op"`
	Name      string   `json:"name,omitempty"`
	PublicKey string   `json:"public_key,omitempty"`
	Route     []string `json:"route,omitempty"`
	// для relayOpAdmin: запрос админки, который нужно выполнить на узле
	Method string          `json:"method,omitempty"`
	Path   string          `json:"path,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type relayResponse struct {
	AuthKey         string          `json:"auth_key,omitempty"`
	Address4        string          `json:"address4,omitempty"`
	Address6        string          `json:"address6,omitempty"`
	ServerPublicKey string          `json:"server_public_key,omitempty"`
	MTU             int             `json:"mtu,omitempty"`
	Hops            []string        `json:"hops,omitempty"`
	Upstreams       []string        `json:"upstreams,omitempty"`
	Self            string          `json:"self,omitempty"`
	Company         string          `json:"company,omitempty"`
	Links           []LinkLatency   `json:"links,omitempty"`
	Status          int             `json:"status,omitempty"`
	Raw             json.RawMessage `json:"raw,omitempty"`
}

func (s *Server) counter(peer string) *relayCounter {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	c := s.counters[peer]
	if c == nil {
		c = &relayCounter{}
		s.counters[peer] = c
	}
	return c
}

func (s *Server) preferredKind(upstream string) client.Kind {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	return s.hopKinds[upstream]
}

func (s *Server) rememberKind(upstream string, k client.Kind) {
	s.sessMu.Lock()
	s.hopKinds[upstream] = k
	s.sessMu.Unlock()
}

func (s *Server) upstreamLocked(name string) *config.Upstream {
	for i := range s.cfg.Upstreams {
		if s.cfg.Upstreams[i].Name == name {
			return &s.cfg.Upstreams[i]
		}
	}
	return nil
}

func (s *Server) upstreamDialer(name, authKey string) (*client.Dialer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	up := s.upstreamLocked(name)
	if up == nil {
		return nil, fmt.Errorf("на этом сервере нет следующего узла «%s»", name)
	}
	prof := up.Link.DialProfile()
	if authKey != "" {
		prof.AuthKey = authKey
	}
	return &client.Dialer{Profile: prof}, nil
}

func (s *Server) nextHop(peer string) (hop, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.cfg.Peers {
		if p.Name != peer || len(p.Route) == 0 {
			continue
		}
		up := s.upstreamLocked(p.Route[0])
		if up == nil {
			return hop{name: p.Route[0]}, true
		}
		prof := up.Link.DialProfile()
		prof.AuthKey = p.UpAuth
		return hop{name: up.Name, profile: prof}, true
	}
	return hop{}, false
}

func (s *Server) relaySession(ctx context.Context, peer string, h hop, ds downstream) error {
	if h.profile == nil {
		return fmt.Errorf("следующий узел %s не настроен", h.name)
	}
	ctr := s.counter(peer)
	pending := make(chan []byte, relayQueue)
	readErr := make(chan error, 1)
	go func() {
		readErr <- ds.read(func(p []byte) {
			ctr.rx.Add(uint64(len(p)))
			ctr.touch()
			select {
			case pending <- append([]byte(nil), p...):
			default:
			}
		})
	}()
	up, err := (&client.Dialer{Profile: h.profile, Prefer: s.preferredKind(h.name)}).Connect(ctx)
	if err != nil {
		ds.kill()
		<-readErr
		return fmt.Errorf("next hop %s: %w", h.name, err)
	}
	defer up.Close()
	s.rememberKind(h.name, up.Kind())
	upErr := make(chan error, 1)
	go func() {
		upErr <- up.ReadLoop(func(p []byte) {
			ctr.tx.Add(uint64(len(p)))
			ctr.touch()
			_ = ds.sink.WritePacket(p)
		})
	}()
	for {
		select {
		case p := <-pending:
			if err := up.WritePacket(p); err != nil {
				ds.kill()
				return err
			}
		case err := <-readErr:
			return err
		case err := <-upErr:
			ds.kill()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Server) serveRelayAPI(w http.ResponseWriter, r *http.Request, from string) {
	var req relayRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRelayBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), relayCallTime)
	defer cancel()
	var resp relayResponse
	var err error
	switch req.Op {
	case relayOpRegister:
		if _, kerr := wgkey.Hex(req.PublicKey); kerr != nil || req.Name == "" || len(req.Route) > maxRouteLen {
			err = errors.New("bad register request")
			break
		}
		resp, err = s.createPeer(ctx, from+"/"+req.Name, "", from, req.PublicKey, req.Route, "")
	case relayOpRemove:
		err = s.removePeer(ctx, from+"/"+req.Name, from)
	case relayOpExplore:
		resp.Upstreams, err = s.Explore(ctx, req.Route)
	case relayOpAdmin:
		resp, err = s.relayAdminOp(ctx, from, req)
	case relayOpPing:
	case relayOpLatency:
		if len(req.Route) == 0 {
			resp = s.latencyReport()
			break
		}
		if len(req.Route) > maxRouteLen {
			err = errors.New("route too long")
			break
		}
		var d *client.Dialer
		if d, err = s.upstreamDialer(req.Route[0], ""); err == nil {
			err = d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpLatency, Route: req.Route[1:]}, &resp)
		}
	default:
		err = errors.New("unknown op")
	}
	if err != nil {
		s.log.Warn("relay api", "from", from, "op", req.Op, "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) ownPublicKey() string {
	pub, _ := wgkey.Public(s.cfg.PrivateKey)
	return pub
}

func (s *Server) aliasLocked(name, note, relayOf, pub, authKey string) (relayResponse, bool) {
	for _, p := range s.cfg.Peers {
		if p.PublicKey != pub || len(p.Route) > 0 {
			continue
		}
		if s.peerIndexLocked(name) >= 0 {
			return relayResponse{}, false
		}
		s.cfg.Peers = append(s.cfg.Peers, config.Peer{
			Name: name, PublicKey: pub, AuthKey: authKey, Address4: p.Address4, Address6: p.Address6,
			Created: time.Now().UTC().Truncate(time.Second), RelayOf: relayOf, Note: note,
		})
		if err := s.saveLocked(); err != nil {
			s.log.Error("save config", "err", err)
		}
		s.log.Info("additional access path for existing client", "name", name, "shares_with", p.Name)
		return relayResponse{AuthKey: authKey, Address4: p.Address4, Address6: p.Address6, ServerPublicKey: s.ownPublicKey(), MTU: s.cfg.MTU, Hops: []string{s.cfg.Site.Company}}, true
	}
	return relayResponse{}, false
}

func (s *Server) sharedKeyLocked(pub string, except int) bool {
	for i, p := range s.cfg.Peers {
		if i != except && p.PublicKey == pub && len(p.Route) == 0 {
			return true
		}
	}
	return false
}

func (s *Server) createPeer(ctx context.Context, name, note, relayOf, pub string, route []string, upName string) (relayResponse, error) {
	authKey := provision.NewAuthKey()
	if len(route) == 0 {
		s.mu.Lock()
		if resp, ok := s.aliasLocked(name, note, relayOf, pub, authKey); ok {
			s.mu.Unlock()
			if err := s.authorize(name, authKey); err != nil {
				return relayResponse{}, err
			}
			return resp, nil
		}
		defer s.mu.Unlock()
		if s.peerIndexLocked(name) >= 0 {
			return relayResponse{}, fmt.Errorf("клиент «%s» уже существует", name)
		}
		peer, err := provision.AddRemotePeer(s.cfg, name, pub, relayOf)
		if err != nil {
			return relayResponse{}, err
		}
		idx := len(s.cfg.Peers) - 1
		s.cfg.Peers[idx].AuthKey, s.cfg.Peers[idx].Note = authKey, note
		uapi, err := peerConfig(peer)
		if err == nil {
			err = s.dev.IpcSet(uapi)
		}
		if err != nil {
			s.cfg.Peers = s.cfg.Peers[:idx]
			return relayResponse{}, fmt.Errorf("wireguard: %w", err)
		}
		if err := s.authorize(name, authKey); err != nil {
			return relayResponse{}, err
		}
		if err := s.saveLocked(); err != nil {
			s.log.Error("save config", "err", err)
		}
		s.log.Info("client added", "name", name, "relay_of", relayOf)
		return relayResponse{AuthKey: authKey, Address4: peer.Address4, Address6: peer.Address6, ServerPublicKey: s.ownPublicKey(), MTU: s.cfg.MTU, Hops: []string{s.cfg.Site.Company}}, nil
	}

	s.mu.Lock()
	exists := s.peerIndexLocked(name) >= 0
	company := s.cfg.Site.Company
	s.mu.Unlock()
	if exists {
		return relayResponse{}, fmt.Errorf("клиент «%s» уже существует", name)
	}
	if upName == "" {
		upName = strings.ReplaceAll(name, "/", "_")
	}
	d, err := s.upstreamDialer(route[0], "")
	if err != nil {
		return relayResponse{}, err
	}
	var resp relayResponse
	if err := d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpRegister, Name: upName, PublicKey: pub, Route: route[1:]}, &resp); err != nil {
		return relayResponse{}, fmt.Errorf("узел %s: %w", route[0], err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peerIndexLocked(name) >= 0 {
		return relayResponse{}, fmt.Errorf("клиент «%s» уже существует", name)
	}
	s.cfg.Peers = append(s.cfg.Peers, config.Peer{
		Name: name, PublicKey: pub, AuthKey: authKey, Address4: resp.Address4, Address6: resp.Address6,
		Created: time.Now().UTC().Truncate(time.Second), Route: append([]string(nil), route...),
		UpAuth: resp.AuthKey, UpName: upName, RelayOf: relayOf, Note: note,
	})
	if err := s.authorize(name, authKey); err != nil {
		return relayResponse{}, err
	}
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.log.Info("cascade client added", "name", name, "route", strings.Join(route, " → "))
	resp.AuthKey = authKey
	resp.Hops = append([]string{company}, resp.Hops...)
	return resp, nil
}

func (s *Server) peerIndexLocked(name string) int {
	return slices.IndexFunc(s.cfg.Peers, func(p config.Peer) bool { return p.Name == name })
}

func (s *Server) removePeer(ctx context.Context, name, relayOf string) error {
	s.mu.Lock()
	idx := s.peerIndexLocked(name)
	if idx < 0 || s.cfg.Peers[idx].RelayOf != relayOf {
		s.mu.Unlock()
		return fmt.Errorf("клиент «%s» не найден", name)
	}
	peer := s.cfg.Peers[idx]
	if localPeer(peer) && !s.sharedKeyLocked(peer.PublicKey, idx) {
		if hex, err := wgkey.Hex(peer.PublicKey); err == nil {
			if err := s.dev.IpcSet(fmt.Sprintf("public_key=%s\nremove=true\n", hex)); err != nil {
				s.mu.Unlock()
				return fmt.Errorf("wireguard: %w", err)
			}
		}
	}
	s.cfg.Peers = slices.Delete(s.cfg.Peers, idx, idx+1)
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.mu.Unlock()
	s.verifier.Remove(name)
	closed := s.killSessions(func(p string) bool { return p == name })
	s.sessMu.Lock()
	delete(s.counters, name)
	s.sessMu.Unlock()
	if len(peer.Route) > 0 {
		if d, err := s.upstreamDialer(peer.Route[0], ""); err == nil {
			if err := d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpRemove, Name: peer.UpName}, nil); err != nil {
				s.log.Warn("remove on next hop", "name", name, "next", peer.Route[0], "err", err)
			}
		}
	}
	s.log.Info("client removed", "name", name, "sessions_closed", closed)
	return nil
}

func (s *Server) Explore(ctx context.Context, route []string) ([]string, error) {
	if len(route) > maxRouteLen {
		return nil, errors.New("маршрут слишком длинный")
	}
	if len(route) == 0 {
		s.mu.Lock()
		defer s.mu.Unlock()
		names := []string{}
		for _, u := range s.cfg.Upstreams {
			names = append(names, u.Name)
		}
		return names, nil
	}
	d, err := s.upstreamDialer(route[0], "")
	if err != nil {
		return nil, err
	}
	var resp relayResponse
	if err := d.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpExplore, Route: route[1:]}, &resp); err != nil {
		return nil, fmt.Errorf("узел %s: %w", route[0], err)
	}
	return resp.Upstreams, nil
}

func (s *Server) transportLink() config.RelayLink {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := config.RelayLink{
		Company:         s.cfg.Site.Company,
		IPv4:            s.cfg.PublicIPv4,
		IPv6:            s.cfg.PublicIPv6,
		Port:            s.cfg.Port(),
		Path:            s.cfg.Path,
		ServerPublicKey: s.ownPublicKey(),
	}
	switch {
	case s.cfg.Front != nil && s.cfg.Front.SNI != "":
		l.FrontSNI, l.FrontKey = s.cfg.Front.SNI, s.cfg.Front.Key
		l.CertPin, _ = provision.CertPin(s.cfg.CertPEM)
	case s.cfg.Domain != "" && s.certs.Ready():
		l.ServerName = s.cfg.Domain
	default:
		l.CertPin, _ = provision.CertPin(s.cfg.CertPEM)
	}
	return l
}

func (s *Server) IssueRelay(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := validClientName(name); err != nil {
		return "", err
	}
	if strings.Contains(name, "/") {
		return "", errors.New("имя не должно содержать «/»")
	}
	link := s.transportLink()
	if link.IPv4 == "" && link.IPv6 == "" {
		return "", errors.New("серверу неизвестен его публичный адрес — переустановите сервер из приложения")
	}
	s.mu.Lock()
	if slices.ContainsFunc(s.cfg.Relays, func(r config.Relay) bool { return r.Name == name }) {
		s.mu.Unlock()
		return "", fmt.Errorf("ключ каскада «%s» уже выдан", name)
	}
	authKey := provision.NewAuthKey()
	s.cfg.Relays = append(s.cfg.Relays, config.Relay{Name: name, AuthKey: authKey, Created: time.Now().UTC().Truncate(time.Second)})
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.mu.Unlock()
	if err := s.authorize(relayPrefix+name, authKey); err != nil {
		return "", err
	}
	link.AuthKey = authKey
	return config.EncodeRelayLink(&link)
}

func (s *Server) RemoveRelay(ctx context.Context, name string) error {
	s.mu.Lock()
	idx := slices.IndexFunc(s.cfg.Relays, func(r config.Relay) bool { return r.Name == name })
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("ключ каскада «%s» не найден", name)
	}
	s.cfg.Relays = slices.Delete(s.cfg.Relays, idx, idx+1)
	var dependents []string
	for _, p := range s.cfg.Peers {
		if p.RelayOf == name {
			dependents = append(dependents, p.Name)
		}
	}
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.mu.Unlock()
	s.verifier.Remove(relayPrefix + name)
	for _, p := range dependents {
		_ = s.removePeer(ctx, p, name)
	}
	return nil
}

func (s *Server) AddUpstream(ctx context.Context, name, linkText string) error {
	name = strings.TrimSpace(name)
	if err := validClientName(name); err != nil {
		return err
	}
	link, err := config.DecodeRelayLink(linkText)
	if err != nil {
		return err
	}
	probe := &client.Dialer{Profile: link.DialProfile()}
	var resp relayResponse
	if err := probe.Call(ctx, relayAPISuffix, relayRequest{Op: relayOpExplore}, &resp); err != nil {
		return fmt.Errorf("следующий узел не принял ключ каскада: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.upstreamLocked(name) != nil {
		return fmt.Errorf("узел «%s» уже добавлен", name)
	}
	s.cfg.Upstreams = append(s.cfg.Upstreams, config.Upstream{Name: name, Link: *link, Added: time.Now().UTC().Truncate(time.Second)})
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.log.Info("upstream added", "name", name, "company", link.Company)
	return nil
}

func (s *Server) RemoveUpstream(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := slices.IndexFunc(s.cfg.Upstreams, func(u config.Upstream) bool { return u.Name == name })
	if idx < 0 {
		return fmt.Errorf("узел «%s» не найден", name)
	}
	used := 0
	for _, p := range s.cfg.Peers {
		if len(p.Route) > 0 && p.Route[0] == name {
			used++
		}
	}
	if used > 0 {
		return fmt.Errorf("через узел «%s» работают клиентов: %d — сначала удалите их", name, used)
	}
	s.cfg.Upstreams = slices.Delete(s.cfg.Upstreams, idx, idx+1)
	return s.saveLocked()
}
