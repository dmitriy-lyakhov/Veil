package server

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"strings"

	"veil/internal/config"
)

type NetSetup struct {
	undo []func()
	log  *slog.Logger
}

func SetupNetwork(cfg *config.Server, iface string, log *slog.Logger) (*NetSetup, error) {
	n := &NetSetup{log: log}
	p4, err := netip.ParsePrefix(cfg.Address4)
	if err != nil {
		return nil, err
	}
	p6, err := netip.ParsePrefix(cfg.Address6)
	if err != nil {
		return nil, err
	}
	mtu := cfg.MTU
	if mtu == 0 {
		mtu = config.DefaultMTU
	}
	steps := [][]string{
		{"ip", "addr", "replace", p4.String(), "dev", iface},
		{"ip", "-6", "addr", "replace", p6.String(), "dev", iface},
		{"ip", "link", "set", "dev", iface, "mtu", fmt.Sprint(mtu), "up"},
	}
	for _, s := range steps {
		if err := run(s...); err != nil {
			return nil, err
		}
	}
	for _, f := range []string{"/proc/sys/net/ipv4/ip_forward", "/proc/sys/net/ipv6/conf/all/forwarding"} {
		if err := os.WriteFile(f, []byte("1"), 0o644); err != nil {
			return nil, fmt.Errorf("enable forwarding: %w", err)
		}
	}
	raiseUDPBuffers(log)
	if err := n.firewall(iface, p4.Masked(), p6.Masked()); err != nil {
		n.Teardown()
		return nil, err
	}
	return n, nil
}

const udpBufferBytes = 7500000

func raiseUDPBuffers(log *slog.Logger) {
	for _, f := range []string{"/proc/sys/net/core/rmem_max", "/proc/sys/net/core/wmem_max"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var cur int
		fmt.Sscan(strings.TrimSpace(string(b)), &cur)
		if cur >= udpBufferBytes {
			continue
		}
		if err := os.WriteFile(f, []byte(fmt.Sprint(udpBufferBytes)), 0o644); err != nil {
			log.Warn("raise udp buffer", "file", f, "err", err)
		}
	}
}

func (n *NetSetup) firewall(iface string, p4, p6 netip.Prefix) error {
	if _, err := exec.LookPath("iptables"); err == nil {
		return n.iptables(iface, p4, p6)
	}
	if _, err := exec.LookPath("nft"); err == nil {
		return n.nftables(iface, p4, p6)
	}
	return fmt.Errorf("не найден ни iptables, ни nft")
}

func (n *NetSetup) iptables(iface string, p4, p6 netip.Prefix) error {
	rules := []struct {
		bin   string
		table string
		chain string
		spec  []string
	}{
		{"iptables", "nat", "POSTROUTING", []string{"-s", p4.String(), "!", "-o", iface, "-j", "MASQUERADE"}},
		{"iptables", "filter", "FORWARD", []string{"-i", iface, "-j", "ACCEPT"}},
		{"iptables", "filter", "FORWARD", []string{"-o", iface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}},
		{"ip6tables", "nat", "POSTROUTING", []string{"-s", p6.String(), "!", "-o", iface, "-j", "MASQUERADE"}},
		{"ip6tables", "filter", "FORWARD", []string{"-i", iface, "-j", "ACCEPT"}},
		{"ip6tables", "filter", "FORWARD", []string{"-o", iface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}},
	}
	for _, r := range rules {
		base := []string{r.bin, "-w", "-t", r.table}
		ins := append(append(append([]string{}, base...), "-I", r.chain, "1"), r.spec...)
		if err := run(ins...); err != nil {
			if r.bin == "ip6tables" {
				n.log.Warn("ipv6 firewall rule failed, IPv6 egress may not work", "err", err)
				continue
			}
			return err
		}
		del := append(append(append([]string{}, base...), "-D", r.chain), r.spec...)
		n.undo = append(n.undo, func() { _ = run(del...) })
	}
	return nil
}

func (n *NetSetup) nftables(iface string, p4, p6 netip.Prefix) error {
	const table = "veil"
	ruleset := fmt.Sprintf(`table inet %[1]s {
	chain post {
		type nat hook postrouting priority 100;
		ip saddr %[2]s oifname != "%[4]s" masquerade
		ip6 saddr %[3]s oifname != "%[4]s" masquerade
	}
	chain fwd {
		type filter hook forward priority -10;
		iifname "%[4]s" accept
		oifname "%[4]s" ct state established,related accept
	}
}
`, table, p4, p6, iface)
	_ = run("nft", "delete", "table", "inet", table)
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft: %v: %s", err, out)
	}
	n.undo = append(n.undo, func() { _ = run("nft", "delete", "table", "inet", table) })
	return nil
}

func (n *NetSetup) Teardown() {
	for i := len(n.undo) - 1; i >= 0; i-- {
		n.undo[i]()
	}
	n.undo = nil
}

func run(args ...string) error {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
