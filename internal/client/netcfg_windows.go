package client

import (
	"errors"
	"log/slog"

	"golang.zx2c4.com/wireguard/tun"
)

var errWindowsPending = errors.New("системный туннель для Windows ещё не реализован")

func createTUN(int) (tun.Device, error) { return nil, errWindowsPending }

func applyNetwork(NetConfig, *slog.Logger) (func(), error) { return nil, errWindowsPending }
