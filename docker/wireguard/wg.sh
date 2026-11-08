#!/bin/bash
# WireGuard / AmneziaWG в контейнере. Один скрипт для обоих: WG_FLAVOR=wg | awg.
#   wg.sh start               — создать конфиг (при первом запуске), поднять интерфейс, ждать сигнала
#   client add NAME           — выдать клиента (конфиг .conf + QR)
#   client show NAME          — показать конфиг клиента
#   client list               — список клиентов
#   client remove NAME        — удалить клиента
# Подсеть IPv4 — /24 (сервер .1, клиенты .2–.254); при включённом IPv6 внутри туннеля ещё ULA /64 с NAT66.
set -euo pipefail
. /usr/local/lib/veil/net.sh

FLAVOR=${WG_FLAVOR:-wg}
if [ "$FLAVOR" = awg ]; then
	TOOL=awg QUICK=awg-quick DIR=/etc/amnezia/amneziawg IFACE=veilawg
	DEF_PORT=51821 DEF_NET=10.9.1.0/24 MTU=1280 TITLE=AmneziaWG
else
	TOOL=wg QUICK=wg-quick DIR=/etc/wireguard IFACE=veilwg
	DEF_PORT=51820 DEF_NET=10.8.1.0/24 MTU=1380 TITLE=WireGuard
fi
CONF=$DIR/$IFACE.conf
STATE=$DIR/veil.env
CLIENTS=$DIR/clients
OBFS_KEYS="Jc Jmin Jmax S1 S2 S3 S4 H1 H2 H3 H4"

# conf_get KEY — значение из [Interface]; разделитель — первый «=» (ключи base64 сами кончаются на «=»)
conf_get() { sed -n "s/^$1[[:space:]]*=[[:space:]]*//p" "$CONF" | sed -n 1p; }

init_server() {
	[ -f "$CONF" ] && return 0
	local host port net4 base4 ula="" priv addr obfs="" up down
	host=$(public_host)
	port=${WG_PORT:-$DEF_PORT}
	net4=${WG_SUBNET:-$DEF_NET}
	echo "$net4" | grep -Eq '^([0-9]{1,3}\.){3}0/24$' || die "WG_SUBNET должна быть подсетью /24 вида 10.8.1.0/24"
	base4=${net4%.0/24}
	if ipv6_enabled; then ula=$(random_ula64); fi
	mkdir -p "$DIR" "$CLIENTS"
	umask 077
	priv=$($TOOL genkey)
	addr="$base4.1/24"
	up="iptables -w -t nat -A POSTROUTING -s $net4 ! -o %i -j MASQUERADE"
	down="iptables -w -t nat -D POSTROUTING -s $net4 ! -o %i -j MASQUERADE"
	if [ -n "$ula" ]; then
		addr="$addr, $ula::1/64"
		up="$up; ip6tables -w -t nat -A POSTROUTING -s $ula::/64 ! -o %i -j MASQUERADE"
		down="$down; ip6tables -w -t nat -D POSTROUTING -s $ula::/64 ! -o %i -j MASQUERADE"
	fi
	if [ "$FLAVOR" = awg ]; then
		local s1 s2 h1 h2 h3 h4
		while :; do
			s1=$(rand_range 15 150) s2=$(rand_range 15 150)
			[ $((s1 + 56)) -ne "$s2" ] && break
		done
		while :; do
			h1=$(rand_range 5 2147483647) h2=$(rand_range 5 2147483647) h3=$(rand_range 5 2147483647) h4=$(rand_range 5 2147483647)
			[ "$(printf '%s\n' $h1 $h2 $h3 $h4 | sort -u | wc -l)" -eq 4 ] && break
		done
		# параметры, выбранные в приложении, приходят переменными AWG_*; иначе — случайные
		obfs=$(printf 'Jc = %s\nJmin = %s\nJmax = %s\nS1 = %s\nS2 = %s\nH1 = %s\nH2 = %s\nH3 = %s\nH4 = %s' \
			"${AWG_JC:-4}" "${AWG_JMIN:-10}" "${AWG_JMAX:-50}" "${AWG_S1:-$s1}" "${AWG_S2:-$s2}" \
			"${AWG_H1:-$h1}" "${AWG_H2:-$h2}" "${AWG_H3:-$h3}" "${AWG_H4:-$h4}")
		if [ -n "${AWG_S3:-}" ]; then obfs="$obfs"$'\n'"S3 = $AWG_S3"; fi
		if [ -n "${AWG_S4:-}" ]; then obfs="$obfs"$'\n'"S4 = $AWG_S4"; fi
	fi
	{
		echo "[Interface]"
		echo "Address = $addr"
		echo "ListenPort = $port"
		echo "PrivateKey = $priv"
		[ -n "$obfs" ] && echo "$obfs"
		echo "PostUp = $up"
		echo "PostDown = $down"
	} >"$CONF"
	chmod 600 "$CONF"
	printf 'HOST=%s\nPORT=%s\nBASE4=%s\nULA=%s\n' "$host" "${WG_PUBLIC_PORT:-$port}" "$base4" "$ula" >"$STATE"
	log "$TITLE: создан конфиг сервера, порт $port/udp, подсеть $net4${ula:+ и $ula::/64}, адрес для клиентов $host"
}

