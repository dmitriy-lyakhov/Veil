package deploy

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"strconv"
	"strings"

	"veil/internal/vpnconf/wgconf"
	"veil/internal/vpnconf/wgkey"
)

// AmneziaWG на сервере: интерфейс veilawg, конфиг /etc/amnezia/amneziawg/veilawg.conf, служба
// awg-quick@veilawg. Отдельно от WireGuard (veilwg): можно держать оба на одном сервере.
const (
	AWGIface         = "veilawg"
	awgConf          = "/etc/amnezia/amneziawg/" + AWGIface + ".conf"
	AWGDefaultSubnet = "10.9.1.0/24"
	awgClientMTU     = 1280
)

// AWGInstall — параметры установки AmneziaWG. Незаданные Jc/Jmin/Jmax, S1/S2 и H1–H4 заменяются
// значениями по умолчанию (Jc=4, Jmin=10, Jmax=50, S1/S2 и H1–H4 случайные).
type AWGInstall struct {
	Port   int
	Subnet string // IPv4-подсеть интерфейса, по умолчанию 10.9.1.0/24
	AWG    wgconf.AWG
}

const awgInstallScript = `set -e
IFACE=@IFACE@
CONF=@CONF@
if [ ! -f "$CONF" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq software-properties-common gnupg curl iptables
    apt-get install -y -qq "linux-headers-$(uname -r)" || echo "заголовки ядра не найдены, модуль может не собраться" >&2
    . /etc/os-release
    if [ "$ID" = "ubuntu" ]; then
      add-apt-repository -y ppa:amnezia/ppa
    else
      # Debian и производные: тот же PPA, собранный для Ubuntu focal
      mkdir -p /etc/apt/keyrings
      gpg --batch --keyserver hkps://keyserver.ubuntu.com --recv-keys 75C9DD72C799870E310542E24166F2C257290828
      gpg --batch --export 75C9DD72C799870E310542E24166F2C257290828 > /etc/apt/keyrings/amnezia-ppa.gpg
      echo "deb [signed-by=/etc/apt/keyrings/amnezia-ppa.gpg] https://ppa.launchpadcontent.net/amnezia/ppa/ubuntu focal main" > /etc/apt/sources.list.d/amnezia-ppa.list
    fi
    apt-get update -qq
    apt-get install -y -qq amneziawg
  else
    echo "AmneziaWG ставится только на Ubuntu и Debian (нужен apt)" >&2
    exit 1
  fi
  modprobe amneziawg || { echo "не удалось загрузить модуль ядра amneziawg (нужны заголовки ядра linux-headers-$(uname -r))" >&2; exit 1; }
  mkdir -p /etc/amnezia/amneziawg
  umask 077
  PRIV=$(awg genkey)
  NIC=$(ip -o -4 route show to default | awk '{print $5}' | head -1)
  [ -n "$NIC" ] || { echo "не найден сетевой интерфейс по умолчанию" >&2; exit 1; }
  cat > "$CONF" <<AWGEOF
[Interface]
Address = @SERVER@
ListenPort = @PORT@
PrivateKey = $PRIV
@OBFS@
PostUp = iptables -t nat -A POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -A FORWARD -i %i -j ACCEPT; iptables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
AWGEOF
  chmod 600 "$CONF"
  echo 'net.ipv4.ip_forward=1' > /etc/sysctl.d/99-veilawg.conf
  sysctl -q -p /etc/sysctl.d/99-veilawg.conf || sysctl -qw net.ipv4.ip_forward=1
fi
PORT=$(awk -F' *= *' '/^ListenPort/{print $2}' "$CONF")
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
  ufw allow "$PORT"/udp >/dev/null
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="$PORT"/udp >/dev/null && firewall-cmd --reload >/dev/null
fi
systemctl enable "awg-quick@$IFACE" >/dev/null 2>&1 || true
systemctl restart "awg-quick@$IFACE"
sleep 1
systemctl is-active --quiet "awg-quick@$IFACE" || { journalctl -u "awg-quick@$IFACE" -n 20 --no-pager >&2; exit 1; }
echo "PORT=$PORT"
echo "PUBKEY=$(awg show $IFACE public-key)"
`

func randInt(lo, hi int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(hi-lo+1)))
	if err != nil {
		panic(err) // источник случайности ОС недоступен: продолжать небезопасно
	}
	return lo + int(n.Int64())
}

