//go:build linux

package client

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
)

func prepareRoutes(NetConfig) {}

// defaultVia возвращает шлюз и интерфейс маршрута по умолчанию для семейства.
func defaultVia(family string) (gw, dev string) {
	out, err := run("ip", family, "route", "show", "default")
	if err != nil {
		return "", ""
	}
	f := strings.Fields(strings.SplitN(out, "\n", 2)[0])
	for i := 0; i+1 < len(f); i++ {
		switch f[i] {
		case "via":
			gw = f[i+1]
		case "dev":
			dev = f[i+1]
		}
	}
	return gw, dev
}

func ipBatch(lines []string) error {
	f, err := os.CreateTemp("", "veil-routes-*.batch")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		f.Close()
		return err
	}
	f.Close()
	_, err = run("ip", "-force", "-batch", f.Name())
	return err
}

// applyRoutes добавляет маршруты одним пакетным вызовом ip: тысячи записей за доли секунды.
func applyRoutes(nc NetConfig, via, direct []netip.Prefix, u *undoStack, log *slog.Logger) error {
	var add, del []string
	for _, p := range via {
		add = append(add, fmt.Sprintf("route replace %s dev %s", p, nc.Iface))
		del = append(del, fmt.Sprintf("route delete %s dev %s", p, nc.Iface))
	}
	if len(direct) > 0 {
		gw4, dev4 := defaultVia("-4")
		gw6, dev6 := defaultVia("-6")
		for _, p := range direct {
			gw, dev := gw4, dev4
			if p.Addr().Is6() {
				gw, dev = gw6, dev6
			}
			if dev == "" {
				continue
			}
			if gw != "" {
				add = append(add, fmt.Sprintf("route replace %s via %s dev %s", p, gw, dev))
			} else {
				add = append(add, fmt.Sprintf("route replace %s dev %s", p, dev))
			}
			del = append(del, fmt.Sprintf("route delete %s dev %s", p, dev))
		}
	}
	if len(add) == 0 {
		return nil
	}
	if err := ipBatch(add); err != nil {
		log.Warn("some routes were not added", "err", err)
	}
	u.push(func() { _ = ipBatch(del) })
	log.Info("routes added", "count", len(add))
	return nil
}
