package deploy

import (
	"strings"
	"testing"
)

func TestValidateWGInstall(t *testing.T) {
	o, srv, err := ValidateWGInstall(WGInstall{Port: 51820})
	if err != nil || o.Subnet != "10.8.1.0/24" || srv != "10.8.1.1/24" {
		t.Fatalf("%+v %s %v", o, srv, err)
	}
	if _, srv, err := ValidateWGInstall(WGInstall{Port: 1, Subnet: "10.9.0.77/24"}); err != nil || srv != "10.9.0.1/24" {
		t.Fatalf("%s %v", srv, err)
	}
	for _, bad := range []WGInstall{{Port: 0}, {Port: 70000}, {Port: 1, Subnet: "x"}, {Port: 1, Subnet: "10.0.0.0/8"}, {Port: 1, Subnet: "10.0.0.0/31"}, {Port: 1, Subnet: "fd00::/64"}, {Port: 1, Subnet: "10.8.1.0/24; rm -rf /"}} {
		if _, _, err := ValidateWGInstall(bad); err == nil {
			t.Errorf("%+v: ожидалась ошибка", bad)
		}
	}
}

func TestWGScript(t *testing.T) {
	o, srv, _ := ValidateWGInstall(WGInstall{Port: 49126})
	s := wgScript(o, srv)
	for _, want := range []string{"ListenPort = 49126", "Address = 10.8.1.1/24", "-s 10.8.1.0/24", "wg-quick@$IFACE", "IFACE=veilwg"} {
		if !strings.Contains(s, want) {
			t.Errorf("в скрипте нет %q", want)
		}
	}
	if strings.Contains(s, "@") && strings.Contains(s, "@PORT@") {
		t.Error("остались неподставленные метки")
	}
}

func TestFreeClientIP(t *testing.T) {
	conf := "[Interface]\nAddress = 10.8.1.1/24\nListenPort = 1\n\n[Peer]\nAllowedIPs = 10.8.1.2/32\n\n[Peer]\nAllowedIPs = 10.8.1.4/32\n"
	ip, _, err := freeClientIP(conf)
	if err != nil || ip.String() != "10.8.1.3" {
		t.Fatalf("%v %v", ip, err)
	}
	full := "[Interface]\nAddress = 10.8.1.1/30\n[Peer]\nAllowedIPs = 10.8.1.2/32\n"
	if _, _, err := freeClientIP(full); err == nil {
		t.Fatal("в /30 после двух адресов свободных не остаётся")
	}
	if _, _, err := freeClientIP("[Peer]\nAllowedIPs = 10.8.1.2/32\n"); err == nil {
		t.Fatal("нет адреса интерфейса")
	}
}

func TestParseKV(t *testing.T) {
	m := parseKV("шум\nPORT=51820\nPUBKEY=abc=\nlower=x\n")
	if m["PORT"] != "51820" || m["PUBKEY"] != "abc=" || len(m) != 2 {
		t.Fatalf("%v", m)
	}
}
