//go:build darwin

package client

import (
	"errors"
	"log/slog"
	"net/netip"
)

// В macOS у маршрутов нет метрик, и запасной маршрут нельзя положить рядом с основным.
func addBlackholes([]netip.Prefix, *slog.Logger) error {
	return errors.New("KillSwitch в macOS не поддерживается")
}

func removeBlackholes([]netip.Prefix) {}
