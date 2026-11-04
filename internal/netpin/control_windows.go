//go:build windows

package netpin

import (
	"math/bits"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const sockoptUnicastIf = 31 // IP_UNICAST_IF и IPV6_UNICAST_IF

// Control подходит для net.Dialer.Control и net.ListenConfig.Control.
func Control(network, _ string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		a, b := Get()
		if strings.HasSuffix(network, "6") {
			if b != 0 {
				serr = windows.SetsockoptInt(h, windows.IPPROTO_IPV6, sockoptUnicastIf, int(b))
			}
			return
		}
		if a != 0 {
			// для IPv4 индекс интерфейса передаётся в сетевом порядке байт
			serr = windows.SetsockoptInt(h, windows.IPPROTO_IP, sockoptUnicastIf, int(bits.ReverseBytes32(a)))
		}
	})
	if err != nil {
		return err
	}
	return serr
}
