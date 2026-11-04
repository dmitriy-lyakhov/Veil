package deploy

import (
	"strings"
	"testing"
)

func TestValidateIPsecHost(t *testing.T) {
	for _, ok := range []string{"203.0.113.5", "vpn.example.com", "2001:db8::1"} {
		if err := ValidateIPsecHost(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a b", "1.2.3.4; rm -rf /", "$(id)", "x\"y", "-evil"} {
		if err := ValidateIPsecHost(bad); err == nil {
			t.Errorf("%q: ожидалась ошибка", bad)
		}
	}
}

func TestIPsecScript(t *testing.T) {
	s := ipsecScript("203.0.113.5")
	for _, want := range []string{
		"HOST=203.0.113.5", "addrs = 10.10.0.0/24", "--san \"$HOST\"", "--flag serverAuth", "--flag ikeIntermediate",
		"eap-mschapv2", "ufw allow 500/udp", "ufw allow 4500/udp", "firewall-cmd --permanent --add-service=ipsec",
		"net.ipv4.ip_forward=1", "MASQUERADE", "swanctl --load-all", "systemctl enable --now veil-ipsec-nat.service",
		"/etc/strongswan/swanctl",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("в скрипте нет %q", want)
		}
	}
	if strings.Contains(s, "@HOST@") || strings.Contains(s, "@POOL@") {
		t.Error("остались неподставленные метки")
	}
}

func TestIPsecUserHelpers(t *testing.T) {
	if got := ipsecUserName("  Клиент 1 "); got != "1" {
		t.Errorf("%q", got)
	}
	if got := ipsecUserName("Клиент"); got != "client" {
		t.Errorf("%q", got)
	}
	if got := ipsecUserName("a b'c;d"); got != "abcd" {
		t.Errorf("%q", got)
	}
	if got := uniqueName("bob", []string{"bob", "bob-2"}); got != "bob-3" {
		t.Errorf("%q", got)
	}
	b := ipsecUserBlock("bob", "pw")
	if !strings.HasPrefix(b, "# veil-user: bob\n") || !strings.Contains(b, `secret = "pw"`) || !strings.HasSuffix(b, "}\n") {
		t.Errorf("%q", b)
	}
	users := parseIPsecUsers("# veil-user: bob\n# veil-user: eve\nнезнакомая строка")
	if len(users) != 2 || users[0] != "bob" || users[1] != "eve" {
		t.Errorf("%v", users)
	}
	p, err := ipsecPassword()
	if err != nil || len(p) != 16 {
		t.Errorf("%q %v", p, err)
	}
}

func TestIPsecCertScript(t *testing.T) {
	s := strings.NewReplacer("@NAME@", "bob", "@PASS@", "pw").Replace(ipsecCertScript)
	if !strings.Contains(s, "--flag clientAuth") || !strings.Contains(s, "-passout pass:pw") || strings.Contains(s, "@") {
		t.Error(s)
	}
}

func TestDefaultPoolsDoNotOverlap(t *testing.T) {
	pools := map[string]string{
		"Veil": "10.66.0.0/24", "WireGuard": WGDefaultSubnet, "AmneziaWG": AWGDefaultSubnet,
		"OpenVPN": OVDefaultSubnet, "IPsec": IPsecDefaultPool,
	}
	seen := map[string]string{}
	for name, cidr := range pools {
		if other, dup := seen[cidr]; dup {
			t.Errorf("%s and %s share the default subnet %s: both on one server would clash", name, other, cidr)
		}
		seen[cidr] = name
	}
}
