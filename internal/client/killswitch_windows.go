//go:build windows

package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
)

const blackholeMetric = 9000

func loopbackIndex() (uint32, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	for _, i := range ifs {
		if i.Flags&net.FlagLoopback != 0 {
			return uint32(i.Index), nil
		}
	}
	return 0, errors.New("не найден loopback-интерфейс")
}

func blackholeRow(idx uint32, p netip.Prefix) mibIpforwardRow2 {
	hop := netip.MustParseAddr("127.0.0.1")
	if p.Addr().Is6() {
		hop = netip.IPv6Loopback()
	}
	return newForwardRow(idx, p, hop, blackholeMetric)
}

func addBlackholes(prefixes []netip.Prefix, log *slog.Logger) error {
	idx, err := loopbackIndex()
	if err != nil {
		return err
	}
	ok := 0
	for _, p := range prefixes {
		r := blackholeRow(idx, p)
		if err := createRoute(&r); err != nil {
			log.Warn("kill switch route", "prefix", p, "err", err)
			continue
		}
		ok++
	}
	if ok == 0 && len(prefixes) > 0 {
		return fmt.Errorf("ни один запасной маршрут не добавлен")
	}
	return nil
}

func removeBlackholes(prefixes []netip.Prefix) {
	idx, err := loopbackIndex()
	if err != nil {
		return
	}
	for _, p := range prefixes {
		r := blackholeRow(idx, p)
		deleteRoute(&r)
	}
}
