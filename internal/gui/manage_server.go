package gui

import (
	"fmt"
	"context"
	"errors"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
)

// renderManagement — вкладка «Управление»: проверка, обновление пакетов, перезагрузка и удаление.
func (m *manager) renderManagement() {
	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	// пароль спрашиваем только в момент действия
	needSSH := func(f func()) func() {
		return func() { m.ensureSession(f, nil) }
	}
	m.body.Add(listRow("Проверить сервер", "Обновить сведения о версии и клиентах", arrow(), needSSH(m.refresh)))
	m.body.Add(listRowColored("Обновить пакеты сервера", "Установить обновления системы менеджером пакетов дистрибутива", theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm("Обновить пакеты", "На сервере будет запущено обновление системных пакетов. Это может занять несколько минут. Продолжить?", func(ok bool) {
			if !ok {
				return
			}
			m.status.SetText("Обновление пакетов сервера…")
			go func() {
				err := m.sess.UpdatePackages(context.Background())
				fyne.Do(func() {
					if err != nil {
						m.status.SetText("Не удалось обновить пакеты")
						showError(err, m.w)
						return
					}
					m.status.SetText("Пакеты сервера обновлены")
				})
			}()
		}, m.w)
	})))
	m.body.Add(listRowColored("Перезагрузить сервер", "", theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm("Перезагрузка", "Сервер будет перезагружен, VPN на время перезагрузки недоступен. Продолжить?", func(ok bool) {
			if !ok {
				return
			}
			err := m.sess.Reboot()
			m.u.dropSession(m.p.Name)
			m.sess = nil
			if err != nil {
				showError(err, m.w)
				return
			}
			m.status.SetText("Сервер перезагружается…")
		}, m.w)
	})))
	m.body.Add(listRowColored("Удалить сервер из приложения", "", theme.ColorNameError, arrow(), func() {
		showConfirm("Удалить сервер", "Профиль «"+m.p.Name+"» будет удалён с этого устройства. Сам сервер не изменится. Продолжить?", func(ok bool) {
			if !ok {
				return
			}
			if m.p.Name == m.u.active {
				showError(errors.New("сначала отключите VPN"), m.w)
				return
			}
			if path, err := config.ProfilePath(m.p.Name); err == nil {
				_ = os.Remove(path)
			}
			m.u.dropSession(m.p.Name)
			m.u.reload()
			m.w.Close()
		}, m.w)
	}))
	m.body.Add(listRowColored("Удалить Veil с сервера", "Остановить службу и удалить программу и настройки на сервере", theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm("Удалить Veil", "Veil будет остановлен и удалён с сервера, все его клиенты перестанут подключаться. Продолжить?", func(ok bool) {
			if !ok {
				return
			}
			if err := m.sess.Uninstall(); err != nil {
				showError(err, m.w)
				return
			}
			m.status.SetText("Veil удалён с сервера")
			showInfo("Готово", "Veil удалён с сервера. Профиль в приложении остался: удалите его отдельно.", m.w)
		}, m.w)
	})))
}

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
	btn := widget.NewButtonWithIcon("Войти по SSH", theme.LoginIcon(), func() {
		m.ensureSession(m.refresh, nil)
	})
	btn.Importance = widget.HighImportance
	m.body.Add(note(msg))
	m.body.Add(btn)
}

// renderProtocol — вкладка «Протокол»: сведения из профиля, без входа на сервер.
func (m *manager) renderProtocol() {
	p := m.p
	m.body.Add(listRow("Veil",
		"WireGuard внутри HTTPS: HTTP/3 (QUIC) с запасным HTTP/2 на порту 443. Для посторонних сервер выглядит как обычный сайт.",
		widget.NewIcon(theme.ConfirmIcon()), nil))
	m.body.Add(listRow("Адрес сервера", addresses(p), nil, nil))
	if p.SSH != nil {
		auth := "по паролю"
		if p.SSH.KeyPath != "" {
			auth = "по ключу"
		}
		port := p.SSH.Port
		if port == 0 {
			port = 22
		}
		m.body.Add(listRow("Управление по SSH", fmt.Sprintf("%s · порт %d · вход %s", p.SSH.User, port, auth), nil, nil))
	} else {
		m.body.Add(listRow("Управление", "Сервер добавлен по файлу или ссылке, управлять им из этого приложения нельзя", nil, nil))
	}
	if m.loaded {
		st := m.state.Server
		m.body.Add(listRow("Версия на сервере", st.Version, nil, nil))
		m.body.Add(listRow("Сайт-прикрытие", st.Company, nil, nil))
	} else if p.SSH != nil {
		m.body.Add(navRow("Сведения с сервера", "Версия, сайт-прикрытие и клиенты — понадобится вход по SSH", func() {
			m.ensureSession(m.refresh, nil)
		}))
	}
	if p.SSH != nil {
		m.renderProtocolList()
	}
}
