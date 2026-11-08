package gui

import (
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/client/adminapi"
	"veil/internal/config"
)

// sshSummary — «root · 203.0.113.7» под заголовком страницы сервера.
func sshSummary(p *config.Profile) string {
	host := p.IPv4
	if host == "" {
		host = p.IPv6
	}
	if p.SSH != nil && p.SSH.User != "" {
		return p.SSH.User + " · " + host
	}
	return addresses(p)
}

// renderLogin — вместо запроса пароля при открытии страницы: пояснение и кнопка входа.
func (m *manager) renderLogin(msg string) {
	if adminapi.Allowed(m.p) {
		// администратор открывает настройки по туннелю — без пароля SSH и пароля админки
		m.body.Add(note(L("You are an administrator of this server: connect to it, and the settings open over the VPN without any password.", "Вы администратор этого сервера: подключитесь к нему, и настройки откроются через VPN без пароля.")))
		open := widget.NewButtonWithIcon(L("Open settings", "Открыть настройки"), theme.ViewRefreshIcon(), func() {
			if !m.viaTunnel() && m.sess == nil {
				kit.ShowError(errors.New(L("connect to this server first", "сначала подключитесь к этому серверу")), m.w)
				return
			}
			m.refresh()
		})
		open.Importance = widget.HighImportance
		m.body.Add(open)
		if m.p.SSH == nil {
			return
		}
		msg = L("Or sign in over SSH.", "Или войдите по SSH.")
	}
	btn := widget.NewButtonWithIcon(L("Sign in over SSH", "Войти по SSH"), theme.LoginIcon(), func() {
		m.ensureSession(m.refresh, nil)
	})
	btn.Importance = widget.HighImportance
	if adminapi.Allowed(m.p) {
		btn.Importance = widget.MediumImportance
	}
	m.body.Add(note(msg))
	m.body.Add(btn)
}

// renderProtocol — вкладка «Протокол»: сведения из профиля, без входа на сервер.
func (m *manager) renderProtocol() {
	p := m.p
	m.body.Add(kit.ListRow("Veil",
		L("WireGuard inside HTTPS: HTTP/3 (QUIC) with HTTP/2 fallback on port 443. To outsiders the server looks like an ordinary website.", "WireGuard внутри HTTPS: HTTP/3 (QUIC) с запасным HTTP/2 на порту 443. Для посторонних сервер выглядит как обычный сайт."),
		widget.NewIcon(theme.ConfirmIcon()), nil))
	m.body.Add(kit.ListRow(L("Server address", "Адрес сервера"), addresses(p), nil, nil))
	m.body.Add(m.frontRow())
	if p.SSH != nil {
		auth := L("by password", "по паролю")
		if p.SSH.KeyPath != "" {
			auth = L("by key", "по ключу")
		}
		port := p.SSH.Port
		if port == 0 {
			port = 22
		}
		m.body.Add(kit.ListRow(L("SSH management", "Управление по SSH"), fmt.Sprintf(L("%s · port %d · login %s", "%s · порт %d · вход %s"), p.SSH.User, port, auth), nil, nil))
	} else if adminapi.Allowed(p) {
		m.body.Add(kit.ListRow(L("Management", "Управление"), L("As an administrator over the VPN: no password needed. Updating the server and installing protocols require SSH", "Как администратор через VPN, без пароля. Обновление сервера и установка протоколов — только по SSH"), nil, nil))
	} else {
		m.body.Add(kit.ListRow(L("Management", "Управление"), L("The server was added from a file or link; it cannot be managed from this app", "Сервер добавлен по файлу или ссылке, управлять им из этого приложения нельзя"), nil, nil))
	}
	if m.loaded {
		st := m.state.Server
		m.body.Add(kit.ListRow(L("Server version", "Версия на сервере"), st.Version, nil, nil))
		m.body.Add(kit.ListRow(L("Cover site", "Сайт-прикрытие"), st.Company, nil, nil))
	} else if p.SSH != nil {
		m.body.Add(kit.NavRow(L("Server information", "Сведения с сервера"), L("Version, cover site and clients — requires SSH sign-in", "Версия, сайт-прикрытие и клиенты — понадобится вход по SSH"), func() {
			m.ensureSession(m.refresh, nil)
		}))
	}
	if p.SSH != nil {
		m.renderProtocolList()
	}
}

