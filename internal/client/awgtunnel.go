package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/conn"
	awgdevice "github.com/amnezia-vpn/amneziawg-go/device"
	awgtun "github.com/amnezia-vpn/amneziawg-go/tun"
	wgtun "golang.zx2c4.com/wireguard/tun"

	"veil/internal/config"
	"veil/internal/vpnconf/wgconf"
	"veil/internal/vpnconf/wgkey"
)

// amneziawg-go — отдельная копия wireguard-go со своими пакетами tun/conn/device: типы не смешиваются.
// TUN создаётся пакетом golang.zx2c4.com/wireguard/tun (wintun, кернел-TUN), поэтому оборачиваем его в
// интерфейс tun.Device AmneziaWG: методы совпадают, отличается только тип событий.
type awgTUN struct {
	wgtun.Device
	events chan awgtun.Event
}

func adaptTUN(d wgtun.Device) awgtun.Device {
	a := &awgTUN{Device: d, events: make(chan awgtun.Event, 10)}
	go func() {
		defer close(a.events)
		for e := range d.Events() {
			var out awgtun.Event
			if e&wgtun.EventUp != 0 {
				out |= awgtun.EventUp
			}
			if e&wgtun.EventDown != 0 {
				out |= awgtun.EventDown
			}
			if e&wgtun.EventMTUUpdate != 0 {
				out |= awgtun.EventMTUUpdate
			}
			if out != 0 {
				a.events <- out
			}
		}
	}()
	return a
}

func (a *awgTUN) File() *os.File              { return a.Device.File() }
func (a *awgTUN) Events() <-chan awgtun.Event { return a.events }

// awgTunnel — AmneziaWG: WireGuard с обфускацией рукопожатия, прямой UDP до сервера.
type awgTunnel struct {
	dev      *awgdevice.Device
	log      *slog.Logger
	endpoint netip.AddrPort
	started  time.Time
	closed   sync.Once
}

// awgUAPI формирует настройку устройства: параметры обфускации идут до пира.
func awgUAPI(cfg *wgconf.Config, ep netip.AddrPort) (string, error) {
	a := cfg.AWG
	if a == nil {
		a = &wgconf.AWG{}
	}
	if err := a.Validate(); err != nil {
		return "", err
	}
	priv, err := wgkey.Hex(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("PrivateKey: %w", err)
	}
	pub, err := wgkey.Hex(cfg.PublicKey)
	if err != nil {
		return "", fmt.Errorf("PublicKey: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\n", priv)
	num := func(k string, v int) {
		if v != 0 {
			fmt.Fprintf(&b, "%s=%d\n", k, v)
		}
	}
	num("jc", a.Jc)
	num("jmin", a.Jmin)
	num("jmax", a.Jmax)
	num("s1", a.S1)
	num("s2", a.S2)
	num("s3", a.S3)
	num("s4", a.S4)
	for i, h := range []string{a.H1, a.H2, a.H3, a.H4} {
		if h != "" {
			fmt.Fprintf(&b, "h%d=%s\n", i+1, h)
		}
	}
	for i, v := range a.I {
		if v != "" {
			fmt.Fprintf(&b, "i%d=%s\n", i+1, v)
		}
	}
	fmt.Fprintf(&b, "public_key=%s\n", pub)
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

func startAmneziaWG(prof *config.Profile, tunDev wgtun.Device, log *slog.Logger) (*awgTunnel, error) {
	if prof.WG == nil {
		return nil, errors.New("в профиле нет настроек AmneziaWG")
	}
	ep, err := wgEndpoint(prof.WG)
	if err != nil {
		return nil, err
	}
	uapi, err := awgUAPI(prof.WG, ep)
	if err != nil {
		return nil, err
	}
	logger := awgdevice.NewLogger(awgdevice.LogLevelError, "amneziawg: ")
	dev := awgdevice.NewDevice(adaptTUN(tunDev), awgconn.NewDefaultBind(), logger)
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("настройка AmneziaWG: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("запуск AmneziaWG: %w", err)
	}
	return &awgTunnel{dev: dev, log: log, endpoint: ep, started: time.Now()}, nil
}

func (t *awgTunnel) stats() wgStats {
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
			sec, _ = strconv.ParseInt(v, 10, 64)
		case "last_handshake_time_nsec":
			nsec, _ = strconv.ParseInt(v, 10, 64)
		case "rx_bytes":
			st.rx, _ = strconv.ParseUint(v, 10, 64)
		case "tx_bytes":
			st.tx, _ = strconv.ParseUint(v, 10, 64)
		}
	}
	if sec != 0 {
		st.handshake = time.Unix(sec, nsec)
	}
	return st
}

func (t *awgTunnel) Status() Status {
	s := t.stats()
	family := "IPv4"
	if t.endpoint.Addr().Is6() {
		family = "IPv6"
	}
	st := Status{State: StateConnecting, Family: family, Transport: "AmneziaWG", Remote: t.endpoint.String(),
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

func (t *awgTunnel) WaitHandshake(ctx context.Context) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if !t.stats().handshake.IsZero() {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("сервер не ответил на рукопожатие AmneziaWG: проверьте адрес, порт (UDP), ключи и совпадение параметров Jc/S1/S2/H1–H4 с сервером")
		case <-tick.C:
		}
	}
}

func (t *awgTunnel) Close()    { t.closed.Do(func() { t.dev.Close() }) }
func (t *awgTunnel) Reselect() {}
