package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/vpnconf/wgkey"
)

const (
	maxClientName     = 40
	maxNote           = 200
	minPasswordLength = 8
	onlineWindow      = 3 * time.Minute
)

type SessionInfo struct {
	Transport string    `json:"transport"`
	Remote    string    `json:"remote"`
	Since     time.Time `json:"since"`
}

type ClientInfo struct {
	Name          string        `json:"name"`
	Note          string        `json:"note,omitempty"`
	Address4      string        `json:"address4"`
	Address6      string        `json:"address6"`
	Created       time.Time     `json:"created,omitzero"`
	Disabled      bool          `json:"disabled"`
	Role          string        `json:"role,omitempty"` // "user" | "admin" | "" (клиент без роли — вход в панель по паролю)
	Subnets       []string      `json:"subnets,omitempty"` // сети за клиентом-шлюзом
	Route         []string      `json:"route"`
	RelayOf       string        `json:"relay_of,omitempty"`
	Online        bool          `json:"online"`
	Sessions      []SessionInfo `json:"sessions"`
	LastHandshake time.Time     `json:"last_handshake,omitzero"`
	RxBytes       uint64        `json:"rx_bytes"`
	TxBytes       uint64        `json:"tx_bytes"`
	Auto          *AutoInfo     `json:"auto,omitempty"`
}

type AutoInfo struct {
	Exit     string    `json:"exit"`
	MaxHops  int       `json:"max_hops"`
	CostMs   float64   `json:"cost_ms"`
	Rerouted time.Time `json:"rerouted,omitzero"`
}

type RelayInfo struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created,omitzero"`
	Clients int       `json:"clients"`
	Online  bool      `json:"online"`
	Admin   bool      `json:"admin,omitempty"` // узлу разрешено администрировать этот сервер
}

type UpstreamInfo struct {
	Name    string    `json:"name"`
	Company string    `json:"company"`
	IPv4    string    `json:"ipv4,omitempty"`
	IPv6    string    `json:"ipv6,omitempty"`
	Domain  string    `json:"domain,omitempty"`
	Added   time.Time `json:"added,omitzero"`
	Clients int       `json:"clients"`
}

type ServerInfo struct {
	Version    string     `json:"version"`
	Company    string     `json:"company"`
	PublicIPv4 string     `json:"public_ipv4"`
	PublicIPv6 string     `json:"public_ipv6"`
	Port       int        `json:"port"`
	Domain     string     `json:"domain"`
	Cert       certStatus `json:"cert"`
	AdminURL   string     `json:"admin_url"`
	Front      FrontInfo  `json:"front"`
	Links      config.Links `json:"links"`
	Started    time.Time  `json:"started"`
	Clients    int        `json:"clients"`
	Online     int        `json:"online"`
	RxBytes    uint64     `json:"rx_bytes"`
	TxBytes    uint64     `json:"tx_bytes"`
}

type State struct {
	Server    ServerInfo     `json:"server"`
	Clients   []ClientInfo   `json:"clients"`
	Relays    []RelayInfo    `json:"relays"`
	Upstreams []UpstreamInfo `json:"upstreams"`
	Latency   []LinkLatency  `json:"latency"`
	SiteRepo  SiteRepoInfo   `json:"site_repo"`
}

type peerStats struct {
	handshake time.Time
	rx, tx    uint64
}

func (s *Server) wgStats() map[string]peerStats {
	out := map[string]peerStats{}
	raw, err := s.dev.IpcGet()
	if err != nil {
		return out
	}
	var cur string
	var st peerStats
	var sec int64
	flush := func() {
		if cur != "" {
			if sec > 0 {
				st.handshake = time.Unix(sec, 0)
			}
			out[cur] = st
		}
	}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "public_key":
			flush()
			cur, st, sec = v, peerStats{}, 0
		case "rx_bytes":
			st.rx, _ = strconv.ParseUint(v, 10, 64)
		case "tx_bytes":
			st.tx, _ = strconv.ParseUint(v, 10, 64)
		case "last_handshake_time_sec":
			sec, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	flush()
	return out
}

