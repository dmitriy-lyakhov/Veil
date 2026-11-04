package deploy

import (
	"strings"
	"testing"

	"veil/internal/ovpnconf"
)

func TestValidateOVInstall(t *testing.T) {
	o, err := ValidateOVInstall(OVInstall{Port: 1194})
	if err != nil || o.Proto != "udp" || o.Subnet != "10.9.0.0/24" {
		t.Fatalf("%+v %v", o, err)
	}
	if o, err := ValidateOVInstall(OVInstall{Port: 443, Proto: "tcp", Subnet: "10.9.5.77/24"}); err != nil || o.Subnet != "10.9.5.0/24" {
		t.Fatalf("%+v %v", o, err)
	}
	for _, bad := range []OVInstall{{Port: 0}, {Port: 70000}, {Port: 1, Proto: "sctp"}, {Port: 1, Proto: "udp; reboot"},
		{Port: 1, Subnet: "x"}, {Port: 1, Subnet: "10.0.0.0/8"}, {Port: 1, Subnet: "10.9.0.0/24; id"}} {
		if _, err := ValidateOVInstall(bad); err == nil {
			t.Errorf("%+v: ожидалась ошибка", bad)
		}
	}
}

func TestOVScript(t *testing.T) {
	o, _ := ValidateOVInstall(OVInstall{Port: 443, Proto: "tcp"})
	s := ovScript(o)
	for _, want := range []string{"port 443", "proto tcp", "server 10.9.0.0 255.255.255.0", "-s 10.9.0.0/24 -o $NIC",
		"openvpn-server@veil", "tls-crypt", "ufw allow", "firewall-cmd", "net.ipv4.ip_forward=1", "build-server-full server"} {
		if !strings.Contains(s, want) {
			t.Errorf("в скрипте нет %q", want)
		}
	}
	if strings.Contains(s, "@") && strings.Contains(strings.ReplaceAll(s, "openvpn-server@veil", ""), "@PORT@") {
		t.Error("остались неподставленные метки")
	}
}

func TestOVClientConfigRoundTrip(t *testing.T) {
	pem := func(n string) string { return "-----BEGIN " + n + "-----\nAAA\n-----END " + n + "-----" }
	text := OVClientConfig("203.0.113.5", 1194, "udp", pem("CERTIFICATE"), pem("CERTIFICATE"), pem("PRIVATE KEY"), "#\n# key\n"+pem("OpenVPN Static key V1"))
	c, err := ovpnconf.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	h, p, pr, _ := c.Remote()
	if h != "203.0.113.5" || p != 1194 || pr != "udp" {
		t.Fatal(h, p, pr)
	}
	for _, want := range []string{"<tls-crypt>", "<key>", "explicit-exit-notify 1", "remote-cert-tls server"} {
		if !strings.Contains(c.Text, want) {
			t.Errorf("нет %q", want)
		}
	}
}

func TestExtractPEMAndSections(t *testing.T) {
	in := "Certificate:\n  Data: x\n-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----\n"
	if got := extractPEM(in); got != "-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----" {
		t.Fatalf("%q", got)
	}
	if extractPEM("нет") != "" {
		t.Error("ожидалась пустая строка")
	}
	m := splitSections("PORT=1\n@@CA\nca\n@@CERT\ncert\n@@END\n")
	if strings.TrimSpace(m["CA"]) != "ca" || strings.TrimSpace(m["CERT"]) != "cert" {
		t.Fatalf("%v", m)
	}
}

func TestOVClientName(t *testing.T) {
	for _, l := range []string{"Клиент 1", "my laptop", "a;rm -rf /", ""} {
		n := ovClientName(l)
		if !ovNameOK.MatchString(n) {
			t.Errorf("%q → %q недопустимо", l, n)
		}
	}
}