// DefaultAWG — параметры обфускации по умолчанию: Jc=4, Jmin=10, Jmax=50 и свежие случайные S1, S2, H1–H4.
func DefaultAWG() wgconf.AWG {
	a := wgconf.AWG{Jc: 4, Jmin: 10, Jmax: 50}
	for {
		a.S1, a.S2 = randInt(15, 150), randInt(15, 150)
		if a.S1+56 != a.S2 {
			break
		}
	}
	seen := map[int]bool{}
	var h [4]string
	for i := range h {
		for {
			v := randInt(5, 2147483647)
			if !seen[v] {
				seen[v] = true
				h[i] = strconv.Itoa(v)
				break
			}
		}
	}
	a.H1, a.H2, a.H3, a.H4 = h[0], h[1], h[2], h[3]
	return a
}

// ValidateAWGInstall проверяет параметры, подставляет значения по умолчанию и возвращает адрес сервера в подсети.
func ValidateAWGInstall(o AWGInstall) (AWGInstall, string, error) {
	if o.Subnet == "" {
		o.Subnet = AWGDefaultSubnet
	}
	w, srv, err := ValidateWGInstall(WGInstall{Port: o.Port, Subnet: o.Subnet})
	if err != nil {
		return o, "", err
	}
	o.Subnet = w.Subnet
	def := DefaultAWG()
	a := &o.AWG
	if a.Jc == 0 && a.Jmin == 0 && a.Jmax == 0 {
		a.Jc, a.Jmin, a.Jmax = def.Jc, def.Jmin, def.Jmax
	}
	if a.S1 == 0 && a.S2 == 0 {
		a.S1, a.S2 = def.S1, def.S2
	}
	if a.H1 == "" && a.H2 == "" && a.H3 == "" && a.H4 == "" {
		a.H1, a.H2, a.H3, a.H4 = def.H1, def.H2, def.H3, def.H4
	}
	if a.H1 == "" || a.H2 == "" || a.H3 == "" || a.H4 == "" {
		return o, "", errors.New("H1–H4 нужно задать все сразу или не задавать совсем")
	}
	if err := a.Validate(); err != nil {
		return o, "", err
	}
	for _, i := range a.I {
		if i != "" {
			return o, "", errors.New("сигнатурные пакеты I1–I5 задаются только на стороне клиента")
		}
	}
	return o, srv, nil
}

// obfsLines — строки параметров обфускации для [Interface] серверного конфига.
func obfsLines(a wgconf.AWG) string {
	var lines []string
	add := func(k string, v int) {
		if v != 0 {
			lines = append(lines, fmt.Sprintf("%s = %d", k, v))
		}
	}
	add("Jc", a.Jc)
	add("Jmin", a.Jmin)
	add("Jmax", a.Jmax)
	add("S1", a.S1)
	add("S2", a.S2)
	add("S3", a.S3)
	add("S4", a.S4)
	for i, h := range []string{a.H1, a.H2, a.H3, a.H4} {
		if h != "" {
			lines = append(lines, fmt.Sprintf("H%d = %s", i+1, h))
		}
	}
	return strings.Join(lines, "\n")
}

func awgScript(o AWGInstall, serverAddr string) string {
	return strings.NewReplacer(
		"@IFACE@", AWGIface, "@CONF@", awgConf, "@PORT@", strconv.Itoa(o.Port),
		"@SUBNET@", o.Subnet, "@SERVER@", serverAddr, "@OBFS@", obfsLines(o.AWG),
	).Replace(awgInstallScript)
}

// AmneziaWGInstalled — есть ли на сервере установка Veil-AmneziaWG.
func (s *Session) AmneziaWGInstalled() bool {
	if s.inDocker("amneziawg") {
		return true
	}
	_, err := s.r.exec("test -f "+awgConf, nil, true)
	return err == nil
}

// awgEnv — переменные compose для контейнера AmneziaWG с параметрами, выбранными в приложении.
func awgEnv(o AWGInstall, host string) map[string]string {
	a := o.AWG
	env := map[string]string{
		"AWG_PORT": strconv.Itoa(o.Port), "AWG_SUBNET": o.Subnet, "VEIL_HOST": host, "AWG_FIRST_CLIENT": "", "VEIL_DETECT_IP": "0",
		"AWG_JC": strconv.Itoa(a.Jc), "AWG_JMIN": strconv.Itoa(a.Jmin), "AWG_JMAX": strconv.Itoa(a.Jmax),
		"AWG_S1": strconv.Itoa(a.S1), "AWG_S2": strconv.Itoa(a.S2),
		"AWG_H1": a.H1, "AWG_H2": a.H2, "AWG_H3": a.H3, "AWG_H4": a.H4,
	}
	if a.S3 != 0 {
		env["AWG_S3"] = strconv.Itoa(a.S3)
	}
	if a.S4 != 0 {
		env["AWG_S4"] = strconv.Itoa(a.S4)
	}
	return env
}