func (s *Server) State() State {
	stats := s.wgStats()
	latency := s.Latencies()
	live := map[string][]SessionInfo{}
	relayStats := map[string]peerStats{}
	s.sessMu.Lock()
	for sess := range s.sessions {
		live[sess.peer] = append(live[sess.peer], SessionInfo{sess.transport, sess.remote, sess.since})
	}
	for name, c := range s.counters {
		ps := peerStats{rx: c.rx.Load(), tx: c.tx.Load()}
		if t := c.last.Load(); t > 0 {
			ps.handshake = time.Unix(t, 0)
		}
		relayStats[name] = ps
	}
	s.sessMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	st := State{Server: ServerInfo{
		Version:    Version,
		Company:    s.cfg.Site.Company,
		PublicIPv4: s.cfg.PublicIPv4,
		PublicIPv6: s.cfg.PublicIPv6,
		Port:       s.cfg.Port(),
		Domain:     s.cfg.Domain,
		Cert:       s.certs.Status(),
		AdminURL:   provision.AdminURL(s.cfg),
		Front:      s.frontInfoLocked(),
		Links:      s.cfg.EffectiveLinks(),
		Started:    s.started,
	}, Clients: []ClientInfo{}, Relays: []RelayInfo{}, Upstreams: []UpstreamInfo{}, Latency: latency, SiteRepo: s.siteRepo.get()}
	relayClients := map[string]int{}
	upClients := map[string]int{}
	for _, p := range s.cfg.Peers {
		ci := ClientInfo{
			Name: p.Name, Note: p.Note, Address4: p.Address4, Address6: p.Address6, Created: p.Created,
			Disabled: p.Disabled, Role: p.Role, Route: p.Route, RelayOf: p.RelayOf, Sessions: live[p.Name], Subnets: p.Subnets,
		}
		if ci.Route == nil {
			ci.Route = []string{}
		}
		if p.Auto != nil {
			cost := s.autoCostOf(p.Name)
			if cost == 0 {
				cost = p.Auto.CostMs
			}
			if math.IsInf(cost, 1) {
				cost = -1
			}
			ci.Auto = &AutoInfo{Exit: p.Auto.ExitName, MaxHops: p.Auto.MaxHops, CostMs: cost, Rerouted: p.Auto.Rerouted}
		}
		if ci.Sessions == nil {
			ci.Sessions = []SessionInfo{}
		}
		slices.SortFunc(ci.Sessions, func(a, b SessionInfo) int { return b.Since.Compare(a.Since) })
		ps := relayStats[p.Name]
		if len(p.Route) == 0 {
			if hex, err := wgkey.Hex(p.PublicKey); err == nil {
				ps = stats[hex]
			}
		} else {
			upClients[p.Route[0]]++
		}
		ci.LastHandshake, ci.RxBytes, ci.TxBytes = ps.handshake, ps.rx, ps.tx
		ci.Online = !p.Disabled && len(ci.Sessions) > 0 && time.Since(ci.LastHandshake) < onlineWindow
		if p.RelayOf != "" {
			relayClients[p.RelayOf]++
			ci.Online = !p.Disabled && time.Since(ci.LastHandshake) < onlineWindow
		}
		if ci.Online {
			st.Server.Online++
		}
		if p.RelayOf == "" {
			st.Server.Clients++
		}
		st.Server.RxBytes += ci.RxBytes
		st.Server.TxBytes += ci.TxBytes
		st.Clients = append(st.Clients, ci)
	}
	for _, r := range s.cfg.Relays {
		st.Relays = append(st.Relays, RelayInfo{Name: r.Name, Created: r.Created, Clients: relayClients[r.Name], Online: len(live[relayPrefix+r.Name]) > 0, Admin: r.Admin})
	}
	for _, u := range s.cfg.Upstreams {
		st.Upstreams = append(st.Upstreams, UpstreamInfo{Name: u.Name, Company: u.Link.Company, IPv4: u.Link.IPv4, IPv6: u.Link.IPv6, Domain: u.Link.ServerName, Added: u.Added, Clients: upClients[u.Name]})
	}
	return st
}

