package deploy

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"veil/internal/vpnconf/ovpnconf"
)

// OpenVPN на сервере: PKI easy-rsa в /etc/openvpn/veil, конфиг /etc/openvpn/server/veil.conf,
// служба openvpn-server@veil, NAT — отдельной службой veil-openvpn-nat (чужие правила не трогаются).
const (
	ovDir           = "/etc/openvpn/veil"
	ovConf          = "/etc/openvpn/server/veil.conf"
	ovService       = "openvpn-server@veil"
	OVDefaultSubnet = "10.9.0.0/24"
)

// OVInstall — параметры установки OpenVPN.
type OVInstall struct {
	Port   int
	Proto  string // "udp" или "tcp"
	Subnet string // IPv4-подсеть клиентов, по умолчанию 10.9.0.0/24
}

const ovInstallScript = `set -e
DIR=@DIR@
CONF=@CONF@
if [ ! -f "$CONF" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq openvpn easy-rsa iptables openssl
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y epel-release || true
    dnf install -y openvpn easy-rsa iptables openssl
  elif command -v yum >/dev/null 2>&1; then
    yum install -y epel-release || true
    yum install -y openvpn easy-rsa iptables openssl
  else
    echo "не найден поддерживаемый менеджер пакетов (apt, dnf, yum)" >&2
    exit 1
  fi
  ERSA=""
  for c in /usr/share/easy-rsa/easyrsa /usr/share/easy-rsa/3/easyrsa /usr/share/easy-rsa/3.0/easyrsa; do
    [ -x "$c" ] && ERSA="$c" && break
  done
  [ -n "$ERSA" ] || { echo "не найден easy-rsa после установки" >&2; exit 1; }
  NIC=$(ip -o -4 route show to default | awk '{print $5}' | head -1)
  [ -n "$NIC" ] || { echo "не найден сетевой интерфейс по умолчанию" >&2; exit 1; }
  umask 077
  mkdir -p "$DIR" /etc/openvpn/server
  cp "$ERSA" "$DIR/easyrsa"
  for d in "$(dirname "$ERSA")"/x509-types "$(dirname "$ERSA")"/openssl-easyrsa.cnf "$(dirname "$ERSA")"/vars.example; do
    [ -e "$d" ] && cp -r "$d" "$DIR/" || true
  done
  cd "$DIR"
  export EASYRSA_BATCH=1 EASYRSA_ALGO=ec EASYRSA_CURVE=prime256v1 EASYRSA_CA_EXPIRE=3650 EASYRSA_CERT_EXPIRE=3650 EASYRSA_CRL_DAYS=3650
  export EASYRSA_REQ_CN=Veil-CA
  ./easyrsa init-pki >/dev/null
  ./easyrsa build-ca nopass >/dev/null 2>&1
  ./easyrsa build-server-full server nopass >/dev/null 2>&1
  ./easyrsa gen-crl >/dev/null 2>&1
  (openvpn --genkey secret "$DIR/tls-crypt.key" 2>/dev/null) || openvpn --genkey --secret "$DIR/tls-crypt.key"
  cp pki/crl.pem "$DIR/crl.pem"
  chmod 644 "$DIR/crl.pem"
  GRP=nogroup
  getent group nogroup >/dev/null 2>&1 || GRP=nobody
  EXTRA=""
  [ "@PROTO@" = "udp" ] && EXTRA="explicit-exit-notify 1"
  cat > "$CONF" <<OVEOF
port @PORT@
proto @PROTO@
dev tun
topology subnet
server @NET@ @MASK@
ca $DIR/pki/ca.crt
cert $DIR/pki/issued/server.crt
key $DIR/pki/private/server.key
dh none
ecdh-curve prime256v1
tls-crypt $DIR/tls-crypt.key
crl-verify $DIR/crl.pem
data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305
auth SHA256
tls-version-min 1.2
push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 1.0.0.1"
keepalive 10 120
persist-key
persist-tun
user nobody
group $GRP
verb 3
$EXTRA
OVEOF
  chmod 600 "$CONF"
  echo 'net.ipv4.ip_forward=1' > /etc/sysctl.d/99-veilovpn.conf
  sysctl -q -p /etc/sysctl.d/99-veilovpn.conf || sysctl -qw net.ipv4.ip_forward=1
  cat > /etc/systemd/system/veil-openvpn-nat.service <<NATEOF
[Unit]
Description=Veil OpenVPN NAT
After=network-online.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c 'iptables -t nat -C POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -C FORWARD -s @SUBNET@ -j ACCEPT 2>/dev/null || iptables -I FORWARD -s @SUBNET@ -j ACCEPT; iptables -C FORWARD -d @SUBNET@ -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -I FORWARD -d @SUBNET@ -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT'
ExecStop=/bin/sh -c 'iptables -t nat -D POSTROUTING -s @SUBNET@ -o $NIC -j MASQUERADE; iptables -D FORWARD -s @SUBNET@ -j ACCEPT; iptables -D FORWARD -d @SUBNET@ -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT; true'
[Install]
WantedBy=multi-user.target
NATEOF
  systemctl daemon-reload
fi
PORT=$(awk '/^port /{print $2}' "$CONF")
PROTO=$(awk '/^proto /{print $2}' "$CONF")
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
  ufw allow "$PORT"/"$PROTO" >/dev/null
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="$PORT"/"$PROTO" >/dev/null && firewall-cmd --reload >/dev/null
fi
systemctl enable --now veil-openvpn-nat >/dev/null 2>&1 || true
systemctl enable @SVC@ >/dev/null 2>&1 || true
systemctl restart @SVC@
sleep 2
systemctl is-active --quiet @SVC@ || { journalctl -u @SVC@ -n 20 --no-pager >&2; exit 1; }
echo "PORT=$PORT"
echo "PROTO=$PROTO"
`

