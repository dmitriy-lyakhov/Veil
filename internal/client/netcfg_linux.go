package client

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os/exec"
	"strings"

	"golang.zx2c4.com/wireguard/tun"
)

const linuxIfname = "veilc0"

func createTUN(mtu int) (tun.Device, error) {
	return tun.CreateTUN(linuxIfname, mtu)
}

func applyNetwork(nc NetConfig, log *slog.Logger) (func(), error) {
	u := &undoStack{}
	steps := [][]string{
		{"ip", "addr", "replace", nc.Addr4.String(), "dev", nc.Iface},
		{"ip", "-6", "addr", "replace", nc.Addr6.String(), "dev", nc.Iface},
		{"ip", "link", "set", "dev", nc.Iface, "mtu", fmt.Sprint(nc.MTU), "up"},
	}
	for _, s := range steps {
		if _, err := run(s...); err != nil {
			return nil, err
		}
	}
	for _, srv := range nc.Servers {
		if err := bypassRoute(srv, u); err != nil {
			log.Warn("server bypass route", "server", srv, "err", err)
		}
	}
	for _, r := range []struct{ family, prefix string }{
		{"-4", "0.0.0.0/1"}, {"-4", "128.0.0.0/1"},
		{"-6", "::/1"}, {"-6", "8000::/1"},
	} {
		if _, err := run("ip", r.family, "route", "replace", r.prefix, "dev", nc.Iface); err != nil {
			u.run()
			return nil, err
		}
	}
	if len(nc.DNS) > 0 {
		if _, err := exec.LookPath("resolvectl"); err == nil {
			args := []string{"resolvectl", "dns", nc.Iface}
			for _, d := range nc.DNS {
				args = append(args, d.String())
			}
			if _, err := run(args...); err != nil {
				log.Warn("dns", "err", err)
			} else {
				_, _ = run("resolvectl", "domain", nc.Iface, "~.")
				_, _ = run("resolvectl", "default-route", nc.Iface, "true")
				u.push(func() { _, _ = run("resolvectl", "revert", nc.Iface) })
			}
		} else {
			log.Warn("resolvectl not found, DNS left unchanged")
		}
	}
	return u.run, nil
}

func bypassRoute(srv netip.Addr, u *undoStack) error {
	family, host := "-4", srv.String()+"/32"
	if srv.Is6() {
		family, host = "-6", srv.String()+"/128"
	}
	out, err := run("ip", family, "route", "get", srv.String())
	if err != nil {
		return err
	}
	f := strings.Fields(out)
	args := []string{"ip", family, "route", "replace", host}
	for i := 0; i+1 < len(f); i++ {
		switch f[i] {
		case "via", "dev":
			args = append(args, f[i], f[i+1])
		}
	}
	if _, err := run(args...); err != nil {
		return err
	}
	u.push(func() { _, _ = run("ip", family, "route", "delete", host) })
	return nil
}
