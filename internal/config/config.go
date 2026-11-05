package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"veil/internal/ipsecconf"
	"veil/internal/ovpnconf"
	"veil/internal/site"
	"veil/internal/wgconf"
)

const (
	DefaultPort      = 443
	DefaultMTU       = 1380
	DefaultInterface = "veil0"
)

type Server struct {
	Listen     string     `json:"listen"`
	Path       string     `json:"path"`
	CertPEM    string     `json:"cert_pem"`
	KeyPEM     string     `json:"key_pem"`
	PrivateKey string     `json:"private_key"`
	Address4   string     `json:"address4"`
	Address6   string     `json:"address6"`
	MTU        int        `json:"mtu"`
	Interface  string     `json:"interface"`
	Peers      []Peer     `json:"peers"`
	PublicIPv4 string     `json:"public_ipv4,omitempty"`
	PublicIPv6 string     `json:"public_ipv6,omitempty"`
	Site       *site.Site `json:"site,omitempty"`
	Admin      Admin      `json:"admin"`
	Domain     string     `json:"domain,omitempty"`
	Front      *Front     `json:"front,omitempty"`
	Relays     []Relay    `json:"relays,omitempty"`
	Upstreams  []Upstream `json:"upstreams,omitempty"`
}

// Front — маскировка входа под чужой сайт: SNI, куда проксировать посторонних и общий ключ (base64).
type Front struct {
	SNI  string `json:"sni"`
	Dest string `json:"dest,omitempty"`
	Key  string `json:"key"`
}

type Admin struct {
	User         string `json:"user,omitempty"`
	PasswordHash string `json:"password_hash,omitempty"`
}

func (s *Server) Port() int {
	_, p, err := net.SplitHostPort(s.Listen)
	if err != nil {
		return DefaultPort
	}
	n, err := strconv.Atoi(p)
	if err != nil || n <= 0 {
		return DefaultPort
	}
	return n
}

type Peer struct {
	Name      string     `json:"name"`
	PublicKey string     `json:"public_key"`
	AuthKey   string     `json:"auth_key"`
	Address4  string     `json:"address4"`
	Address6  string     `json:"address6"`
	Created   time.Time  `json:"created,omitempty"`
	Route     []string   `json:"route,omitempty"`
	UpAuth    string     `json:"up_auth,omitempty"`
	UpName    string     `json:"up_name,omitempty"`
	RelayOf   string     `json:"relay_of,omitempty"`
	Note      string     `json:"note,omitempty"`
	Disabled  bool       `json:"disabled,omitempty"`
	Auto      *AutoRoute `json:"auto,omitempty"`
}

type AutoRoute struct {
	Exit     string    `json:"exit"`
	ExitName string    `json:"exit_name"`
	MaxHops  int       `json:"max_hops"`
	Path     []string  `json:"path"`
	Gen      int       `json:"gen"`
	CostMs   float64   `json:"cost_ms"`
	Rerouted time.Time `json:"rerouted,omitzero"`
}

type Relay struct {
	Name    string    `json:"name"`
	AuthKey string    `json:"auth_key"`
	Created time.Time `json:"created,omitempty"`
}

type Upstream struct {
	Name  string    `json:"name"`
	Link  RelayLink `json:"link"`
	Added time.Time `json:"added,omitempty"`
}

type RelayLink struct {
	Company         string   `json:"company,omitempty"`
	IPv4            string   `json:"ipv4,omitempty"`
	IPv6            string   `json:"ipv6,omitempty"`
	Port            int      `json:"port"`
	Path            string   `json:"path"`
	CertPin         string   `json:"cert_pin"`
	AuthKey         string   `json:"auth_key"`
	ServerPublicKey string   `json:"server_public_key"`
	ServerName      string   `json:"server_name,omitempty"`
	Transports      []string `json:"transports,omitempty"`
	FrontSNI        string   `json:"front_sni,omitempty"`
	FrontKey        string   `json:"front_key,omitempty"`
}

func (l *RelayLink) DialProfile() *Profile {
	return &Profile{
		Name: l.Company, IPv4: l.IPv4, IPv6: l.IPv6, Port: l.Port, Path: l.Path, ServerName: l.ServerName,
		CertPin: l.CertPin, AuthKey: l.AuthKey, ServerPublicKey: l.ServerPublicKey, Transports: l.Transports,
		FrontSNI: l.FrontSNI, FrontKey: l.FrontKey,
	}
}

type Profile struct {
	Name            string   `json:"name"`
	IPv4            string   `json:"ipv4,omitempty"`
	IPv6            string   `json:"ipv6,omitempty"`
	Port            int      `json:"port"`
	ServerName      string   `json:"server_name,omitempty"`
	Path            string   `json:"path"`
	CertPin         string   `json:"cert_pin"`
	ServerPublicKey string   `json:"server_public_key"`
	PrivateKey      string   `json:"private_key"`
	AuthKey         string   `json:"auth_key"`
	Address4        string   `json:"address4"`
	Address6        string   `json:"address6"`
	DNS             []string `json:"dns"`
	MTU             int      `json:"mtu"`
	Transports      []string `json:"transports,omitempty"`
	FrontSNI        string   `json:"front_sni,omitempty"` // SNI, под который маскируется вход
	FrontKey        string   `json:"front_key,omitempty"` // общий ключ метки (base64)
	AdminUser       string   `json:"admin_user,omitempty"`
	AdminPassword   string   `json:"admin_password,omitempty"`
	AdminURL        string   `json:"admin_url,omitempty"`
	Via             string   `json:"via,omitempty"`
	ExitName        string   `json:"exit_name,omitempty"`
	Entries         []Entry  `json:"entries,omitempty"`
	PreferredEntry  string   `json:"preferred_entry,omitempty"`
	SSH             *SSHInfo `json:"ssh,omitempty"`

	// Protocol — протокол профиля: пусто или "veil" (по умолчанию), "wireguard", "openvpn".
	Protocol  string            `json:"protocol,omitempty"`
	WG        *wgconf.Config    `json:"wg,omitempty"`
	IPsec     *ipsecconf.Config `json:"ipsec,omitempty"`
	OVPN      *ovpnconf.Config  `json:"ovpn,omitempty"`
	ServerRef string            `json:"server_ref,omitempty"` // имя профиля сервера, с которого поставлен протокол
}