// ValidateOVInstall проверяет параметры, подставляя значения по умолчанию.
func ValidateOVInstall(o OVInstall) (OVInstall, error) {
	if o.Port < 1 || o.Port > 65535 {
		return o, errors.New("порт должен быть от 1 до 65535")
	}
	if o.Proto == "" {
		o.Proto = "udp"
	}
	if o.Proto != "udp" && o.Proto != "tcp" {
		return o, errors.New("сетевой протокол должен быть UDP или TCP")
	}
	if o.Subnet == "" {
		o.Subnet = OVDefaultSubnet
	}
	if !wgSubnetRe.MatchString(o.Subnet) {
		return o, errors.New("подсеть должна быть вида 10.9.0.0/24")
	}
	p, err := netip.ParsePrefix(o.Subnet)
	if err != nil || !p.Addr().Is4() || p.Bits() < 16 || p.Bits() > 29 {
		return o, errors.New("нужна IPv4-подсеть с маской от /16 до /29")
	}
	o.Subnet = p.Masked().String()
	return o, nil
}

func ovScript(o OVInstall) string {
	p := netip.MustParsePrefix(o.Subnet)
	mask := net.IP(net.CIDRMask(p.Bits(), 32)).String()
	return strings.NewReplacer(
		"@DIR@", ovDir, "@CONF@", ovConf, "@PORT@", strconv.Itoa(o.Port), "@PROTO@", o.Proto,
		"@NET@", p.Addr().String(), "@MASK@", mask, "@SUBNET@", o.Subnet, "@SVC@", ovService,
	).Replace(ovInstallScript)
}

// OVClientConfig собирает клиентский .ovpn с вложенными сертификатами и ключами.
func OVClientConfig(host string, port int, proto, ca, cert, key, tlsCrypt string) string {
	var b strings.Builder
	b.WriteString("client\ndev tun\n")
	fmt.Fprintf(&b, "proto %s\nremote %s %d\n", proto, host, port)
	b.WriteString("resolv-retry infinite\nnobind\npersist-key\npersist-tun\nremote-cert-tls server\n")
	b.WriteString("data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305\nauth SHA256\nverb 3\n")
	if proto == "udp" {
		b.WriteString("explicit-exit-notify 1\n")
	}
	for _, blk := range []struct{ tag, body string }{{"ca", ca}, {"cert", cert}, {"key", key}, {"tls-crypt", tlsCrypt}} {
		fmt.Fprintf(&b, "<%s>\n%s\n</%s>\n", blk.tag, strings.TrimSpace(blk.body), blk.tag)
	}
	return b.String()
}

