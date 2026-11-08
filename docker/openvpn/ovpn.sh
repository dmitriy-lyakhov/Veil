#!/bin/bash
# OpenVPN в контейнере: PKI easy-rsa (EC P-256), tls-crypt, CRL; dual-stack (IPv4 + ULA /64 с NAT66).
#   ovpn.sh start        — создать PKI и конфиг (при первом запуске), запустить OpenVPN
#   client add NAME      — выпустить сертификат и собрать .ovpn
#   client show NAME     — показать .ovpn
#   client list          — список клиентов
#   client remove NAME   — отозвать сертификат (CRL перечитывается при каждом подключении)
set -euo pipefail
. /usr/local/lib/veil/net.sh

DIR=/etc/openvpn/veil
CONF=$DIR/server.conf
STATE=$DIR/veil.env
CLIENTS=$DIR/clients
export EASYRSA_PKI=$DIR/pki EASYRSA_BATCH=1 EASYRSA_ALGO=ec EASYRSA_CURVE=prime256v1 \
	EASYRSA_CA_EXPIRE=3650 EASYRSA_CERT_EXPIRE=3650 EASYRSA_CRL_DAYS=3650
ERSA=$(command -v easyrsa || echo /usr/share/easy-rsa/easyrsa)

cidr2mask() {
	local bits=$1 mask="" i n
	for i in 1 2 3 4; do
		if [ "$bits" -ge 8 ]; then n=255; bits=$((bits - 8)); else n=$((256 - (1 << (8 - bits)))); bits=0; fi
		mask="$mask${mask:+.}$n"
	done
	echo "$mask"
}

init_server() {
	[ -f "$CONF" ] && return 0
	local host port proto net4 ula="" sock grp
	host=$(public_host)
	port=${OVPN_PORT:-1194}
	proto=${OVPN_PROTO:-udp}
	[ "$proto" = udp ] || [ "$proto" = tcp ] || die "OVPN_PROTO: udp или tcp"
	net4=${OVPN_SUBNET:-10.9.0.0/24}
	echo "$net4" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}/(1[6-9]|2[0-9])$' || die "OVPN_SUBNET вида 10.9.0.0/24 (маска /16–/29)"
	sock=$proto
	if ipv6_enabled; then
		ula=$(random_ula64)
		sock=${proto}6 # сокет [::] принимает и IPv4, и IPv6
	fi
	mkdir -p "$DIR" "$CLIENTS"
	umask 077
	log "OpenVPN: создание PKI (до минуты)…"
	"$ERSA" init-pki >/dev/null
	EASYRSA_REQ_CN=Veil-CA "$ERSA" build-ca nopass >/dev/null 2>&1
	"$ERSA" build-server-full server nopass >/dev/null 2>&1
	"$ERSA" gen-crl >/dev/null 2>&1
	openvpn --genkey secret "$DIR/tls-crypt.key"
	cp "$EASYRSA_PKI/crl.pem" "$DIR/crl.pem" && chmod 644 "$DIR/crl.pem"
	grp=nogroup
	getent group nogroup >/dev/null 2>&1 || grp=nobody
	cat >"$CONF" <<EOF
port $port
proto $sock
dev tun
topology subnet
server ${net4%/*} $(cidr2mask "${net4#*/}")
${ula:+server-ipv6 $ula::/64}
ca $EASYRSA_PKI/ca.crt
cert $EASYRSA_PKI/issued/server.crt
key $EASYRSA_PKI/private/server.key
dh none
ecdh-curve prime256v1
tls-crypt $DIR/tls-crypt.key
crl-verify $DIR/crl.pem
data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305
auth SHA256
tls-version-min 1.2
push "redirect-gateway def1 ${ula:+ipv6 }bypass-dhcp"
push "dhcp-option DNS 1.1.1.1"
push "dhcp-option DNS 1.0.0.1"
keepalive 10 120
persist-key
persist-tun
user nobody
group $grp
verb 3
$([ "$proto" = udp ] && echo "explicit-exit-notify 1")
EOF
	chmod 600 "$CONF"
	printf 'HOST=%s\nPORT=%s\nPROTO=%s\nNET4=%s\nULA=%s\n' "$host" "${OVPN_PUBLIC_PORT:-$port}" "$proto" "$net4" "$ula" >"$STATE"
	log "OpenVPN: создан конфиг сервера, порт $port/$proto, подсеть $net4${ula:+ и $ula::/64}, адрес для клиентов $host"
}

