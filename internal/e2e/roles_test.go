package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Роли клиентов: «пользователь» не видит админку, «администратор» входит без пароля, имя https://admin
// перенаправляет на админку; клиенты без роли (старые) работают как раньше — по паролю.
func TestClientRolesControlAdminAccess(t *testing.T) {
	a := adminEnv(t)
	cl := a.e.srv.State().Clients
	if len(cl) == 0 {
		t.Fatal("нет клиентов")
	}
	name := cl[0].Name

	// без роли: страница входа, как раньше
	if code, _ := get(t, a.hc, a.base+"/admin"); code != 200 {
		t.Fatalf("клиент без роли: страница входа %d", code)
	}

	// пользователь: админки для него нет
	if err := a.e.srv.SetClientRole(name, "user"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/admin", "/admin/api/state"} {
		if code, body := get(t, a.hc, a.base+p); code != 404 || !strings.Contains(body, "nginx") {
			t.Fatalf("пользователь %s: ждали 404 nginx, получили %d", p, code)
		}
	}
	if code, _ := login(t, a.hc, a.base, adminPassword); code != 404 {
		t.Fatalf("пользователь не должен входить даже с паролем: %d", code)
	}

	// администратор: без пароля сразу в панель
	if err := a.e.srv.SetClientRole(name, "admin"); err != nil {
		t.Fatal(err)
	}
	if code, body := get(t, a.hc, a.base+"/admin/api/state"); code != 200 || !strings.Contains(body, "clients") {
		t.Fatalf("администратор: api/state %d", code)
	}
	if code, _ := get(t, a.hc, a.base+"/admin"); code != 200 {
		t.Fatalf("администратор: панель %d", code)
	}

	// короткое имя https://admin
	req, _ := http.NewRequest(http.MethodGet, a.base+"/", nil)
	req.Host = "admin"
	resp, err := a.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/admin" {
		t.Fatalf("Host: admin: ждали 302 на /admin, получили %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	// неверная роль отвергается
	if err := a.e.srv.SetClientRole(name, "root"); err == nil {
		t.Fatal("роль root не должна приниматься")
	}
}
