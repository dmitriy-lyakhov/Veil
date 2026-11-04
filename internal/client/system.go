package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os/exec"
	"strings"
	"sync"
	"time"

	"veil/internal/config"
	"veil/internal/routing"
)

const (
	handshakeTimeout  = 20 * time.Second
	routeRefreshEvery = 10 * time.Minute
)

type NetConfig struct {
	Iface   string
	Addr4   netip.Prefix
	Addr6   netip.Prefix
	MTU     int
	Servers []netip.Addr
	DNS     []netip.Addr
	Plan    *routing.Plan // nil — весь трафик в туннель

	// BypassServers — собственные пакеты движка идут до сервера в обход туннеля отдельным маршрутом
	// (протоколы без привязки сокетов к интерфейсу: обычный WireGuard и т.п.)
	BypassServers bool
}

type SystemTunnel struct {
	Engine
	Iface string

	nc      NetConfig
	log     *slog.Logger
	u       *undoStack
	rcfg    *routing.Config
	stop    chan struct{}
	stopped sync.Once

	mu            sync.Mutex
	appliedVia    map[netip.Prefix]bool
	appliedDirect map[netip.Prefix]bool
}

// StartSystem поднимает туннель и настраивает систему. rc — списки «через VPN»/«мимо VPN»
// (nil или пустые — весь трафик в туннель).
// Options — параметры подключения из настроек приложения.
type Options struct {
	KillSwitch bool // блокировать интернет, если VPN оборвался неожиданно
}

func StartSystem(prof *config.Profile, rc *routing.Config, opt Options, log *slog.Logger) (*SystemTunnel, error) {
	if prof.Protocol == "openvpn" {
		return startOpenVPNSystem(prof, rc, opt, log)
	}
	if prof.Protocol == "ipsec" {
		return startIPsec(prof, rc, opt, log)
	}
	nc, err := netConfigFor(prof)
	if err != nil {
		return nil, err
	}
	// остатки блокировки после аварийного завершения прошлого сеанса
	if err := Unblock(); err != nil {
		log.Warn("unblock", "err", err)
	}
	if rc != nil && rc.Enabled && !rc.Empty() {
		// домены разрешаются до подъёма туннеля — обычным путём, мимо него
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		plan, err := routing.Build(ctx, rc, nil)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("списки адресов: %w", err)
		}
		for _, w := range plan.Warnings {
			log.Warn("routing list", "warning", w)
		}
		log.Info("routing plan", "summary", plan.Summary())
		nc.Plan = plan
	}
	if err := prepareNetwork(nc); err != nil {
		return nil, fmt.Errorf("подготовка сети: %w", err)
	}
	tunDev, err := createTUN(nc.MTU)
	if err != nil {
		return nil, fmt.Errorf("создание TUN: %w", err)
	}
	if nc.Iface, err = tunDev.Name(); err != nil {
		tunDev.Close()
		return nil, err
	}
	var tn Engine
	if prof.IsVeil() {
		tn, err = Start(prof, tunDev, nil, log)
	} else {
		if prof.Protocol == "amneziawg" {
			tn, err = startAmneziaWG(prof, tunDev, log)
		} else {
			tn, err = startWireGuard(prof, tunDev, log)
		}
	}
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
	u, err := applyNetwork(nc, log)
	if err != nil {
		tn.Close()
		return nil, fmt.Errorf("настройка сети: %w", err)
	}
	if opt.KillSwitch {
		if err := armKillSwitch(nc, u, log); err != nil {
			log.Warn("kill switch", "err", err)
		}
	}
	st := &SystemTunnel{Engine: tn, Iface: nc.Iface, nc: nc, log: log, u: u, rcfg: rc, stop: make(chan struct{}),
		appliedVia: map[netip.Prefix]bool{}, appliedDirect: map[netip.Prefix]bool{}}
	if nc.Plan != nil {
		for _, p := range nc.Plan.Via {
			st.appliedVia[p] = true
		}
		for _, p := range nc.Plan.Direct {
			st.appliedDirect[p] = true
		}
		if len(nc.Plan.ViaDomains)+len(nc.Plan.DirDomains) > 0 {
			go st.refreshLoop()
		}
	}
	log.Info("tunnel up", "iface", nc.Iface, "status", tn.Status())
	return st, nil
}

func (s *SystemTunnel) Close() {
	s.stopped.Do(func() { close(s.stop) })
	s.u.run()
	s.Engine.Close()
}

