package gui

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/ovpnconf"
)

// ovProfileFor — профиль OpenVPN, поставленный с этого сервера (nil, если не ставился).
func (u *ui) ovProfileFor(srv *config.Profile) *config.Profile {
	for _, p := range u.profiles {
		if p.Protocol == "openvpn" && p.ServerRef == srv.Name {
			return p
		}
	}
	return nil
}

// ---------- установка OpenVPN ----------

func (u *ui) openOVInstall(m *manager) {
	w := u.newSubPage("Установка OpenVPN")
	port := widget.NewEntry()
	port.SetText("1194")
	proto := widget.NewRadioGroup([]string{"UDP", "TCP"}, nil)
	proto.Horizontal = true
	proto.SetSelected("UDP")
	proto.OnChanged = func(s string) {
		// стандартные порты: 1194 для UDP и 443 для TCP — меняем, только если порт не правили
		switch {
		case s == "TCP" && port.Text == "1194":
			port.SetText("443")
		case s == "UDP" && port.Text == "443":
			port.SetText("1194")
		}
	}
	status := prose("")
	setStatus := func(s string) {
		status.Segments[0].(*widget.TextSegment).Text = s
		status.Refresh()
	}
	bar := newThinBar()
	barBox := container.New(layoutPadLeft(), bar)
	barBox.Hide()
	install := widget.NewButton("Установить", nil)
	install.Importance = widget.HighImportance
	install.OnTapped = func() {
		n, err := strconv.Atoi(strings.TrimSpace(port.Text))
		if err != nil || n < 1 || n > 65535 {
			showError(errors.New("порт должен быть числом от 1 до 65535"), w)
			return
		}
		pr := strings.ToLower(proto.Selected)
		install.Disable()
		port.Disable()
		proto.Disable()
		fail := func(err error) {
			barBox.Hide()
			bar.Stop()
			install.Enable()
			port.Enable()
			proto.Enable()
			if err != nil && err.Error() != "отменено" {
				setStatus("Не удалось установить: " + shortErr(err))
				showError(err, w)
			}
		}
		m.ensureSession(func() {
			barBox.Show()
			bar.Start()
			setStatus("Установка…")
			go func() {
				cfg, err := m.sess.InstallOpenVPN(deploy.OVInstall{Port: n, Proto: pr}, serverHost(m.p), func(f string, a ...any) {
					line := fmt.Sprintf(f, a...)
					fyne.Do(func() { setStatus(line) })
				})
				fyne.Do(func() {
					if err != nil {
						fail(err)
						return
					}
					p := u.saveOVProfile(m.p, "", cfg)
					if p == nil {
						fail(errors.New("не удалось сохранить профиль"))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.showOVShare("Клиент OpenVPN", cfg)
				})
			}()
		}, fail)
	}
	body := container.NewVBox(
		note("OpenVPN — самый популярный VPN-протокол с гибкими настройками; своя защита на SSL/TLS. Устанавливается на сервер по SSH; для подключения из этого приложения на компьютере должен быть установлен OpenVPN (openvpn.net/community-downloads)."),
		lead(text("Сетевой протокол", 12, colMuted, false)), lead(proto),
		lead(text("Порт", 12, colMuted, false)), port,
		note("UDP быстрее; TCP на порту 443 чаще проходит через ограничивающие сети."),
		status, barBox,
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, install, nil, nil, scrolled(body))))
	w.Show()
}

// saveOVProfile сохраняет профиль OpenVPN для сервера srv; label — суффикс имени.
func (u *ui) saveOVProfile(srv *config.Profile, label string, cfg *ovpnconf.Config) *config.Profile {
	base := srv.Name + " · OpenVPN"
	if label != "" {
		base += " · " + label
	}
	p := &config.Profile{Name: u.uniqueProfileName(base), Protocol: "openvpn", OVPN: cfg, ServerRef: srv.Name}
	setProfileHost(p, cfg)
	if err := saveProfile(p); err != nil {
		showError(err, u.win)
		return nil
	}
	u.reload()
	return p
}

func setProfileHost(p *config.Profile, cfg *ovpnconf.Config) {
	if h, _, _, err := cfg.Remote(); err == nil {
		if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil && ip.To4() == nil {
			p.IPv6 = ip.String()
		} else {
			p.IPv4 = h
		}
	}
}

// ---------- управление установленным OpenVPN ----------

