package gui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"veil/internal/config"
)

// Профиль с ролью «администратор» управляет сервером по туннелю: без SSH и паролей, когда VPN поднят к этому серверу.
func TestTunnelTransportForAdminProfile(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Panel-Request") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/admin/api/state":
			_, _ = io.WriteString(w, `{"server":{"company":"Ромашка"},"clients":[{"name":"Аня","role":"admin"}]}`)
		case "/admin/api/settings/password":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = io.WriteString(w, `{"ok":"1"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"not found"}`)
		}
	}))
	defer srv.Close()

	p := &config.Profile{Name: "Офис", Role: "admin", AdminURL: srv.URL + "/admin"}
	u := &ui{active: p.Name, connected: true}
	m := &manager{u: u, p: p}
	if !managed(p) || sshManaged(p) {
		t.Fatal("профиль администратора без SSH: управление по туннелю, но не по SSH")
	}
	do := m.transport()
	if do == nil {
		t.Fatal("при поднятом туннеле запросы должны идти по туннелю")
	}
	var st stateRoles
	if err := do("GET", "/state", nil, &st); err != nil {
		t.Fatal(err)
	}
	if st.Server.Company != "Ромашка" || st.roles["Аня"] != "admin" {
		t.Fatalf("состояние разобрано неверно: %+v %v", st.Server, st.roles)
	}
	if err := do("POST", "/settings/password", map[string]string{"new": "x"}, nil); err != nil || gotBody["new"] != "x" {
		t.Fatalf("POST по туннелю: %v %v", err, gotBody)
	}
	if err := do("GET", "/nope", nil, nil); err == nil {
		t.Fatal("ответ «не найдено» должен быть ошибкой")
	}

	u.connected = false
	if m.transport() != nil {
		t.Fatal("без туннеля — только вход по SSH")
	}
	u.connected, p.Role = true, "user"
	if m.transport() != nil || managed(p) {
		t.Fatal("роль «пользователь» сервером не управляет")
	}
}
