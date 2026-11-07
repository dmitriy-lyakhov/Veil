package wgconf

import (
	"strings"
	"testing"
)

const sample = `# комментарий
[Interface]
PrivateKey = 4Ke6tZ3uQx0z2hX8qQmHcX6o3m1p7w9o3s5l7b8c9dE=
Address = 10.8.1.2/32, fd00:8:1::2/128
DNS = 1.1.1.1, 2606:4700:4700::1111
MTU = 1280

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
PresharedKey = cGVzaGFyZWRrZXlwZXNoYXJlZGtleXBlc2hhcmVkaw==
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 203.0.113.7:51820
PersistentKeepalive = 25
`

func TestParseRoundTrip(t *testing.T) {
	c, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Address) != 2 || c.MTU != 1280 || len(c.DNS) != 2 || c.Keepalive != 25 || c.Endpoint != "203.0.113.7:51820" {
		t.Fatalf("%+v", c)
	}
	c2, err := Parse(c.String())
	if err != nil {
		t.Fatal(err)
	}
	if c2.String() != c.String() {
		t.Fatalf("круг не замкнулся:\n%s\n---\n%s", c.String(), c2.String())
	}
	h, p, err := c.EndpointHostPort()
	if err != nil || h != "203.0.113.7" || p != 51820 {
		t.Fatalf("%s %d %v", h, p, err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"нет peer":   "[Interface]\nPrivateKey = a\nAddress = 10.0.0.2/32\n",
		"два peer":   strings.ReplaceAll(sample, "[Peer]", "[Peer]\nPublicKey = z\n[Peer]"),
		"amnezia":    strings.Replace(sample, "MTU = 1280", "Jc = 5", 1),
		"плохой ip":  strings.Replace(sample, "10.8.1.2/32", "nope", 1),
		"без порта":  strings.Replace(sample, "203.0.113.7:51820", "203.0.113.7", 1),
		"без ключей": "[Interface]\nAddress = 10.0.0.2/32\n[Peer]\nEndpoint = 1.2.3.4:1\n",
	}
	for name, text := range cases {
		if _, err := Parse(text); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestDefaultRoutes(t *testing.T) {
	c, err := Parse(strings.Replace(sample, "AllowedIPs = 0.0.0.0/0, ::/0\n", "", 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Routes()) != 2 {
		t.Fatalf("%v", c.Routes())
	}
}
