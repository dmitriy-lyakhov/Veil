package gui

// Вкладка «Управление» страницы сервера (дизайн Containers): версия Docker, контейнеры Veil с журналом
// и перезапуском, переключатель админ-панели (veil-admin), обновление и удаление Veil, прочие действия.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/config"
	"veil/internal/deploy"
)

// Вкладки страницы сервера.
const (
	tabProtocol = iota
	tabServices
	tabLinks
	tabCascade
	tabManagement
)

func managerTabs() []string {
	return []string{L("Protocol", "Протокол"), L("Services", "Сервисы"), L("Links", "Связи"), L("Cascade", "Каскад"), L("Management", "Управление")}
}

// dockerInfo — Docker и контейнеры на сервере (снимок для вкладки «Управление»).
type dockerInfo struct {
	version    string
	list       []deploy.ContainerInfo
	adminOn    bool
	adminAvail bool
	err        error
}

// loadDocker читает сведения о контейнерах по SSH в фоне и перерисовывает вкладку.
func (m *manager) loadDocker() {
	if m.sess == nil || m.dockerLoading {
		return
	}
	m.dockerLoading = true
	s := m.sess
	go func() {
		d := &dockerInfo{version: s.DockerVersion()}
		d.list, d.err = s.Containers()
		d.adminOn, d.adminAvail = s.AdminPanel()
		fyne.Do(func() {
			m.dockerLoading = false
			m.docker = d
			if m.section == tabManagement {
				m.render()
			}
		})
	}()
}

// containerRow — строка контейнера: состояние, имя, образ и порты; справа «Журнал» и «Перезапустить».
func (m *manager) containerRow(c deploy.ContainerInfo) fyne.CanvasObject {
	busy := m.dockerBusy[c.Name]
	title := c.Name
	if what := containerWhat(c.Protocol); what != "" {
		title += " · " + what
	}
	sub := containerSub(c)
	var mark fyne.CanvasObject = statusDot(c.Running())
	if busy {
		sub = L("restarting…", "перезапуск…")
		var stop func()
		mark, stop = pulse(12)
		m.anims = append(m.anims, stop)
	}
	text := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 2, 12), mark), nil, kit.Vcenter(kit.RowText(title, sub, theme.ColorNameForeground)))
	actions := container.New(kit.Hrow{Gap: 8},
		smallButton(L("Log", "Журнал"), func() { m.showContainerLog(c.Name) }),
		smallButton(L("Restart", "Перезапустить"), func() { m.restartContainer(c.Name) }),
	)
	row := container.New(&sideActions{minText: 200, gap: 12}, text, container.NewCenter(actions))
	card, _ := padCard(row, 8, 14, 58)
	return card
}

func (m *manager) showContainerLog(name string) {
	m.ensureSession(func() {
		m.status.SetText(fmt.Sprintf(L("Reading the log of %s…", "Читаем журнал %s…"), name))
		s := m.sess
		go func() {
			out, err := s.ContainerLogs(name, 200)
			fyne.Do(func() {
				m.status.SetText("")
				if err != nil {
					kit.ShowError(err, m.w)
					return
				}
				m.u.showLogSheet(m.w, name, out)
			})
		}()
	}, nil)
}

