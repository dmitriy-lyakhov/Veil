package veilcore

import (
	"context"

	"veil/internal/client/adminapi"
	"veil/internal/config"
)

// ErrNoAdmin — у профиля нет роли администратора (или это не сервер Veil), либо сервер отвечает как на несуществующую страницу.
const ErrNoAdmin = adminapi.NoAdmin

// adminBase возвращает адрес сервера внутри туннеля («https://10.66.0.1») из профиля администратора.
func adminBase(p *config.Profile) (string, error) { return adminapi.Base(p) }

// Admin выполняет запрос к админ-API сервера из профиля администратора: GET state, POST clients, clients/update, clients/reissue,
// clients/delete, settings/… и т. д. (см. internal/server/api.go). Запрос идёт ПО ТУННЕЛЮ обычным сокетом (приложение при этом
// не исключается из VPN), поэтому вызывать его нужно при поднятом туннеле. Пароль не нужен: сервер узнаёт администратора по
// ключу туннеля. method — GET или POST, path — например "state", body — JSON запроса (для POST).
// Возвращает тело ответа (JSON). Ошибки: E_NOADMIN (нет прав), иначе текст сервера или сети.
func Admin(link, method, path, body string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	var b []byte
	if body != "" {
		b = []byte(body)
	}
	raw, err := adminapi.Do(context.Background(), p, method, path, b)
	return string(raw), err
}