func validClientName(name string) error {
	if name == "" {
		return errors.New("укажите имя")
	}
	if utf8.RuneCountInString(name) > maxClientName {
		return fmt.Errorf("имя длиннее %d символов", maxClientName)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return errors.New("имя содержит недопустимые символы")
		}
	}
	return nil
}

func validNote(note string) error {
	if utf8.RuneCountInString(note) > maxNote {
		return fmt.Errorf("заметка длиннее %d символов", maxNote)
	}
	return nil
}

func (s *Server) clientProfile(name, priv string, info relayResponse) *config.Profile {
	link := s.transportLink()
	mtu := info.MTU
	if mtu == 0 {
		mtu = config.DefaultMTU
	}
	p := &config.Profile{
		Name: name, IPv4: link.IPv4, IPv6: link.IPv6, Port: link.Port, ServerName: link.ServerName, Path: link.Path,
		CertPin: link.CertPin, FrontSNI: link.FrontSNI, FrontKey: link.FrontKey,
		ServerPublicKey: info.ServerPublicKey, PrivateKey: priv, AuthKey: info.AuthKey,
		Address4: info.Address4, Address6: info.Address6, DNS: append([]string(nil), provision.DefaultDNS...), MTU: mtu,
	}
	if len(info.Hops) > 1 {
		p.Via = strings.Join(info.Hops, " → ")
	}
	return p
}

func (s *Server) AddClient(ctx context.Context, name, note string, route []string) (*config.Profile, error) {
	return s.AddClientWithKey(ctx, name, note, route, "")
}

func keyPair(pub string) (string, string, error) {
	if pub == "" {
		return wgkey.Generate()
	}
	if _, err := wgkey.Hex(pub); err != nil {
		return "", "", err
	}
	return "", pub, nil
}

func (s *Server) AddClientWithKey(ctx context.Context, name, note string, route []string, publicKey string) (*config.Profile, error) {
	name = strings.TrimSpace(name)
	if err := validClientName(name); err != nil {
		return nil, err
	}
	if strings.Contains(name, "/") {
		return nil, errors.New("имя не должно содержать «/»")
	}
	if err := validNote(note); err != nil {
		return nil, err
	}
	if len(route) > maxRouteLen {
		return nil, errors.New("маршрут слишком длинный")
	}
	s.mu.Lock()
	noAddr := s.cfg.PublicIPv4 == "" && s.cfg.PublicIPv6 == ""
	s.mu.Unlock()
	if noAddr {
		return nil, errors.New("серверу неизвестен его публичный адрес — переустановите сервер из приложения")
	}
	priv, pub, err := keyPair(publicKey)
	if err != nil {
		return nil, err
	}
	info, err := s.createPeer(ctx, name, strings.TrimSpace(note), "", pub, route, "")
	if err != nil {
		return nil, err
	}
	return s.clientProfile(name, priv, info), nil
}

func (s *Server) RemoveClient(ctx context.Context, name string) error {
	err := s.removePeer(ctx, name, "")
	if err == nil {
		s.linksChanged() // вместе с клиентом уходят и сети за ним
	}
	return err
}

func (s *Server) ReissueClient(ctx context.Context, name string) (*config.Profile, error) {
	s.mu.Lock()
	idx := s.peerIndexLocked(name)
	if idx < 0 || s.cfg.Peers[idx].RelayOf != "" {
		s.mu.Unlock()
		return nil, fmt.Errorf("клиент «%s» не найден", name)
	}
	p := s.cfg.Peers[idx]
	s.mu.Unlock()
	if err := s.removePeer(ctx, name, ""); err != nil {
		return nil, err
	}
	var prof *config.Profile
	var err error
	if p.Auto != nil {
		prof, err = s.AddAutoClient(ctx, name, p.Note, p.Auto.ExitName, p.Auto.MaxHops)
	} else {
		prof, err = s.AddClient(ctx, name, p.Note, p.Route)
	}
	if err != nil {
		return nil, err
	}
	if p.Disabled {
		disabled := true
		_ = s.UpdateClient(name, ClientUpdate{Disabled: &disabled})
	}
	return prof, nil
}

