package wgconf

import (
	"strings"
	"testing"
)

const awgSample = `[Interface]
PrivateKey = 4Ke6tZ3uQx0z2hX8qQmHcX6o3m1p7w9o3s5l7b8c9dE=
Address = 10.9.1.2/32
DNS = 1.1.1.1
MTU = 1280
Jc = 4
Jmin = 10
Jmax = 50
S1 = 77
S2 = 99
H1 = 1234567891
H2 = 2234567891
H3 = 3234567891
H4 = 4234567891
I1 = <b 0xf6ab3267fa><c><b 0xf6ab><t><r 10>

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 203.0.113.7:39999
PersistentKeepalive = 25
`

func TestParseAWGRoundTrip(t *testing.T) {
	c, err := ParseAWG(awgSample)
	if err != nil {
		t.Fatal(err)
	}
	a := c.AWG
	if a == nil || a.Jc != 4 || a.Jmin != 10 || a.Jmax != 50 || a.S1 != 77 || a.S2 != 99 || a.H4 != "4234567891" ||
		a.I[0] != "<b 0xf6ab3267fa><c><b 0xf6ab><t><r 10>" || a.HasV2() {
		t.Fatalf("%+v", a)
	}
	c2, err := ParseAWG(c.String())
	if err != nil || c2.String() != c.String() {
		t.Fatalf("круг не замкнулся: %v\n%s", err, c.String())
	}
	if !strings.Contains(c.String(), "Jc = 4\nJmin = 10\nJmax = 50\nS1 = 77\nS2 = 99\nH1 = ") {
		t.Fatalf("порядок параметров:\n%s", c.String())
	}
}

func TestPlainParseRejectsAWG(t *testing.T) {
	if _, err := Parse(awgSample); err == nil {
		t.Fatal("обычный разбор должен отвергать параметры AmneziaWG")
	}
	c, err := ParseAWG(sample)
	if err != nil || c.AWG != nil {
		t.Fatalf("конфиг без параметров: AWG должен быть nil: %v %+v", err, c)
	}
}

func TestAWGValidate(t *testing.T) {
	// каждый случай: что удалить из образца и что дописать в [Interface]
	bad := []struct{ name, drop, add string }{
		{"jmin>jmax", "Jmin = 10\n", "Jmin = 60"},
		{"jmax", "Jmax = 50\n", "Jmax = 2000"},
		{"s1+56=s2", "S1 = 77\nS2 = 99\n", "S1 = 40\nS2 = 96"},
		{"h равны", "H1 = 1234567891\nH2 = 2234567891\n", "H1 = 100\nH2 = 100"},
		{"h мал", "H1 = 1234567891\n", "H1 = 3"},
		{"jc", "Jc = 4\n", "Jc = 500"},
		{"не число", "S1 = 77\n", "S1 = abc"},
	}
	for _, b := range bad {
		text := strings.Replace(awgSample, b.drop, "", 1)
		text = strings.Replace(text, "\n[Peer]", b.add+"\n\n[Peer]", 1)
		if _, err := ParseAWG(text); err == nil {
			t.Errorf("%s: ожидалась ошибка", b.name)
		}
	}
	v2 := strings.Replace(awgSample, "H1 = 1234567891\n", "", 1)
	v2 = strings.Replace(v2, "\n[Peer]", "S3 = 12\nH1 = 100-200\n\n[Peer]", 1)
	if c, err := ParseAWG(v2); err != nil || !c.AWG.HasV2() {
		t.Fatalf("AWG 2.0: %v", err)
	}
}

func TestParseAWGInterface(t *testing.T) {
	a, port := ParseAWGInterface("[Interface]\nPrivateKey = x\nListenPort = 4444\nJc = 5\nS1 = 20\nH3 = 777\nPostUp = a=b\n[Peer]\nJc = 9\n")
	if port != 4444 || a.Jc != 5 || a.S1 != 20 || a.H3 != "777" {
		t.Fatalf("%d %+v", port, a)
	}
}
