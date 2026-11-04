package deploy

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"veil/internal/wgconf"
	"veil/internal/wgkey"
)

// Обычный WireGuard на сервере: интерфейс veilwg, конфиг /etc/wireguard/veilwg.conf,
// служба wg-quick@veilwg. Отдельное имя, чтобы не задеть чужие настройки wg0.
const (
	WGIface         = "veilwg"
	wgConf          = "/etc/wireguard/" + WGIface + ".conf"
	WGDefaultSubnet = "10.8.1.0/24"
	wgClientMTU     = 1380
)

// WGInstall — параметры установки.
type WGInstall struct {
	Port   int
	Subnet string // IPv4-подсеть интерфейса, по умолчанию 10.8.1.0/24
}

const wgInstallScript = `set -e
IFACE=@IFACE@
CONF=@CONF@
if [ ! -f "$CONF" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq wireguard wireguard-tools iptables
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y wireguard-tools iptables
  elif command -v yum >/dev/null 2>&1; then
    yum install -y epel-release || true
    yum install -y wireguard-tools iptables
  else
    echo "не найден поддерживаемый менеджер пакетов (apt, dnf, yum)" >&2
    exit 1
  fi
  mkdir -p /etc/wireguard
  umask 077
  PRIV=$(wg genkey)
  NIC=$(ip -o -4 route show to default | awk '{print $5}' | head -1)
  [ -n "$NIC" ] || { echo "не найден сетевой интерфейс по умолчанию" >&2; exit 1; }
  cat > "$CONF" <<WGEOF
[Interface]
Address = @SERVER@
ListenPort = @PORT@
PrivateKey = $PRIV
PostUp = iptables -t nat -A POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
WGEOF
  chmod 600 "$CONF"
  echo 'net.ipv4.ip_forward=1' > /etc/sysctl.d/99-veilwg.conf
  sysctl -q -p /etc/sysctl.d/99-veilwg.conf || sysctl -qw net.ipv4.ip_forward=1
fi
PORT=$(awk -F' *= *' '/^ListenPort/{print $2}' "$CONF")
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
  ufw allow "$PORT"/udp >/dev/null
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="$PORT"/udp >/dev/null && firewall-cmd --reload >/dev/null
fi
systemctl enable "wg-quick@$IFACE" >/dev/null 2>&1 || true
systemctl restart "wg-quick@$IFACE"
sleep 1
systemctl is-active --quiet "wg-quick@$IFACE" || { journalctl -u "wg-quick@$IFACE" -n 20 --no-pager >&2; exit 1; }
echo "PORT=$PORT"
echo "PUBKEY=$(wg show $IFACE public-key)"
`

