package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/client"
	"veil/internal/client/dialer"
	"veil/internal/server"
)

func serveH3(t *testing.T, e *env, host string) {
	t.Helper()
	h3, err := e.srv.HTTP3Server()
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp", net.JoinHostPort(host, fmt.Sprint(e.port)))
	if err != nil {
		t.Fatal(err)
	}
	go h3.Serve(pc)
	t.Cleanup(func() { h3.Close(); pc.Close() })
}

func fetchBig(t *testing.T, tnet *netstack.Net, addr netip.Addr) {
	t.Helper()
	hc := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(addr, 80))
	}}}
	start := time.Now()
	resp, err := hc.Get("http://inside/big")
	if err != nil {
		t.Fatalf("GET /big via %s: %v", addr, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, bigBody) {
		t.Fatalf("/big corrupted: got %d bytes", len(b))
	}
	t.Logf("512 KiB via %s in %v", addr, time.Since(start).Round(time.Millisecond))
}

func TestH3PreferredOverIPv6(t *testing.T) {
	e := startServerOnBoth(t)
	serveH3(t, e, "::1")
	serveH3(t, e, "127.0.0.1")
	tn, tnet := startClient(t, e.profile)
	st := tn.Status()
	if st.Transport != string(dialer.KindH3) || st.Family != "IPv6" {
		t.Fatalf("expected HTTP/3 over IPv6, got %+v", st)
	}
	for _, a := range []string{e.srvCfg.Address4, e.srvCfg.Address6} {
		fetchBig(t, tnet, netip.MustParsePrefix(a).Addr())
	}
	if got := fetchInside(t, tnet, netip.MustParsePrefix(e.srvCfg.Address6).Addr()); !strings.HasPrefix(got, insideBody) {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestH3OverIPv4WhenNoIPv6(t *testing.T) {
	e := startServer(t, "tcp4")
	serveH3(t, e, "127.0.0.1")
	e.profile.IPv6 = ""
	tn, tnet := startClient(t, e.profile)
	if st := tn.Status(); st.Transport != string(dialer.KindH3) || st.Family != "IPv4" {
		t.Fatalf("expected HTTP/3 over IPv4, got %+v", st)
	}
	fetchBig(t, tnet, netip.MustParsePrefix(e.srvCfg.Address4).Addr())
}

func TestFallsBackToH2WhenUDPBlocked(t *testing.T) {
	e := startServer(t, "tcp4")
	e.profile.IPv6 = ""
	start := time.Now()
	tn, tnet := startClient(t, e.profile)
	if st := tn.Status(); st.Transport != string(dialer.KindH2) {
		t.Fatalf("expected HTTP/2 fallback, got %+v", st)
	}
	t.Logf("HTTP/2 fallback in %v", time.Since(start).Round(time.Millisecond))
	fetchBig(t, tnet, netip.MustParsePrefix(e.srvCfg.Address4).Addr())
}

func TestH2OnlyProfileSkipsQUIC(t *testing.T) {
	e := startServer(t, "tcp4")
	serveH3(t, e, "127.0.0.1")
	e.profile.IPv6 = ""
	e.profile.Transports = []string{"h2"}
	tn, _ := startClient(t, e.profile)
	if st := tn.Status(); st.Transport != string(dialer.KindH2) {
		t.Fatalf("expected HTTP/2 only, got %+v", st)
	}
}

func TestQUICProbeSeesDecoy(t *testing.T) {
	e := startServer(t, "tcp4")
	serveH3(t, e, "127.0.0.1")
	tr := &http3.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	defer tr.Close()
	hc := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	base := fmt.Sprintf("https://127.0.0.1:%d", e.port)
	resp, err := hc.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.ProtoMajor != 3 || resp.Header.Get("Server") != "nginx" {
		t.Fatalf("h3 decoy: %d %s %v", resp.StatusCode, resp.Proto, resp.Header)
	}
	resp, err = hc.Post(base+e.profile.Path, "application/octet-stream", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("unauthenticated h3 tunnel request got %d", resp.StatusCode)
	}

	tcp := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: true}}
	resp, err = tcp.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if want := fmt.Sprintf(`h3=":%d"; ma=86400`, e.port); resp.Header.Get("Alt-Svc") != want {
		t.Fatalf("Alt-Svc = %q, want %q", resp.Header.Get("Alt-Svc"), want)
	}
}

func TestH3ClientNoticesServerSideTermination(t *testing.T) {
	e := startServer(t, "tcp4")
	serveH3(t, e, "127.0.0.1")
	ctx := context.Background()
	prof, err := e.srv.AddClient(ctx, "phone-h3", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	prof.IPv6 = ""
	tn, tnet := startClient(t, prof)
	if st := tn.Status(); st.Transport != string(dialer.KindH3) {
		t.Fatalf("want HTTP/3, got %+v", st)
	}
	fetchInside(t, tnet, mustAddr(e.srvCfg.Address4))
	yes, no := true, false
	if err := e.srv.UpdateClient("phone-h3", server.ClientUpdate{Disabled: &yes}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tn.Status().State == client.StateConnected {
		time.Sleep(50 * time.Millisecond)
	}
	if tn.Status().State == client.StateConnected {
		t.Fatal("HTTP/3 client did not notice that its session was closed by the server")
	}
	if err := e.srv.UpdateClient("phone-h3", server.ClientUpdate{Disabled: &no}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && tn.Status().State != client.StateConnected {
		time.Sleep(100 * time.Millisecond)
	}
	if st := tn.Status(); st.State != client.StateConnected {
		t.Fatalf("client did not come back after unblock: %+v", st)
	}
	if got := fetchInside(t, tnet, mustAddr(e.srvCfg.Address4)); !strings.HasPrefix(got, insideBody) {
		t.Fatalf("after unblock: %q", got)
	}
}
