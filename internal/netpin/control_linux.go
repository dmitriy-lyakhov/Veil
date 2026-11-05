//go:build linux

package netpin

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

// Control привязывает сокет к выбранному интерфейсу (SO_BINDTODEVICE), если интерфейс задан через SetName.
// Нужны права CAP_NET_RAW (помощник туннеля работает с повышенными правами).
// Если задан хук SetProtect (Android), он вызывается первым.
func Control(network, _ string, c syscall.RawConn) error {
	var perr error
	if err := c.Control(func(fd uintptr) {
		if !protectFD(int(fd)) {
			perr = errors.New("netpin: VpnService.protect отклонил сокет")
		}
	}); err != nil {
		return err
	}
	if perr != nil {
		return perr
	}
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
