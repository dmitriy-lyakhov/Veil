package deploy

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"veil/internal/vpnconf/ipsecconf"
)

// IPsec/IKEv2 на сервере: strongSwan (swanctl), собственный центр сертификации Veil, серверный
// сертификат с адресом сервера в SAN, аутентификация клиентов по EAP-MSCHAPv2 или сертификату.
// Файлы Veil: <swanctl>/conf.d/veil.conf (соединения и пул), veil-users.conf (логины),
// <swanctl>/veil/ca.key (ключ центра), служба veil-ipsec-nat (NAT и пересылка).
const (
	IPsecDefaultPool = "10.10.0.0/24"
	ipsecPool        = IPsecDefaultPool
)

// ipsecDirSh — определение каталога swanctl (Debian/Ubuntu: /etc/swanctl, RPM: /etc/strongswan/swanctl).
const ipsecDirSh = `D=/etc/swanctl; [ -d /etc/strongswan/swanctl ] && D=/etc/strongswan/swanctl; `

const ipsecInstallScript = `set -e
HOST=@HOST@
` + ipsecDirSh + `
if [ ! -f "$D/conf.d/veil.conf" ]; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -qq
    apt-get install -y -qq charon-systemd strongswan-swanctl strongswan-pki libcharon-extra-plugins libcharon-extauth-plugins libstrongswan-extra-plugins iptables openssl
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y epel-release >/dev/null 2>&1 || true
    dnf install -y strongswan iptables openssl
  elif command -v yum >/dev/null 2>&1; then
    yum install -y epel-release || true
    yum install -y strongswan iptables openssl
  else
    echo "не найден поддерживаемый менеджер пакетов (apt, dnf, yum)" >&2
    exit 1
  fi
  ` + ipsecDirSh + `
  command -v pki >/dev/null 2>&1 || PKI=/usr/lib/ipsec/pki
  PKI=${PKI:-pki}
  NIC=$(ip -o -4 route show to default | awk '{print $5}' | head -1)
  [ -n "$NIC" ] || { echo "не найден сетевой интерфейс по умолчанию" >&2; exit 1; }
  umask 077
  mkdir -p "$D/x509ca" "$D/x509" "$D/private" "$D/conf.d" "$D/veil"
  $PKI --gen --type rsa --size 3072 --outform pem > "$D/veil/ca.key"
  $PKI --self --ca --lifetime 3650 --in "$D/veil/ca.key" --type rsa --dn "CN=Veil VPN CA" --outform pem > "$D/x509ca/veil-ca.pem"
  $PKI --gen --type rsa --size 3072 --outform pem > "$D/private/veil-server.key"
  $PKI --pub --in "$D/private/veil-server.key" --type rsa | $PKI --issue --lifetime 1825 --cacert "$D/x509ca/veil-ca.pem" --cakey "$D/veil/ca.key" --dn "CN=$HOST" --san "$HOST" --flag serverAuth --flag ikeIntermediate --outform pem > "$D/x509/veil-server.pem"
  chmod 644 "$D/x509ca/veil-ca.pem" "$D/x509/veil-server.pem"
  cat > "$D/conf.d/veil.conf" <<SWEOF
connections {
  veil-eap {
    version = 2
    local_addrs = %any
    pools = veil-pool
    fragmentation = yes
    dpd_delay = 30s
    send_certreq = no
    proposals = aes256-sha256-modp2048,aes128-sha256-modp2048,default
    local {
      auth = pubkey
      certs = veil-server.pem
      id = $HOST
    }
    remote {
      auth = eap-mschapv2
      eap_id = %any
    }
    children {
      veil {
        local_ts = 0.0.0.0/0
        esp_proposals = aes256-sha256,aes256gcm16,aes128gcm16,default
      }
    }
  }
  veil-cert {
    version = 2
    local_addrs = %any
    pools = veil-pool
    fragmentation = yes
    dpd_delay = 30s
    send_certreq = no
    proposals = aes256-sha256-modp2048,aes128-sha256-modp2048,default
    local {
      auth = pubkey
      certs = veil-server.pem
      id = $HOST
    }
    remote {
      auth = pubkey
      cacerts = veil-ca.pem
    }
    children {
      veil {
        local_ts = 0.0.0.0/0
        esp_proposals = aes256-sha256,aes256gcm16,aes128gcm16,default
      }
    }
  }
}
pools {
  veil-pool {
    addrs = @POOL@
    dns = 1.1.1.1, 1.0.0.1
  }
}
SWEOF
  [ -f "$D/conf.d/veil-users.conf" ] || echo "# Veil: пользователи EAP" > "$D/conf.d/veil-users.conf"
  chmod 600 "$D/conf.d/veil-users.conf"
  grep -qs 'conf.d' "$D/swanctl.conf" || echo 'include conf.d/*.conf' >> "$D/swanctl.conf"
  cat > /usr/local/sbin/veil-ipsec-nat <<'NATEOF'
#!/bin/sh
POOL=@POOL@
NIC=$(ip -o -4 route show to default | awk '{print $5}' | head -1)
case "$1" in
up)
  iptables -t nat -C POSTROUTING -s $POOL -o $NIC -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s $POOL -o $NIC -j MASQUERADE
  iptables -C FORWARD -s $POOL -j ACCEPT 2>/dev/null || iptables -I FORWARD 1 -s $POOL -j ACCEPT
  iptables -C FORWARD -d $POOL -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || iptables -I FORWARD 1 -d $POOL -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
  iptables -t mangle -C FORWARD -s $POOL -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || iptables -t mangle -A FORWARD -s $POOL -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
  ;;
down)
  iptables -t nat -D POSTROUTING -s $POOL -o $NIC -j MASQUERADE 2>/dev/null || true
  iptables -D FORWARD -s $POOL -j ACCEPT 2>/dev/null || true
  iptables -D FORWARD -d $POOL -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || true
  iptables -t mangle -D FORWARD -s $POOL -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null || true
  ;;
esac
exit 0
NATEOF
  chmod 755 /usr/local/sbin/veil-ipsec-nat
  cat > /etc/systemd/system/veil-ipsec-nat.service <<'UNITEOF'
[Unit]
Description=Veil IPsec NAT and forwarding
After=network-online.target ufw.service firewalld.service
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/local/sbin/veil-ipsec-nat up
ExecStop=/usr/local/sbin/veil-ipsec-nat down

[Install]
WantedBy=multi-user.target
UNITEOF
  echo 'net.ipv4.ip_forward=1' > /etc/sysctl.d/99-veilipsec.conf
  sysctl -q -p /etc/sysctl.d/99-veilipsec.conf || sysctl -qw net.ipv4.ip_forward=1
  systemctl daemon-reload
fi
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
  ufw allow 500/udp >/dev/null
  ufw allow 4500/udp >/dev/null
  ufw allow proto esp from any to any >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-service=ipsec >/dev/null 2>&1 || { firewall-cmd --permanent --add-port=500/udp; firewall-cmd --permanent --add-port=4500/udp; } >/dev/null
  firewall-cmd --permanent --add-masquerade >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null
fi
UNIT=strongswan
systemctl list-unit-files 2>/dev/null | grep -q '^strongswan-swanctl.service' && UNIT=strongswan-swanctl
systemctl enable --now veil-ipsec-nat.service >/dev/null 2>&1 || /usr/local/sbin/veil-ipsec-nat up
systemctl enable "$UNIT" >/dev/null 2>&1 || true
systemctl restart "$UNIT"
sleep 2
systemctl is-active --quiet "$UNIT" || { journalctl -u "$UNIT" -n 20 --no-pager >&2; exit 1; }
swanctl --load-all >/dev/null || { echo "swanctl --load-all не удался" >&2; exit 1; }
echo "UNIT=$UNIT"
`

var ipsecHostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:-]*$`)

// ValidateIPsecHost проверяет адрес сервера, который попадёт в сертификат и в скрипты.
func ValidateIPsecHost(host string) error {
	if len(host) > 253 || !ipsecHostRe.MatchString(host) {
		return errors.New("некорректный адрес сервера")
	}
	return nil
}

func ipsecScript(host string) string {
	return strings.NewReplacer("@HOST@", host, "@POOL@", ipsecPool).Replace(ipsecInstallScript)
}

// IPsecInstalled — есть ли на сервере установка Veil-IPsec.
func (s *Session) IPsecInstalled() bool {
	if s.inDocker("ipsec") {
		return true
	}
	_, err := s.r.exec(ipsecDirSh+`test -f "$D/conf.d/veil.conf"`, nil, true)
	return err == nil
}

// InstallIPsec ставит strongSwan (если ещё не стоит) и выдаёт первого клиента.
func (s *Session) InstallIPsec(host string, logf Logf) (*ipsecconf.Config, error) {
	if err := ValidateIPsecHost(host); err != nil {
		return nil, err
	}
	if s.inDocker("ipsec") {
		logf("IPsec уже работает в контейнере — выдаю клиента")
		return s.IPsecAddClient("Клиент 1", host)
	}
	env := map[string]string{"VEIL_HOST": host, "IPSEC_POOL": ipsecPool, "IPSEC_FIRST_CLIENT": "", "VEIL_DETECT_IP": "0"}
	if s.Profile != nil {
		env["VEIL_PUBLIC_IPV4"], env["VEIL_PUBLIC_IPV6"] = s.Profile.IPv4, s.Profile.IPv6
	}
	docker, err := s.tryDocker("ipsec", "IPsec", ipsecDirSh+`test -f "$D/conf.d/veil.conf"`, env, logf)
	if err != nil {
		return nil, err
	}
	if docker {
		logf("IPsec запущен в контейнере %s, порты 500 и 4500/UDP", containers["ipsec"].name)
		return s.IPsecAddClient("Клиент 1", host)
	}
	logf("Установка IPsec (strongSwan) на сервер…")
	if _, err := s.r.exec(ipsecScript(host), nil, true); err != nil {
		return nil, fmt.Errorf("установка IPsec: %w", err)
	}
	logf("IPsec запущен, порты 500 и 4500/UDP")
	return s.IPsecAddClient("Клиент 1", host)
}

var ipsecNameBad = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// ipsecUserName приводит имя к виду, безопасному для конфига strongSwan и логина Windows.
func ipsecUserName(name string) string {
	n := strings.Trim(ipsecNameBad.ReplaceAllString(strings.TrimSpace(name), ""), ".-_")
	if n == "" {
		n = "client"
	}
	if len(n) > 32 {
		n = n[:32]
	}
	return n
}

const pwAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func ipsecPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = pwAlphabet[int(b[i])%len(pwAlphabet)]
	}
	return string(b), nil
}

// ipsecUserBlock — запись пользователя EAP в veil-users.conf.
func ipsecUserBlock(user, pass string) string {
	return fmt.Sprintf("# veil-user: %s\nsecrets {\n  eap-%s {\n    id = %s\n    secret = \"%s\"\n  }\n}\n", user, user, user, pass)
}

func parseIPsecUsers(out string) []string {
	var users []string
	for _, ln := range strings.Split(out, "\n") {
		if u, ok := strings.CutPrefix(strings.TrimSpace(ln), "# veil-user: "); ok && u != "" {
			users = append(users, u)
		}
	}
	return users
}

// IPsecUsers — список логинов EAP на сервере.
func (s *Session) IPsecUsers() ([]string, error) {
	out, err := s.proto("ipsec").exec(ipsecDirSh+`grep '^# veil-user: ' "$D/conf.d/veil-users.conf" || true`, nil, true)
	if err != nil {
		return nil, err
	}
	return parseIPsecUsers(out), nil
}

func (s *Session) ipsecCA() (string, error) {
	return s.proto("ipsec").exec(ipsecDirSh+`cat "$D/x509ca/veil-ca.pem"`, nil, true)
}

// IPsecAddClient добавляет клиента (логин и пароль EAP) и возвращает его данные.
func (s *Session) IPsecAddClient(name, host string) (*ipsecconf.Config, error) {
	users, err := s.IPsecUsers()
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать список клиентов IPsec: %w", err)
	}
	user := uniqueName(ipsecUserName(name), users)
	pass, err := ipsecPassword()
	if err != nil {
		return nil, err
	}
	ca, err := s.ipsecCA()
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать сертификат центра: %w", err)
	}
	if _, err := s.proto("ipsec").exec(ipsecDirSh+`cat >> "$D/conf.d/veil-users.conf" && swanctl --load-creds >/dev/null`, []byte(ipsecUserBlock(user, pass)), true); err != nil {
		return nil, fmt.Errorf("не удалось добавить клиента: %w", err)
	}
	cfg := &ipsecconf.Config{Server: host, CAPEM: strings.TrimSpace(ca) + "\n", Auth: ipsecconf.AuthEAP,
		Username: user, Password: pass, DNS: []string{"1.1.1.1", "1.0.0.1"}}
	return cfg, cfg.Validate()
}

func uniqueName(base string, used []string) string {
	has := func(n string) bool {
		for _, u := range used {
			if u == n {
				return true
			}
		}
		return false
	}
	n := base
	for i := 2; has(n); i++ {
		n = fmt.Sprintf("%s-%d", base, i)
	}
	return n
}

// IPsecRemoveClient удаляет клиента EAP с сервера.
func (s *Session) IPsecRemoveClient(user string) error {
	if user != ipsecUserName(user) {
		return errors.New("некорректное имя клиента")
	}
	_, err := s.proto("ipsec").exec(ipsecDirSh+`sed -i '/^# veil-user: `+user+`$/,/^}$/d' "$D/conf.d/veil-users.conf" && swanctl --load-all >/dev/null && swanctl --terminate --ike veil-eap >/dev/null 2>&1; true`, nil, true)
	return err
}

const ipsecCertScript = `set -e
` + ipsecDirSh + `
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
umask 077
pki --gen --type rsa --size 3072 --outform pem > "$T/c.key"
pki --pub --in "$T/c.key" --type rsa | pki --issue --lifetime 1095 --cacert "$D/x509ca/veil-ca.pem" --cakey "$D/veil/ca.key" --dn "CN=@NAME@" --san "@NAME@" --flag clientAuth --outform pem > "$T/c.pem"
openssl pkcs12 -export -inkey "$T/c.key" -in "$T/c.pem" -certfile "$D/x509ca/veil-ca.pem" -name "@NAME@" -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg sha1 -passout pass:@PASS@ -out "$T/c.p12"
echo "P12=$(base64 -w0 "$T/c.p12")"
`

// IPsecAddCertClient выпускает клиенту сертификат (PKCS#12). Отозвать его по отдельности нельзя:
// проще удалить IPsec с сервера и поставить заново.
func (s *Session) IPsecAddCertClient(name, host string) (*ipsecconf.Config, error) {
	n := ipsecUserName(name)
	pass, err := ipsecPassword()
	if err != nil {
		return nil, err
	}
	ca, err := s.ipsecCA()
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать сертификат центра: %w", err)
	}
	out, err := s.proto("ipsec").exec(strings.NewReplacer("@NAME@", n, "@PASS@", pass).Replace(ipsecCertScript), nil, true)
	if err != nil {
		return nil, fmt.Errorf("не удалось выпустить сертификат клиента: %w", err)
	}
	p12 := parseKV(out)["P12"]
	if p12 == "" {
		return nil, errors.New("сервер не вернул сертификат клиента")
	}
	cfg := &ipsecconf.Config{Server: host, CAPEM: strings.TrimSpace(ca) + "\n", Auth: ipsecconf.AuthCert,
		Username: n, P12: p12, P12Pass: pass, DNS: []string{"1.1.1.1", "1.0.0.1"}}
	return cfg, cfg.Validate()
}

// RemoveIPsec останавливает IPsec и удаляет настройки, сертификаты, ключ центра и пользователей Veil (пакеты остаются).
func (s *Session) RemoveIPsec() error {
	if s.inDocker("ipsec") {
		return s.r.removeContainer("ipsec")
	}
	const script = ipsecDirSh + `UNIT=strongswan
systemctl list-unit-files 2>/dev/null | grep -q '^strongswan-swanctl.service' && UNIT=strongswan-swanctl
systemctl disable --now veil-ipsec-nat.service >/dev/null 2>&1 || true
[ -x /usr/local/sbin/veil-ipsec-nat ] && /usr/local/sbin/veil-ipsec-nat down || true
rm -f /etc/systemd/system/veil-ipsec-nat.service /usr/local/sbin/veil-ipsec-nat /etc/sysctl.d/99-veilipsec.conf
rm -f "$D/conf.d/veil.conf" "$D/conf.d/veil-users.conf" "$D/x509ca/veil-ca.pem" "$D/x509/veil-server.pem" "$D/private/veil-server.key"
rm -rf "$D/veil"
systemctl daemon-reload
systemctl restart "$UNIT" >/dev/null 2>&1 || true`
	_, err := s.r.exec(script, nil, true)
	return err
}
