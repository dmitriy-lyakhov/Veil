package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/front"
	"veil/internal/provision"
	"veil/internal/server"
)

const (
	frontSNI      = "stub.example"
	stubBody      = "REAL-SITE-STUB"
	tunnelFrontTL = 4443
)

type frontEnv struct {
	*env
	stub     *httptest.Server
	frontURL string
	tnet     *netstack.Net
}

func startFrontEnv(t *testing.T) *frontEnv {
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

	inner := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s at %s", insideBody, cfg.Site.Company)
	})}
	il, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(s4, 80))
	if err != nil {
		t.Fatal(err)
	}
	go inner.Serve(il)
	t.Cleanup(func() { inner.Close() })

	stub := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, stubBody)
	}))
	t.Cleanup(stub.Close)

	hs, err := srv.HTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	publicBypass := func(c net.Conn) bool {
		ap, err := netip.ParseAddrPort(c.RemoteAddr().String())
		return err == nil && !ap.Addr().IsLoopback() && srv.FrontBypass(c)
	}
	go hs.ServeTLS(front.NewListener(ln, srv.FrontConfig, publicBypass), "", "")
	t.Cleanup(func() { hs.Close() })

	tl, err := tnet.ListenTCPAddrPort(netip.AddrPortFrom(s4, tunnelFrontTL))
	if err != nil {
		t.Fatal(err)
	}
	go hs.ServeTLS(front.NewListener(tl, srv.FrontConfig, srv.FrontBypass), "", "")

	port := ln.Addr().(*net.TCPAddr).Port
	cfg.Listen = fmt.Sprintf(":%d", port)
	cfg.PublicIPv4 = "127.0.0.1"
	prof.Port, prof.IPv4 = port, "127.0.0.1"
	e := &env{srv: srv, srvCfg: cfg, profile: prof, port: port}
	return &frontEnv{env: e, stub: stub, frontURL: fmt.Sprintf("https://127.0.0.1:%d", port), tnet: tnet}
}

func (f *frontEnv) probe(t *testing.T, sni string) (cert []byte, body string) {
	t.Helper()
	conf := &tls.Config{InsecureSkipVerify: true, ServerName: sni, NextProtos: []string{"http/1.1"}}
	hc := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: conf, DisableKeepAlives: true}}
	req, _ := http.NewRequest(http.MethodGet, f.frontURL+"/", nil)
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("probe sni=%q: %v", sni, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.TLS.PeerCertificates[0].Raw, string(b)
}

func TestFrontMaskingEndToEnd(t *testing.T) {
	f := startFrontEnv(t)
	ctx := context.Background()

	if err := f.srv.SetFront(frontSNI, f.stub.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}

	prof, err := f.srv.AddClient(ctx, "masked", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if prof.FrontSNI != frontSNI || prof.FrontKey == "" {
		t.Fatalf("issued config has no masking data: sni=%q key=%q", prof.FrontSNI, prof.FrontKey)
	}
	prof.Port, prof.IPv4, prof.IPv6 = f.port, "127.0.0.1", ""
	if err := prof.Validate(); err != nil {
		t.Fatal(err)
	}
	if link, err := config.EncodeLink(prof); err != nil {
		t.Fatal(err)
	} else if back, err := config.DecodeLink(link); err != nil || back.FrontKey != prof.FrontKey {
		t.Fatalf("masking key lost in link: %v", err)
	}

	tn, tnet := startClient(t, prof)
	if st := tn.Status(); st.Transport != string(client.KindH2) {
		t.Fatalf("masked client must use TCP/HTTP2, got %+v", st)
	}
	if got := fetchInside(t, tnet, mustAddr(f.srvCfg.Address4)); !strings.HasPrefix(got, insideBody) {
		t.Fatalf("masked tunnel does not pass traffic: %q", got)
	}

	stubCert := f.stub.Certificate().Raw
	for _, sni := range []string{frontSNI, "", "another.example"} {
		cert, body := f.probe(t, sni)
		if string(cert) != string(stubCert) || body != stubBody {
			t.Fatalf("probe with sni=%q did not get the real site (body %q)", sni, body)
		}
	}
}

func TestFrontOldConfigIsRejected(t *testing.T) {
	f := startFrontEnv(t)
	ctx := context.Background()
	old, err := f.srv.AddClient(ctx, "issued-before", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	old.Port, old.IPv4, old.IPv6 = f.port, "127.0.0.1", ""
	if err := f.srv.SetFront(frontSNI, f.stub.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	d := &client.Dialer{Profile: old}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if _, err := d.Connect(cctx); err == nil {
		t.Fatal("a config without the masking key connected to a masked server")
	}
	if err := f.srv.SetFront("", ""); err != nil {
		t.Fatal(err)
	}
	cctx, cancel = context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	conn, err := d.Connect(cctx)
	if err != nil {
		t.Fatalf("after masking was turned off the old config must work again: %v", err)
	}
	conn.Close()
}

func TestFrontKeepsAdminPanelReachableInsideTunnel(t *testing.T) {
	f := startFrontEnv(t)
	ctx := context.Background()
	hash, err := provision.HashAdminPassword(adminPassword)
	if err != nil {
		t.Fatal(err)
	}
	f.srvCfg.Admin = config.Admin{User: "admin", PasswordHash: hash}
	if err := f.srv.SetFront(frontSNI, f.stub.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	prof, err := f.srv.AddClient(ctx, "admin-user", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	prof.Port, prof.IPv4, prof.IPv6 = f.port, "127.0.0.1", ""
	_, tnet := startClient(t, prof)

	inner := mustAddr(f.srvCfg.Address4)
	hc := newHTTPClient(func(ctx context.Context, _, _ string) (net.Conn, error) {
		return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(inner, tunnelFrontTL))
	})
	code, body := get(t, hc, "https://"+inner.String()+"/admin")
	if code != 200 || !strings.Contains(body, f.srvCfg.Site.Company) {
		t.Fatalf("admin login page through the tunnel with masking on: %d", code)
	}
	if body == stubBody {
		t.Fatal("tunnel client was forwarded to the stub site")
	}
}
