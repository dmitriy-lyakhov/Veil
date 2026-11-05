//go:build darwin

package netpin

import (
	"net"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Control привязывает сокет к выбранному интерфейсу (IP_BOUND_IF / IPV6_BOUND_IF), если он задан через SetName.
func Control(network, _ string, c syscall.RawConn) error {
	name := Name()
	if name == "" {
		return nil
	}
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return err
	}
	var serr error
	if err := c.Control(func(fd uintptr) {
		if strings.HasSuffix(network, "6") {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifc.Index)
		} else {
			serr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, ifc.Index)
		}
	}); err != nil {
		return err
	}
	return serr
}
