// Package veilcore — Go-ядро Android-клиента Veil для gomobile bind: Veil (WireGuard внутри HTTPS),
// WireGuard и AmneziaWG. Весь обмен с Kotlin — строками (JSON).
//
// Жизненный цикл: Import/Parse → (Plan) → настройка VpnService.Builder → establish() → Start(link, fd, protector).
package veilcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"veil/internal/client"
	"veil/internal/config"
)

// Protector исключает сокет из VPN (на Android — VpnService.protect(fd)).
type Protector interface {
	Protect(fd int32) bool
}

var (
	mu      sync.Mutex
	tunnel  client.Engine
	profile *config.Profile
)

// Коды ошибок импорта (Kotlin показывает их локализованными).
const (
	ErrOpenVPN = "E_OPENVPN"
	ErrIPsec   = "E_IPSEC"
	ErrRelay   = "E_RELAY"
	ErrFormat  = "E_FORMAT"
	ErrEmpty   = "E_EMPTY"
)

type addr struct {
	Addr   string `json:"addr"`
	Prefix int    `json:"prefix"`
	V6     bool   `json:"v6"`
}

type parsed struct {
	Name      string   `json:"name"`
	Protocol  string   `json:"protocol"` // veil, wireguard, amneziawg
	Addresses []addr   `json:"addresses"`
	DNS       []string `json:"dns"`
	MTU       int      `json:"mtu"`
	Entries   []string `json:"entries"`
	Routes    []string `json:"routes"` // не пусто — в туннель идут только эти сети (AllowedIPs WireGuard)
	Role      string   `json:"role,omitempty"`
	Host      string   `json:"host,omitempty"`
}

func protocolOf(p *config.Profile) string {
	if p.IsVeil() {
		return "veil"
	}
	return p.Protocol
}

// decode разбирает ссылку veil://, JSON-профиль или текст .conf WireGuard/AmneziaWG.
func decode(text string) (*config.Profile, error) {
	t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), string(rune(0xFEFF))))
	if t == "" {
		return nil, errors.New(ErrEmpty)
	}
	switch {
	case strings.HasPrefix(t, config.RelayLinkScheme):
		return nil, errors.New(ErrRelay)
	case strings.Contains(t, "[Interface]"):
		return fromConf(t)
	case strings.HasPrefix(t, "{") && isWGJSON(t):
		return fromJSON(t)
	case strings.HasPrefix(t, config.LinkScheme) || strings.HasPrefix(t, "{"):
		p, err := config.DecodeLink(t)
		if err != nil {
			return nil, err
		}
		switch p.Protocol {
		case "openvpn":
			return nil, errors.New(ErrOpenVPN)
		case "ipsec":
			return nil, errors.New(ErrIPsec)
		}
		if !p.IsVeil() && p.WG == nil {
			return nil, errors.New(ErrFormat)
		}
		return p, nil
	case looksOpenVPN(t):
		return nil, errors.New(ErrOpenVPN)
	}
	return nil, errors.New(ErrFormat)
}

func prefixOf(s string) (addr, error) {
	pf, err := netip.ParsePrefix(s)
	if err != nil {
		a, err2 := netip.ParseAddr(s)
		if err2 != nil {
			return addr{}, fmt.Errorf("некорректный адрес туннеля %q", s)
		}
		pf = netip.PrefixFrom(a, a.BitLen())
	}
	return addr{Addr: pf.Addr().String(), Prefix: pf.Bits(), V6: pf.Addr().Is6()}, nil
}

