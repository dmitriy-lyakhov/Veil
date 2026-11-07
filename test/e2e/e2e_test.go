package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/client"
	"veil/internal/client/dialer"
	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/server"
	"veil/internal/transport/proto"
)

const insideBody = "hello from inside the tunnel"

var bigBody = func() []byte {
	b := make([]byte, 512<<10)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}()

type env struct {
	srv     *server.Server
	srvCfg  *config.Server
	profile *config.Profile
	port    int
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func startServer(t *testing.T, network string) *env {
	t.Helper()
	cfg, err := provision.NewServer("ru")
	if err != nil {
		t.Fatal(err)
	}
	prof, err := provision.AddPeer(cfg, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	s4 := netip.MustParsePrefix(cfg.Address4).Addr()
	s6 := netip.MustParsePrefix(cfg.Address6).Addr()
	tunDev, tnet, err := netstack.CreateNetTUN([]netip.Addr{s4, s6}, nil, cfg.MTU)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(cfg, "", tunDev, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)

	for _, a := range []netip.Addr{s4, s6} {
		ln, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(a, 80))
		if err != nil {
			t.Fatal(err)
		}
		inner := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/big" {
				_, _ = w.Write(bigBody)
				return
			}
			fmt.Fprintf(w, "%s via %s at %s", insideBody, r.RemoteAddr, cfg.Site.Company)
		})}
		go inner.Serve(ln)
		t.Cleanup(func() { inner.Close() })
	}

	hs, err := srv.HTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	laddr := "[::1]:0"
	if network == "tcp4" {
		laddr = "127.0.0.1:0"
	}
	ln, err := net.Listen(network, laddr)
	if err != nil {
		t.Fatal(err)
	}
	go hs.ServeTLS(ln, "", "")
	t.Cleanup(func() { hs.Close() })
	for _, a := range []netip.Addr{s4, s6} {
		tl, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(a, 443))
		if err != nil {
			t.Fatal(err)
		}
		go hs.ServeTLS(tl, "", "")
	}

	port := ln.Addr().(*net.TCPAddr).Port
	cfg.Listen = fmt.Sprintf(":%d", port)
	cfg.PublicIPv4 = "127.0.0.1"
	prof.Port = port
	prof.IPv4 = "127.0.0.1"
	prof.IPv6 = "::1"
	return &env{srv: srv, srvCfg: cfg, profile: prof, port: port}
}

func startClient(t *testing.T, prof *config.Profile) (*client.Tunnel, *netstack.Net) {
	t.Helper()
	c4 := netip.MustParsePrefix(prof.Address4).Addr()
	c6 := netip.MustParsePrefix(prof.Address6).Addr()
	tunDev, tnet, err := netstack.CreateNetTUN([]netip.Addr{c4, c6}, nil, prof.EffectiveMTU())
	if err != nil {
		t.Fatal(err)
	}
	tn, err := client.Start(prof, tunDev, nil, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tn.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatalf("handshake: %v (status %+v)", err, tn.Status())
	}
	return tn, tnet
}

func fetchInside(t *testing.T, tnet *netstack.Net, addr netip.Addr) string {
	t.Helper()
	hc := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, network, a string) (net.Conn, error) {
			return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(addr, 80))
		}},
	}
	resp, err := hc.Get("http://inside/")
	if err != nil {
		t.Fatalf("GET via tunnel to %s: %v", addr, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestTunnelPrefersIPv6(t *testing.T) {
	e := startServerOnBoth(t)
	tn, tnet := startClient(t, e.profile)
	if st := tn.Status(); st.Family != "IPv6" {
		t.Fatalf("expected IPv6 transport, got %+v", st)
	}
	for _, a := range []string{e.srvCfg.Address4, e.srvCfg.Address6} {
		got := fetchInside(t, tnet, netip.MustParsePrefix(a).Addr())
		if !strings.HasPrefix(got, insideBody) {
			t.Fatalf("unexpected body %q", got)
		}
		t.Logf("inner %s -> %q", a, got)
	}
	st := tn.Status()
	if st.RxBytes == 0 || st.TxBytes == 0 {
		t.Fatalf("no traffic accounted: %+v", st)
	}
}

func startServerOnBoth(t *testing.T) *env {
	t.Helper()
	e := startServer(t, "tcp6")
	ln4, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", e.port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln4.Close() })
	go proxy(ln4, fmt.Sprintf("[::1]:%d", e.port))
	return e
}

func proxy(ln net.Listener, to string) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			u, err := net.Dial("tcp", to)
			if err != nil {
				return
			}
			defer u.Close()
			go io.Copy(u, c)
			io.Copy(c, u)
		}()
	}
}