// IsVeil — профиль работает по протоколу Veil (а не по одному из обычных протоколов).
func (p *Profile) IsVeil() bool { return p.Protocol == "" || p.Protocol == "veil" }

type Entry struct {
	Name       string   `json:"name"`
	IPv4       string   `json:"ipv4,omitempty"`
	IPv6       string   `json:"ipv6,omitempty"`
	Port       int      `json:"port"`
	ServerName string   `json:"server_name,omitempty"`
	Path       string   `json:"path"`
	CertPin    string   `json:"cert_pin,omitempty"`
	AuthKey    string   `json:"auth_key"`
	Via        string   `json:"via,omitempty"`
	Transports []string `json:"transports,omitempty"`
	FrontSNI   string   `json:"front_sni,omitempty"`
	FrontKey   string   `json:"front_key,omitempty"`
}

func (p *Profile) ForEntry(e Entry) *Profile {
	c := *p
	c.Entries = nil
	c.IPv4, c.IPv6, c.Port, c.ServerName, c.Path, c.CertPin, c.AuthKey = e.IPv4, e.IPv6, e.Port, e.ServerName, e.Path, e.CertPin, e.AuthKey
	c.FrontSNI, c.FrontKey = e.FrontSNI, e.FrontKey
	if len(e.Transports) > 0 {
		c.Transports = e.Transports
	}
	if e.Via != "" {
		c.Via = e.Via
	}
	return &c
}

func (p *Profile) AllTargets() ([]Target, error) {
	if len(p.Entries) == 0 {
		return p.Targets()
	}
	var out []Target
	for _, e := range p.Entries {
		t, err := p.ForEntry(e).Targets()
		if err != nil {
			return nil, fmt.Errorf("вход %s: %w", e.Name, err)
		}
		out = append(out, t...)
	}
	return out, nil
}

type SSHInfo struct {
	Port    int    `json:"port"`
	User    string `json:"user"`
	HostKey string `json:"host_key"`
	KeyPath string `json:"key_path,omitempty"`
}

type Target struct {
	Family string
	Addr   netip.AddrPort
}

func (p *Profile) Targets() ([]Target, error) {
	port := p.Port
	if port == 0 {
		port = DefaultPort
	}
	var out []Target
	if p.IPv6 != "" {
		a, err := netip.ParseAddr(p.IPv6)
		if err != nil || !a.Is6() || a.Is4In6() {
			return nil, fmt.Errorf("некорректный IPv6-адрес %q", p.IPv6)
		}
		out = append(out, Target{"IPv6", netip.AddrPortFrom(a, uint16(port))})
	}
	if p.IPv4 != "" {
		a, err := netip.ParseAddr(p.IPv4)
		if err != nil || !a.Is4() {
			return nil, fmt.Errorf("некорректный IPv4-адрес %q", p.IPv4)
		}
		out = append(out, Target{"IPv4", netip.AddrPortFrom(a, uint16(port))})
	}
	if len(out) == 0 {
		return nil, errors.New("не указан ни IPv6, ни IPv4 адрес сервера")
	}
	return out, nil
}

func (p *Profile) HostHeader(t Target) string {
	if p.ServerName != "" {
		return p.ServerName
	}
	if t.Addr.Port() == DefaultPort {
		if t.Addr.Addr().Is6() {
			return "[" + t.Addr.Addr().String() + "]"
		}
		return t.Addr.Addr().String()
	}
	return t.Addr.String()
}

func (p *Profile) EffectiveMTU() int {
	if p.MTU > 0 {
		return p.MTU
	}
	return DefaultMTU
}

func Load[T any](path string) (*T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v := new(T)
	if err := json.Unmarshal(b, v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

func Save(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ProfilesDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Veil", "profiles"), nil
}

func ListProfiles() ([]*Profile, error) {
	dir, err := ProfilesDir()
	if err != nil {
		return nil, err
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var out []*Profile
	for _, m := range matches {
		p, err := Load[Profile](m)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func ProfilePath(name string) (string, error) {
	dir, err := ProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, SafeFileName(name)+".json"), nil
}

func SafeFileName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == 0 || r == '.' && len(out) == 0:
			out = append(out, '_')
		case runtime.GOOS == "windows" && strings.ContainsRune(`|*?"<>`, r):
			// символы, недопустимые в именах файлов Windows
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "server"
	}
	return string(out)
}

// TLSName — SNI для TLS и QUIC: маскировочный, если задан, иначе имя сервера.
func (p *Profile) TLSName() string {
	if p.FrontSNI != "" {
		return p.FrontSNI
	}
	return p.ServerName
}
