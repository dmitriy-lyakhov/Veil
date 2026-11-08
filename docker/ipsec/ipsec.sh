#!/bin/bash
# IPsec/IKEv2 (strongSwan, swanctl) в контейнере: собственный центр сертификации, вход по логину
# и паролю (EAP-MSCHAPv2) или по сертификату; пулы IPv4 и IPv6 (ULA с NAT66).
#   ipsec.sh start           — создать ЦС, сертификат сервера и конфиг (при первом запуске), запустить charon
#   client add NAME          — логин и пароль EAP
#   client add-cert NAME     — сертификат клиента (PKCS#12)
#   client list              — список логинов
#   client remove NAME       — удалить логин
set -euo pipefail
. /usr/local/lib/veil/net.sh

D=/etc/swanctl
STATE=$D/veil.env
USERS=$D/conf.d/veil-users.conf
CLIENTS=$D/clients
CHARON=/usr/lib/ipsec/charon
VICI=/var/run/charon.vici

init_server() {
	[ -f "$D/conf.d/veil.conf" ] && return 0
	local host pool4 ula="" pools sans="" a local_ts="0.0.0.0/0"
	host=$(public_host)
	pool4=${IPSEC_POOL:-10.10.0.0/24}
	pools="veil-pool"
	if ipv6_enabled; then
		ula=$(random_ula64)
		pools="veil-pool, veil-pool6"
		local_ts="0.0.0.0/0, ::/0"
	fi
	# в сертификат сервера — все адреса и имена, по которым к нему подключаются
	for a in "$host" ${IPSEC_EXTRA_SANS:-} ${VEIL_PUBLIC_IPV4:-} ${VEIL_PUBLIC_IPV6:-}; do
		case " $sans " in *" $a "*) ;; *) sans="$sans $a" ;; esac
	done
	mkdir -p "$D/x509ca" "$D/x509" "$D/private" "$D/conf.d" "$D/veil" "$CLIENTS"
	umask 077
	log "IPsec: создание центра сертификации и сертификата сервера…"
	pki --gen --type rsa --size 3072 --outform pem >"$D/veil/ca.key"
	pki --self --ca --lifetime 3650 --in "$D/veil/ca.key" --type rsa --dn "CN=Veil VPN CA" --outform pem >"$D/x509ca/veil-ca.pem"
	pki --gen --type rsa --size 3072 --outform pem >"$D/private/veil-server.key"
	# shellcheck disable=SC2046
	pki --pub --in "$D/private/veil-server.key" --type rsa |
		pki --issue --lifetime 1825 --cacert "$D/x509ca/veil-ca.pem" --cakey "$D/veil/ca.key" \
			--dn "CN=$host" $(for a in $sans; do printf -- '--san %s ' "$a"; done) \
			--flag serverAuth --flag ikeIntermediate --outform pem >"$D/x509/veil-server.pem"
	chmod 644 "$D/x509ca/veil-ca.pem" "$D/x509/veil-server.pem"
	cp "$D/x509ca/veil-ca.pem" "$CLIENTS/veil-ca.pem"
	conn() {
		cat <<EOF
  $1 {
    version = 2
    local_addrs = %any
    pools = $pools
    fragmentation = yes
    dpd_delay = 30s
    send_certreq = no
    proposals = aes256-sha256-modp2048,aes128-sha256-modp2048,default
    local {
      auth = pubkey
      certs = veil-server.pem
      id = $host
    }
    remote {
$2
    }
    children {
      veil {
        local_ts = $local_ts
        esp_proposals = aes256-sha256,aes256gcm16,aes128gcm16,default
      }
    }
  }
EOF
	}
	{
		echo "connections {"
		conn veil-eap $'      auth = eap-mschapv2\n      eap_id = %any'
		conn veil-cert $'      auth = pubkey\n      cacerts = veil-ca.pem'
		echo "}"
		echo "pools {"
		printf '  veil-pool {\n    addrs = %s\n    dns = 1.1.1.1, 1.0.0.1\n  }\n' "$pool4"
		[ -n "$ula" ] && printf '  veil-pool6 {\n    addrs = %s::/112\n    dns = 2606:4700:4700::1111\n  }\n' "$ula"
		echo "}"
	} >"$D/conf.d/veil.conf"
	[ -f "$USERS" ] || echo "# Veil: пользователи EAP" >"$USERS"
	chmod 600 "$USERS"
	echo 'include conf.d/*.conf' >"$D/swanctl.conf"
	printf 'HOST=%s\nPOOL4=%s\nULA=%s\n' "$host" "$pool4" "$ula" >"$STATE"
	log "IPsec: конфиг создан, пул $pool4${ula:+ и $ula::/112}, сертификат сервера на:$sans"
}

load_state() {
	[ -f "$STATE" ] || die "сервер ещё не настроен — сначала запустите контейнер"
	# shellcheck disable=SC1090
	. "$STATE"
	HOST=${VEIL_HOST:-$HOST}
}

