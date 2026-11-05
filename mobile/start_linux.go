//go:build linux

package veilcore

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/client"
	"veil/internal/netpin"
)

// Start поднимает туннель на дескрипторе TUN, полученном из VpnService.Builder.establish() (detachFd:
// дескриптором владеет Go). Возвращается сразу; ход подключения — в Status().
func Start(link string, tunFd int32, protector Protector) error {
	mu.Lock()
	defer mu.Unlock()
	if tunnel != nil {
		return errors.New("туннель уже запущен")
	}
	p, err := decode(link)
	if err != nil {
		return err
	}
	if protector != nil {
		netpin.SetProtect(func(fd int) bool { return protector.Protect(int32(fd)) })
	}
	dev, _, err := tun.CreateUnmonitoredTUNFromFD(int(tunFd))
	if err != nil {
		netpin.SetProtect(nil)
		return fmt.Errorf("TUN: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	t, err := client.StartEngine(p, dev, nil, log)
	if err != nil {
		_ = dev.Close()
		netpin.SetProtect(nil)
		return err
	}
	tunnel, profile = t, p
	return nil
}

func releaseProtect() { netpin.SetProtect(nil) }