// showLogSheet — журнал контейнера в нижней панели (дизайн: диалог «Журнал · имя» у нижнего края).
func (u *ui) showLogSheet(win fyne.Window, name, text string) *kit.Sheet {
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		text = L("The log is empty.", "Журнал пуст.")
	}
	var sheet *kit.Sheet
	closeBtn := container.NewGridWrap(fyne.NewSize(36, 36), kit.NewTapCardR(container.NewCenter(kit.IconImg("close", kit.ColText, 18)), kit.ColCard2, kit.ColCard2Hover, 11, 1, func() {
		if sheet != nil {
			sheet.Close()
		}
	}))
	copyBtn := container.NewGridWrap(fyne.NewSize(36, 36), kit.NewTapCardR(container.NewCenter(kit.IconImg("copy", kit.ColText, 18)), kit.ColCard2, kit.ColCard2Hover, 11, 1, func() {
		u.app.Clipboard().SetContent(text)
	}))
	head := container.NewBorder(nil, nil, nil, container.New(kit.Hrow{Gap: 8}, copyBtn, closeBtn),
		kit.Vcenter(kit.NewFlowText(fmt.Sprintf(L("Log · %s", "Журнал · %s"), name), 16, kit.ColText, true, 22)))
	logBG := canvas0()
	logBox := container.NewStack(logBG, container.New(layout.NewCustomPaddedLayout(12, 12, 12, 12), kit.NewFlowText(text, 12, kit.ColText, false, 18)))
	content := container.New(layout.NewCustomPaddedLayout(6, 6, 12, 12), container.New(kit.VboxFit{Gap: 10}, head, kit.ScrollMax(logBox, 300)))
	sheet = kit.ShowSheet(win, content)
	return sheet
}

func (m *manager) restartContainer(name string) {
	m.ensureSession(func() {
		if m.dockerBusy == nil {
			m.dockerBusy = map[string]bool{}
		}
		m.dockerBusy[name] = true
		m.render()
		s := m.sess
		go func() {
			err := s.RestartContainer(name)
			fyne.Do(func() {
				delete(m.dockerBusy, name)
				if err != nil {
					m.status.SetText(fmt.Sprintf(L("Failed to restart %s", "Не удалось перезапустить %s"), name))
					kit.ShowError(err, m.w)
				} else {
					m.status.SetText(fmt.Sprintf(L("%s restarted", "%s перезапущен"), name))
				}
				m.docker = nil
				m.loadDocker()
				m.render()
			})
		}()
	}, nil)
}

// adminPanelRow — переключатель «Админ-панель (veil-admin)»: включение и выключение — после подтверждения.
func (m *manager) adminPanelRow(d *dockerInfo) fyne.CanvasObject {
	title := L("Admin panel (veil-admin)", "Админ-панель (veil-admin)")
	if !d.adminAvail {
		row, sw, _ := switchRowText(title, L("Available when Veil runs in Docker; with a system service the panel stays built into veil-server.", "Доступна, когда Veil работает в Docker; при системной службе панель встроена в veil-server."), false, nil)
		sw.SetDisabled(true)
		return row
	}
	sub := func(on bool) string {
		if on {
			return L("Running: https://10.66.0.1/admin, only over VPN", "Работает: https://10.66.0.1/admin, только через VPN")
		}
		return L("Off: /admin answers 404, like a page that does not exist", "Выключена: /admin отвечает 404, как несуществующая страница")
	}
	if m.dockerBusy["veil-admin"] {
		row, sw, _ := switchRowText(title, L("Applying…", "Применяется…"), d.adminOn, nil)
		sw.SetDisabled(true)
		return row
	}
	var sw *kit.SwitchWidget
	var row fyne.CanvasObject
	row, sw, _ = switchRowText(title, sub(d.adminOn), d.adminOn, func(on bool) {
		sw.SetOn(!on) // состояние меняется только после подтверждения
		t, msg, ok := L("Turn the admin panel on?", "Включить админ-панель?"),
			L("The veil-admin container will start on the server. The panel will open only over VPN: https://10.66.0.1/admin.", "На сервере запустится контейнер veil-admin. Панель будет открываться только через VPN: https://10.66.0.1/admin."),
			L("Turn on", "Включить")
		if !on {
			t, msg, ok = L("Turn the admin panel off?", "Выключить админ-панель?"),
				L("The veil-admin container will be stopped and removed; /admin will answer 404. Management from the app keeps working.", "Контейнер veil-admin будет остановлен и удалён; /admin начнёт отвечать 404. Управление из приложения продолжит работать."),
				L("Turn off", "Выключить")
		}
		dlg := kit.NewCustomConfirm(t, ok, L("Cancel", "Отмена"), container.NewGridWrap(fyne.NewSize(320, 120), kit.NewFlowText(msg, 13, kit.ColMuted, false, 19)), func(yes bool) {
			if yes {
				m.setAdminPanel(on)
			}
		}, m.w)
		dlg.Show()
	})
	return row
}

