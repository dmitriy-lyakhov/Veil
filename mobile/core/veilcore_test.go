package veilcore

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/vpnconf/wgkey"
)

func wgConf(t *testing.T, extra string, allowed string) string {
	t.Helper()
	priv, _, err := wgkey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	_, pub, _ := wgkey.Generate()
	return fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = 10.8.0.2/32, fd00::2/128\nDNS = 10.8.0.1\nMTU = 1280\n%s\n[Peer]\nPublicKey = %s\nEndpoint = 203.0.113.5:51820\nAllowedIPs = %s\nPersistentKeepalive = 25\n",
		priv, extra, pub, allowed)
}

func importOK(t *testing.T, text string) imported {
	t.Helper()
	s, err := Import(text, "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	var im imported
	if err := json.Unmarshal([]byte(s), &im); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(im.Link, "veil://") && !strings.HasPrefix(im.Link, "{") {
		t.Fatalf("link %q", im.Link)
	}
	return im
}

func TestImportWireGuard(t *testing.T) {
	im := importOK(t, wgConf(t, "", "0.0.0.0/0, ::/0"))
	if im.Protocol != "wireguard" || !strings.Contains(im.Name, "203.0.113.5") {
		t.Fatalf("%+v", im)
	}
	s, err := Parse(im.Link)
	if err != nil {
		t.Fatal(err)
	}
	var p parsed
	_ = json.Unmarshal([]byte(s), &p)
	if len(p.Addresses) != 2 || p.MTU != 1280 || len(p.DNS) != 1 || len(p.Routes) != 0 || p.Host != "203.0.113.5" {
		t.Fatalf("%+v", p)
	}
	if strings.Contains(s, "PrivateKey") || strings.Contains(s, im.Link) {
		t.Fatal("Parse leaks secrets")
	}
}

func TestImportWireGuardPartialRoutes(t *testing.T) {
	im := importOK(t, wgConf(t, "", "10.0.0.0/8, 192.168.5.0/24"))
	s, _ := Parse(im.Link)
	var p parsed
	_ = json.Unmarshal([]byte(s), &p)
	if len(p.Routes) != 2 {
		t.Fatalf("routes %v", p.Routes)
	}
}

func TestImportAmneziaWG(t *testing.T) {
	im := importOK(t, wgConf(t, "Jc = 4\nJmin = 40\nJmax = 70\nS1 = 15\nS2 = 23\nH1 = 1234567\nH2 = 2345678\nH3 = 3456789\nH4 = 4567890", "0.0.0.0/0"))
	if im.Protocol != "amneziawg" {
		t.Fatalf("%+v", im)
	}
}

func TestImportVeilLink(t *testing.T) {
	cfg, err := provision.NewServer("ru")
	if err != nil {
		t.Fatal(err)
	}
	prof, err := provision.AddPeer(cfg, "phone")
	if err != nil {
		t.Fatal(err)
	}
	prof.IPv4 = "127.0.0.1"
	link, err := config.EncodeLink(prof)
	if err != nil {
		t.Fatal(err)
	}
	im := importOK(t, "  "+link+"\n")
	if im.Protocol != "veil" {
		t.Fatalf("%+v", im)
	}
	s, err := Parse(im.Link)
	if err != nil {
		t.Fatal(err)
	}
	var p parsed
	_ = json.Unmarshal([]byte(s), &p)
	if p.Protocol != "veil" || len(p.Addresses) != 2 || p.Host != "127.0.0.1" {
		t.Fatalf("%+v", p)
	}
	re, err := Rename(im.Link, "Мой сервер")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := Parse(re); !strings.Contains(s, "Мой сервер") {
		t.Fatalf("rename: %s", s)
	}
	if got, _ := Status(), 0; !strings.Contains(got, `"stopped"`) {
		t.Fatalf("status %s", got)
	}
}

func TestImportRejects(t *testing.T) {
	cases := map[string]string{
		"client\ndev tun\nproto udp\nremote vpn.example.com 1194\n": ErrOpenVPN,
		"veil-relay://abcd": ErrRelay,
		"hello world":       ErrFormat,
		"   ":               ErrEmpty,
	}
	for in, code := range cases {
		if _, err := Import(in, ""); err == nil || err.Error() != code {
			t.Errorf("Import(%q) = %v, want %s", in, err, code)
		}
	}
}

func TestComplement(t *testing.T) {
	ex := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("203.0.113.7/32"), netip.MustParsePrefix("2001:db8::/32")}
	got := Complement(ex)
	// ни один адрес исключений не покрыт, а соседние адреса покрыты
	covered := func(a string) bool {
		ip := netip.MustParseAddr(a)
		for _, p := range got {
			if p.Contains(ip) {
				return true
			}
		}
		return false
	}
	for _, a := range []string{"10.0.0.1", "10.255.255.255", "203.0.113.7", "2001:db8::1"} {
		if covered(a) {
			t.Errorf("%s не должен быть в маршрутах", a)
		}
	}
	for _, a := range []string{"9.255.255.255", "11.0.0.0", "203.0.113.6", "203.0.113.8", "8.8.8.8", "255.255.255.255", "0.0.0.0", "2001:db9::1", "2606:4700::1"} {
		if !covered(a) {
			t.Errorf("%s должен быть в маршрутах", a)
		}
	}
	// непересекаются
	for i, a := range got {
		for _, b := range got[i+1:] {
			if a.Overlaps(b) {
				t.Fatalf("пересечение %s и %s", a, b)
			}
		}
	}
}

func TestPlanViaAndDirect(t *testing.T) {
	s, err := Plan(`{"mode":"via","text":"8.8.8.0/24\n1.1.1.1\n# комментарий\n2001:db8::/32"}`)
	if err != nil {
		t.Fatal(err)
	}
	var o splitOut
	_ = json.Unmarshal([]byte(s), &o)
	if len(o.Routes) != 3 || len(o.Exclude) != 0 {
		t.Fatalf("%+v", o)
	}
	s, err = Plan(`{"mode":"direct","text":"10.0.0.0/8\n192.168.0.0/16"}`)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(s), &o)
	if len(o.Exclude) != 2 || len(o.Routes) < 10 {
		t.Fatalf("%+v", o)
	}
	if _, err := Plan(`{"mode":"via","text":"# пусто"}`); err == nil || err.Error() != ErrEmpty {
		t.Fatalf("пустой список: %v", err)
	}
}
