package server

import (
	"net"
	"net/netip"
	"testing"
)

type fakeConn struct {
	net.Conn
	local, remote net.Addr
}

func (f fakeConn) LocalAddr() net.Addr  { return f.local }
func (f fakeConn) RemoteAddr() net.Addr { return f.remote }

func tcp(s string) net.Addr { return net.TCPAddrFromAddrPort(netip.MustParseAddrPort(s)) }

func TestFrontBypassDecidesBySource(t *testing.T) {
	s := &Server{tunnelNets: []netip.Prefix{netip.MustParsePrefix("10.66.0.0/24"), netip.MustParsePrefix("fd00:1::/64")}}
	cases := []struct {
		name          string
		local, remote string
		want          bool
	}{
		{"client inside the tunnel to the tunnel address", "10.66.0.1:443", "10.66.0.7:50000", true},
		{"client inside the tunnel to the public address", "203.0.113.5:443", "10.66.0.7:50000", true},
		{"client inside the tunnel over IPv6", "[fd00:1::1]:443", "[fd00:1::7]:50000", true},
		{"local process", "127.0.0.1:443", "127.0.0.1:50000", true},
		{"stranger from the internet", "203.0.113.5:443", "198.51.100.9:50000", false},
		{"stranger sending to the tunnel address", "10.66.0.1:443", "198.51.100.9:50000", false},
		{"stranger spoofing nothing but IPv4-mapped", "203.0.113.5:443", "[::ffff:198.51.100.9]:50000", false},
	}
	for _, c := range cases {
		got := s.FrontBypass(fakeConn{local: tcp(c.local), remote: tcp(c.remote)})
		if got != c.want {
			t.Errorf("%s: bypass = %v, want %v", c.name, got, c.want)
		}
	}
}
