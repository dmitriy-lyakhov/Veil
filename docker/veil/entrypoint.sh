#!/bin/sh
# Точка входа контейнера Veil: при первом запуске создаёт /etc/veil/server.json и первого клиента,
# затем запускает veil-server. Данные (ключи, клиенты, сертификаты) — в томе /etc/veil.
set -eu
. /usr/local/lib/veil/net.sh

CFG=${VEIL_CONFIG:-/etc/veil/server.json}

ensure_tun
ensure_forwarding

V4=${VEIL_PUBLIC_IPV4:-}
V6=${VEIL_PUBLIC_IPV6:-}
if [ ! -f "$CFG" ]; then
	log "первый запуск: создаю конфигурацию сервера"
	[ -n "$V4" ] || V4=$(detect_ip 4)
	[ -n "$V6" ] || ! ipv6_enabled || V6=$(detect_ip 6)
	log "публичные адреса: IPv4=${V4:-нет} IPv6=${V6:-нет}"
fi
# для существующего конфига init только обновит адреса, если они заданы явно
veil-server init -config "$CFG" \
	-ipv4 "$V4" -ipv6 "$V6" \
	-lang "${VEIL_SITE_LANG:-ru}" \
	-front "${VEIL_FRONT:-}" \
	-domain "${VEIL_DOMAIN:-}" \
	-client "${VEIL_FIRST_CLIENT-client}" \
	-admin "${VEIL_ADMIN:-container}"

exec veil-server -config "$CFG" "$@"
