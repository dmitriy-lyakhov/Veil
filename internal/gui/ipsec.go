package gui

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/ipsecconf"
)

// IPsec/IKEv2: установка strongSwan на сервер, выдача данных клиентам, подключение системным VPN Windows.

// ipsecProfileFor — профиль IPsec, поставленный с этого сервера (nil, если не ставился).
func (u *ui) ipsecProfileFor(srv *config.Profile) *config.Profile {
	for _, p := range u.profiles {
		if p.Protocol == "ipsec" && p.ServerRef == srv.Name {
			return p
		}
	}
	return nil
}

// ipsecRow — правая часть и действие пункта IPsec в списке протоколов.
func (m *manager) ipsecRow() (fyne.CanvasObject, func()) {
	if p := m.u.ipsecProfileFor(m.p); p != nil {
		return widget.NewIcon(theme.NavigateNextIcon()), func() { m.u.openIPsecManage(p) }
	}
	return widget.NewIcon(theme.DownloadIcon()), func() { m.u.openIPsecInstall(m) }
}

// ---------- установка ----------

func (u *ui) openIPsecInstall(m *manager) {
	w := u.newSubPage(L("Install IPsec", "Установка IPsec"))
	status := prose("")
	setStatus := func(s string) {
		status.Segments[0].(*widget.TextSegment).Text = s
		status.Refresh()
	}
	bar := newThinBar()
	barBox := container.New(layoutPadLeft(), bar)
	barBox.Hide()
	install := widget.NewButton(L("Install", "Установить"), nil)
	install.Importance = widget.HighImportance
	install.OnTapped = func() {
		install.Disable()
		fail := func(err error) {
			barBox.Hide()
			bar.Stop()
			install.Enable()
			if err != nil && err.Error() != "отменено" {
				setStatus(L("Install failed: ", "Не удалось установить: ") + shortErr(err))
				showError(err, w)
			}
		}
		m.ensureSession(func() {
			barBox.Show()
			bar.Start()
			setStatus(L("Installing…", "Установка…"))
			go func() {
				cfg, err := m.sess.InstallIPsec(serverHost(m.p), func(f string, a ...any) {
					line := fmt.Sprintf(f, a...)
					fyne.Do(func() { setStatus(line) })
				})
				fyne.Do(func() {
					if err != nil {
						fail(err)
						return
					}
					p := u.saveIPsecProfile(m.p, "", cfg)
					if p == nil {
						fail(errors.New(L("failed to save profile", "не удалось сохранить профиль")))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.askTrustCA(p, func() { u.showIPsecShare(L("IPsec client", "Клиент IPsec"), cfg) })
				})
			}()
		}, fail)
	}
	body := container.NewVBox(
		note(L("IPsec/IKEv2 is a modern, stable protocol that restores the connection after signal loss and is built into Windows, Android and iOS. strongSwan with its own certificate authority is installed on the server over SSH; afterwards you can connect to it from this app (Windows) or with the phone's built-in VPN.", "IPsec/IKEv2 — современный стабильный протокол, восстанавливает соединение после потери сигнала и встроен в Windows, Android и iOS. На сервер по SSH устанавливается strongSwan со своим центром сертификации; затем к нему можно подключиться из этого приложения (Windows) или встроенным VPN телефона.")),
		status, barBox,
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, install, nil, nil, scrolled(body))))
	w.Show()
}

// saveIPsecProfile сохраняет профиль IPsec для сервера srv; label — суффикс имени.
func (u *ui) saveIPsecProfile(srv *config.Profile, label string, cfg *ipsecconf.Config) *config.Profile {
	base := srv.Name + " · IPsec"
	if label != "" {
		base += " · " + label
	}
	p := &config.Profile{Name: u.uniqueProfileName(base), Protocol: "ipsec", IPsec: cfg, ServerRef: srv.Name}
	if ip := net.ParseIP(cfg.Server); ip != nil && ip.To4() == nil {
		p.IPv6 = ip.String()
	} else {
		p.IPv4 = cfg.Server
	}
	if err := saveProfile(p); err != nil {
		showError(err, u.win)
		return nil
	}
	u.reload()
	return p
}