load_state() {
	[ -f "$STATE" ] || die "сервер ещё не настроен — сначала запустите контейнер"
	# shellcheck disable=SC1090
	. "$STATE"
	HOST=${VEIL_HOST:-$HOST}
	PORT=${OVPN_PUBLIC_PORT:-$PORT}
}

pem() { sed -n '/-----BEGIN/,/-----END/p' "$1"; }

client_add() {
	local name out
	name=$(safe_name "$1")
	load_state
	[ "$name" != server ] || die "имя server занято"
	[ -f "$EASYRSA_PKI/issued/$name.crt" ] && die "клиент $name уже есть (client show $name)"
	"$ERSA" build-client-full "$name" nopass >/dev/null 2>&1 || die "не удалось выпустить сертификат"
	out="$CLIENTS/$name.ovpn"
	umask 077
	{
		echo "client"
		echo "dev tun"
		echo "proto $PROTO"
		echo "remote $HOST $PORT"
		echo "resolv-retry infinite"
		echo "nobind"
		echo "persist-key"
		echo "persist-tun"
		echo "remote-cert-tls server"
		echo "data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305"
		echo "auth SHA256"
		echo "verb 3"
		[ "$PROTO" = udp ] && echo "explicit-exit-notify 1"
		echo "<ca>"; pem "$EASYRSA_PKI/ca.crt"; echo "</ca>"
		echo "<cert>"; pem "$EASYRSA_PKI/issued/$name.crt"; echo "</cert>"
		echo "<key>"; cat "$EASYRSA_PKI/private/$name.key"; echo "</key>"
		echo "<tls-crypt>"; cat "$DIR/tls-crypt.key"; echo "</tls-crypt>"
	} >"$out"
	log "OpenVPN: клиент $name"
	client_show "$name"
}

client_show() {
	local f
	f="$CLIENTS/$(safe_name "$1").ovpn"
	[ -f "$f" ] || die "нет клиента $1"
	cat "$f"
	echo "файл: $f (скопировать: docker compose cp openvpn:$f .)"
}

client_list() {
	local f n
	for f in "$EASYRSA_PKI"/issued/*.crt; do
		[ -e "$f" ] || continue
		n=$(basename "$f" .crt)
		[ "$n" = server ] || echo "$n"
	done
	[ -f /run/openvpn-status.log ] && { echo; sed -n '/^CLIENT_LIST/p' /run/openvpn-status.log; } || true
}

client_remove() {
	local name
	name=$(safe_name "$1")
	[ "$name" != server ] || die "серверный сертификат удалять нельзя"
	[ -f "$EASYRSA_PKI/issued/$name.crt" ] || die "нет клиента $name"
	"$ERSA" revoke "$name" >/dev/null 2>&1
	"$ERSA" gen-crl >/dev/null 2>&1
	cp "$EASYRSA_PKI/crl.pem" "$DIR/crl.pem" && chmod 644 "$DIR/crl.pem"
	rm -f "$CLIENTS/$name.ovpn"
	log "OpenVPN: сертификат $name отозван"
}

# Пути, которыми пользуется установщик приложения (docker exec): ./easyrsa в каталоге PKI и
# /etc/openvpn/server/veil.conf — как при установке без Docker.
compat_links() {
	ln -sf "$ERSA" "$DIR/easyrsa"
	mkdir -p /etc/openvpn/server
	ln -sf "$CONF" /etc/openvpn/server/veil.conf
}

start() {
	ensure_tun
	ensure_forwarding
	init_server
	compat_links
	load_state
	nat_rule iptables "$NET4" tun0
	if [ -n "$ULA" ]; then
		nat_rule ip6tables "$ULA::/64" tun0 || log "внимание: правило NAT66 не добавлено — IPv6 внутри туннеля может не работать"
	fi
	if ! ls "$EASYRSA_PKI"/issued/*.crt 2>/dev/null | grep -qv '/server.crt$' && [ -n "${OVPN_FIRST_CLIENT-client}" ]; then
		client_add "${OVPN_FIRST_CLIENT-client}"
	fi
	exec openvpn --config "$CONF" --status /run/openvpn-status.log 30
}

cmd=${1:-start}
[ $# -gt 0 ] && shift
case "$cmd" in
add) client_add "${1:?имя клиента}" ;;
show) client_show "${1:?имя клиента}" ;;
list | ls) client_list ;;
remove | rm | delete | revoke) client_remove "${1:?имя клиента}" ;;
start) start ;;
*) echo "usage: client add|show|remove NAME | client list" >&2; exit 2 ;;
esac