// frontRow — строка «Маскировка»: под какой сайт замаскирован вход; нажатие открывает настройку.
func (m *manager) frontRow() fyne.CanvasObject {
	current := m.p.FrontSNI
	if m.loaded {
		current = m.state.Server.Front.SNI
	}
	sub := L("Off — the server answers as its own cover site", "Выключена — сервер отвечает как собственный сайт-прикрытие")
	if current != "" {
		sub = fmt.Sprintf(L("Strangers are sent to the real %s", "Посторонние уходят на настоящий %s"), current)
	}
	if m.loaded && m.p.FrontKey == "" && m.state.Server.Front.Enabled {
		sub += "\n" + L("This config has no masking key: add the server again from a new config", "В этом конфиге нет ключа маскировки: получите новый конфиг сервера")
	}
	return kit.ListRow(L("Masquerade as another site", "Маскировка под чужой сайт"), sub, widget.NewIcon(theme.NavigateNextIcon()), func() { m.showFrontDialog(current) })
}

func (m *manager) showFrontDialog(current string) {
	picker := newFrontPicker(m.u, current)
	hint := note(L("Real visitors and probes see the genuine site and its certificate; only your clients get through to the tunnel. TCP only: QUIC is turned off while masquerading is on.", "Посторонние и проверяющие видят настоящий сайт и его сертификат, в туннель проходят только ваши клиенты. Только TCP: пока маскировка включена, QUIC отключён."))
	content := container.NewVBox(hint, picker.View())
	d := kit.NewCustomConfirm(L("Masquerade as another site", "Маскировка под чужой сайт"), L("Apply", "Применить"), L("Cancel", "Отмена"), container.NewGridWrap(fyne.NewSize(340, 260), content), func(ok bool) {
		if !ok {
			return
		}
		sni, err := picker.SNI()
		if err != nil {
			kit.ShowError(err, m.w)
			return
		}
		if sni != "" && sni != current {
			kit.ShowConfirm(L("Turn masquerading on", "Включить маскировку"), frontWarning(sni), func(yes bool) {
				if yes {
					m.applyFront(sni)
				}
			}, m.w)
			return
		}
		m.applyFront(sni)
	}, m.w)
	d.Show()
}

// applyFront включает или выключает маскировку на сервере и обновляет профиль на этом устройстве:
// ключ общий для сервера, поэтому конфиг владельца перевыпускать не нужно.
func (m *manager) applyFront(sni string) {
	var resp struct {
		SNI string `json:"sni"`
		Key string `json:"key"`
	}
	m.call("POST", "/settings/front", map[string]string{"sni": sni}, &resp, func(err error) {
		if err != nil {
			kit.ShowError(err, m.w)
			return
		}
		m.p.FrontSNI, m.p.FrontKey = resp.SNI, resp.Key
		if sni == "" {
			m.p.FrontSNI, m.p.FrontKey = "", ""
		}
		if err := saveProfile(m.p); err != nil {
			kit.ShowError(err, m.w)
			return
		}
		msg := L("Masquerading is off.", "Маскировка выключена.")
		if sni != "" {
			msg = fmt.Sprintf(L("Masquerading as %s is on. Issue configs for other clients again, and reconnect this device.", "Маскировка под %s включена. Конфиги других клиентов выпустите заново, а это устройство переподключите."), sni)
		}
		kit.ShowInfo(L("Done", "Готово"), msg, m.w)
		m.refresh()
	})
}
