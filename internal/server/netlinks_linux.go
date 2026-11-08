package server

// Правила связывания (client-to-client, client-to-site, site-to-site) и маршруты к сетям за шлюзами.
// iptables: свои цепочки VEIL-LINKS (filter, переход из FORWARD для трафика TUN→TUN) и VEIL-SITES
// (nat, маскарадинг сетей шлюзов при выходе в интернет); nftables: таблица inet veil пересобирается целиком.

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

const (
	linksChain = "VEIL-LINKS"
	sitesChain = "VEIL-SITES"
)

// ApplyLinks применяет политику связывания: маршруты к сетям шлюзов в TUN и правила пересылки.
func (n *NetSetup) ApplyLinks(p LinkPolicy) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.policy = p
	if err := n.syncRoutes(p.Sites); err != nil {
		n.log.Warn("site routes", "err", err)
	}
	if n.backend == "nft" {
		return n.nftApply()
	}
	return n.iptablesLinks(p)
}

// syncRoutes приводит маршруты «сеть шлюза → TUN» к списку sites.
func (n *NetSetup) syncRoutes(sites []netip.Prefix) error {
	if n.routes == nil {
		n.routes = map[netip.Prefix]bool{}
		n.undo = append(n.undo, func() {
			for r := range n.routes {
				_ = run(routeCmd("del", r, n.iface)...)
			}
		})
	}
	want := map[netip.Prefix]bool{}
	var errs []string
	for _, s := range sites {
		want[s] = true
		if err := run(routeCmd("replace", s, n.iface)...); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		n.routes[s] = true
	}
	for r := range n.routes {
		if !want[r] {
			_ = run(routeCmd("del", r, n.iface)...)
			delete(n.routes, r)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func routeCmd(op string, p netip.Prefix, iface string) []string {
	if p.Addr().Is6() {
		return []string{"ip", "-6", "route", op, p.String(), "dev", iface}
	}
	return []string{"ip", "route", op, p.String(), "dev", iface}
}

func (n *NetSetup) iptablesLinks(p LinkPolicy) error {
	for _, bin := range []string{"iptables", "ip6tables"} {
		v6 := bin == "ip6tables"
		if err := n.iptablesLinksFamily(bin, p, v6); err != nil {
			if v6 {
				n.log.Warn("ipv6 link rules failed", "err", err)
				continue
			}
			return err
		}
	}
	return nil
}

func (n *NetSetup) iptablesLinksFamily(bin string, p LinkPolicy, v6 bool) error {
	ipt := func(args ...string) error { return run(append([]string{bin, "-w"}, args...)...) }
	// цепочки и переходы в них создаются один раз; дальше цепочки только перезаполняются
	key := bin + "-chains"
	if !n.done[key] {
		_ = ipt("-N", linksChain)
		_ = ipt("-t", "nat", "-N", sitesChain)
		jump := []string{"FORWARD", "-i", n.iface, "-o", n.iface, "-j", linksChain}
		if ipt(append([]string{"-C"}, jump...)...) != nil {
			if err := ipt(append([]string{"-I", jump[0], "1"}, jump[1:]...)...); err != nil {
				return err
			}
		}
		natJump := []string{"-t", "nat", "-I", "POSTROUTING", "1", "-j", sitesChain}
		if ipt("-t", "nat", "-C", "POSTROUTING", "-j", sitesChain) != nil {
			if err := ipt(natJump...); err != nil {
				return err
			}
		}
		n.undo = append(n.undo, func() {
			_ = ipt(append([]string{"-D"}, jump...)...)
			_ = ipt("-t", "nat", "-D", "POSTROUTING", "-j", sitesChain)
			_ = ipt("-F", linksChain)
			_ = ipt("-X", linksChain)
			_ = ipt("-t", "nat", "-F", sitesChain)
			_ = ipt("-t", "nat", "-X", sitesChain)
		})
		n.done[key] = true
	}
	if err := ipt("-F", linksChain); err != nil {
		return err
	}
	for _, r := range p.Rules(v6) {
		args := []string{"-A", linksChain}
		if r.Src.IsValid() {
			args = append(args, "-s", r.Src.String())
		}
		if r.Dst.IsValid() {
			args = append(args, "-d", r.Dst.String())
		}
		if r.Established {
			args = append(args, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED")
		}
		verdict := "DROP"
		if r.Accept {
			verdict = "ACCEPT"
		}
		if err := ipt(append(args, "-j", verdict)...); err != nil {
			return err
		}
	}
	if err := ipt("-t", "nat", "-F", sitesChain); err != nil {
		return err
	}
	for _, s := range p.Sites {
		if s.Addr().Is6() != v6 {
			continue
		}
		if err := ipt("-t", "nat", "-A", sitesChain, "-s", s.String(), "!", "-o", n.iface, "-j", "MASQUERADE"); err != nil {
			return err
		}
	}
	return nil
}

// nftApply пересобирает таблицу inet veil: NAT, пересылка и правила связывания.
func (n *NetSetup) nftApply() error {
	const table = "veil"
	ifname := n.iface
	var post, links strings.Builder
	fmt.Fprintf(&post, "\t\tip saddr %s oifname != \"%s\" masquerade\n", n.p4, ifname)
	fmt.Fprintf(&post, "\t\tip6 saddr %s oifname != \"%s\" masquerade\n", n.p6, ifname)
	for _, s := range n.policy.Sites {
		fmt.Fprintf(&post, "\t\t%s saddr %s oifname != \"%s\" masquerade\n", nftFamily(s), s, ifname)
	}
	for _, v6 := range []bool{false, true} {
		for _, r := range n.policy.Rules(v6) {
			var m []string
			if r.Src.IsValid() {
				m = append(m, nftFamily(r.Src)+" saddr "+r.Src.String())
			}
			if r.Dst.IsValid() {
				m = append(m, nftFamily(r.Dst)+" daddr "+r.Dst.String())
			}
			if r.Established {
				m = append(m, "ct state established,related")
			}
			if len(m) == 0 && !v6 {
				continue // общее правило «всё остальное» — одно, в конце
			}
			verdict := "drop"
			if r.Accept {
				verdict = "accept"
			}
			fmt.Fprintf(&links, "\t\t%s %s\n", strings.Join(m, " "), verdict)
		}
	}
	ruleset := fmt.Sprintf(`table inet %[1]s {
	chain post {
		type nat hook postrouting priority 100;
%[2]s	}
	chain links {
%[3]s	}
	chain fwd {
		type filter hook forward priority -10;
		iifname "%[4]s" oifname "%[4]s" jump links
		iifname "%[4]s" accept
		oifname "%[4]s" ct state established,related accept
	}
}
`, table, post.String(), links.String(), ifname)
	_ = run("nft", "delete", "table", "inet", table)
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft: %v: %s", err, out)
	}
	if !n.done["nft"] {
		n.undo = append(n.undo, func() { _ = run("nft", "delete", "table", "inet", table) })
		n.done["nft"] = true
	}
	return nil
}

func nftFamily(p netip.Prefix) string {
	if p.Addr().Is6() {
		return "ip6"
	}
	return "ip"
}
