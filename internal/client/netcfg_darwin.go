package client

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"

	"golang.zx2c4.com/wireguard/tun"
)

func createTUN(mtu int) (tun.Device, error) {
	return tun.CreateTUN("utun", mtu)
}

func applyNetwork(nc NetConfig, log *slog.Logger) (*undoStack, error) {
	u := &undoStack{}
	steps := [][]string{
		{"ifconfig", nc.Iface, "inet", nc.Addr4.String(), nc.Addr4.Addr().String(), "alias"},
		{"ifconfig", nc.Iface, "inet6", nc.Addr6.Addr().String(), "prefixlen", fmt.Sprint(nc.Addr6.Bits()), "alias"},
		{"ifconfig", nc.Iface, "mtu", fmt.Sprint(nc.MTU), "up"},
	}
	for _, s := range steps {
		if !nc.Addr6.IsValid() && stepIsV6(s) {
			continue
		}
		if _, err := run(s...); err != nil {
			return nil, err
		}
	}
	for _, srv := range nc.Servers {
		if err := bypassRoute(srv, u); err != nil {
			log.Warn("server bypass route", "server", srv, "err", err)
		}
	}
	routes := []struct{ family, prefix string }{
		{"-inet", "0.0.0.0/1"}, {"-inet", "128.0.0.0/1"},
		{"-inet6", "::/1"}, {"-inet6", "8000::/1"},
	}
	if !fullTunnel(nc) {
		routes = nil
	}
	for _, r := range routes {
		if _, err := run("route", "-q", "-n", "add", r.family, r.prefix, "-interface", nc.Iface); err != nil {
			u.run()
			return nil, err
		}
		r := r
		u.push(func() { _, _ = run("route", "-q", "-n", "delete", r.family, r.prefix, "-interface", nc.Iface) })
	}
	if err := applyPlan(nc, u, log); err != nil {
		log.Warn("routing lists", "err", err)
	}
	if err := setDNS(nc.DNS, u, log); err != nil {
		log.Warn("dns", "err", err)
	}
	return u, nil
}

func bypassRoute(srv netip.Addr, u *undoStack) error {
	family, host := "-inet", srv.String()+"/32"
	if srv.Is6() {
		family, host = "-inet6", srv.String()+"/128"
	}
	out, err := run("route", "-n", "get", family, "default")
	if err != nil {
		return fmt.Errorf("нет маршрута по умолчанию: %w", err)
	}
	gw, iface := fields(out, "gateway"), fields(out, "interface")
	args := []string{"route", "-q", "-n", "add", family, host}
	switch {
	case gw != "":
		args = append(args, gw)
	case iface != "":
		args = append(args, "-interface", iface)
	default:
		return fmt.Errorf("не удалось определить шлюз")
	}
	if _, err := run(args...); err != nil {
		return err
	}
	u.push(func() { _, _ = run("route", "-q", "-n", "delete", family, host) })
	return nil
}

func setDNS(servers []netip.Addr, u *undoStack, log *slog.Logger) error {
	if len(servers) == 0 {
		return nil
	}
	out, err := run("networksetup", "-listallnetworkservices")
	if err != nil {
		return err
	}
	dns := make([]string, len(servers))
	for i, s := range servers {
		dns[i] = s.String()
	}
	lines := strings.Split(out, "\n")
	for _, svc := range lines[1:] {
		svc = strings.TrimSpace(svc)
		if svc == "" || strings.HasPrefix(svc, "*") {
			continue
		}
		prev, err := run("networksetup", "-getdnsservers", svc)
		if err != nil {
			continue
		}
		restore := []string{"Empty"}
		if !strings.Contains(prev, " ") {
			restore = strings.Fields(prev)
		}
		if _, err := run(append([]string{"networksetup", "-setdnsservers", svc}, dns...)...); err != nil {
			log.Warn("set dns", "service", svc, "err", err)
			continue
		}
		svc := svc
		u.push(func() { _, _ = run(append([]string{"networksetup", "-setdnsservers", svc}, restore...)...) })
	}
	return nil
}
