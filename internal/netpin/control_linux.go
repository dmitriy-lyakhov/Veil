//go:build linux

package netpin

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// Control привязывает сокет к выбранному интерфейсу (SO_BINDTODEVICE), если интерфейс задан через SetName.
// Нужны права CAP_NET_RAW (помощник туннеля работает с повышенными правами).
func Control(network, _ string, c syscall.RawConn) error {
	name := Name()
	if name == "" {
		return nil
	}
	var serr error
	if err := c.Control(func(fd uintptr) {
		serr = unix.BindToDevice(int(fd), name)
	}); err != nil {
		return err
	}
	return serr
}
