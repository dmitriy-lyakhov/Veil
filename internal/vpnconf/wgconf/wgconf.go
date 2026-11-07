// Package wgconf разбирает и формирует настройки WireGuard в стандартном формате .conf.
package wgconf

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// Config — настройки клиента WireGuard: свой интерфейс и один сервер (peer).
type Config struct {
	PrivateKey   string         `json:"private_key"`
	Address      []netip.Prefix `json:"address"`
	DNS          []netip.Addr   `json:"dns,omitempty"`
	MTU          int            `json:"mtu,omitempty"`
	PublicKey    string         `json:"public_key"` // сервера
	PresharedKey string         `json:"preshared_key,omitempty"`
	Endpoint     string         `json:"endpoint"` // host:port
	AllowedIPs   []netip.Prefix `json:"allowed_ips,omitempty"`
	Keepalive    int            `json:"keepalive,omitempty"`
	// AWG — параметры AmneziaWG; nil для обычного WireGuard.
	AWG *AWG `json:"awg,omitempty"`
}

var allTraffic = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}

// Parse разбирает текст .conf. Неизвестные ключи (например, параметры AmneziaWG) отвергаются
// явно, чтобы не подключаться по обычному WireGuard к серверу, которому нужны они.
func Parse(text string) (*Config, error) { return parse(text, false) }

// ParseAWG разбирает .conf AmneziaWG: то же, что Parse, плюс Jc, Jmin, Jmax, S1…S4, H1…H4, I1…I5.
func ParseAWG(text string) (*Config, error) { return parse(text, true) }

func parse(text string, awg bool) (*Config, error) {
	c := &Config{}
	section := ""
	peers := 0
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line := strings.TrimSpace(raw)
		if j := strings.IndexAny(line, "#;"); j == 0 {
			continue
		} else if j > 0 {
			line = strings.TrimSpace(line[:j])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "peer" {
				peers++
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("строка %d: ожидалось «ключ = значение»", i+1)
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		var err error
		switch section + "." + k {
		case "interface.privatekey":
			c.PrivateKey = v
		case "interface.address":
			c.Address, err = parsePrefixes(v, true)
		case "interface.dns":
			for _, f := range splitList(v) {
				if a, e := netip.ParseAddr(f); e == nil {
					c.DNS = append(c.DNS, a)
				}
			}
		case "interface.mtu":
			c.MTU, err = strconv.Atoi(v)
		case "peer.publickey":
			c.PublicKey = v
		case "peer.presharedkey":
			c.PresharedKey = v
		case "peer.endpoint":
			c.Endpoint = v
		case "peer.allowedips":
			c.AllowedIPs, err = parsePrefixes(v, false)
		case "peer.persistentkeepalive":
			c.Keepalive, err = strconv.Atoi(v)
		case "interface.listenport", "interface.table", "interface.preup", "interface.postup", "interface.predown", "interface.postdown", "interface.saveconfig", "interface.fwmark":
			// параметры серверной стороны и маршрутизации wg-quick: для клиента Veil не нужны
		case "interface.jc", "interface.jmin", "interface.jmax", "interface.s1", "interface.s2", "interface.s3", "interface.s4",
			"interface.h1", "interface.h2", "interface.h3", "interface.h4",
			"interface.i1", "interface.i2", "interface.i3", "interface.i4", "interface.i5":
			if !awg {
				return nil, errors.New("это конфигурация AmneziaWG (есть параметры Jc/Jmin/S1…): обычный WireGuard к такому серверу не подключится")
			}
			if c.AWG == nil {
				c.AWG = &AWG{}
			}
			err = c.AWG.set(k, v)
		default:
			if section == "" {
				return nil, fmt.Errorf("строка %d: настройка вне раздела", i+1)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("строка %d (%s): %w", i+1, k, err)
		}
	}
	if peers != 1 {
		return nil, fmt.Errorf("в конфигурации должен быть ровно один [Peer], найдено: %d", peers)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := c.AWG.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

func parsePrefixes(s string, hostBits bool) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, f := range splitList(s) {
		if p, err := netip.ParsePrefix(f); err == nil {
			if !hostBits {
				p = p.Masked()
			}
			out = append(out, p)
			continue
		}
		a, err := netip.ParseAddr(f)
		if err != nil {
			return nil, fmt.Errorf("«%s» — не адрес и не подсеть", f)
		}
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// Validate проверяет обязательные поля.
func (c *Config) Validate() error {
	switch {
	case c.PrivateKey == "":
		return errors.New("нет PrivateKey в разделе [Interface]")
	case c.PublicKey == "":
		return errors.New("нет PublicKey в разделе [Peer]")
	case len(c.Address) == 0:
		return errors.New("нет Address в разделе [Interface]")
	case c.Endpoint == "":
		return errors.New("нет Endpoint в разделе [Peer]")
	}
	if _, _, err := net.SplitHostPort(c.Endpoint); err != nil {
		return fmt.Errorf("Endpoint «%s» должен быть вида адрес:порт", c.Endpoint)
	}
	return nil
}

// Routes — что направлять в туннель (по умолчанию весь трафик).
func (c *Config) Routes() []netip.Prefix {
	if len(c.AllowedIPs) == 0 {
		return allTraffic
	}
	return c.AllowedIPs
}

// EndpointHostPort разделяет Endpoint на адрес и порт.
func (c *Config) EndpointHostPort() (string, int, error) {
	h, p, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		return "", 0, err
	}
	n, err := strconv.Atoi(p)
	return h, n, err
}

// String формирует стандартный .conf для приложений WireGuard.
func (c *Config) String() string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", c.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", joinPrefixes(c.Address))
	if len(c.DNS) > 0 {
		ds := make([]string, len(c.DNS))
		for i, d := range c.DNS {
			ds[i] = d.String()
		}
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(ds, ", "))
	}
	if c.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", c.MTU)
	}
	if c.AWG != nil {
		c.AWG.write(&b)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", c.PublicKey)
	if c.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", c.PresharedKey)
	}
	fmt.Fprintf(&b, "AllowedIPs = %s\n", joinPrefixes(c.Routes()))
	fmt.Fprintf(&b, "Endpoint = %s\n", c.Endpoint)
	if c.Keepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", c.Keepalive)
	}
	return b.String()
}

func joinPrefixes(ps []netip.Prefix) string {
	ss := make([]string, len(ps))
	for i, p := range ps {
		ss[i] = p.String()
	}
	return strings.Join(ss, ", ")
}
