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

	"veil/internal/app/gui/kit"
	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/vpnconf/ovpnconf"
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
	w := u.newSubPage(L("Install OpenVPN", "Установка OpenVPN"))
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
	status := kit.Prose("")
	setStatus := func(s string) {
		status.Segments[0].(*widget.TextSegment).Text = s
		status.Refresh()
	}
	bar := kit.NewThinBar()
	barBox := container.New(kit.LayoutPadLeft(), bar)
	barBox.Hide()
	install := widget.NewButton(L("Install", "Установить"), nil)
	install.Importance = widget.HighImportance
	install.OnTapped = func() {
		n, err := strconv.Atoi(strings.TrimSpace(port.Text))
		if err != nil || n < 1 || n > 65535 {
			kit.ShowError(errors.New(L("port must be a number from 1 to 65535", "порт должен быть числом от 1 до 65535")), w)
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
				setStatus(L("Install failed: ", "Не удалось установить: ") + shortErr(err))
				kit.ShowError(err, w)
			}
		}
		m.ensureSession(func() {
			barBox.Show()
			bar.Start()
			setStatus(L("Installing…", "Установка…"))
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
						fail(errors.New(L("failed to save profile", "не удалось сохранить профиль")))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.showOVShare(L("OpenVPN client", "Клиент OpenVPN"), cfg)
				})
			}()
		}, fail)
	}
	body := container.New(kit.VboxFit{},
		note(L("OpenVPN is the most popular VPN protocol with flexible settings; its own SSL/TLS protection. It is installed on the server over SSH; to connect from this app, OpenVPN must be installed on the computer (openvpn.net/community-downloads).", "OpenVPN — самый популярный VPN-протокол с гибкими настройками; своя защита на SSL/TLS. Устанавливается на сервер по SSH; для подключения из этого приложения на компьютере должен быть установлен OpenVPN (openvpn.net/community-downloads).")),
		kit.Lead(kit.Text(L("Network protocol", "Сетевой протокол"), 12, kit.ColMuted, false)), kit.Lead(proto),
		kit.Lead(kit.Text(L("Port", "Порт"), 12, kit.ColMuted, false)), port,
		note(L("UDP is faster; TCP on port 443 more often passes through restrictive networks.", "UDP быстрее; TCP на порту 443 чаще проходит через ограничивающие сети.")),
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
		kit.ShowError(err, u.win)
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
				kit.ShowError(errors.New(L("the profile of the server OpenVPN was installed from was not found: management unavailable", "профиль сервера, с которого ставился OpenVPN, не найден: управление недоступно")), w)
				return
			}
			u.withSession(w, srv, func(s *deploy.Session, err error) {
				if err != nil {
					if err.Error() != "отменено" {
						kit.ShowError(err, w)
					}
					return
				}
				f(s, srv)
			})
		}
	}
	rows := container.New(kit.VboxFit{},
		kit.ListRow(L("This device's .ovpn file", "Файл .ovpn этого устройства"), L("For the OpenVPN app on another device", "Для приложения OpenVPN на другом устройстве"), arrow(), func() { u.showOVShare(p.Name, p.OVPN) }),
		kit.ListRow(L("Add client", "Добавить клиента"), L("Issue a certificate and .ovpn file for another device", "Выпустить сертификат и файл .ovpn для другого устройства"), arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			name := widget.NewEntry()
			name.SetText(L("Client", "Клиент"))
			kit.ShowForm(L("New OpenVPN client", "Новый клиент OpenVPN"), L("Create", "Создать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("Name", "Имя"), name)}, func(ok bool) {
				if !ok {
					return
				}
				go func() {
					cfg, err := s.OpenVPNAddClient(name.Text, serverHost(srv))
					fyne.Do(func() {
						if err != nil {
							kit.ShowError(err, w)
							return
						}
						u.showOVShare(fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), strings.TrimSpace(name.Text)), cfg)
					})
				}()
			}, w)
		})),
		kit.ListRow(L("Clients on the server", "Клиенты на сервере"), L("List of certificates; revoke access", "Список сертификатов; отзыв доступа"), arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			go func() {
				names, err := s.OpenVPNClients()
				fyne.Do(func() {
					if err != nil {
						kit.ShowError(err, w)
						return
					}
					u.showOVClients(s, names)
				})
			}()
		})),
		kit.ListRowColored(L("Uninstall OpenVPN from server", "Удалить OpenVPN с сервера"), L("Stop the service and delete the certificates and settings on the server", "Остановить службу и удалить сертификаты и настройки на сервере"), "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			kit.ShowConfirm(L("Uninstall OpenVPN", "Удалить OpenVPN"), L("OpenVPN will be stopped on the server; all its clients will stop connecting. Continue?", "OpenVPN будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveOpenVPN(); err != nil {
					kit.ShowError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				kit.ShowInfo(L("Done", "Готово"), L("OpenVPN has been removed from the server and the profile deleted from the app.", "OpenVPN удалён с сервера, профиль удалён из приложения."), u.win)
			}, w)
		})),
		kit.ListRowColored(L("Delete profile from the app", "Удалить профиль из приложения"), L("The server will not change", "Сервер не изменится"), "error", arrow(), func() {
			kit.ShowConfirm(L("Delete profile", "Удалить профиль"), fmt.Sprintf(L("Profile “%s” will be deleted from this device. The server will not change. Continue?", "Профиль «%s» будет удалён с этого устройства. Сервер не изменится. Продолжить?"), p.Name), func(ok bool) {
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
	w := u.newSubPage(L("OpenVPN clients", "Клиенты OpenVPN"))
	box := container.NewVBox()
	if len(names) == 0 {
		box.Add(note(L("No clients have been issued.", "Выпущенных клиентов нет.")))
	}
	for _, n := range names {
		n := n
		box.Add(kit.ListRow(n, L("Tap to revoke access", "Нажмите, чтобы отозвать доступ"), widget.NewIcon(theme.DeleteIcon()), func() {
			kit.ShowConfirm(L("Revoke client", "Отозвать клиента"), fmt.Sprintf(L("Certificate “%s” will be revoked: this device will no longer be able to connect. Continue?", "Сертификат «%s» будет отозван: это устройство больше не сможет подключиться. Продолжить?"), n), func(ok bool) {
				if !ok {
					return
				}
				go func() {
					err := s.OpenVPNRevoke(n)
					fyne.Do(func() {
						if err != nil {
							kit.ShowError(err, w)
							return
						}
						w.Close()
						kit.ShowInfo(L("Done", "Готово"), fmt.Sprintf(L("Access for client “%s” has been revoked. The existing connection will last until reconnect.", "Доступ клиента «%s» отозван. Уже установленное соединение продлится до переподключения."), n), u.win)
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
	copyBtn := widget.NewButtonWithIcon(L("Copy", "Скопировать"), theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	saveBtn := widget.NewButtonWithIcon(L(".ovpn file…", "Файл .ovpn…"), theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write([]byte(text)); err != nil {
				kit.ShowError(err, w)
			}
		}, w)
		fd.SetFileName("veil-openvpn.ovpn")
		fd.Show()
	})
	body := container.New(kit.VboxFit{},
		note(L("Import the file into the OpenVPN app or save it. The file contains a private key: do not show it to others.", "Импортируйте файл в приложение OpenVPN или сохраните его. В файле закрытый ключ: не показывайте его посторонним.")),
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
