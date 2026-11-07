//go:build windows

package netsys

import (
	"net/netip"
	"testing"
	"unsafe"
)

// Раскладка MIB_IPFORWARD_ROW2 должна совпадать с заголовком Windows (iphlpapi).
func TestForwardRowLayout(t *testing.T) {
	var r mibIpforwardRow2
	if got := unsafe.Sizeof(r); got != 104 {
		t.Fatalf("sizeof = %d, want 104", got)
	}
	offs := map[string]uintptr{
		"InterfaceIndex": unsafe.Offsetof(r.InterfaceIndex), "DestPrefix": unsafe.Offsetof(r.DestPrefix),
		"DestPrefixLen": unsafe.Offsetof(r.DestPrefixLen), "NextHop": unsafe.Offsetof(r.NextHop),
		"SitePrefixLength": unsafe.Offsetof(r.SitePrefixLength), "ValidLifetime": unsafe.Offsetof(r.ValidLifetime),
		"Metric": unsafe.Offsetof(r.Metric), "Protocol": unsafe.Offsetof(r.Protocol), "Loopback": unsafe.Offsetof(r.Loopback),
		"Age": unsafe.Offsetof(r.Age), "Origin": unsafe.Offsetof(r.Origin),
	}
	want := map[string]uintptr{"InterfaceIndex": 8, "DestPrefix": 12, "DestPrefixLen": 40, "NextHop": 44,
		"SitePrefixLength": 72, "ValidLifetime": 76, "Metric": 84, "Protocol": 88, "Loopback": 92, "Age": 96, "Origin": 100}
	for k, w := range want {
		if offs[k] != w {
			t.Errorf("%s offset = %d, want %d", k, offs[k], w)
		}
	}
}

func TestSockaddr(t *testing.T) {
	var sa sockaddrInet
	putSockaddr(&sa, netip.MustParseAddr("203.0.113.9"))
	if sa[0] != afInet || sa[4] != 203 || sa[7] != 9 {
		t.Fatalf("v4 sockaddr: %v", sa)
	}
	putSockaddr(&sa, netip.MustParseAddr("2001:db8::1"))
	if sa[0] != afInet6 || sa[8] != 0x20 || sa[9] != 0x01 || sa[23] != 1 {
		t.Fatalf("v6 sockaddr: %v", sa)
	}
}
