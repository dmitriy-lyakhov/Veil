package e2e

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/client"
	"veil/internal/config"
)

func TestLiveServer(t *testing.T) {
	path := os.Getenv("VEIL_LIVE_PROFILE")
	if path == "" {
		t.Skip("set VEIL_LIVE_PROFILE to run against a deployed server")
	}
	prof, err := config.Load[config.Profile](path)
	if err != nil {
		t.Fatal(err)
	}
	if p := os.Getenv("VEIL_LIVE_PORT"); p != "" {
		prof.Port, _ = strconv.Atoi(p)
	}
	c4 := netip.MustParsePrefix(prof.Address4).Addr()
	c6 := netip.MustParsePrefix(prof.Address6).Addr()
	tunDev, tnet, err := netstack.CreateNetTUN([]netip.Addr{c4, c6}, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, prof.EffectiveMTU())
	if err != nil {
		t.Fatal(err)
	}
	tn, err := client.Start(prof, tunDev, nil, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatalf("handshake: %v %+v", err, tn.Status())
	}
	t.Logf("status: %+v", tn.Status())

	hc := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return tnet.DialContext(ctx, network, addr)
	}}}
	targets := strings.Fields(os.Getenv("VEIL_LIVE_URLS"))
	if len(targets) == 0 {
		targets = []string{"http://1.1.1.1/"}
	}
	for _, u := range targets {
		resp, err := hc.Get(u)
		if err != nil {
			t.Fatalf("%s via tunnel: %v; status %+v", u, err, tn.Status())
		}
		start := time.Now()
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("%s via tunnel: HTTP %d after %d bytes: %v", u, resp.StatusCode, len(body), err)
		}
		t.Logf("%s via tunnel: HTTP %d, %d bytes in %v", u, resp.StatusCode, len(body), time.Since(start).Round(time.Millisecond))
	}

	st := tn.Status()
	t.Logf("transport=%s remote=%s rx=%d tx=%d", st.Family, st.Remote, st.RxBytes, st.TxBytes)
}
