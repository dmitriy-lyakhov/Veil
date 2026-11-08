// Package adminapi — запросы клиента-администратора к админ-API сервера Veil по туннелю.
//
// Сервер узнаёт администратора по ключу туннеля (роль клиента «admin»), поэтому ни пароль админ-панели,
// ни SSH не нужны: достаточно поднятого туннеля к этому серверу. Используется мобильным ядром и
// настольным приложением.
package adminapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"veil/internal/config"
	"veil/internal/provision"
)

// NoAdmin — у профиля нет роли администратора (или это не сервер Veil), либо сервер отвечает как на несуществующую страницу.
const NoAdmin = "E_NOADMIN"

const (
	// Timeout — предел одного запроса (создание клиента в каскаде может идти через несколько узлов).
	Timeout  = 40 * time.Second
	maxReply = 4 << 20
)

// Allowed сообщает, может ли профиль управлять сервером по туннелю (сервер Veil и роль администратора).
func Allowed(p *config.Profile) bool { return p != nil && p.IsVeil() && p.Role == "admin" }

// Base возвращает адрес сервера внутри туннеля («https://10.66.0.1» или с портом, если сервер слушает не 443)
// из профиля администратора. Если в ссылке адреса нет (выдана до появления поля), берётся первый адрес подсети
// клиента: пулы клиентов нарезаются от адреса сервера (.1).
func Base(p *config.Profile) (string, error) {
	if !Allowed(p) {
		return "", errors.New(NoAdmin)
	}
	if u := strings.TrimSpace(p.AdminURL); u != "" {
		return strings.TrimSuffix(strings.TrimRight(u, "/"), "/admin"), nil
	}
	if pf, err := netip.ParsePrefix(p.Address4); err == nil && pf.Addr().Is4() {
		a := pf.Addr().As4()
		return fmt.Sprintf("https://%d.%d.%d.1", a[0], a[1], a[2]), nil
	}
	return "", errors.New(NoAdmin)
}

// pinCheck проверяет сертификат сервера по закреплённому отпечатку профиля (пусто — без проверки: канал уже внутри туннеля,
// а клиент определяется сервером по ключу туннеля).
func pinCheck(pin string) func([][]byte, [][]*x509.Certificate) error {
	return func(raw [][]byte, _ [][]*x509.Certificate) error {
		if pin == "" {
			return nil
		}
		if len(raw) == 0 {
			return errors.New("нет сертификата сервера")
		}
		cert, err := x509.ParseCertificate(raw[0])
		if err != nil {
			return err
		}
		if provision.SPKIPin(cert) != pin {
			return errors.New("отпечаток сертификата сервера не совпал")
		}
		return nil
	}
}

// Do выполняет запрос к админ-API: method — GET или POST, path — например "state" или "/clients/update"
// (ведущая косая черта не важна, можно с параметрами «?…»), body — JSON для POST. Запрос идёт обычным сокетом,
// то есть по туннелю, поэтому туннель к серверу должен быть поднят. Возвращает тело ответа (JSON).
// Ошибки: NoAdmin (нет прав), иначе текст сервера или сети.
func Do(ctx context.Context, p *config.Profile, method, path string, body []byte) ([]byte, error) {
	base, err := Base(p)
	if err != nil {
		return nil, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("метод %q не поддерживается", method)
	}
	tr := &http.Transport{
		// сертификат сервера самоподписанный: доверяем только закреплённому отпечатку
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: pinCheck(p.CertPin), MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true,
		Proxy:             nil, // системный прокси не должен перехватывать адрес внутри туннеля
	}
	defer tr.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var rd io.Reader
	if method == http.MethodPost {
		if body == nil {
			body = []byte("{}")
		}
		rd = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, base+"/admin/api/"+strings.TrimLeft(path, "/"), rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Panel-Request", "1") // защита панели от запросов с чужих страниц
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return raw, nil
	}
	var e struct {
		Error string `json:"error"`
	}
	if resp.StatusCode == http.StatusNotFound {
		// «пользователь» и посторонние видят панель как несуществующую страницу
		if json.Unmarshal(raw, &e) != nil || e.Error == "" || e.Error == "not found" {
			return nil, errors.New(NoAdmin)
		}
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		return nil, errors.New(e.Error)
	}
	return nil, fmt.Errorf("сервер ответил %d", resp.StatusCode)
}