func (m *manager) setAdminPanel(on bool) {
	m.ensureSession(func() {
		if m.dockerBusy == nil {
			m.dockerBusy = map[string]bool{}
		}
		m.dockerBusy["veil-admin"] = true
		m.render()
		s := m.sess
		go func() {
			err := s.SetAdminPanel(on, func(f string, a ...any) {
				line := fmt.Sprintf(f, a...)
				fyne.Do(func() { m.status.SetText(line) })
			})
			fyne.Do(func() {
				delete(m.dockerBusy, "veil-admin")
				if err != nil {
					m.status.SetText(L("Failed to change the admin panel", "Не удалось изменить админ-панель"))
					kit.ShowError(err, m.w)
				} else if on {
					m.status.SetText(L("Admin panel is on", "Админ-панель включена"))
				} else {
					m.status.SetText(L("Admin panel is off", "Админ-панель выключена"))
				}
				m.docker = nil
				m.loadDocker()
				m.render()
			})
		}()
	}, nil)
}

// renderDocker — блок контейнеров: без входа по SSH — строка входа, пока читается — подпись.
func (m *manager) renderDocker() {
	d := m.docker
	if d == nil {
		if m.sess == nil && m.p.SSH == nil {
			// профиль администратора без SSH: настройки — по туннелю, а Docker на сервере доступен только по SSH
			m.body.Add(kit.ListRow(L("Containers and Docker", "Контейнеры и Docker"), L("Available only to the app that installed the server (over SSH)", "Доступно только приложению, которое установило сервер (по SSH)"), nil, nil))
			return
		}
		if m.sess == nil {
			m.body.Add(kit.NavRow(L("Containers and Docker", "Контейнеры и Docker"), L("Docker version, container logs and restarts, admin panel — requires SSH sign-in", "Версия Docker, журналы и перезапуск контейнеров, админ-панель — понадобится вход по SSH"), func() {
				m.ensureSession(func() {
					m.loadDocker()
					m.render()
				}, nil)
			}))
			return
		}
		m.loadDocker()
		m.body.Add(kit.SectionLabel(L("Reading containers on the server…", "Читаем контейнеры на сервере…")))
		return
	}
	line := L("Docker is not installed — Veil runs as a system service", "Docker не установлен — Veil работает системной службой")
	if d.version != "" {
		line = fmt.Sprintf(L("Docker %s · containers: %d · /opt/veil", "Docker %s · контейнеров: %d · /opt/veil"), d.version, len(d.list))
	}
	m.body.Add(container.New(layout.NewCustomPaddedLayout(0, 0, 4, 0), kit.NewFlowText(line, 12, kit.ColMuted, true, 17)))
	if d.err != nil {
		m.body.Add(noteIcon("warn", L("Failed to list containers: ", "Не удалось получить список контейнеров: ")+d.err.Error(), kit.ColAccentInk))
	}
	list := container.New(kit.VboxFit{Gap: 6})
	for _, c := range d.list {
		list.Add(m.containerRow(c))
	}
	if len(d.list) > 0 {
		m.body.Add(list)
	}
	m.body.Add(m.adminPanelRow(d))
}