type ClientUpdate struct {
	NewName  string  `json:"new_name,omitempty"`
	Note     *string `json:"note,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
	Role     *string `json:"role,omitempty"` // "user" | "admin"
}

func (s *Server) UpdateClient(name string, u ClientUpdate) error {
	s.mu.Lock()
	idx := s.peerIndexLocked(name)
	if idx < 0 || s.cfg.Peers[idx].RelayOf != "" {
		s.mu.Unlock()
		return fmt.Errorf("клиент «%s» не найден", name)
	}
	p := &s.cfg.Peers[idx]
	role := p.Role
	if u.Role != nil {
		r, err := normalizeRole(*u.Role)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		role = r
	}
	newName := strings.TrimSpace(u.NewName)
	if newName != "" && newName != name {
		if err := validClientName(newName); err != nil {
			s.mu.Unlock()
			return err
		}
		if strings.Contains(newName, "/") {
			s.mu.Unlock()
			return errors.New("имя не должно содержать «/»")
		}
		if s.peerIndexLocked(newName) >= 0 {
			s.mu.Unlock()
			return fmt.Errorf("клиент «%s» уже существует", newName)
		}
	} else {
		newName = name
	}
	if u.Note != nil {
		if err := validNote(*u.Note); err != nil {
			s.mu.Unlock()
			return err
		}
		p.Note = strings.TrimSpace(*u.Note)
	}
	wasDisabled := p.Disabled
	if u.Disabled != nil {
		p.Disabled = *u.Disabled
	}
	p.Name = newName
	p.Role = role
	authKey, disabled := p.AuthKey, p.Disabled
	if err := s.saveLocked(); err != nil {
		s.log.Error("save config", "err", err)
	}
	s.mu.Unlock()

	s.verifier.Remove(name)
	if !disabled {
		_ = s.authorize(newName, authKey)
	}
	if newName != name {
		s.sessMu.Lock()
		for sess := range s.sessions {
			if sess.peer == name {
				sess.peer = newName
			}
		}
		if c, ok := s.counters[name]; ok {
			s.counters[newName] = c
			delete(s.counters, name)
		}
		s.sessMu.Unlock()
	}
	if disabled && !wasDisabled {
		s.killSessions(func(peer string) bool { return peer == newName })
		s.log.Info("client blocked", "name", newName)
	}
	if !disabled && wasDisabled {
		s.log.Info("client unblocked", "name", newName)
	}
	if disabled != wasDisabled {
		s.linksChanged() // сети за заблокированным шлюзом не маршрутизируются
	}
	return nil
}

func (s *Server) SetAdminPassword(current *string, password string) error {
	if utf8.RuneCountInString(password) < minPasswordLength {
		return fmt.Errorf("пароль должен быть не короче %d символов", minPasswordLength)
	}
	s.mu.Lock()
	hash := s.cfg.Admin.PasswordHash
	s.mu.Unlock()
	if current != nil && (hash == "" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(*current)) != nil) {
		return errors.New("текущий пароль указан неверно")
	}
	newHash, err := provision.HashAdminPassword(password)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.Admin.User == "" {
		s.cfg.Admin.User = defaultAdminUser
	}
	s.cfg.Admin.PasswordHash = newHash
	s.admin.dropAll()
	s.log.Info("admin password changed")
	return s.saveLocked()
}

func (s *Server) frontInfoLocked() FrontInfo {
	if s.cfg.Front == nil || s.cfg.Front.SNI == "" {
		return FrontInfo{}
	}
	dest := s.cfg.Front.Dest
	if dest == "" {
		dest = net.JoinHostPort(s.cfg.Front.SNI, "443")
	}
	return FrontInfo{Enabled: true, SNI: s.cfg.Front.SNI, Dest: dest}
}
