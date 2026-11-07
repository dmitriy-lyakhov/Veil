package client

import (
	"log/slog"

	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/client/dialer"
	"veil/internal/config"
)

// StartEngine запускает движок по протоколу профиля (Veil, WireGuard, AmneziaWG) на готовом TUN без настройки системы.
// Используется встраиваемыми клиентами (Android), где сеть настраивает сама платформа.
func StartEngine(prof *config.Profile, tunDev tun.Device, dial dialer.DialFunc, log *slog.Logger) (Engine, error) {
	switch {
	case prof.IsVeil():
		return Start(prof, tunDev, dial, log)
	case prof.Protocol == "amneziawg":
		return startAmneziaWG(prof, tunDev, log)
	default:
		return startWireGuard(prof, tunDev, log)
	}
}
