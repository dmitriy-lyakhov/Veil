package server

import (
	"net/netip"
	"strings"
	"testing"

	"veil/internal/config"
)

func TestNormalizeSubnets(t *testing.T) {
	tunnel := []netip.Prefix{netip.MustParsePrefix("10.66.0.0/24"), netip.MustParsePrefix("fd00:1::/64")}
	got, err := normalizeSubnets([]string{" 192.168.1.7/24 ", "192.168.1.0/24", "", "10.20.0.1", "2001:db8:5::/48"}, tunnel)
	if err != nil {
		t.Fatal(err)
	}
	want := "192.168.1.0/24 10.20.0.1/32 2001:db8:5::/48"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	for _, bad := range []string{"0.0.0.0/0", "10.0.0.0/7", "10.66.0.128/25", "127.0.0.0/8", "fe80::/64", "мусор", "fd00:1::/80"} {
		if _, err := normalizeSubnets([]string{bad}, tunnel); err == nil {
			t.Errorf("%s: ожидалась ошибка", bad)
		}
	}
}

// verdict — решение политики для пакета src→dst (первое совпавшее правило; установленные соединения не учитываются).
func verdict(p LinkPolicy, src, dst string) bool {
	s, d := netip.MustParseAddr(src), netip.MustParseAddr(dst)
	for _, r := range p.Rules(s.Is6()) {
		if r.Established {
			continue
		}
		if (!r.Src.IsValid() || r.Src.Contains(s)) && (!r.Dst.IsValid() || r.Dst.Contains(d)) {
			return r.Accept
		}
	}
	return false
}

func TestLinkRules(t *testing.T) {
	base := LinkPolicy{
		Tunnel: []netip.Prefix{netip.MustParsePrefix("10.66.0.0/24"), netip.MustParsePrefix("fd00:1::/64")},
		Sites:  []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24"), netip.MustParsePrefix("192.168.2.0/24"), netip.MustParsePrefix("2001:db8:5::/48")},
	}
	cases := []struct {
		links            config.Links
		c2c, c2s, s2s, x bool // клиент↔клиент, клиент→сайт, сайт↔сайт, прочее
	}{
		{config.Links{ClientToClient: true, ClientToSite: true, SiteToSite: true}, true, true, true, true},
		{config.Links{}, false, false, false, false},
		{config.Links{SiteToSite: true}, false, false, true, false},
		{config.Links{ClientToSite: true}, false, true, false, false},
		{config.Links{ClientToClient: true}, true, false, false, false},
	}
	for _, c := range cases {
		p := base
		p.Links = c.links
		check := func(name string, got, want bool) {
			if got != want {
				t.Errorf("%+v: %s = %v, want %v", c.links, name, got, want)
			}
		}
		check("c2c", verdict(p, "10.66.0.2", "10.66.0.3"), c.c2c)
		check("c2c v6", verdict(p, "fd00:1::2", "fd00:1::3"), c.c2c)
		check("c2s", verdict(p, "10.66.0.2", "192.168.1.10"), c.c2s)
		check("s2c", verdict(p, "192.168.2.10", "10.66.0.2"), c.c2s)
		check("c2s v6", verdict(p, "fd00:1::2", "2001:db8:5::1"), c.c2s)
		check("s2s", verdict(p, "192.168.1.10", "192.168.2.10"), c.s2s)
		check("s2s back", verdict(p, "192.168.2.10", "192.168.1.10"), c.s2s)
		check("other", verdict(p, "10.66.0.2", "172.16.0.1"), c.x)
	}
}
