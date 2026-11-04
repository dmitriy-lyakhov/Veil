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
	w := u.newSubPage("Установка IPsec")
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
		install.Disable()
		fail := func(err error) {
			barBox.Hide()
			bar.Stop()
			install.Enable()
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
						fail(errors.New("не удалось сохранить профиль"))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.askTrustCA(p, func() { u.showIPsecShare("Клиент IPsec", cfg) })
				})
			}()
		}, fail)
	}
	body := container.NewVBox(
		note("IPsec/IKEv2 — современный стабильный протокол, восстанавливает соединение после потери сигнала и встроен в Windows, Android и iOS. На сервер по SSH устанавливается strongSwan со своим центром сертификации; затем к нему можно подключиться из этого приложения (Windows) или встроенным VPN телефона."),
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
	showConfirm("Сертификат центра сертификации",
		"Чтобы подключаться к серверу с этого компьютера, сертификат центра сертификации Veil нужно добавить в доверенные корневые сертификаты Windows (хранилище компьютера). "+
			"Он нужен только для проверки вашего сервера, но любой доверенный центр может выпускать сертификаты для любых сайтов, поэтому добавляйте его только если сервер ваш. Добавить при подключении?",
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
				showError(errors.New("профиль сервера, с которого ставился IPsec, не найден: управление недоступно"), w)
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
			name.SetText("Клиент")
			showForm("Новый клиент IPsec", "Создать", "Отмена", []*widget.FormItem{widget.NewFormItem("Имя", name)}, func(ok bool) {
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
						u.showIPsecShare("Клиент «"+strings.TrimSpace(name.Text)+"»", cfg)
					})
				}()
			}, w)
		})
	}
	trustSub := func() string {
		if p.IPsec != nil && p.IPsec.TrustCA {
			return "Разрешено: при подключении сертификат центра будет добавлен в доверенные корневые Windows"
		}
		return "Не разрешено: подключение с этого компьютера не удастся, пока сертификат не добавлен в доверенные"
	}
	trustRow := listRow("Доверие сертификату центра на этом компьютере", trustSub(), arrow(), func() {
		u.askTrustCA(p, func() { w.Close(); u.openIPsecManage(p) })
	})
	rows := container.NewVBox(
		listRow("Данные этого устройства", "Адрес, логин, пароль, сертификат центра и инструкция для Windows, Android и iOS", arrow(), func() { u.showIPsecShare(p.Name, p.IPsec) }),
		trustRow,
		listRow("Добавить клиента (логин и пароль)", "Выдать новый логин и пароль для другого устройства", arrow(), addClient(false)),
		listRow("Добавить клиента (сертификат)", "Выпустить сертификат клиента (.p12); отозвать его по отдельности нельзя", arrow(), addClient(true)),
		listRow("Клиенты на сервере", "Список логинов; удаление клиента", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
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
		listRowColored("Удалить IPsec с сервера", "Остановить службу и удалить настройки, сертификаты и всех клиентов на сервере", "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			showConfirm("Удалить IPsec", "IPsec будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?", func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveIPsec(); err != nil {
					showError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				showInfo("Готово", "IPsec удалён с сервера, профиль удалён из приложения.", u.win)
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

func (u *ui) showIPsecClients(s *deploy.Session, users []string) {
	w := u.newSubPage("Клиенты IPsec")
	box := container.NewVBox()
	if len(users) == 0 {
		box.Add(note("Клиентов с логином и паролем на сервере нет."))
	}
	for _, name := range users {
		name := name
		box.Add(listRowColored(name, "Нажмите, чтобы удалить клиента с сервера", "error", widget.NewIcon(theme.DeleteIcon()), func() {
			showConfirm("Удалить клиента", "Клиент «"+name+"» больше не сможет подключаться. Продолжить?", func(ok bool) {
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
	copyBtn := widget.NewButtonWithIcon("Скопировать", theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	caBtn := widget.NewButtonWithIcon("Сертификат центра…", theme.DocumentSaveIcon(), func() { save("veil-ca.crt", []byte(cfg.CAPEM)) })
	btns := container.NewGridWithColumns(2, copyBtn, caBtn)
	if cfg.Auth == ipsecconf.AuthCert {
		p12Btn := widget.NewButtonWithIcon("Сертификат клиента (.p12)…", theme.DocumentSaveIcon(), func() {
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
		note("Здесь пароль или ключ клиента: не показывайте эти данные посторонним. Сертификат центра нужно установить на устройство как доверенный."),
		entry, btns,
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}
