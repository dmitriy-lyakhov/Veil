//go:build darwin

package netsys

import (
	"log/slog"
	"net/netip"
)

func PrepareRoutes(NetConfig) {}

func routeFamily(p netip.Prefix) string {
	if p.Addr().Is6() {
		return "-inet6"
	}
	return "-inet"
}

// applyRoutes добавляет маршруты по одному (route(8) пакетов не умеет), поэтому на
// очень больших списках применение занимает заметное время.
func ApplyRoutes(nc NetConfig, via, direct []netip.Prefix, u *Undo, log *slog.Logger) error {
	var added []func()
	for _, p := range via {
		p := p
		if _, err := Run("route", "-q", "-n", "add", routeFamily(p), p.String(), "-interface", nc.Iface); err == nil {
			added = append(added, func() { _, _ = Run("route", "-q", "-n", "delete", routeFamily(p), p.String()) })
		}
	}
	if len(direct) > 0 {
		gws := map[string]string{}
		for _, fam := range []string{"-inet", "-inet6"} {
			if out, err := Run("route", "-n", "get", fam, "default"); err == nil {
				gws[fam] = fields(out, "gateway")
			}
		}
		for _, p := range direct {
			p := p
			gw := gws[routeFamily(p)]
			if gw == "" {
				continue
			}
			if _, err := Run("route", "-q", "-n", "add", routeFamily(p), p.String(), gw); err == nil {
				added = append(added, func() { _, _ = Run("route", "-q", "-n", "delete", routeFamily(p), p.String()) })
			}
		}
	}
	if len(added) > 0 {
		log.Info("routes added", "count", len(added))
		u.Push(func() {
			for _, f := range added {
				f()
			}
		})
	}
	return nil
}
