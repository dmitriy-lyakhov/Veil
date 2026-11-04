package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/server"
)

type delayProxy struct {
	ln    net.Listener
	to    string
	delay atomic.Int64
	dead  atomic.Bool
	mu    sync.Mutex
	conns []net.Conn
}

func newDelayProxy(t *testing.T, to string) *delayProxy {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &delayProxy{ln: ln, to: to}
	t.Cleanup(func() { ln.Close(); p.cut() })
	go p.serve()
	return p
}

func (p *delayProxy) port() int { return p.ln.Addr().(*net.TCPAddr).Port }

func (p *delayProxy) set(ms int) { p.delay.Store(int64(ms) * int64(time.Millisecond)) }

func (p *delayProxy) cut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = nil
}

func (p *delayProxy) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		if p.dead.Load() {
			c.Close()
			continue
		}
		u, err := net.Dial("tcp", p.to)
		if err != nil {
			c.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, c, u)
		p.mu.Unlock()
		go p.pipe(u, c)
		go p.pipe(c, u)
	}
}

func (p *delayProxy) pipe(dst, src net.Conn) {
	defer dst.Close()
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			time.Sleep(time.Duration(p.delay.Load()))
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

type mesh struct {
	nodes   map[string]*env
	proxies map[[2]string]*delayProxy
}

func buildMesh(t *testing.T, names []string, exit string) *mesh {
	t.Helper()
	m := &mesh{nodes: map[string]*env{}, proxies: map[[2]string]*delayProxy{}}
	for _, n := range names {
		m.nodes[n] = startServer(t, "tcp4")
		m.nodes[n].srv.SetIntervals(time.Hour, time.Hour)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, a := range names {
		if a == exit {
			continue
		}
		for _, b := range names {
			if a == b || b == names[0] {
				continue
			}
			px := newDelayProxy(t, fmt.Sprintf("127.0.0.1:%d", m.nodes[b].port))
			m.proxies[[2]string{a, b}] = px
			raw, err := m.nodes[b].srv.IssueRelay(a)
			if err != nil {
				t.Fatal(err)
			}
			link, err := config.DecodeRelayLink(raw)
			if err != nil {
				t.Fatal(err)
			}
			link.Port, link.IPv6, link.Transports = px.port(), "", []string{"h2"}
			via, _ := config.EncodeRelayLink(link)
			if err := m.nodes[a].srv.AddUpstream(ctx, b, via); err != nil {
				t.Fatalf("%s→%s: %v", a, b, err)
			}
		}
	}
	return m
}

func (m *mesh) delays(d map[string]int) {
	for k, px := range m.proxies {
		px.set(d[k[0]+">"+k[1]])
	}
}

func (m *mesh) probe(rounds int) {
	ctx := context.Background()
	for i := 0; i < rounds; i++ {
		var wg sync.WaitGroup
		for _, n := range m.nodes {
			wg.Add(1)
			go func() { defer wg.Done(); n.srv.ProbeNow(ctx) }()
		}
		wg.Wait()
	}
}

func routeOf(srv *server.Server, name string) []string {
	for _, c := range srv.State().Clients {
		if c.Name == name {
			return c.Route
		}
	}
	return nil
}

func TestAutoRouteFollowsLatency(t *testing.T) {
	names := []string{"E", "A", "B", "X"}
	m := buildMesh(t, names, "X")
	slow, fast := 60, 2
	m.delays(map[string]int{
		"E>A": fast, "A>B": fast, "B>X": fast,
		"E>B": slow, "B>A": slow, "A>X": slow, "E>X": slow,
	})
	m.probe(3)
	entry := m.nodes["E"].srv
	ctx := context.Background()
	prof, err := entry.AddAutoClient(ctx, "auto-laptop", "", "X", server.AllHops)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(routeOf(entry, "auto-laptop"), ">"); got != "A>B>X" {
		t.Fatalf("initial route %q, want A>B>X", got)
	}
	prof.IPv6 = ""
	tn, tnet := startClient(t, prof)
	exitCompany := m.nodes["X"].srvCfg.Site.Company
	if got := fetchInside(t, tnet, mustAddr(m.nodes["X"].srvCfg.Address4)); !strings.Contains(got, exitCompany) {
		t.Fatalf("not exiting via X: %q", got)
	}

	m.delays(map[string]int{
		"E>B": fast, "B>A": fast, "A>X": fast,
		"E>A": slow, "A>B": slow, "B>X": slow, "E>X": slow,
	})
	m.probe(6)
	if ev := entry.OptimizeNow(ctx); len(ev) != 0 {
		t.Fatalf("rerouted on the first sighting, hysteresis expected: %+v", ev)
	}
	ev := entry.OptimizeNow(ctx)
	if len(ev) != 1 || ev[0].Error != "" {
		t.Fatalf("expected one reroute, got %+v", ev)
	}
	t.Logf("reroute: %s (%.1f ms) → %s (%.1f ms)", ev[0].From, ev[0].OldMs, ev[0].To, ev[0].NewMs)
	if got := strings.Join(routeOf(entry, "auto-laptop"), ">"); got != "B>A>X" {
		t.Fatalf("route after reroute %q, want B>A>X", got)
	}
	deadline := time.Now().Add(15 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		if tn.Status().State == client.StateConnected {
			if got = fetchInsideMaybe(tnet, m.nodes["X"].srvCfg.Address4); strings.Contains(got, exitCompany) {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !strings.Contains(got, exitCompany) {
		t.Fatalf("client lost connectivity after reroute: %q", got)
	}
	for _, n := range []string{"A", "B"} {
		count := 0
		for _, c := range m.nodes[n].srv.State().Clients {
			if strings.Contains(c.Name, "auto-laptop") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("node %s keeps %d records for the client after reroute, want 1", n, count)
		}
	}

	m.proxies[[2]string{"E", "B"}].dead.Store(true)
	m.proxies[[2]string{"E", "B"}].cut()
	m.probe(4)
	ev = entry.OptimizeNow(ctx)
	if len(ev) != 1 || ev[0].OldMs != -1 {
		t.Fatalf("broken first hop must reroute immediately, got %+v", ev)
	}
	if got := strings.Join(routeOf(entry, "auto-laptop"), ">"); got != "A>B>X" {
		t.Fatalf("route still uses dead hop: %v", got)
	}
	t.Logf("after E→B died: %s", strings.Join(routeOf(entry, "auto-laptop"), " → "))
}

func fetchInsideMaybe(tnet *netstack.Net, prefix string) string {
	addr := mustAddr(prefix)
	hc := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(addr, 80))
	}}}
	resp, err := hc.Get("http://inside/")
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
