package e2e

import (
	"net/http"
	"strings"
	"testing"

	"veil/internal/server"
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

// Роль меняется через POST clients/update (как делает панель): доступ к админке меняется сразу,
// в state клиента видна role, неверная роль отвергается, а переименование/заметка роль не сбрасывают.
func TestClientRoleChangeViaAPIUpdate(t *testing.T) {
	a := adminEnv(t)
	name := a.e.srv.State().Clients[0].Name
	if err := a.e.srv.SetClientRole(name, "admin"); err != nil {
		t.Fatal(err)
	}
	if code, body := get(t, a.hc, a.base+"/admin/api/state"); code != 200 || !strings.Contains(body, `"role":"admin"`) {
		t.Fatalf("state администратора: %d, ждали role=admin в JSON клиента", code)
	}
	url := a.base + "/admin/api/clients/update"

	if code, out := apiPost(t, a.hc, url, map[string]any{"name": name, "role": "root"}, true); code != 400 {
		t.Fatalf("неверная роль: ждали 400, получили %d %v", code, out)
	}
	if code, out := apiPost(t, a.hc, url, map[string]any{"name": name, "note": "заметка"}, true); code != 200 {
		t.Fatalf("смена заметки: %d %v", code, out)
	}
	if got := a.e.srv.State().Clients[0].Role; got != "admin" {
		t.Fatalf("заметка не должна менять роль, роль = %q", got)
	}

	// администратор сам понижает себя до пользователя: доступ пропадает сразу
	if code, out := apiPost(t, a.hc, url, map[string]any{"name": name, "role": "user"}, true); code != 200 {
		t.Fatalf("смена роли на user: %d %v", code, out)
	}
	if got := a.e.srv.State().Clients[0].Role; got != "user" {
		t.Fatalf("роль после смены = %q, ждали user", got)
	}
	if code, body := get(t, a.hc, a.base+"/admin/api/state"); code != 404 || !strings.Contains(body, "nginx") {
		t.Fatalf("пользователь после смены роли: ждали 404 nginx, получили %d", code)
	}

	// и обратно — через сервер (панель недоступна пользователю), доступ возвращается
	if err := a.e.srv.UpdateClient(name, server.ClientUpdate{Role: strPtr("admin")}); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, a.hc, a.base+"/admin/api/state"); code != 200 {
		t.Fatalf("администратор после UpdateClient: %d", code)
	}
}

func strPtr(s string) *string { return &s }