// extractPEM оставляет только PEM-часть вывода (easy-rsa добавляет к сертификату текстовое описание).
func extractPEM(s string) string {
	i := strings.Index(s, "-----BEGIN")
	j := strings.LastIndex(s, "-----END")
	if i < 0 || j < i {
		return ""
	}
	if k := strings.Index(s[j:], "\n"); k >= 0 {
		return strings.TrimSpace(s[i : j+k])
	}
	return strings.TrimSpace(s[i:])
}

var ovNameBad = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
var ovNameOK = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ovClientName делает из подписи имя сертификата: латиница, цифры, «-», «_» и короткий случайный хвост.
func ovClientName(label string) string {
	base := strings.Trim(ovNameBad.ReplaceAllString(label, "-"), "-_")
	if len(base) > 40 {
		base = base[:40]
	}
	if base == "" {
		base = "client"
	}
	return base + "-" + randomHex(2)
}

// OpenVPNInstalled — есть ли на сервере установка Veil-OpenVPN.
func (s *Session) OpenVPNInstalled() bool {
	if s.inDocker("openvpn") {
		return true
	}
	_, err := s.r.exec("test -f "+ovConf, nil, true)
	return err == nil
}

// InstallOpenVPN ставит OpenVPN на сервер (если ещё не стоит) и выдаёт первого клиента.
// По возможности — в контейнер veil-openvpn (dual-stack), иначе службой openvpn-server@veil.
func (s *Session) InstallOpenVPN(o OVInstall, host string, logf Logf) (*ovpnconf.Config, error) {
	o, err := ValidateOVInstall(o)
	if err != nil {
		return nil, err
	}
	if s.inDocker("openvpn") {
		logf("OpenVPN уже работает в контейнере — выдаю клиента")
		return s.OpenVPNAddClient("Клиент 1", host)
	}
	env := map[string]string{"OVPN_PORT": strconv.Itoa(o.Port), "OVPN_PROTO": o.Proto, "OVPN_SUBNET": o.Subnet,
		"VEIL_HOST": host, "OVPN_FIRST_CLIENT": "", "VEIL_DETECT_IP": "0"}
	docker, err := s.tryDocker("openvpn", "OpenVPN", "test -f "+ovConf, env, logf)
	if err != nil {
		return nil, err
	}
	if docker {
		logf("OpenVPN запущен в контейнере %s, порт %d/%s", containers["openvpn"].name, o.Port, strings.ToUpper(o.Proto))
		return s.OpenVPNAddClient("Клиент 1", host)
	}
	logf("Установка OpenVPN на сервер (создание сертификатов может занять около минуты)…")
	out, err := s.r.exec(ovScript(o), nil, true)
	if err != nil {
		return nil, fmt.Errorf("установка OpenVPN: %w", err)
	}
	info := parseKV(out)
	port, _ := strconv.Atoi(info["PORT"])
	if port == 0 || info["PROTO"] == "" {
		return nil, fmt.Errorf("установка не вернула порт и протокол: %q", out)
	}
	logf("OpenVPN запущен, порт %d/%s", port, strings.ToUpper(info["PROTO"]))
	return s.OpenVPNAddClient("Клиент 1", host)
}

const ovAddScript = `set -e
cd @DIR@
export EASYRSA_BATCH=1 EASYRSA_ALGO=ec EASYRSA_CURVE=prime256v1 EASYRSA_CERT_EXPIRE=3650
./easyrsa build-client-full @NAME@ nopass >/dev/null 2>&1 || { echo "не удалось выпустить сертификат" >&2; exit 1; }
echo "PORT=$(awk '/^port /{print $2}' @CONF@)"
echo "PROTO=$(awk '/^proto /{print $2}' @CONF@)"
echo "@@CA"
cat pki/ca.crt
echo "@@CERT"
cat pki/issued/@NAME@.crt
echo "@@KEY"
cat pki/private/@NAME@.key
echo "@@TC"
cat tls-crypt.key
echo "@@END"
`

