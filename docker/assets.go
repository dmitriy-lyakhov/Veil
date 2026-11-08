// Package dockerfiles — Dockerfile'ы, compose-файлы и скрипты контейнеров Veil.
// Встраиваются в приложение: установщик по SSH выкладывает их на сервер (/opt/veil/docker)
// вместе с готовыми бинарниками (/opt/veil/bin) и собирает образы там же (BIN_FROM=prebuilt).
package dockerfiles

import "embed"

//go:embed common veil wireguard amneziawg openvpn ipsec
var FS embed.FS
