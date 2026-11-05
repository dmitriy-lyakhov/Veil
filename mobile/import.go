package veilcore

import (
	"encoding/json"
	"errors"
	"net"
	"strings"

	"veil/internal/config"
	"veil/internal/ovpnconf"
	"veil/internal/wgconf"
)

func looksOpenVPN(t string) bool { return ovpnconf.Looks(t) }

func fromConf(t string) (*config.Profile, error) {
	cfg, err := wgconf.ParseAWG(t)
	if err != nil {
		return nil, err
	}
	proto, title := "wireguard", "WireGuard"
	if !cfg.AWG.Empty() {
		proto, title = "amneziawg", "AmneziaWG"
	} else {
		cfg.AWG = nil
	}
	p := &config.Profile{Name: title, Protocol: proto, WG: cfg}
	if h, _, err := cfg.EndpointHostPort(); err == nil {
		if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil && ip.To4() == nil {
			p.IPv6 = ip.String()
		} else {
			p.IPv4 = h
			p.Name = title + " " + h
		}
	}
	return p, nil
}

type imported struct {
	Link     string `json:"link"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

// Import принимает ссылку veil://, JSON-профиль или текст .conf (WireGuard/AmneziaWG) и возвращает JSON
// {link, name, protocol}, где link — нормализованная ссылка veil:// (хранится и подаётся в Parse/Start).
// name — необязательное имя (пусто — из профиля). Ошибки E_OPENVPN, E_IPSEC, E_RELAY, E_FORMAT, E_EMPTY —
// коды для локализации; прочие — текст.
func Import(text, name string) (string, error) {
	p, err := decode(text)
	if err != nil {
		return "", err
	}
	if _, err := describe(p); err != nil {
		return "", err
	}
	if n := strings.TrimSpace(name); n != "" {
		p.Name = n
	}
	link, err := encode(p)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(imported{Link: link, Name: p.Name, Protocol: protocolOf(p)})
	return string(b), nil
}

// Rename возвращает ссылку с новым именем профиля.
func Rename(link, name string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", errors.New(ErrEmpty)
	}
	p.Name = strings.TrimSpace(name)
	return encode(p)
}

// Entries возвращает JSON-массив имён входов каскада профиля Veil (пусто — один вход).
func Entries(link string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, e := range p.Entries {
		names = append(names, e.Name)
	}
	b, _ := json.Marshal(names)
	return string(b), nil
}

// isWGJSON — JSON-профиль не-Veil протокола (WireGuard/AmneziaWG): хранится как JSON, а не ссылка.
func isWGJSON(t string) bool {
	var h struct {
		Protocol string `json:"protocol"`
	}
	if json.Unmarshal([]byte(t), &h) != nil {
		return false
	}
	return h.Protocol != "" && h.Protocol != "veil"
}

func fromJSON(t string) (*config.Profile, error) {
	var p config.Profile
	if err := json.Unmarshal([]byte(t), &p); err != nil {
		return nil, errors.New(ErrFormat)
	}
	switch p.Protocol {
	case "openvpn":
		return nil, errors.New(ErrOpenVPN)
	case "ipsec":
		return nil, errors.New(ErrIPsec)
	case "wireguard", "amneziawg":
		if p.WG == nil {
			return nil, errors.New(ErrFormat)
		}
		if err := p.WG.Validate(); err != nil {
			return nil, err
		}
		p.SSH = nil
		return &p, nil
	}
	return nil, errors.New(ErrFormat)
}

// encode — каноническая строка профиля для хранения: ссылка veil:// для Veil, JSON для WireGuard/AmneziaWG.
func encode(p *config.Profile) (string, error) {
	if p.IsVeil() {
		return config.EncodeLink(p)
	}
	b, err := json.Marshal(p)
	return string(b), err
}