func TestTunnelFallsBackToIPv4(t *testing.T) {
	e := startServer(t, "tcp4")
	start := time.Now()
	tn, tnet := startClient(t, e.profile)
	if st := tn.Status(); st.Family != "IPv4" {
		t.Fatalf("expected IPv4 fallback, got %+v", st)
	}
	t.Logf("connected via IPv4 fallback in %v", time.Since(start))
	got := fetchInside(t, tnet, netip.MustParsePrefix(e.srvCfg.Address6).Addr())
	if !strings.HasPrefix(got, insideBody) {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestTunnelIPv4OnlyProfile(t *testing.T) {
	e := startServer(t, "tcp4")
	e.profile.IPv6 = ""
	tn, _ := startClient(t, e.profile)
	if st := tn.Status(); st.Family != "IPv4" {
		t.Fatalf("expected IPv4, got %+v", st)
	}
}

func TestProbeSeesDecoySite(t *testing.T) {
	e := startServer(t, "tcp4")
	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
	}}
	base := fmt.Sprintf("https://127.0.0.1:%d", e.port)

	resp, err := hc.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Server") != "nginx" || !strings.Contains(string(body), "<title>") {
		t.Fatalf("decoy index: %d %v", resp.StatusCode, resp.Header)
	}
	if resp.ProtoMajor != 2 {
		t.Fatalf("decoy should speak h2, got %s", resp.Proto)
	}

	for _, auth := range []string{"", "Bearer garbage", "Bearer " + proto.NewToken([]byte("wrong-key"), time.Now())} {
		req, _ := http.NewRequest(http.MethodPost, base+e.profile.Path, strings.NewReader("x"))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 || resp.Header.Get("Server") != "nginx" {
			t.Fatalf("probe with %q got %d, want decoy 404", auth, resp.StatusCode)
		}
	}
}

func TestReplayedTokenRejected(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	v := proto.NewVerifier(map[string][]byte{"p": key})
	tok := proto.NewToken(key, time.Now())
	if _, err := v.Verify(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(tok); err != proto.ErrReplay {
		t.Fatalf("replay accepted: %v", err)
	}
	if _, err := v.Verify(proto.NewToken(key, time.Now().Add(-5*time.Minute))); err == nil {
		t.Fatal("stale token accepted")
	}
}

func TestWrongPinRejected(t *testing.T) {
	e := startServer(t, "tcp4")
	e.profile.IPv6 = ""
	e.profile.CertPin = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	d := &dialer.Dialer{Profile: e.profile}
	if _, err := d.Connect(context.Background()); err == nil || !strings.Contains(err.Error(), "pin") {
		t.Fatalf("expected pin mismatch, got %v", err)
	}
}

type killableProxy struct {
	ln    net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func (p *killableProxy) serve(to string) {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		u, err := net.Dial("tcp", to)
		if err != nil {
			c.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, c, u)
		p.mu.Unlock()
		go io.Copy(u, c)
		go io.Copy(c, u)
	}
}

func (p *killableProxy) killAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = nil
}

func TestTunnelRecoversAfterTransportLoss(t *testing.T) {
	e := startServer(t, "tcp4")
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	px := &killableProxy{ln: ln}
	go px.serve(fmt.Sprintf("127.0.0.1:%d", e.port))
	e.profile.IPv6 = ""
	e.profile.Port = ln.Addr().(*net.TCPAddr).Port

	tn, tnet := startClient(t, e.profile)
	inner := netip.MustParsePrefix(e.srvCfg.Address4).Addr()
	fetchInside(t, tnet, inner)
	firstSince := tn.Status().Since

	px.killAll()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st := tn.Status(); st.State == client.StateConnected && st.Since.After(firstSince) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	st := tn.Status()
	if !st.Since.After(firstSince) {
		t.Fatalf("transport did not reconnect: %+v", st)
	}
	if got := fetchInside(t, tnet, inner); !strings.HasPrefix(got, insideBody) {
		t.Fatalf("after reconnect: %q", got)
	}
	t.Logf("reconnected after %v", st.Since.Sub(firstSince))
}

func mustAddr(prefix string) netip.Addr { return netip.MustParsePrefix(prefix).Addr() }
