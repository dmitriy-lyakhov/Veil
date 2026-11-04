package gui

import (
	"context"
	"errors"
	"fmt"
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
	m.body.Add(listRow(L("Check server", "Проверить сервер"), L("Refresh version and client information", "Обновить сведения о версии и клиентах"), arrow(), needSSH(m.refresh)))
	m.body.Add(listRowColored(L("Update server packages", "Обновить пакеты сервера"), L("Install system updates with the distribution package manager", "Установить обновления системы менеджером пакетов дистрибутива"), theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm(L("Update packages", "Обновить пакеты"), L("System packages will be updated on the server. This may take a few minutes. Continue?", "На сервере будет запущено обновление системных пакетов. Это может занять несколько минут. Продолжить?"), func(ok bool) {
			if !ok {
				return
			}
			m.status.SetText(L("Updating server packages…", "Обновление пакетов сервера…"))
			go func() {
				err := m.sess.UpdatePackages(context.Background())
				fyne.Do(func() {
					if err != nil {
						m.status.SetText(L("Failed to update packages", "Не удалось обновить пакеты"))
						showError(err, m.w)
						return
					}
					m.status.SetText(L("Server packages updated", "Пакеты сервера обновлены"))
				})
			}()
		}, m.w)
	})))
	m.body.Add(listRowColored(L("Reboot server", "Перезагрузить сервер"), "", theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm(L("Reboot", "Перезагрузка"), L("The server will be rebooted; VPN is unavailable during the reboot. Continue?", "Сервер будет перезагружен, VPN на время перезагрузки недоступен. Продолжить?"), func(ok bool) {
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
			m.status.SetText(L("Server is rebooting…", "Сервер перезагружается…"))
		}, m.w)
	})))
	m.body.Add(listRowColored(L("Remove server from the app", "Удалить сервер из приложения"), "", theme.ColorNameError, arrow(), func() {
		showConfirm(L("Remove server", "Удалить сервер"), fmt.Sprintf(L("Profile “%s” will be deleted from this device. The server itself will not change. Continue?", "Профиль «%s» будет удалён с этого устройства. Сам сервер не изменится. Продолжить?"), m.p.Name), func(ok bool) {
			if !ok {
				return
			}
			if m.p.Name == m.u.active {
				showError(errors.New(L("disconnect the VPN first", "сначала отключите VPN")), m.w)
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
	m.body.Add(listRowColored(L("Uninstall Veil from server", "Удалить Veil с сервера"), L("Stop the service and delete the program and settings on the server", "Остановить службу и удалить программу и настройки на сервере"), theme.ColorNameError, arrow(), needSSH(func() {
		showConfirm(L("Uninstall Veil", "Удалить Veil"), L("Veil will be stopped and removed from the server; all its clients will stop connecting. Continue?", "Veil будет остановлен и удалён с сервера, все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
			if !ok {
				return
			}
			if err := m.sess.Uninstall(); err != nil {
				showError(err, m.w)
				return
			}
			m.status.SetText(L("Veil removed from server", "Veil удалён с сервера"))
			showInfo(L("Done", "Готово"), L("Veil has been removed from the server. The profile remains in the app: delete it separately.", "Veil удалён с сервера. Профиль в приложении остался: удалите его отдельно."), m.w)
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
	btn := widget.NewButtonWithIcon(L("Sign in over SSH", "Войти по SSH"), theme.LoginIcon(), func() {
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
		L("WireGuard inside HTTPS: HTTP/3 (QUIC) with HTTP/2 fallback on port 443. To outsiders the server looks like an ordinary website.", "WireGuard внутри HTTPS: HTTP/3 (QUIC) с запасным HTTP/2 на порту 443. Для посторонних сервер выглядит как обычный сайт."),
		widget.NewIcon(theme.ConfirmIcon()), nil))
	m.body.Add(listRow(L("Server address", "Адрес сервера"), addresses(p), nil, nil))
	if p.SSH != nil {
		auth := L("by password", "по паролю")
		if p.SSH.KeyPath != "" {
			auth = L("by key", "по ключу")
		}
		port := p.SSH.Port
		if port == 0 {
			port = 22
		}
		m.body.Add(listRow(L("SSH management", "Управление по SSH"), fmt.Sprintf(L("%s · port %d · login %s", "%s · порт %d · вход %s"), p.SSH.User, port, auth), nil, nil))
	} else {
		m.body.Add(listRow(L("Management", "Управление"), L("The server was added from a file or link; it cannot be managed from this app", "Сервер добавлен по файлу или ссылке, управлять им из этого приложения нельзя"), nil, nil))
	}
	if m.loaded {
		st := m.state.Server
		m.body.Add(listRow(L("Server version", "Версия на сервере"), st.Version, nil, nil))
		m.body.Add(listRow(L("Cover site", "Сайт-прикрытие"), st.Company, nil, nil))
	} else if p.SSH != nil {
		m.body.Add(navRow(L("Server information", "Сведения с сервера"), L("Version, cover site and clients — requires SSH sign-in", "Версия, сайт-прикрытие и клиенты — понадобится вход по SSH"), func() {
			m.ensureSession(m.refresh, nil)
		}))
	}
	if p.SSH != nil {
		m.renderProtocolList()
	}
}
