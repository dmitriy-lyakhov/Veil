package netsys

// Режим шлюза сайта (site-to-site, client-to-site): устройство с туннелем пересылает трафик своей
// локальной сети в туннель и обратно. Включает пересылку IPv4/IPv6 и разрешает её через интерфейс
// туннеля (на случай политики FORWARD DROP, например при установленном Docker). NAT не делается:
// адреса сети видны на другой стороне как есть — сервер маршрутизирует их к этому шлюзу.

import (
	"fmt"
	"os"
	"strings"
)

// EnableGateway включает режим шлюза для интерфейса туннеля iface; возвращает функцию отката и
// предупреждения (например, IPv6-пересылку включить не удалось — тогда шлюз работает только для IPv4).
func EnableGateway(iface string) (func(), []string, error) {
	var undo []func()
	var warns []string
	restore := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	for _, f := range []string{"/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/ipv6/conf/all/forwarding"} {
		old, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(old)) == "1" {
			continue
		}
		if err := os.WriteFile(f, []byte("1"), 0o644); err != nil {
			if strings.Contains(f, "ipv6") {
				warns = append(warns, fmt.Sprintf("IPv6-пересылка не включена (%v): шлюз работает только для IPv4", err))
				continue
			}
			restore()
			return nil, nil, fmt.Errorf("включение пересылки (%s): %w", f, err)
		}
		f, old := f, old
		undo = append(undo, func() { _ = os.WriteFile(f, old, 0o644) })
	}
	for _, bin := range []string{"iptables", "ip6tables"} {
		for _, spec := range [][]string{{"-i", iface, "-j", "ACCEPT"}, {"-o", iface, "-j", "ACCEPT"}} {
			if _, err := Run(append([]string{bin, "-w", "-I", "FORWARD", "1"}, spec...)...); err != nil {
				continue // нет iptables — пересылку ничто не запрещает
			}
			bin, spec := bin, spec
			undo = append(undo, func() { _, _ = Run(append([]string{bin, "-w", "-D", "FORWARD"}, spec...)...) })
		}
	}
	return restore, warns, nil
}
