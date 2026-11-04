package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func sampleProfile() *Profile {
	k := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return &Profile{
		Name: "office", IPv4: "203.0.113.10", IPv6: "2001:db8::10", Port: 443, Path: "/api/v2/abc",
		CertPin: k, ServerPublicKey: k, PrivateKey: k, AuthKey: k,
		Address4: "10.66.0.2/32", Address6: "fd00::2/128", DNS: []string{"1.1.1.1"}, MTU: 1380,
		AdminUser: "admin", AdminPassword: "secret", SSH: &SSHInfo{Port: 22, User: "root"},
	}
}

func TestLinkRoundTrip(t *testing.T) {
	link, err := EncodeLink(sampleProfile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, LinkScheme) || len(link) > 1200 {
		t.Fatalf("link %d chars: %s", len(link), link)
	}
	p, err := DecodeLink("  " + link + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if p.IPv6 != "2001:db8::10" || p.Path != "/api/v2/abc" || p.Name != "office" {
		t.Fatalf("decoded %+v", p)
	}
	if p.SSH != nil || p.AdminPassword != "" || p.AdminUser != "" {
		t.Fatal("secrets leaked into shared link")
	}
	t.Logf("link length %d", len(link))
}

func TestDecodeRejectsGarbage(t *testing.T) {
	bad := sampleProfile()
	bad.PrivateKey = "short"
	link, _ := EncodeLink(bad)
	for _, in := range []string{"", "hello", "veil://!!!", "veil://AAAA", `{"name":"x"}`, link} {
		if _, err := DecodeLink(in); err == nil {
			t.Fatalf("accepted %q", in)
		}
	}
}
