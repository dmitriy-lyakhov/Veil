# Общие функции контейнеров Veil (POSIX sh). Подключается через «. /usr/local/lib/veil/net.sh».

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }
die() { log "ОШИБКА: $*"; exit 1; }

is_ipv4() { echo "$1" | grep -Eq '^([0-9]{1,3}\.){3}[0-9]{1,3}$'; }
is_ipv6() { echo "$1" | grep -Eq '^[0-9A-Fa-f:]+:[0-9A-Fa-f:.]*$'; }

# detect_ip 4|6 — публичный адрес сервера через внешний сервис; отключается VEIL_DETECT_IP=0.
detect_ip() {
	[ "${VEIL_DETECT_IP:-1}" = "0" ] && return 0
	if [ "$1" = 4 ]; then urls="https://api.ipify.org https://ipv4.icanhazip.com"; else urls="https://api6.ipify.org https://ipv6.icanhazip.com"; fi
	for u in $urls; do
		ip=$(curl "-$1" -fsS --max-time 5 "$u" 2>/dev/null | tr -d ' \r\n') || continue
		if { [ "$1" = 4 ] && is_ipv4 "$ip"; } || { [ "$1" = 6 ] && is_ipv6 "$ip"; }; then
			echo "$ip"
			return 0
		fi
	done
	return 0
}

# public_host — адрес, который попадает в конфиги клиентов: VEIL_HOST, иначе IPv4, иначе IPv6.
public_host() {
	if [ -n "${VEIL_HOST:-}" ]; then echo "$VEIL_HOST"; return 0; fi
	h=$(detect_ip 4)
	[ -n "$h" ] || h=$(detect_ip 6)
	[ -n "$h" ] || die "не удалось определить публичный адрес сервера: задайте VEIL_HOST"
	echo "$h"
}

# hostport HOST PORT — «host:port», IPv6 в квадратных скобках.
hostport() {
	if is_ipv6 "$1"; then echo "[$1]:$2"; else echo "$1:$2"; fi
}

# ipv6_enabled — в контейнере включён IPv6 (сеть Docker с enable_ipv6 или network_mode: host).
ipv6_enabled() {
	[ "${VEIL_IPV6:-1}" != "0" ] || return 1
	[ "$(cat /proc/sys/net/ipv6/conf/all/disable_ipv6 2>/dev/null)" = "0" ]
}

# random_ula64 — случайный префикс ULA /64 без завершающего «::», например fd3c:91a2:77e0:1.
random_ula64() {
	set -- $(od -An -N5 -tx1 /dev/urandom)
	echo "fd$1:$2$3:$4$5:1"
}

# rand_range LO HI — случайное целое из [LO, HI].
rand_range() {
	n=$(od -An -N4 -tu4 /dev/urandom | tr -d ' ')
	echo $(($1 + n % ($2 - $1 + 1)))
}

ensure_tun() {
	[ -c /dev/net/tun ] && return 0
	mkdir -p /dev/net && mknod /dev/net/tun c 10 200 2>/dev/null && chmod 600 /dev/net/tun && return 0
	die "нет /dev/net/tun: запустите контейнер с --device /dev/net/tun --cap-add NET_ADMIN"
}

ensure_forwarding() {
	for f in /proc/sys/net/ipv4/ip_forward /proc/sys/net/ipv6/conf/all/forwarding; do
		[ "$(cat $f 2>/dev/null)" = "1" ] && continue
		echo 1 >"$f" 2>/dev/null || log "внимание: $f=0 — задайте sysctls в docker-compose.yml"
	done
}

# nat_rule iptables|ip6tables SUBNET IFACE — маскарадинг и пересылка для подсети туннеля (идемпотентно).
nat_rule() {
	"$1" -w -t nat -C POSTROUTING -s "$2" ! -o "$3" -j MASQUERADE 2>/dev/null ||
		"$1" -w -t nat -A POSTROUTING -s "$2" ! -o "$3" -j MASQUERADE
	"$1" -w -C FORWARD -s "$2" -j ACCEPT 2>/dev/null || "$1" -w -I FORWARD 1 -s "$2" -j ACCEPT
	"$1" -w -C FORWARD -d "$2" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT 2>/dev/null ||
		"$1" -w -I FORWARD 1 -d "$2" -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
}

# safe_name NAME — имя клиента для файлов и конфигов: латиница, цифры, «.», «_», «-».
safe_name() {
	n=$(echo "$1" | tr -c 'A-Za-z0-9._-' '-' | sed 's/^[-._]*//; s/[-._]*$//' | cut -c1-48)
	[ -n "$n" ] || die "некорректное имя клиента: «$1» (допустимы латиница, цифры, «.», «_», «-»)"
	echo "$n"
}

# show_qr FILE — QR-код конфига в терминале (если поместится).
show_qr() {
	command -v qrencode >/dev/null 2>&1 || return 0
	qrencode -t ansiutf8 <"$1" 2>/dev/null || log "конфиг слишком велик для QR-кода — используйте файл"
}
