package veilcore

import (
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veil/internal/config"
	"veil/internal/provision"
)

func adminLink(t *testing.T, srv *httptest.Server, pin, role string) string {
	t.Helper()
	cfg, err := provision.NewServer("ru")
	if err != nil {
		t.Fatal(err)
	}
	p, err := provision.AddPeer(cfg, "t")
	if err != nil {
		t.Fatal(err)
	}
	p.IPv4, p.Role, p.CertPin, p.AdminURL = "203.0.113.1", role, pin, srv.URL+"/admin"
	link, err := config.EncodeLink(p)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func srvPin(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return provision.SPKIPin(cert)
}

func TestAdminCall(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Panel-Request") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/admin/api/state":
			w.Write([]byte(`{"clients":[{"name":"a"}]}`))
		case "/admin/api/clients":
			b := make([]byte, 64)
			n, _ := r.Body.Read(b)
			w.Write([]byte(`{"got":` + string(b[:n]) + `}`))
		case "/admin/api/clients/delete":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"клиент не найден"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer srv.Close()
	pin := srvPin(t, srv)

	got, err := Admin(adminLink(t, srv, pin, "admin"), "GET", "state", "")
	if err != nil || !strings.Contains(got, `"clients"`) {
		t.Fatalf("state: %q %v", got, err)
	}
	got, err = Admin(adminLink(t, srv, pin, "admin"), "POST", "clients", `{"name":"x"}`)
	if err != nil || !strings.Contains(got, `"name":"x"`) {
		t.Fatalf("clients: %q %v", got, err)
	}
	if _, err = Admin(adminLink(t, srv, pin, "admin"), "POST", "clients/delete", `{}`); err == nil || err.Error() != "клиент не найден" {
		t.Fatalf("текст ошибки сервера: %v", err)
	}
	if _, err = Admin(adminLink(t, srv, pin, "admin"), "GET", "nope", ""); err == nil || err.Error() != ErrNoAdmin {
		t.Fatalf("404 должен давать E_NOADMIN: %v", err)
	}
	// не администратор: запрос даже не отправляется
	if _, err = Admin(adminLink(t, srv, pin, "user"), "GET", "state", ""); err == nil || err.Error() != ErrNoAdmin {
		t.Fatalf("роль user: %v", err)
	}
	// неверный отпечаток сертификата
	if _, err = Admin(adminLink(t, srv, "AAAA", "admin"), "GET", "state", ""); err == nil {
		t.Fatal("чужой сертификат должен отклоняться")
	}
	if _, err = Admin(adminLink(t, srv, pin, "admin"), "DELETE", "state", ""); err == nil {
		t.Fatal("метод DELETE не поддерживается")
	}
}

func TestAdminBaseFallback(t *testing.T) {
	p := &config.Profile{Role: "admin", Address4: "10.66.0.7/32"}
	if b, err := adminBase(p); err != nil || b != "https://10.66.0.1" {
		t.Fatalf("запасной адрес: %q %v", b, err)
	}
	p.AdminURL = "https://10.9.9.9/admin"
	if b, _ := adminBase(p); b != "https://10.9.9.9" {
		t.Fatalf("адрес из профиля: %q", b)
	}
	if _, err := adminBase(&config.Profile{Role: "user", Address4: "10.66.0.7/32"}); err == nil {
		t.Fatal("у роли user админки нет")
	}
}