load_state() {
	[ -f "$STATE" ] || die "сервер ещё не настроен — сначала запустите контейнер"
	# shellcheck disable=SC1090
	. "$STATE"
	# адрес и порт для клиентов можно сменить переменными, не пересоздавая сервер
	HOST=${VEIL_HOST:-$HOST}
	PORT=${WG_PUBLIC_PORT:-$PORT}
}

sync_conf() {
	ip link show "$IFACE" >/dev/null 2>&1 || return 0
	$TOOL syncconf "$IFACE" <($QUICK strip "$CONF")
}

client_add() {
	local name idx ip4 ip6="" priv pub psk srvpub out addr dns obfs="" k v
	name=$(safe_name "$1")
	load_state
	mkdir -p "$CLIENTS"
	grep -qx "# veil: $name" "$CONF" && die "клиент $name уже есть (client show $name)"
	for idx in $(seq 2 254); do
		grep -Eq "AllowedIPs = $BASE4\.$idx/32(,|$)" "$CONF" || break
	done
	grep -Eq "AllowedIPs = $BASE4\.$idx/32(,|$)" "$CONF" && die "в подсети закончились адреса"
	ip4="$BASE4.$idx"
	[ -n "$ULA" ] && ip6="$ULA::$(printf '%x' "$idx")"
	umask 077
	priv=$($TOOL genkey)
	pub=$(echo "$priv" | $TOOL pubkey)
	psk=$($TOOL genpsk)
	srvpub=$(conf_get PrivateKey | $TOOL pubkey)
	{
		echo
		echo "# veil: $name"
		echo "[Peer]"
		echo "PublicKey = $pub"
		echo "PresharedKey = $psk"
		echo "AllowedIPs = $ip4/32${ip6:+, $ip6/128}"
	} >>"$CONF"
	sync_conf
	addr="$ip4/32${ip6:+, $ip6/128}"
	dns="1.1.1.1, 1.0.0.1${ip6:+, 2606:4700:4700::1111}"
	if [ "$FLAVOR" = awg ]; then
		for k in $OBFS_KEYS; do
			v=$(conf_get "$k")
			[ -n "$v" ] && obfs="$obfs$k = $v"$'\n'
		done
	fi
	out="$CLIENTS/$name.conf"
	{
		echo "[Interface]"
		echo "PrivateKey = $priv"
		echo "Address = $addr"
		echo "DNS = $dns"
		echo "MTU = $MTU"
		printf '%s' "$obfs"
		echo
		echo "[Peer]"
		echo "PublicKey = $srvpub"
		echo "PresharedKey = $psk"
		echo "Endpoint = $(hostport "$HOST" "$PORT")"
		echo "AllowedIPs = 0.0.0.0/0, ::/0"
		echo "PersistentKeepalive = 25"
	} >"$out"
	log "$TITLE: клиент $name, адрес $addr"
	client_show "$name"
}

client_show() {
	local f
	f="$CLIENTS/$(safe_name "$1").conf"
	[ -f "$f" ] || die "нет клиента $1"
	cat "$f"
	echo
	show_qr "$f"
	echo "файл: $f (в томе; скопировать: docker compose cp <сервис>:$f .)"
}

client_list() {
	local name
	grep '^# veil: ' "$CONF" | sed 's/^# veil: //' | while read -r name; do
		printf '%-32s %s\n' "$name" "$(awk -v m="# veil: $name" '$0==m{f=1} f&&/^AllowedIPs/{sub(/^AllowedIPs = /,""); print; exit}' "$CONF")"
	done
	if ip link show "$IFACE" >/dev/null 2>&1; then echo; $TOOL show "$IFACE" latest-handshakes; fi
}

client_remove() {
	local name tmp
	name=$(safe_name "$1")
	grep -qx "# veil: $name" "$CONF" || die "нет клиента $name"
	tmp=$(mktemp)
	awk -v m="# veil: $name" '$0==m{skip=1; next} skip&&/^$/{skip=0} !skip' "$CONF" >"$tmp"
	cat "$tmp" >"$CONF" && rm -f "$tmp"
	sync_conf
	rm -f "$CLIENTS/$name.conf"
	log "$TITLE: клиент $name удалён"
}

start() {
	ensure_tun
	ensure_forwarding
	init_server
	$QUICK up "$CONF"
	if ! grep -q '^# veil: ' "$CONF" && [ -n "${WG_FIRST_CLIENT-client}" ]; then
		client_add "${WG_FIRST_CLIENT-client}"
	fi
	log "$TITLE запущен: $($TOOL show "$IFACE" listen-port)/udp, клиентов: $(grep -c '^# veil: ' "$CONF" || true)"
	trap '$QUICK down "$CONF"; exit 0' TERM INT
	sleep infinity &
	wait $!
}

cmd=${1:-start}
[ $# -gt 0 ] && shift
case "$(basename "$0"):$cmd" in
client:add | *:add) client_add "${1:?имя клиента}" ;;
client:show | *:show) client_show "${1:?имя клиента}" ;;
client:list | *:list | *:ls) client_list ;;
client:remove | *:remove | *:rm | *:delete) client_remove "${1:?имя клиента}" ;;
*:start) start ;;
*) echo "usage: client add|show|remove NAME | client list" >&2; exit 2 ;;
esac