func (u *ui) openOVManage(p *config.Profile) {
	w := u.newSubPage("OpenVPN")
	srv := u.profileByName(p.ServerRef)
	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	needServer := func(f func(s *deploy.Session, srv *config.Profile)) func() {
		return func() {
			if srv == nil || srv.SSH == nil {
				showError(errors.New("профиль сервера, с которого ставился OpenVPN, не найден: управление недоступно"), w)
				return
			}
			u.withSession(w, srv, func(s *deploy.Session, err error) {
				if err != nil {
					if err.Error() != "отменено" {
						showError(err, w)
					}
					return
				}
				f(s, srv)
			})
		}
	}
	rows := container.NewVBox(
		listRow("Файл .ovpn этого устройства", "Для приложения OpenVPN на другом устройстве", arrow(), func() { u.showOVShare(p.Name, p.OVPN) }),
		listRow("Добавить клиента", "Выпустить сертификат и файл .ovpn для другого устройства", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			name := widget.NewEntry()
			name.SetText("Клиент")
			showForm("Новый клиент OpenVPN", "Создать", "Отмена", []*widget.FormItem{widget.NewFormItem("Имя", name)}, func(ok bool) {
				if !ok {
					return
				}
				go func() {
					cfg, err := s.OpenVPNAddClient(name.Text, serverHost(srv))
					fyne.Do(func() {
						if err != nil {
							showError(err, w)
							return
						}
						u.showOVShare("Клиент «"+strings.TrimSpace(name.Text)+"»", cfg)
					})
				}()
			}, w)
		})),
		listRow("Клиенты на сервере", "Список сертификатов; отзыв доступа", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			go func() {
				names, err := s.OpenVPNClients()
				fyne.Do(func() {
					if err != nil {
						showError(err, w)
						return
					}
					u.showOVClients(s, names)
				})
			}()
		})),
		listRowColored("Удалить OpenVPN с сервера", "Остановить службу и удалить сертификаты и настройки на сервере", "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			showConfirm("Удалить OpenVPN", "OpenVPN будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?", func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveOpenVPN(); err != nil {
					showError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				showInfo("Готово", "OpenVPN удалён с сервера, профиль удалён из приложения.", u.win)
			}, w)
		})),
		listRowColored("Удалить профиль из приложения", "Сервер не изменится", "error", arrow(), func() {
			showConfirm("Удалить профиль", "Профиль «"+p.Name+"» будет удалён с этого устройства. Сервер не изменится. Продолжить?", func(ok bool) {
				if ok {
					u.deleteProfile(p)
					w.Close()
				}
			}, w)
		}),
	)
	w.SetContent(container.NewPadded(scrolled(rows)))
	w.Show()
}

func (u *ui) showOVClients(s *deploy.Session, names []string) {
	w := u.newSubPage("Клиенты OpenVPN")
	box := container.NewVBox()
	if len(names) == 0 {
		box.Add(note("Выпущенных клиентов нет."))
	}
	for _, n := range names {
		n := n
		box.Add(listRow(n, "Нажмите, чтобы отозвать доступ", widget.NewIcon(theme.DeleteIcon()), func() {
			showConfirm("Отозвать клиента", "Сертификат «"+n+"» будет отозван: это устройство больше не сможет подключиться. Продолжить?", func(ok bool) {
				if !ok {
					return
				}
				go func() {
					err := s.OpenVPNRevoke(n)
					fyne.Do(func() {
						if err != nil {
							showError(err, w)
							return
						}
						w.Close()
						showInfo("Готово", "Доступ клиента «"+n+"» отозван. Уже установленное соединение продлится до переподключения.", u.win)
					})
				}()
			}, w)
		}))
	}
	w.SetContent(container.NewPadded(scrolled(box)))
	w.Show()
}

// showOVShare показывает файл .ovpn: текст, копирование и сохранение.
func (u *ui) showOVShare(title string, cfg *ovpnconf.Config) {
	w := u.newSubPage(title)
	text := cfg.String()
	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.Wrapping = fyne.TextWrapBreak
	entry.SetMinRowsVisible(10)
	copyBtn := widget.NewButtonWithIcon("Скопировать", theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	saveBtn := widget.NewButtonWithIcon("Файл .ovpn…", theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write([]byte(text)); err != nil {
				showError(err, w)
			}
		}, w)
		fd.SetFileName("veil-openvpn.ovpn")
		fd.Show()
	})
	body := container.NewVBox(
		note("Импортируйте файл в приложение OpenVPN или сохраните его. В файле закрытый ключ: не показывайте его посторонним."),
		entry,
		container.NewGridWithColumns(2, copyBtn, saveBtn),
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}

// checkOpenVPNInstalled — проверка до запроса прав администратора.
func checkOpenVPNInstalled(p *config.Profile) error {
	if p.Protocol != "openvpn" {
		return nil
	}
	_, err := client.OpenVPNPath()
	return err
}
