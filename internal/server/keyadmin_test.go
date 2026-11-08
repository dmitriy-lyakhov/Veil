package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veil/internal/config"
	"veil/internal/provision"
)

// Клиент-администратор, вошедший в панель по ключу туннеля, меняет пароль админки без текущего пароля
// (как приложение по SSH); вход по паролю в браузере по-прежнему требует текущий пароль.
func TestKeyAdminChangesPasswordWithoutCurrent(t *testing.T) {
	hash, err := provision.HashAdminPassword("old-password-123")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		cfg:    &config.Server{Admin: config.Admin{User: "admin", PasswordHash: hash}},
		log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		events: newEventLog(""),
		admin:  newAdminState(),
	}
	call := func(actor string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/admin/api/settings/password", strings.NewReader(`{"new":"new-password-456"}`))
		r.Header.Set(actorHeader, actor)
		w := httptest.NewRecorder()
		s.handleAPI(w, r, "settings/password", false)
		return w
	}
	if w := call(defaultAdminUser); w.Code == http.StatusOK {
		t.Fatalf("вход по паролю: смена без текущего пароля должна быть отклонена, ответ %d %s", w.Code, w.Body)
	}
	if w := call(actorKeyAdmin); w.Code != http.StatusOK {
		t.Fatalf("клиент-администратор: ответ %d %s", w.Code, w.Body)
	}
}

// keyAdmin узнаёт клиента-администратора только по заголовку, который ставит сам сервер или процесс панели.
func TestKeyAdminHeaderOnlyFromServer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if keyAdmin(r) {
		t.Fatal("без заголовка запрос не от клиента-администратора")
	}
	r.Header.Set(actorHeader, actorKeyAdmin)
	if !keyAdmin(r) {
		t.Fatal("заголовок сервера не распознан")
	}
}
