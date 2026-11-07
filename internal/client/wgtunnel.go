package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/config"
	"veil/internal/vpnconf/wgconf"
	"veil/internal/vpnconf/wgkey"
)

// Engine — то, что умеет система поверх любого транспорта: состояние, ожидание рукопожатия, остановка.
type Engine interface {
	Status() Status
	WaitHandshake(ctx context.Context) error
	Close()
	Reselect()
}

// wgTunnel — обычный WireGuard (без обёртки в HTTPS): прямой UDP до сервера.
type wgTunnel struct {
	dev      *device.Device
	log      *slog.Logger
	endpoint netip.AddrPort
	started  time.Time
	closed   sync.Once
}

// wgEndpoint разрешает Endpoint в IP:порт (имя разрешается системным резолвером до подъёма туннеля).
func wgEndpoint(cfg *wgconf.Config) (netip.AddrPort, error) {
	host, port, err := cfg.EndpointHostPort()
	if err != nil {
		return netip.AddrPort{}, err
	}
	if a, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return netip.AddrPortFrom(a.Unmap(), uint16(port)), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return netip.AddrPort{}, fmt.Errorf("не удалось разрешить %s: %v", host, err)
	}
	best := ips[0]
	for _, ip := range ips {
		if ip.To4() != nil {
			best = ip
			break
		}
	}
	a, _ := netip.AddrFromSlice(best)
	return netip.AddrPortFrom(a.Unmap(), uint16(port)), nil
}

func wgUAPI(cfg *wgconf.Config, ep netip.AddrPort) (string, error) {
	priv, err := wgkey.Hex(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("PrivateKey: %w", err)
	}
	pub, err := wgkey.Hex(cfg.PublicKey)
	if err != nil {
		return "", fmt.Errorf("PublicKey: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\npublic_key=%s\n", priv, pub)
	if cfg.PresharedKey != "" {
		psk, err := wgkey.Hex(cfg.PresharedKey)
		if err != nil {
			return "", fmt.Errorf("PresharedKey: %w", err)
		}
		fmt.Fprintf(&b, "preshared_key=%s\n", psk)
	}
	fmt.Fprintf(&b, "endpoint=%s\n", ep)
	keep := cfg.Keepalive
	if keep == 0 {
		keep = 25
	}
	fmt.Fprintf(&b, "persistent_keepalive_interval=%d\nreplace_allowed_ips=true\n", keep)
	for _, p := range cfg.Routes() {
		fmt.Fprintf(&b, "allowed_ip=%s\n", p)
	}
	return b.String(), nil
}

func startWireGuard(prof *config.Profile, tunDev tun.Device, log *slog.Logger) (*wgTunnel, error) {
	if prof.WG == nil {
		return nil, errors.New("в профиле нет настроек WireGuard")
	}
	ep, err := wgEndpoint(prof.WG)
	if err != nil {
		return nil, err
	}
	uapi, err := wgUAPI(prof.WG, ep)
	if err != nil {
		return nil, err
	}
	logger := device.NewLogger(device.LogLevelError, "wireguard: ")
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("настройка WireGuard: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("запуск WireGuard: %w", err)
	}
	return &wgTunnel{dev: dev, log: log, endpoint: ep, started: time.Now()}, nil
}

type wgStats struct {
	handshake time.Time
	rx, tx    uint64
}

func (t *wgTunnel) stats() wgStats {
	cfg, err := t.dev.IpcGet()
	if err != nil {
		return wgStats{}
	}
	var st wgStats
	var sec, nsec int64
	sc := bufio.NewScanner(strings.NewReader(cfg))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "last_handshake_time_sec":
			fmt.Sscan(v, &sec)
		case "last_handshake_time_nsec":
			fmt.Sscan(v, &nsec)
		case "rx_bytes":
			fmt.Sscan(v, &st.rx)
		case "tx_bytes":
			fmt.Sscan(v, &st.tx)
		}
	}
	if sec != 0 {
		st.handshake = time.Unix(sec, nsec)
	}
	return st
}

func (t *wgTunnel) Status() Status {
	s := t.stats()
	family := "IPv4"
	if t.endpoint.Addr().Is6() {
		family = "IPv6"
	}
	st := Status{State: StateConnecting, Family: family, Transport: "WireGuard", Remote: t.endpoint.String(),
		Since: t.started, Handshake: s.handshake, RxBytes: s.rx, TxBytes: s.tx}
	switch {
	case s.handshake.IsZero():
	case time.Since(s.handshake) < 3*time.Minute:
		st.State = StateConnected
	default:
		st.State = StateReconnecting
	}
	return st
}

func (t *wgTunnel) WaitHandshake(ctx context.Context) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if !t.stats().handshake.IsZero() {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("сервер не ответил на рукопожатие WireGuard: проверьте адрес, порт (UDP) и ключи")
		case <-tick.C:
		}
	}
}

func (t *wgTunnel) Close()    { t.closed.Do(func() { t.dev.Close() }) }
func (t *wgTunnel) Reselect() {}
