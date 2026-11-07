package netsys

import "testing"

func TestParseGateways(t *testing.T) {
	w := parseGateways("windows", "Ethernet|192.168.1.1\r\nWi-Fi 2|192.168.0.1\n")
	if w["Ethernet"] != "192.168.1.1" || w["Wi-Fi 2"] != "192.168.0.1" || len(w) != 2 {
		t.Fatal(w)
	}
	l := parseGateways("linux", "default via 10.0.0.1 dev eth0 proto dhcp metric 100\ndefault via 10.1.0.1 dev wlan0 metric 600\n")
	if l["eth0"] != "10.0.0.1" || l["wlan0"] != "10.1.0.1" {
		t.Fatal(l)
	}
	d := parseGateways("darwin", "Destination Gateway Flags Netif Expire\ndefault 192.168.1.1 UGScg en0\ndefault link#17 UCSIg bridge100 !\n")
	if d["en0"] != "192.168.1.1" || len(d) != 1 {
		t.Fatal(d)
	}
}
