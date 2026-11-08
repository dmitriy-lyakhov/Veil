package server

// Связывание через сервер (хаб):
//   client-to-client — клиенты видят друг друга по адресам туннеля;
//   client-to-site   — клиенты ↔ локальные сети за клиентами-шлюзами (Peer.Subnets);
//   site-to-site     — сети за разными шлюзами ↔ друг с другом.
// Сети шлюза сервер маршрутизирует в TUN, WireGuard направляет их к пиру шлюза (allowed_ip),
// а пропускать ли трафик между участниками, решают правила пересылки (NetSetup.ApplyLinks).

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"veil/internal/config"
)

const maxSubnetsPerClient = 32

// LinkPolicy — всё, что нужно сетевой настройке хоста для правил связывания.
type LinkPolicy struct {
	Links  config.Links
	Tunnel []netip.Prefix // подсети туннеля: адреса клиентов
	Sites  []netip.Prefix // сети за шлюзами (всех клиентов, кроме заблокированных)
}

// AllowAll — политика ничего не ограничивает (тогда правила связывания не нужны).
func (p LinkPolicy) AllowAll() bool {
	l := p.Links
	return l.ClientToClient && l.ClientToSite && l.SiteToSite
}

// LinkRule — правило пересылки между участниками (трафик, который и входит, и выходит через TUN).
// Нулевой Src/Dst — любой адрес; Established — правило для уже установленных соединений.
type LinkRule struct {
	Src, Dst    netip.Prefix
	Accept      bool
	Established bool
}

// Rules — упорядоченные правила одного семейства адресов (первое совпадение решает).
// Последнее правило — для всего остального: разрешить, если политика ничего не запрещает.
func (p LinkPolicy) Rules(v6 bool) []LinkRule {
	fam := func(ps []netip.Prefix) []netip.Prefix {
		var out []netip.Prefix
		for _, x := range ps {
			if x.Addr().Is6() == v6 {
				out = append(out, x)
			}
		}
		return out
	}
	sites, tunnel := fam(p.Sites), fam(p.Tunnel)
	rules := []LinkRule{{Accept: true, Established: true}}
	for _, d := range sites {
		for _, s := range sites {
			if s != d {
				rules = append(rules, LinkRule{Src: s, Dst: d, Accept: p.Links.SiteToSite})
			}
		}
		for _, t := range tunnel {
			rules = append(rules,
				LinkRule{Src: t, Dst: d, Accept: p.Links.ClientToSite},
				LinkRule{Src: d, Dst: t, Accept: p.Links.ClientToSite})
		}
	}
	for _, t := range tunnel {
		rules = append(rules, LinkRule{Src: t, Dst: t, Accept: p.Links.ClientToClient})
	}
	return append(rules, LinkRule{Accept: p.AllowAll()})
}

// LinkPolicy возвращает текущую политику связывания.
func (s *Server) LinkPolicy() LinkPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.linkPolicyLocked()
}

func (s *Server) linkPolicyLocked() LinkPolicy {
	p := LinkPolicy{Links: s.cfg.EffectiveLinks(), Tunnel: append([]netip.Prefix(nil), s.tunnelNets...)}
	for _, pe := range s.cfg.Peers {
		if pe.Disabled || !localPeer(pe) {
			continue
		}
		for _, sn := range pe.Subnets {
			if pr, err := netip.ParsePrefix(sn); err == nil {
				p.Sites = append(p.Sites, pr)
			}
		}
	}
	return p
}

// OnLinksChange регистрирует вызов после каждого изменения политики связывания или сетей шлюзов.
func (s *Server) OnLinksChange(f func()) {
	s.mu.Lock()
	s.onLinks = f
	s.mu.Unlock()
}

func (s *Server) linksChanged() {
	s.mu.Lock()
	f := s.onLinks
	s.mu.Unlock()
	if f != nil {
		f()
	}
}

// SetLinks меняет политику связывания.
func (s *Server) SetLinks(l config.Links) error {
	s.mu.Lock()
	s.cfg.Links = &l
	err := s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.log.Info("links changed", "client_to_client", l.ClientToClient, "client_to_site", l.ClientToSite, "site_to_site", l.SiteToSite)
	s.linksChanged()
	return nil
}

// normalizeSubnets проверяет сети за шлюзом: префиксы без лишних бит хоста, не маршрут по умолчанию,
// не служебные адреса и не пересекаются с туннелем.
func normalizeSubnets(in []string, tunnel []netip.Prefix) ([]string, error) {
	var out []string
	seen := map[netip.Prefix]bool{}
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			a, aerr := netip.ParseAddr(raw)
			if aerr != nil {
				return nil, fmt.Errorf("«%s» — не подсеть (нужно, например, 192.168.1.0/24)", raw)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		p = p.Masked()
		a := p.Addr()
		switch {
		case p.Bits() == 0 || (a.Is4() && p.Bits() < 8) || (a.Is6() && p.Bits() < 16):
			return nil, fmt.Errorf("%s: слишком широкая сеть — укажите локальную сеть шлюза", p)
		case a.IsLoopback() || a.IsMulticast() || a.IsLinkLocalUnicast() || a.IsUnspecified():
			return nil, fmt.Errorf("%s: служебные адреса нельзя маршрутизировать", p)
		}
		for _, t := range tunnel {
			if t.Overlaps(p) {
				return nil, fmt.Errorf("%s пересекается с подсетью туннеля %s", p, t)
			}
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p.String())
		}
	}
	if len(out) > maxSubnetsPerClient {
		return nil, fmt.Errorf("не больше %d сетей за одним клиентом", maxSubnetsPerClient)
	}
	return out, nil
}

// SetClientSubnets задаёт сети за клиентом-шлюзом и сразу применяет их (WireGuard, маршруты, правила).
func (s *Server) SetClientSubnets(name string, subnets []string) error {
	s.mu.Lock()
	idx := s.peerIndexLocked(name)
	if idx < 0 || s.cfg.Peers[idx].RelayOf != "" {
		s.mu.Unlock()
		return fmt.Errorf("клиент «%s» не найден", name)
	}
	p := &s.cfg.Peers[idx]
	if !localPeer(*p) {
		s.mu.Unlock()
		return errors.New("сети за клиентом задаются на выходном узле, к которому клиент подключён напрямую")
	}
	norm, err := normalizeSubnets(subnets, s.tunnelNets)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	for i, other := range s.cfg.Peers {
		if i == idx {
			continue
		}
		for _, os := range other.Subnets {
			op, err := netip.ParsePrefix(os)
			if err != nil {
				continue
			}
			for _, ns := range norm {
				if np := netip.MustParsePrefix(ns); np.Overlaps(op) {
					s.mu.Unlock()
					return fmt.Errorf("%s пересекается с сетью %s клиента «%s»", ns, os, other.Name)
				}
			}
		}
	}
	p.Subnets = norm
	uapi, err := peerConfig(*p)
	if err == nil {
		err = s.dev.IpcSet(uapi)
	}
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("wireguard: %w", err)
	}
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.log.Info("client subnets changed", "name", name, "subnets", norm)
	s.linksChanged()
	return nil
}