// refreshLoop периодически заново разрешает домены из списков (адреса CDN меняются)
// и добавляет появившиеся маршруты. Старые остаются до отключения.
func (s *SystemTunnel) refreshLoop() {
	t := time.NewTicker(routeRefreshEvery)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		plan, err := routing.Build(ctx, s.rcfg, nil)
		cancel()
		if err != nil {
			s.log.Warn("routing refresh", "err", err)
			continue
		}
		s.mu.Lock()
		var newVia, newDirect []netip.Prefix
		for _, p := range plan.Via {
			if !s.appliedVia[p] {
				newVia = append(newVia, p)
			}
		}
		for _, p := range plan.Direct {
			if !s.appliedDirect[p] {
				newDirect = append(newDirect, p)
			}
		}
		if len(newVia)+len(newDirect) > 0 {
			if err := applyRoutes(s.nc, newVia, newDirect, s.u, s.log); err != nil {
				s.log.Warn("routing refresh apply", "err", err)
			} else {
				for _, p := range newVia {
					s.appliedVia[p] = true
				}
				for _, p := range newDirect {
					s.appliedDirect[p] = true
				}
				s.log.Info("routing refreshed", "new_via", len(newVia), "new_direct", len(newDirect))
			}
		}
		s.mu.Unlock()
	}
}

// applyPlan добавляет маршруты списков. Вызывается из applyNetwork после основных настроек.
func applyPlan(nc NetConfig, u *undoStack, log *slog.Logger) error {
	if nc.Plan == nil {
		return nil
	}
	via := append([]netip.Prefix(nil), nc.Plan.Via...)
	if nc.Plan.Selective {
		// в выборочном режиме запросы к DNS туннеля тоже должны идти в туннель
		for _, d := range nc.DNS {
			via = append(via, netip.PrefixFrom(d, d.BitLen()))
		}
	}
	return applyRoutes(nc, via, nc.Plan.Direct, u, log)
}

// fullTunnel — нужен ли общий маршрут «всё в туннель» (половинки 0.0.0.0/1 и 128.0.0.0/1).
func fullTunnel(nc NetConfig) bool { return nc.Plan == nil || !nc.Plan.Selective }

func netConfigFor(p *config.Profile) (NetConfig, error) {
	if !p.IsVeil() {
		return protocolNetConfig(p)
	}
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
	mu  sync.Mutex
	fns []func()
}

func (u *undoStack) push(f func()) {
	u.mu.Lock()
	u.fns = append(u.fns, f)
	u.mu.Unlock()
}

func (u *undoStack) run() {
	u.mu.Lock()
	fns := u.fns
	u.fns = nil
	u.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

func run(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	hideWindow(cmd)
	raw, err := cmd.CombinedOutput()
	out := []byte(decodeOutput(raw))
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

// protocolNetConfig — сетевые настройки для профилей обычных протоколов (сейчас WireGuard).
func protocolNetConfig(p *config.Profile) (NetConfig, error) {
	c := p.WG
	if c == nil {
		return NetConfig{}, fmt.Errorf("протокол %q не поддерживается этим клиентом", p.Protocol)
	}
	nc := NetConfig{MTU: c.MTU, BypassServers: true}
	if nc.MTU == 0 {
		nc.MTU = 1380
	}
	for _, a := range c.Address {
		switch {
		case a.Addr().Is4() && !nc.Addr4.IsValid():
			nc.Addr4 = a
		case a.Addr().Is6() && !nc.Addr6.IsValid():
			nc.Addr6 = a
		}
	}
	if !nc.Addr4.IsValid() {
		return nc, fmt.Errorf("в Address нужен адрес IPv4")
	}
	ep, err := wgEndpoint(c)
	if err != nil {
		return nc, err
	}
	nc.Servers = []netip.Addr{ep.Addr()}
	nc.DNS = c.DNS
	if len(nc.DNS) == 0 {
		for _, d := range p.DNS {
			if a, err := netip.ParseAddr(d); err == nil {
				nc.DNS = append(nc.DNS, a)
			}
		}
	}
	// AllowedIPs не «весь трафик»: в туннель идут только они
	if routes := c.Routes(); !coversEverything(routes) {
		nc.Plan = &routing.Plan{Selective: true, Via: routes}
	}
	return nc, nil
}

func coversEverything(ps []netip.Prefix) bool {
	v4, v6 := false, false
	for _, p := range ps {
		if p.Bits() == 0 {
			if p.Addr().Is4() {
				v4 = true
			} else {
				v6 = true
			}
		}
	}
	return v4 && v6
}

// stepIsV6 — шаг настройки адаптера, относящийся к IPv6.
func stepIsV6(args []string) bool {
	for _, a := range args {
		if a == "-6" || a == "inet6" {
			return true
		}
	}
	return false
}