func describe(p *config.Profile) (parsed, error) {
	out := parsed{Name: p.Name, Protocol: protocolOf(p), MTU: p.EffectiveMTU(), Role: p.Role,
		DNS: []string{}, Entries: []string{}, Addresses: []addr{}, Routes: []string{}}
	var raw []string
	if p.IsVeil() {
		raw = []string{p.Address4, p.Address6}
		for _, d := range p.DNS {
			if _, err := netip.ParseAddr(d); err == nil {
				out.DNS = append(out.DNS, d)
			}
		}
		for _, e := range p.Entries {
			out.Entries = append(out.Entries, e.Name)
		}
		if p.IPv4 != "" {
			out.Host = p.IPv4
		} else {
			out.Host = p.IPv6
		}
	} else {
		c := p.WG
		if c == nil {
			return out, errors.New(ErrFormat)
		}
		for _, a := range c.Address {
			raw = append(raw, a.String())
		}
		for _, d := range c.DNS {
			out.DNS = append(out.DNS, d.String())
		}
		if len(out.DNS) == 0 {
			for _, d := range p.DNS {
				if _, err := netip.ParseAddr(d); err == nil {
					out.DNS = append(out.DNS, d)
				}
			}
		}
		if c.MTU > 0 {
			out.MTU = c.MTU
		} else {
			out.MTU = 1380
		}
		if h, _, err := c.EndpointHostPort(); err == nil {
			out.Host = h
		}
		if routes := c.Routes(); !coversAll(routes) {
			for _, r := range routes {
				out.Routes = append(out.Routes, r.String())
			}
		}
	}
	for _, s := range raw {
		if s == "" {
			continue
		}
		a, err := prefixOf(s)
		if err != nil {
			return out, err
		}
		out.Addresses = append(out.Addresses, a)
	}
	if len(out.Addresses) == 0 {
		return out, errors.New("в конфигурации нет адреса туннеля")
	}
	if len(out.DNS) == 0 {
		out.DNS = []string{"1.1.1.1", "8.8.8.8"}
	}
	return out, nil
}

func coversAll(ps []netip.Prefix) bool {
	for _, p := range ps {
		if p.Bits() == 0 {
			return true
		}
	}
	return false
}

// Parse проверяет ссылку/конфигурацию и возвращает JSON для VpnService.Builder:
// {name, protocol, addresses:[{addr,prefix,v6}], dns, mtu, entries, routes, role, host}. Ключи в ответ не попадают.
func Parse(link string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	d, err := describe(p)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(d)
	return string(b), nil
}

// Stop закрывает туннель (и TUN). Безопасно вызывать повторно.
func Stop() {
	mu.Lock()
	t := tunnel
	tunnel, profile = nil, nil
	mu.Unlock()
	if t != nil {
		t.Close()
	}
	releaseProtect()
}

// Running — запущен ли туннель.
func Running() bool {
	mu.Lock()
	defer mu.Unlock()
	return tunnel != nil
}

type statusJSON struct {
	State     string `json:"state"` // stopped, connecting, connected, reconnecting
	Up        bool   `json:"up"`    // рукопожатие выполнено
	Protocol  string `json:"protocol,omitempty"`
	Transport string `json:"transport,omitempty"`
	Entry     string `json:"entry,omitempty"`
	Name      string `json:"name,omitempty"`
	Exit      string `json:"exit,omitempty"`
	Rx        uint64 `json:"rx"`
	Tx        uint64 `json:"tx"`
	Handshake int64  `json:"handshake,omitempty"`
	LastError string `json:"last_error,omitempty"`
	Skipped   int    `json:"skipped_entries,omitempty"`
}

// Status возвращает JSON {state, up, protocol, transport, entry, name, exit, rx, tx, handshake, last_error, skipped_entries}.
func Status() string {
	mu.Lock()
	t, p := tunnel, profile
	mu.Unlock()
	if t == nil {
		b, _ := json.Marshal(statusJSON{State: "stopped"})
		return string(b)
	}
	s := t.Status()
	out := statusJSON{State: string(s.State), Transport: s.Transport, Entry: s.Entry, Exit: s.Exit,
		Rx: s.RxBytes, Tx: s.TxBytes, LastError: strings.TrimSpace(s.LastError), Skipped: len(s.Skipped)}
	if p != nil {
		out.Name, out.Protocol = p.Name, protocolOf(p)
	}
	if !s.Handshake.IsZero() {
		out.Up, out.Handshake = true, s.Handshake.Unix()
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// Reselect просит движок Veil выбрать вход заново (смена сервера каскада).
func Reselect() {
	mu.Lock()
	t := tunnel
	mu.Unlock()
	if t != nil {
		t.Reselect()
	}
}