// askTrustCA спрашивает разрешения добавить сертификат центра в доверенные корневые Windows (только этот компьютер).
func (u *ui) askTrustCA(p *config.Profile, then func()) {
	showConfirm(L("Certificate authority certificate", "Сертификат центра сертификации"),
		L("To connect to the server from this computer, the Veil certificate authority certificate must be added to the Windows trusted root certificates (computer store).", "Чтобы подключаться к серверу с этого компьютера, сертификат центра сертификации Veil нужно добавить в доверенные корневые сертификаты Windows (хранилище компьютера). ")+
			L("It is only used to verify your server, but any trusted authority can issue certificates for any site, so add it only if the server is yours. Add it when connecting?", "Он нужен только для проверки вашего сервера, но любой доверенный центр может выпускать сертификаты для любых сайтов, поэтому добавляйте его только если сервер ваш. Добавить при подключении?"),
		func(ok bool) {
			if ok {
				p.IPsec.TrustCA = true
				if err := saveProfile(p); err != nil {
					showError(err, u.win)
				}
			}
			if then != nil {
				then()
			}
		}, u.win)
}

// ---------- управление ----------

func (u *ui) openIPsecManage(p *config.Profile) {
	w := u.newSubPage("IPsec")
	srv := u.profileByName(p.ServerRef)
	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	needServer := func(f func(s *deploy.Session, srv *config.Profile)) func() {
		return func() {
			if srv == nil || srv.SSH == nil {
				showError(errors.New(L("the profile of the server IPsec was installed from was not found: management unavailable", "профиль сервера, с которого ставился IPsec, не найден: управление недоступно")), w)
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
	addClient := func(cert bool) func() {
		return needServer(func(s *deploy.Session, srv *config.Profile) {
			name := widget.NewEntry()
			name.SetText(L("Client", "Клиент"))
			showForm(L("New IPsec client", "Новый клиент IPsec"), L("Create", "Создать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("Name", "Имя"), name)}, func(ok bool) {
				if !ok {
					return
				}
				go func() {
					var cfg *ipsecconf.Config
					var err error
					if cert {
						cfg, err = s.IPsecAddCertClient(name.Text, serverHost(srv))
					} else {
						cfg, err = s.IPsecAddClient(name.Text, serverHost(srv))
					}
					fyne.Do(func() {
						if err != nil {
							showError(err, w)
							return
						}
						u.showIPsecShare(fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), strings.TrimSpace(name.Text)), cfg)
					})
				}()
			}, w)
		})
	}
	trustSub := func() string {
		if p.IPsec != nil && p.IPsec.TrustCA {
			return L("Allowed: the authority certificate will be added to the Windows trusted roots when connecting", "Разрешено: при подключении сертификат центра будет добавлен в доверенные корневые Windows")
		}
		return L("Not allowed: connecting from this computer will fail until the certificate is added to the trusted ones", "Не разрешено: подключение с этого компьютера не удастся, пока сертификат не добавлен в доверенные")
	}
	trustRow := listRow(L("Trust the authority certificate on this computer", "Доверие сертификату центра на этом компьютере"), trustSub(), arrow(), func() {
		u.askTrustCA(p, func() { w.Close(); u.openIPsecManage(p) })
	})
	rows := container.NewVBox(
		listRow(L("This device's details", "Данные этого устройства"), L("Address, login, password, authority certificate and instructions for Windows, Android and iOS", "Адрес, логин, пароль, сертификат центра и инструкция для Windows, Android и iOS"), arrow(), func() { u.showIPsecShare(p.Name, p.IPsec) }),
		trustRow,
		listRow(L("Add client (login and password)", "Добавить клиента (логин и пароль)"), L("Issue a new login and password for another device", "Выдать новый логин и пароль для другого устройства"), arrow(), addClient(false)),
		listRow(L("Add client (certificate)", "Добавить клиента (сертификат)"), L("Issue a client certificate (.p12); it cannot be revoked individually", "Выпустить сертификат клиента (.p12); отозвать его по отдельности нельзя"), arrow(), addClient(true)),
		listRow(L("Clients on the server", "Клиенты на сервере"), L("List of logins; delete a client", "Список логинов; удаление клиента"), arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			go func() {
				users, err := s.IPsecUsers()
				fyne.Do(func() {
					if err != nil {
						showError(err, w)
						return
					}
					u.showIPsecClients(s, users)
				})
			}()
		})),
		listRowColored(L("Uninstall IPsec from server", "Удалить IPsec с сервера"), L("Stop the service and delete the settings, certificates and all clients on the server", "Остановить службу и удалить настройки, сертификаты и всех клиентов на сервере"), "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			showConfirm(L("Uninstall IPsec", "Удалить IPsec"), L("IPsec will be stopped on the server; all its clients will stop connecting. Continue?", "IPsec будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveIPsec(); err != nil {
					showError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				showInfo(L("Done", "Готово"), L("IPsec has been removed from the server and the profile deleted from the app.", "IPsec удалён с сервера, профиль удалён из приложения."), u.win)
			}, w)
		})),
		listRowColored(L("Delete profile from the app", "Удалить профиль из приложения"), L("The server will not change", "Сервер не изменится"), "error", arrow(), func() {
			showConfirm(L("Delete profile", "Удалить профиль"), fmt.Sprintf(L("Profile “%s” will be deleted from this device. The server will not change. Continue?", "Профиль «%s» будет удалён с этого устройства. Сервер не изменится. Продолжить?"), p.Name), func(ok bool) {
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

func (u *ui) showIPsecClients(s *deploy.Session, users []string) {
	w := u.newSubPage(L("IPsec clients", "Клиенты IPsec"))
	box := container.NewVBox()
	if len(users) == 0 {
		box.Add(note(L("There are no login-and-password clients on the server.", "Клиентов с логином и паролем на сервере нет.")))
	}
	for _, name := range users {
		name := name
		box.Add(listRowColored(name, L("Tap to delete the client from the server", "Нажмите, чтобы удалить клиента с сервера"), "error", widget.NewIcon(theme.DeleteIcon()), func() {
			showConfirm(L("Delete client", "Удалить клиента"), fmt.Sprintf(L("Client “%s” will no longer be able to connect. Continue?", "Клиент «%s» больше не сможет подключаться. Продолжить?"), name), func(ok bool) {
				if !ok {
					return
				}
				go func() {
					err := s.IPsecRemoveClient(name)
					fyne.Do(func() {
						if err != nil {
							showError(err, w)
							return
						}
						w.Close()
						go func() {
							left, err := s.IPsecUsers()
							fyne.Do(func() {
								if err == nil {
									u.showIPsecClients(s, left)
								}
							})
						}()
					})
				}()
			}, w)
		}))
	}
	w.SetContent(container.NewPadded(scrolled(box)))
	w.Show()
}

// showIPsecShare показывает данные клиента IPsec: текст с инструкцией, копирование и сохранение файлов.
func (u *ui) showIPsecShare(title string, cfg *ipsecconf.Config) {
	w := u.newSubPage(title)
	text := cfg.Instructions()
	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.Wrapping = fyne.TextWrapWord
	entry.SetMinRowsVisible(12)
	save := func(name string, data []byte) {
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write(data); err != nil {
				showError(err, w)
			}
		}, w)
		fd.SetFileName(name)
		fd.Show()
	}
	copyBtn := widget.NewButtonWithIcon(L("Copy", "Скопировать"), theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	caBtn := widget.NewButtonWithIcon(L("Authority certificate…", "Сертификат центра…"), theme.DocumentSaveIcon(), func() { save("veil-ca.crt", []byte(cfg.CAPEM)) })
	btns := container.NewGridWithColumns(2, copyBtn, caBtn)
	if cfg.Auth == ipsecconf.AuthCert {
		p12Btn := widget.NewButtonWithIcon(L("Client certificate (.p12)…", "Сертификат клиента (.p12)…"), theme.DocumentSaveIcon(), func() {
			raw, err := base64.StdEncoding.DecodeString(cfg.P12)
			if err != nil {
				showError(err, w)
				return
			}
			save("veil-client.p12", raw)
		})
		btns = container.NewGridWithColumns(1, copyBtn, caBtn, p12Btn)
	}
	body := container.NewVBox(
		note(L("This contains the client's password or key: do not show this data to others. The authority certificate must be installed on the device as trusted.", "Здесь пароль или ключ клиента: не показывайте эти данные посторонним. Сертификат центра нужно установить на устройство как доверенный.")),
		entry, btns,
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}
