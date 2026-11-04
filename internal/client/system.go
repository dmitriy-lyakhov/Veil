package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os/exec"
	"strings"
	"time"

	"veil/internal/config"
)

const handshakeTimeout = 20 * time.Second

type NetConfig struct {
	Iface   string
	Addr4   netip.Prefix
	Addr6   netip.Prefix
	MTU     int
	Servers []netip.Addr
	DNS     []netip.Addr
}

type SystemTunnel struct {
	*Tunnel
	Iface string
	undo  func()
}

func StartSystem(prof *config.Profile, log *slog.Logger) (*SystemTunnel, error) {
	nc, err := netConfigFor(prof)
	if err != nil {
		return nil, err
	}
	tunDev, err := createTUN(nc.MTU)
	if err != nil {
		return nil, fmt.Errorf("создание TUN: %w", err)
	}
	if nc.Iface, err = tunDev.Name(); err != nil {
		tunDev.Close()
		return nil, err
	}
	tn, err := Start(prof, tunDev, nil, log)
	if err != nil {
		tunDev.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		tn.Close()
		return nil, err
	}
	undo, err := applyNetwork(nc, log)
	if err != nil {
		tn.Close()
		return nil, fmt.Errorf("настройка сети: %w", err)
	}
	log.Info("tunnel up", "iface", nc.Iface, "status", tn.Status())
	return &SystemTunnel{Tunnel: tn, Iface: nc.Iface, undo: undo}, nil
}

func (s *SystemTunnel) Close() {
	s.undo()
	s.Tunnel.Close()
}

func netConfigFor(p *config.Profile) (NetConfig, error) {
	nc := NetConfig{MTU: p.EffectiveMTU()}
	var err error
	if nc.Addr4, err = netip.ParsePrefix(p.Address4); err != nil {
		return nc, fmt.Errorf("address4: %w", err)
	}
	if nc.Addr6, err = netip.ParsePrefix(p.Address6); err != nil {
		return nc, fmt.Errorf("address6: %w", err)
	}
	targets, err := p.AllTargets()
	if err != nil {
		return nc, err
	}
	for _, t := range targets {
		nc.Servers = append(nc.Servers, t.Addr.Addr())
	}
	for _, d := range p.DNS {
		if a, err := netip.ParseAddr(d); err == nil {
			nc.DNS = append(nc.DNS, a)
		}
	}
	return nc, nil
}

type undoStack struct {
	fns []func()
}

func (u *undoStack) push(f func()) { u.fns = append(u.fns, f) }

func (u *undoStack) run() {
	for i := len(u.fns) - 1; i >= 0; i-- {
		u.fns[i]()
	}
	u.fns = nil
}

func run(args ...string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("%s: %v: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

func fields(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
