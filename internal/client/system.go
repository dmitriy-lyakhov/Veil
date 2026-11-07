package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"veil/internal/client/netsys"
	"veil/internal/config"
	"veil/internal/routing"
	"veil/internal/transport/netpin"
)

const (
	handshakeTimeout  = 20 * time.Second
	routeRefreshEvery = 10 * time.Minute
)

type SystemTunnel struct {
	Engine
	Iface string

	adminHosts string // результат записи имени «admin» в hosts: AdminHostsOK, AdminHostsFailed или ""

	nc      netsys.NetConfig
	log     *slog.Logger
	u       *netsys.Undo
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
	KillSwitch bool   // блокировать интернет, если VPN оборвался неожиданно
	Interface  string // имя сетевого интерфейса, через который идёт трафик до сервера (пусто — выбирается автоматически)
}

func StartSystem(prof *config.Profile, rc *routing.Config, opt Options, log *slog.Logger) (*SystemTunnel, error) {
	netsys.SetInterface(opt.Interface)
	netpin.SetName(opt.Interface)
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
	netsys.RemoveStaleAdminAlias()
	// остатки блокировки после аварийного завершения прошлого сеанса
	if err := netsys.Unblock(); err != nil {
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
	if err := netsys.PrepareNetwork(nc); err != nil {
		return nil, fmt.Errorf("подготовка сети: %w", err)
	}
	tunDev, err := netsys.CreateTUN(nc.MTU)
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
	u, err := netsys.ApplyNetwork(nc, log)
	if err != nil {
		tn.Close()
		return nil, fmt.Errorf("настройка сети: %w", err)
	}
	adminHosts := netsys.InstallAdminAlias(prof, u, log)
	if opt.KillSwitch {
		if err := netsys.ArmKillSwitch(nc, u, log); err != nil {
			log.Warn("kill switch", "err", err)
		}
	}
	st := &SystemTunnel{Engine: tn, Iface: nc.Iface, adminHosts: adminHosts, nc: nc, log: log, u: u, rcfg: rc, stop: make(chan struct{}),
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

// Status дополняет состояние движка сведением о записи «admin» в hosts.
func (s *SystemTunnel) Status() Status {
	st := s.Engine.Status()
	st.AdminHosts = s.adminHosts
	return st
}

func (s *SystemTunnel) Close() {
	s.stopped.Do(func() { close(s.stop) })
	s.u.Run()
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
			if err := netsys.ApplyRoutes(s.nc, newVia, newDirect, s.u, s.log); err != nil {
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

func netConfigFor(p *config.Profile) (netsys.NetConfig, error) {
	if !p.IsVeil() {
		return protocolNetConfig(p)
	}
	nc := netsys.NetConfig{MTU: p.EffectiveMTU()}
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

// protocolNetConfig — сетевые настройки для профилей обычных протоколов (сейчас WireGuard).
func protocolNetConfig(p *config.Profile) (netsys.NetConfig, error) {
	c := p.WG
	if c == nil {
		return netsys.NetConfig{}, fmt.Errorf("протокол %q не поддерживается этим клиентом", p.Protocol)
	}
	nc := netsys.NetConfig{MTU: c.MTU, BypassServers: true}
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