func splitSections(out string) map[string]string {
	m := map[string]string{}
	var cur string
	var b strings.Builder
	flush := func() {
		if cur != "" {
			m[cur] = b.String()
		}
		b.Reset()
	}
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "@@") {
			flush()
			cur = strings.TrimSpace(ln[2:])
			continue
		}
		if cur != "" {
			b.WriteString(ln + "\n")
		}
	}
	flush()
	return m
}

// OpenVPNAddClient выпускает сертификат клиента и возвращает готовый .ovpn.
func (s *Session) OpenVPNAddClient(label, host string) (*ovpnconf.Config, error) {
	name := ovClientName(label)
	script := strings.NewReplacer("@DIR@", ovDir, "@CONF@", ovConf, "@NAME@", name).Replace(ovAddScript)
	out, err := s.proto("openvpn").exec(script, nil, true)
	if err != nil {
		return nil, fmt.Errorf("не удалось добавить клиента OpenVPN: %w", err)
	}
	head, _, _ := strings.Cut(out, "@@CA")
	info := parseKV(head)
	// в контейнере сервер слушает [::] («udp6», «tcp6-server»): клиенту — обычные udp/tcp
	info["PROTO"] = strings.TrimSuffix(strings.TrimSuffix(info["PROTO"], "-server"), "6")
	port, _ := strconv.Atoi(info["PORT"])
	if port == 0 || (info["PROTO"] != "udp" && info["PROTO"] != "tcp") {
		return nil, errors.New("в конфиге сервера нет порта или протокола OpenVPN")
	}
	sec := splitSections(out)
	ca, cert, key, tc := extractPEM(sec["CA"]), extractPEM(sec["CERT"]), extractPEM(sec["KEY"]), strings.TrimSpace(sec["TC"])
	if ca == "" || cert == "" || key == "" || tc == "" {
		return nil, errors.New("сервер вернул неполный набор сертификатов")
	}
	text := OVClientConfig(host, port, info["PROTO"], ca, cert, key, tc)
	return ovpnconf.Parse(text)
}

// OpenVPNClients возвращает имена выпущенных клиентских сертификатов (кроме серверного).
func (s *Session) OpenVPNClients() ([]string, error) {
	out, err := s.proto("openvpn").exec("ls -1 "+ovDir+"/pki/issued", nil, true)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range strings.Split(out, "\n") {
		n := strings.TrimSuffix(strings.TrimSpace(f), ".crt")
		if n != "" && n != "server" && ovNameOK.MatchString(n) {
			names = append(names, n)
		}
	}
	return names, nil
}

// OpenVPNRevoke отзывает сертификат клиента и обновляет список отзыва.
func (s *Session) OpenVPNRevoke(name string) error {
	if !ovNameOK.MatchString(name) || name == "server" {
		return errors.New("недопустимое имя клиента")
	}
	script := `set -e
cd ` + ovDir + `
export EASYRSA_BATCH=1 EASYRSA_CRL_DAYS=3650
./easyrsa revoke ` + name + ` >/dev/null 2>&1
./easyrsa gen-crl >/dev/null 2>&1
cp pki/crl.pem crl.pem
chmod 644 crl.pem
`
	_, err := s.proto("openvpn").exec(script, nil, true)
	if err != nil {
		return fmt.Errorf("не удалось отозвать клиента: %w", err)
	}
	return nil
}

// RemoveOpenVPN останавливает OpenVPN и удаляет настройки Veil-OpenVPN (пакеты остаются).
func (s *Session) RemoveOpenVPN() error {
	if s.inDocker("openvpn") {
		return s.r.removeContainer("openvpn")
	}
	const script = `systemctl disable --now ` + ovService + ` >/dev/null 2>&1 || true
systemctl disable --now veil-openvpn-nat >/dev/null 2>&1 || true
rm -f ` + ovConf + ` /etc/sysctl.d/99-veilovpn.conf /etc/systemd/system/veil-openvpn-nat.service
rm -rf ` + ovDir + `
systemctl daemon-reload || true`
	_, err := s.r.exec(script, nil, true)
	return err
}