var wgSubnetRe = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+/\d+$`)

// ValidateWGInstall проверяет параметры и возвращает адрес сервера в подсети (первый хост).
func ValidateWGInstall(o WGInstall) (WGInstall, string, error) {
	if o.Port < 1 || o.Port > 65535 {
		return o, "", errors.New("порт должен быть от 1 до 65535")
	}
	if o.Subnet == "" {
		o.Subnet = WGDefaultSubnet
	}
	if !wgSubnetRe.MatchString(o.Subnet) {
		return o, "", errors.New("подсеть должна быть вида 10.8.1.0/24")
	}
	p, err := netip.ParsePrefix(o.Subnet)
	if err != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 29 {
		return o, "", errors.New("нужна IPv4-подсеть с маской от /16 до /29")
	}
	p = p.Masked()
	o.Subnet = p.String()
	srv := p.Addr().Next()
	return o, fmt.Sprintf("%s/%d", srv, p.Bits()), nil
}

func wgScript(o WGInstall, serverAddr string) string {
	return strings.NewReplacer(
		"@IFACE@", WGIface, "@CONF@", wgConf, "@PORT@", strconv.Itoa(o.Port),
		"@SUBNET@", o.Subnet, "@SERVER@", serverAddr,
	).Replace(wgInstallScript)
}

// WireGuardInstalled — есть ли на сервере установка Veil-WireGuard.
func (s *Session) WireGuardInstalled() bool {
	_, err := s.r.exec("test -f "+wgConf, nil, true)
	return err == nil
}

// InstallWireGuard ставит WireGuard на сервер (если ещё не стоит) и выдаёт первого клиента.
func (s *Session) InstallWireGuard(o WGInstall, host string, logf Logf) (*wgconf.Config, error) {
	o, srvAddr, err := ValidateWGInstall(o)
	if err != nil {
		return nil, err
	}
	logf("Установка WireGuard на сервер…")
	out, err := s.r.exec(wgScript(o, srvAddr), nil, true)
	if err != nil {
		return nil, fmt.Errorf("установка WireGuard: %w", err)
	}
	info := parseKV(out)
	port, _ := strconv.Atoi(info["PORT"])
	if port == 0 || info["PUBKEY"] == "" {
		return nil, fmt.Errorf("установка не вернула порт и ключ сервера: %q", out)
	}
	logf("WireGuard запущен, порт %d/UDP", port)
	return s.WireGuardAddClient("Клиент 1", host)
}

func parseKV(out string) map[string]string {
	m := map[string]string{}
	for _, ln := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(ln), "="); ok && k != "" && strings.ToUpper(k) == k {
			m[k] = strings.TrimSpace(v)
		}
	}
	return m
}

// freeClientIP возвращает следующий свободный адрес клиента в подсети интерфейса из текста конфига сервера.
func freeClientIP(conf string) (netip.Addr, netip.Prefix, error) {
	var iface netip.Prefix
	used := map[netip.Addr]bool{}
	for _, ln := range strings.Split(strings.ReplaceAll(conf, "\r", ""), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), "=")
		if !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		switch k {
		case "address":
			for _, f := range strings.Split(v, ",") {
				if p, err := netip.ParsePrefix(strings.TrimSpace(f)); err == nil && p.Addr().Is4() {
					iface = p
					used[p.Addr()] = true
				}
			}
		case "allowedips":
			for _, f := range strings.Split(v, ",") {
				if p, err := netip.ParsePrefix(strings.TrimSpace(f)); err == nil && p.Addr().Is4() && p.Bits() == 32 {
					used[p.Addr()] = true
				}
			}
		}
	}
	if !iface.IsValid() {
		return netip.Addr{}, iface, errors.New("в конфиге сервера нет IPv4-адреса интерфейса")
	}
	net := iface.Masked()
	for a := net.Addr().Next(); net.Contains(a); a = a.Next() {
		// последний адрес подсети — широковещательный
		if nx := a.Next(); !net.Contains(nx) {
			break
		}
		if !used[a] {
			return a, iface, nil
		}
	}
	return netip.Addr{}, iface, errors.New("в подсети WireGuard закончились свободные адреса")
}

func newPSK() (string, error) {
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k[:]), nil
}

var wgNameBad = regexp.MustCompile(`[^\p{L}\p{N} ._@-]`)

// WireGuardAddClient добавляет клиента на сервер и возвращает его готовую конфигурацию.
func (s *Session) WireGuardAddClient(name, host string) (*wgconf.Config, error) {
	conf, err := s.r.exec("cat "+wgConf, nil, true)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать конфиг WireGuard на сервере: %w", err)
	}
	ip, iface, err := freeClientIP(conf)
	if err != nil {
		return nil, err
	}
	var port int
	for _, ln := range strings.Split(conf, "\n") {
		if k, v, ok := strings.Cut(ln, "="); ok && strings.EqualFold(strings.TrimSpace(k), "ListenPort") {
			port, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if port == 0 {
		return nil, errors.New("в конфиге сервера нет ListenPort")
	}
	srvPub, err := s.r.exec("wg show "+WGIface+" public-key", nil, true)
	if err != nil {
		return nil, fmt.Errorf("служба WireGuard не запущена: %w", err)
	}
	priv, pub, err := wgkey.Generate()
	if err != nil {
		return nil, err
	}
	psk, err := newPSK()
	if err != nil {
		return nil, err
	}
	label := strings.TrimSpace(wgNameBad.ReplaceAllString(name, ""))
	if label == "" {
		label = "client"
	}
	peer := fmt.Sprintf("\n# veil: %s\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s/32\n", label, pub, psk, ip)
	if _, err := s.r.exec("cat >> "+wgConf, []byte(peer), true); err != nil {
		return nil, err
	}
	apply := "wg-quick strip " + WGIface + " > /tmp/veilwg.sync && wg syncconf " + WGIface + " /tmp/veilwg.sync; rm -f /tmp/veilwg.sync"
	if _, err := s.r.exec(apply, nil, true); err != nil {
		return nil, fmt.Errorf("не удалось применить клиента: %w", err)
	}
	endpoint := fmt.Sprintf("%s:%d", host, port)
	if a, err := netip.ParseAddr(host); err == nil && a.Is6() {
		endpoint = fmt.Sprintf("[%s]:%d", host, port)
	}
	cfg := &wgconf.Config{
		PrivateKey: priv, Address: []netip.Prefix{netip.PrefixFrom(ip, 32)},
		DNS: []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("1.0.0.1")}, MTU: wgClientMTU,
		PublicKey: strings.TrimSpace(srvPub), PresharedKey: psk, Endpoint: endpoint,
		AllowedIPs: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}, Keepalive: 25,
	}
	_ = iface
	return cfg, cfg.Validate()
}

// RemoveWireGuard останавливает WireGuard и удаляет настройки Veil-WireGuard (пакеты остаются).
func (s *Session) RemoveWireGuard() error {
	const script = `systemctl disable --now wg-quick@` + WGIface + ` >/dev/null 2>&1 || true
rm -f ` + wgConf + ` /etc/sysctl.d/99-veilwg.conf`
	_, err := s.r.exec(script, nil, true)
	return err
}