// InstallAmneziaWG ставит AmneziaWG на сервер (если ещё не стоит) и выдаёт первого клиента.
// По возможности — в контейнер veil-amneziawg (amneziawg-go, модуль ядра не нужен), иначе из PPA Amnezia.
func (s *Session) InstallAmneziaWG(o AWGInstall, host string, logf Logf) (*wgconf.Config, error) {
	o, srvAddr, err := ValidateAWGInstall(o)
	if err != nil {
		return nil, err
	}
	if s.inDocker("amneziawg") {
		logf("AmneziaWG уже работает в контейнере — выдаю клиента")
		return s.AmneziaWGAddClient("Клиент 1", host)
	}
	if strings.HasSuffix(o.Subnet, "/24") {
		docker, err := s.tryDocker("amneziawg", "AmneziaWG", "test -f "+awgConf, awgEnv(o, host), logf)
		if err != nil {
			return nil, err
		}
		if docker {
			logf("AmneziaWG запущен в контейнере %s, порт %d/UDP", containers["amneziawg"].name, o.Port)
			return s.AmneziaWGAddClient("Клиент 1", host)
		}
	}
	logf("Установка AmneziaWG на сервер (может занять несколько минут: собирается модуль ядра)…")
	out, err := s.r.exec(awgScript(o, srvAddr), nil, true)
	if err != nil {
		return nil, fmt.Errorf("установка AmneziaWG: %w", err)
	}
	info := parseKV(out)
	port, _ := strconv.Atoi(info["PORT"])
	if port == 0 || info["PUBKEY"] == "" {
		return nil, fmt.Errorf("установка не вернула порт и ключ сервера: %q", out)
	}
	logf("AmneziaWG запущен, порт %d/UDP", port)
	return s.AmneziaWGAddClient("Клиент 1", host)
}

// AmneziaWGAddClient добавляет клиента на сервер и возвращает его готовую конфигурацию
// (с теми же Jc/Jmin/Jmax, S1, S2, H1–H4, что на сервере).
func (s *Session) AmneziaWGAddClient(name, host string) (*wgconf.Config, error) {
	r := s.proto("amneziawg")
	conf, err := r.exec("cat "+awgConf, nil, true)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать конфиг AmneziaWG на сервере: %w", err)
	}
	ip, iface, err := freeClientIP(conf)
	if err != nil {
		return nil, err
	}
	obfs, port := wgconf.ParseAWGInterface(conf)
	if port == 0 {
		return nil, errors.New("в конфиге сервера нет ListenPort")
	}
	srvPub, err := r.exec("awg show "+AWGIface+" public-key", nil, true)
	if err != nil {
		return nil, fmt.Errorf("служба AmneziaWG не запущена: %w", err)
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
	allowed, addrs, dns := clientAddrs(conf, ip, iface)
	peer := fmt.Sprintf("\n# veil: %s\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s\n", label, pub, psk, allowed)
	if _, err := r.exec("cat >> "+awgConf, []byte(peer), true); err != nil {
		return nil, err
	}
	apply := "awg-quick strip " + AWGIface + " > /tmp/veilawg.sync && awg syncconf " + AWGIface + " /tmp/veilawg.sync; rm -f /tmp/veilawg.sync"
	if _, err := r.exec(apply, nil, true); err != nil {
		return nil, fmt.Errorf("не удалось применить клиента: %w", err)
	}
	cfg := &wgconf.Config{
		PrivateKey: priv, Address: addrs, DNS: dns, MTU: awgClientMTU,
		PublicKey: strings.TrimSpace(srvPub), PresharedKey: psk, Endpoint: endpointFor(host, port),
		AllowedIPs: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}, Keepalive: 25,
		AWG: obfs,
	}
	return cfg, cfg.Validate()
}

// RemoveAmneziaWG останавливает AmneziaWG и удаляет настройки Veil-AmneziaWG (пакеты остаются).
func (s *Session) RemoveAmneziaWG() error {
	if s.inDocker("amneziawg") {
		return s.r.removeContainer("amneziawg")
	}
	const script = `systemctl disable --now awg-quick@` + AWGIface + ` >/dev/null 2>&1 || true
rm -f ` + awgConf + ` /etc/sysctl.d/99-veilawg.conf`
	_, err := s.r.exec(script, nil, true)
	return err
}
