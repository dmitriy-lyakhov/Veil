package veilcore

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

// ErrNoAdmin — у профиля нет роли администратора (или это не сервер Veil), либо сервер отвечает как на несуществующую страницу.
const ErrNoAdmin = "E_NOADMIN"

const (
	adminTimeout  = 40 * time.Second
	adminMaxReply = 4 << 20
)

// adminBase возвращает адрес сервера внутри туннеля («https://10.66.0.1») из профиля администратора. Если в ссылке адреса нет
// (выдана до появления поля), берётся первый адрес подсети клиента: пулы клиентов нарезаются от адреса сервера (.1).
func adminBase(p *config.Profile) (string, error) {
	if !p.IsVeil() || p.Role != "admin" {
		return "", errors.New(ErrNoAdmin)
	}
	if u := strings.TrimSpace(p.AdminURL); u != "" {
		return strings.TrimSuffix(strings.TrimRight(u, "/"), "/admin"), nil
	}
	if pf, err := netip.ParsePrefix(p.Address4); err == nil && pf.Addr().Is4() {
		a := pf.Addr().As4()
		return fmt.Sprintf("https://%d.%d.%d.1", a[0], a[1], a[2]), nil
	}
	return "", errors.New(ErrNoAdmin)
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

// Admin выполняет запрос к админ-API сервера из профиля администратора: GET state, POST clients, clients/update, clients/reissue,
// clients/delete и т. д. (см. internal/server/api.go). Запрос идёт ПО ТУННЕЛЮ обычным сокетом (приложение при этом не исключается из VPN),
// поэтому вызывать его нужно при поднятом туннеле. method — GET или POST, path — например "state", body — JSON запроса (для POST).
// Возвращает тело ответа (JSON). Ошибки: E_NOADMIN (нет прав), иначе текст сервера или сети.
func Admin(link, method, path, body string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	base, err := adminBase(p)
	if err != nil {
		return "", err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodGet && method != http.MethodPost {
		return "", fmt.Errorf("метод %q не поддерживается", method)
	}
	tr := &http.Transport{
		// сертификат сервера самоподписанный: доверяем только закреплённому отпечатку
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: pinCheck(p.CertPin), MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true,
	}
	defer tr.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), adminTimeout)
	defer cancel()
	var rd io.Reader
	if method == http.MethodPost {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+"/admin/api/"+strings.TrimLeft(path, "/"), rd)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Panel-Request", "1") // защита панели от запросов с чужих страниц
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, adminMaxReply))
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusOK {
		return string(raw), nil
	}
	if resp.StatusCode == http.StatusNotFound {
		// «пользователь» и посторонние видят панель как несуществующую страницу
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) != nil || e.Error == "" || e.Error == "not found" {
			return "", errors.New(ErrNoAdmin)
		}
	}
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		return "", errors.New(e.Error)
	}
	return "", fmt.Errorf("сервер ответил %d", resp.StatusCode)
}