// renderManagement — вкладка «Управление».
func (m *manager) renderManagement() {
	m.renderDocker()
	needSSH := func(f func()) func() { return func() { m.ensureSession(f, nil) } }
	hasSSH := m.p.SSH != nil // без SSH (профиль администратора) — только то, что идёт через админ-API по туннелю
	if hasSSH {
		m.body.Add(newAutoGrid(200,
			bigButton(L("Update server", "Обновить сервер"), "download", false, m.updateServer),
			bigButton(L("Update packages", "Обновить пакеты"), "refresh", false, needSSH(m.updatePackages)),
			bigButtonDanger(L("Uninstall Veil from server", "Удалить Veil с сервера"), "trash", needSSH(m.uninstall)),
		))
	}

	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	m.body.Add(kit.SectionLabel(L("More", "Ещё")))
	m.body.Add(kit.ListRow(L("Cover site", "Сайт-прикрытие"), L("Repository, branch, status", "Репозиторий, ветка, статус"), arrow(), func() { m.ready(m.loadCoverSite) }))
	m.body.Add(kit.ListRow(L("Check server", "Проверить сервер"), L("Refresh version, client and container information", "Обновить сведения о версии, клиентах и контейнерах"), arrow(), func() {
		m.ready(func() {
			m.docker = nil
			m.refresh()
		})
	}))
	if !hasSSH {
		m.renderRemoveProfile(arrow)
		return
	}
	m.body.Add(kit.ListRowColored(L("Reboot server", "Перезагрузить сервер"), "", theme.ColorNameError, arrow(), needSSH(func() {
		kit.ShowConfirm(L("Reboot", "Перезагрузка"), L("The server will be rebooted; VPN is unavailable during the reboot. Continue?", "Сервер будет перезагружен, VPN на время перезагрузки недоступен. Продолжить?"), func(ok bool) {
			if !ok {
				return
			}
			err := m.sess.Reboot()
			m.u.dropSession(m.p.Name)
			m.sess = nil
			m.docker = nil
			if err != nil {
				kit.ShowError(err, m.w)
				return
			}
			m.status.SetText(L("Server is rebooting…", "Сервер перезагружается…"))
		}, m.w)
	})))
	m.renderRemoveProfile(arrow)
}

// renderRemoveProfile — строка «Удалить сервер из приложения»: стирает только профиль на этом устройстве.
func (m *manager) renderRemoveProfile(arrow func() fyne.CanvasObject) {
	m.body.Add(kit.ListRowColored(L("Remove server from the app", "Удалить сервер из приложения"), "", theme.ColorNameError, arrow(), func() {
		kit.ShowConfirm(L("Remove server", "Удалить сервер"), fmt.Sprintf(L("Profile “%s” will be deleted from this device. The server itself will not change. Continue?", "Профиль «%s» будет удалён с этого устройства. Сам сервер не изменится. Продолжить?"), m.p.Name), func(ok bool) {
			if !ok {
				return
			}
			if m.p.Name == m.u.active {
				kit.ShowError(errors.New(L("disconnect the VPN first", "сначала отключите VPN")), m.w)
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
}

func (m *manager) updatePackages() {
	kit.ShowConfirm(L("Update packages", "Обновить пакеты"), L("System packages will be updated on the server. This may take a few minutes. Continue?", "На сервере будет запущено обновление системных пакетов. Это может занять несколько минут. Продолжить?"), func(ok bool) {
		if !ok {
			return
		}
		m.status.SetText(L("Updating server packages…", "Обновление пакетов сервера…"))
		s := m.sess
		go func() {
			err := s.UpdatePackages(context.Background())
			fyne.Do(func() {
				if err != nil {
					m.status.SetText(L("Failed to update packages", "Не удалось обновить пакеты"))
					kit.ShowError(err, m.w)
					return
				}
				m.status.SetText(L("Server packages updated", "Пакеты сервера обновлены"))
			})
		}()
	}, m.w)
}

func (m *manager) uninstall() {
	kit.ShowConfirm(L("Uninstall Veil", "Удалить Veil"), L("Veil will be stopped and removed from the server (with its containers); all its clients will stop connecting. Continue?", "Veil будет остановлен и удалён с сервера (вместе с контейнерами), все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
		if !ok {
			return
		}
		m.status.SetText(L("Removing Veil from the server…", "Удаляем Veil с сервера…"))
		s := m.sess
		go func() {
			err := s.Uninstall()
			fyne.Do(func() {
				if err != nil {
					m.status.SetText("")
					kit.ShowError(err, m.w)
					return
				}
				m.docker = nil
				m.status.SetText(L("Veil removed from server", "Veil удалён с сервера"))
				kit.ShowInfo(L("Done", "Готово"), L("Veil has been removed from the server. The profile remains in the app: delete it separately.", "Veil удалён с сервера. Профиль в приложении остался: удалите его отдельно."), m.w)
			})
		}()
	}, m.w)
}