running() { [ -S "$VICI" ]; }

# без «tr </dev/urandom | head»: при pipefail обрыв канала (SIGPIPE) валит скрипт
password() { head -c 1024 /dev/urandom | LC_ALL=C tr -dc 'abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789' | cut -c1-16; }

client_add() {
	local user pass
	user=$(safe_name "$1")
	load_state
	grep -qx "# veil-user: $user" "$USERS" && die "логин $user уже есть"
	pass=$(password)
	printf '# veil-user: %s\nsecrets {\n  eap-%s {\n    id = %s\n    secret = "%s"\n  }\n}\n' "$user" "$user" "$user" "$pass" >>"$USERS"
	running && swanctl --load-creds >/dev/null
	cat <<EOF
IPsec/IKEv2, вход по логину и паролю (EAP-MSCHAPv2):
  сервер:  $HOST
  логин:   $user
  пароль:  $pass
  сертификат центра: $CLIENTS/veil-ca.pem (docker compose cp ipsec:$CLIENTS/veil-ca.pem .)

Windows (PowerShell от администратора, рядом с veil-ca.pem):
  Import-Certificate -FilePath veil-ca.pem -CertStoreLocation Cert:\\LocalMachine\\Root
  Add-VpnConnection -Name Veil-IPsec -ServerAddress $HOST -TunnelType Ikev2 -AuthenticationMethod Eap -EncryptionLevel Required -RememberCredential
  rasdial Veil-IPsec $user <пароль>
EOF
}

client_add_cert() {
	local name pass t
	name=$(safe_name "$1")
	load_state
	pass=$(password)
	t=$(mktemp -d)
	umask 077
	pki --gen --type rsa --size 3072 --outform pem >"$t/c.key"
	pki --pub --in "$t/c.key" --type rsa | pki --issue --lifetime 1095 --cacert "$D/x509ca/veil-ca.pem" \
		--cakey "$D/veil/ca.key" --dn "CN=$name" --san "$name" --flag clientAuth --outform pem >"$t/c.pem"
	openssl pkcs12 -export -inkey "$t/c.key" -in "$t/c.pem" -certfile "$D/x509ca/veil-ca.pem" -name "$name" \
		-keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg sha1 -passout "pass:$pass" -out "$CLIENTS/$name.p12"
	rm -rf "$t"
	cat <<EOF
IPsec/IKEv2, вход по сертификату:
  сервер: $HOST
  файл:   $CLIENTS/$name.p12 (пароль $pass)
  сертификат центра: $CLIENTS/veil-ca.pem
Отозвать отдельный сертификат нельзя: при утечке пересоздайте том ipsec.
EOF
}

client_list() {
	grep '^# veil-user: ' "$USERS" | sed 's/^# veil-user: //' || true
	if running; then echo; swanctl --list-sas --raw 2>/dev/null | grep -o 'remote-eap-id=[^ ]*\|remote-vips=\[[^]]*\]' || true; fi
}

client_remove() {
	local user
	user=$(safe_name "$1")
	grep -qx "# veil-user: $user" "$USERS" || die "нет логина $user"
	sed -i "/^# veil-user: $user\$/,/^}\$/d" "$USERS"
	if running; then
		swanctl --load-creds --clear >/dev/null
		swanctl --terminate --ike veil-eap >/dev/null 2>&1 || true
	fi
	log "IPsec: логин $user удалён"
}

start() {
	ensure_forwarding
	init_server
	load_state
	nat_rule iptables "$POOL4" lo
	iptables -w -t mangle -C FORWARD -s "$POOL4" -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu 2>/dev/null ||
		iptables -w -t mangle -A FORWARD -s "$POOL4" -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
	if [ -n "$ULA" ]; then
		nat_rule ip6tables "$ULA::/112" lo || log "внимание: правило NAT66 не добавлено — IPv6 внутри туннеля может не работать"
	fi
	rm -f "$VICI" /var/run/charon.pid
	"$CHARON" &
	local pid=$!
	trap 'kill -TERM $pid 2>/dev/null; wait $pid; exit 0' TERM INT
	for _ in $(seq 50); do running && break; sleep 0.2; done
	running || die "charon не запустился"
	swanctl --load-all
	if ! grep -q '^# veil-user: ' "$USERS" && [ -n "${IPSEC_FIRST_CLIENT-client}" ]; then
		client_add "${IPSEC_FIRST_CLIENT-client}"
	fi
	log "IPsec запущен: UDP 500 и 4500, сервер $HOST"
	wait $pid
}

cmd=${1:-start}
[ $# -gt 0 ] && shift
case "$cmd" in
add) client_add "${1:?имя клиента}" ;;
add-cert) client_add_cert "${1:?имя клиента}" ;;
list | ls) client_list ;;
remove | rm | delete) client_remove "${1:?имя клиента}" ;;
start) start ;;
*) echo "usage: client add|add-cert|remove NAME | client list" >&2; exit 2 ;;
esac
