package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/server"
)

const adminPassword = "correct horse battery"

type adminCtx struct {
	e       *env
	base    string
	hc      *http.Client
	outside *http.Client
	outBase string
}

func newHTTPClient(dial func(ctx context.Context, network, addr string) (net.Conn, error)) *http.Client {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, ForceAttemptHTTP2: true, DialContext: dial}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second, Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func adminEnv(t *testing.T) *adminCtx {
	t.Helper()
	e := startServer(t, "tcp4")
	hash, err := provision.HashAdminPassword(adminPassword)
	if err != nil {
		t.Fatal(err)
	}
	e.srvCfg.Admin = config.Admin{User: "admin", PasswordHash: hash}
	e.srvCfg.PublicIPv4 = "127.0.0.1"
	e.profile.IPv6 = ""
	_, tnet := startClient(t, e.profile)
	inner := mustAddr(e.srvCfg.Address4)
	return &adminCtx{
		e:    e,
		base: "https://" + inner.String(),
		hc: newHTTPClient(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return tnet.DialContextTCPAddrPort(ctx, netip.AddrPortFrom(inner, 443))
		}),
		outside: newHTTPClient(nil),
		outBase: fmt.Sprintf("https://127.0.0.1:%d", e.port),
	}
}

func get(t *testing.T, hc *http.Client, u string) (int, string) {
	t.Helper()
	resp, err := hc.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func login(t *testing.T, hc *http.Client, base, password string) (int, string) {
	t.Helper()
	resp, err := hc.PostForm(base+"/admin/login", url.Values{"login": {"admin"}, "password": {password}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func apiPost(t *testing.T, hc *http.Client, u string, body any, csrf bool) (int, map[string]string) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, u, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if csrf {
		req.Header.Set("X-Panel-Request", "1")
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]string{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestAdminHiddenOutsideTunnel(t *testing.T) {
	a := adminEnv(t)
	company := a.e.srvCfg.Site.Company
	if code, body := get(t, a.outside, a.outBase+"/"); code != 200 || !strings.Contains(body, company) {
		t.Fatalf("cover site: %d", code)
	}
	for _, p := range []string{"/admin", "/admin/", "/admin/api/state", "/wp-login.php"} {
		if code, body := get(t, a.outside, a.outBase+p); code != 404 || !strings.Contains(body, "nginx") {
			t.Fatalf("outside %s: want nginx 404, got %d", p, code)
		}
	}
	if code, body := login(t, a.outside, a.outBase, adminPassword); code != 404 || !strings.Contains(body, "nginx") {
		t.Fatalf("outside login: want 404, got %d", code)
	}
	if _, body := get(t, a.outside, a.outBase+"/robots.txt"); strings.Contains(body, "admin") {
		t.Fatal("robots.txt hints at /admin")
	}
	resp, err := a.outside.PostForm(a.outBase+"/contact", url.Values{"name": {"x"}, "reply": {"y"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("contact form: %d", resp.StatusCode)
	}
}

func TestAdminLoginPageInsideTunnel(t *testing.T) {
	a := adminEnv(t)
	code, body := get(t, a.hc, a.base+"/admin")
	if code != 200 || !strings.Contains(body, a.e.srvCfg.Site.Company) {
		t.Fatalf("login page via tunnel: %d", code)
	}
	for _, leak := range []string{"VPN", "Veil", "WireGuard"} {
		if strings.Contains(body, leak) {
			t.Fatalf("login page leaks %q", leak)
		}
	}
	if code, _ := get(t, a.hc, a.base+"/admin/api/state"); code != 404 {
		t.Fatalf("api without session: %d", code)
	}
}

func TestAdminLoginBruteforceBlocked(t *testing.T) {
	a := adminEnv(t)
	for i := 0; i < 5; i++ {
		if code, body := login(t, a.hc, a.base, "wrong"); code != 200 || !strings.Contains(body, "error") {
			t.Fatalf("attempt %d: %d", i, code)
		}
	}
	if code, _ := login(t, a.hc, a.base, adminPassword); code == http.StatusSeeOther {
		t.Fatal("correct password accepted while IP is blocked")
	}
}

func TestAdminManagesClients(t *testing.T) {
	a := adminEnv(t)
	e, base, hc := a.e, a.base, a.hc
	if code, _ := login(t, hc, base, adminPassword); code != http.StatusSeeOther {
		t.Fatalf("login: %d", code)
	}
	if code, body := get(t, hc, base+"/admin"); code != 200 || !strings.Contains(body, "Клиенты") {
		t.Fatalf("panel: %d", code)
	}
	if code, _ := apiPost(t, hc, base+"/admin/api/clients", map[string]string{"name": "phone"}, false); code != http.StatusForbidden {
		t.Fatalf("missing CSRF header accepted: %d", code)
	}
	code, created := apiPost(t, hc, base+"/admin/api/clients", map[string]string{"name": "phone"}, true)
	if code != 200 || !strings.HasPrefix(created["link"], config.LinkScheme) || created["file"] != "phone.veil" {
		t.Fatalf("create: %d %v", code, created)
	}
	if code, _ := apiPost(t, hc, base+"/admin/api/clients", map[string]string{"name": "phone"}, true); code != 400 {
		t.Fatalf("duplicate accepted: %d", code)
	}

	prof, err := config.DecodeLink(created["link"])
	if err != nil {
		t.Fatal(err)
	}
	if prof.IPv4 != "127.0.0.1" || prof.Port != e.port {
		t.Fatalf("link profile: %+v", prof)
	}
	prof.Port = e.port
	tn, tnet := startClient(t, prof)
	if got := fetchInside(t, tnet, mustAddr(e.srvCfg.Address4)); !strings.HasPrefix(got, insideBody) {
		t.Fatalf("imported client: %q", got)
	}

	var st server.State
	_, raw := get(t, hc, base+"/admin/api/state")
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatal(err)
	}
	var phone *server.ClientInfo
	for i := range st.Clients {
		if st.Clients[i].Name == "phone" {
			phone = &st.Clients[i]
		}
	}
	if phone == nil || !phone.Online || len(phone.Sessions) != 1 || phone.Sessions[0].Transport != "HTTP/2" || phone.RxBytes == 0 {
		t.Fatalf("state for phone: %+v", phone)
	}
	if st.Server.Clients != 2 || st.Server.Online != 2 {
		t.Fatalf("server info: %+v", st.Server)
	}

	if code, _ := apiPost(t, hc, base+"/admin/api/clients/delete", map[string]string{"name": "phone"}, true); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tn.Status().State == client.StateConnected {
		time.Sleep(50 * time.Millisecond)
	}
	if st := tn.Status(); st.State == client.StateConnected {
		t.Fatalf("removed client still connected: %+v", st)
	}
	d := &client.Dialer{Profile: prof}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := d.Connect(ctx); err == nil {
		t.Fatal("removed client can still authenticate")
	}
}
