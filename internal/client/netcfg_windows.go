//go:build windows

package client

import (
	"veil/internal/netpin"
	"errors"
	"strconv"

	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

const winIfname = "Veil"

func createTUN(mtu int) (tun.Device, error) {
	if err := ensureWintun(); err != nil {
		return nil, err
	}
	return tun.CreateTUN(winIfname, mtu)
}

func ipFamily(a netip.Addr) string {
	if a.Is4() || a.Is4In6() {
		return "ipv4"
	}
	return "ipv6"
}

func v4Mask(p netip.Prefix) string {
	return net.IP(net.CIDRMask(p.Bits(), 32)).String()
}

// netsh сразу после создания адаптера иногда отвечает «файл не найден» —
// интерфейс ещё не зарегистрирован, поэтому первая команда повторяется.
func netshRetry(args ...string) error {
	var err error
	for i := 0; i < 10; i++ {
		if _, err = run(append([]string{"netsh"}, args...)...); err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return err
}

func netsh(args ...string) error {
	_, err := run(append([]string{"netsh"}, args...)...)
	return err
}

func applyNetwork(nc NetConfig, log *slog.Logger) (*undoStack, error) {
	u := &undoStack{}
	name := nc.Iface

	if err := netshRetry("interface", "ipv4", "set", "address", "name="+name, "source=static",
		"address="+nc.Addr4.Addr().String(), "mask="+v4Mask(nc.Addr4)); err != nil {
		return nil, err
	}
	if nc.Addr6.IsValid() {
		if err := netsh("interface", "ipv6", "add", "address", "interface="+name,
			"address="+nc.Addr6.Addr().String()+"/"+fmt.Sprint(nc.Addr6.Bits()), "store=active"); err != nil {
			log.Warn("ipv6 address", "err", err)
		}
	}
	if nc.BypassServers {
		for _, srv := range nc.Servers {
			if err := bypassServer(srv, u); err != nil {
				log.Warn("server bypass route", "server", srv, "err", err)
			}
		}
	}
	mtu := fmt.Sprint(nc.MTU)
	for _, fam := range []string{"ipv4", "ipv6"} {
		if err := netsh("interface", fam, "set", "subinterface", name, "mtu="+mtu, "store=active"); err != nil {
			log.Warn("mtu", "family", fam, "err", err)
		}
		// низкая метрика интерфейса — DNS туннеля опрашивается первым
		if err := netsh("interface", fam, "set", "interface", name, "metric=1"); err != nil {
			log.Warn("metric", "family", fam, "err", err)
		}
	}


	routes := []struct{ fam, prefix, hop string }{
		{"ipv4", "0.0.0.0/1", "0.0.0.0"}, {"ipv4", "128.0.0.0/1", "0.0.0.0"},
		{"ipv6", "::/1", "::"}, {"ipv6", "8000::/1", "::"},
	}
	if !fullTunnel(nc) {
		routes = nil
	}
	for _, r := range routes {
		err := netsh("interface", r.fam, "add", "route", "prefix="+r.prefix, "interface="+name,
			"nexthop="+r.hop, "metric=1", "store=active")
		if err == nil {
			continue
		}
		if r.fam == "ipv4" {
			u.run()
			return nil, err
		}
		log.Warn("ipv6 route, IPv6-трафик может идти мимо туннеля", "prefix", r.prefix, "err", err)
	}

	// Хост-маршрут на сам адрес сервера через туннель. Без него сервер из той же
	// подсети, что и этот компьютер, остаётся доступен напрямую (маршрут подсети
	// точнее /1), и сайт-прикрытие с /admin не видят в запросе клиента VPN.
	for _, srv := range nc.Servers {
		if nc.BypassServers {
			break
		}
		fam, prefix, hop := "ipv4", srv.String()+"/32", "0.0.0.0"
		if srv.Is6() {
			fam, prefix, hop = "ipv6", srv.String()+"/128", "::"
		}
		if err := netsh("interface", fam, "add", "route", "prefix="+prefix, "interface="+name,
			"nexthop="+hop, "metric=1", "store=active"); err != nil {
			log.Warn("server host route", "server", srv, "err", err)
		}
	}

	if err := applyPlan(nc, u, log); err != nil {
		log.Warn("routing lists", "err", err)
	}

	var first4, first6 = true, true
	for _, d := range nc.DNS {
		fam, first := ipFamily(d), &first6
		if fam == "ipv4" {
			first = &first4
		}
		var err error
		if *first {
			err = netsh("interface", fam, "set", "dnsservers", "name="+name, "source=static", "address="+d.String(), "validate=no")
			*first = false
		} else {
			err = netsh("interface", fam, "add", "dnsservers", "name="+name, "address="+d.String(), "validate=no")
		}
		if err != nil {
			log.Warn("dns", "server", d, "err", err)
		}
	}
	return u, nil
}

// Прямого маршрута до сервера в обход туннеля нет: так любой другой трафик на
// адрес сервера (сайт-прикрытие, /admin) идёт через туннель и сервер видит в
// нём клиента VPN. Собственные сокеты Veil привязываются к физическому
// интерфейсу через IP_UNICAST_IF, поэтому в туннель не попадают.
func physicalIf(srv netip.Addr) (uint32, error) {
	ps := fmt.Sprintf(`$r = Find-NetRoute -RemoteIPAddress '%s' | Where-Object { $_.NextHop } | Select-Object -First 1; "$($r.InterfaceIndex)"`, srv)
	out, err := run("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(strings.TrimSpace(out), 10, 32)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("не удалось определить интерфейс до %s: %q", srv, out)
	}
	return uint32(n), nil
}

func prepareNetwork(nc NetConfig) error {
	var v4, v6 uint32
	var lastErr error
	for _, srv := range nc.Servers {
		idx, err := physicalIf(srv)
		if err != nil {
			// например link-local IPv6 без зоны: этот адрес пропускаем,
			// клиент уйдёт на другое семейство адресов
			lastErr = err
			continue
		}
		if srv.Is6() {
			v6 = idx
		} else {
			v4 = idx
		}
	}
	if v4 == 0 && v6 == 0 {
		if lastErr == nil {
			lastErr = errors.New("нет адресов сервера")
		}
		return lastErr
	}
	netpin.Set(v4, v6)
	prepareRoutes(nc)
	return nil
}
