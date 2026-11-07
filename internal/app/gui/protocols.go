package gui

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/vpnconf/ovpnconf"
	"veil/internal/vpnconf/wgconf"
)

// ---------- каталог протоколов на странице сервера ----------

type protoInfo struct {
	id, title, desc string
	ready           bool // установка и подключение уже реализованы
}

func protoList() []protoInfo {
	return []protoInfo{
		{"amneziawg", "AmneziaWG", L("AmneziaWG — a special protocol based on WireGuard: high speed and stable operation even in the most difficult networks.", "AmneziaWG — особый протокол на основе WireGuard: высокая скорость и устойчивая работа даже в самых сложных сетях."), true},
		{"wireguard", "WireGuard", L("WireGuard — a popular VPN protocol with high performance, speed and low power consumption.", "WireGuard — популярный VPN-протокол с высокой производительностью, скоростью и низким энергопотреблением."), true},
		{"openvpn", "OpenVPN", L("OpenVPN — the most popular VPN protocol with flexible settings; its own SSL/TLS protection.", "OpenVPN — самый популярный VPN-протокол с гибкими настройками; своя защита на SSL/TLS."), true},
		{"ipsec", "IPsec", L("IKEv2/IPsec — a modern, stable protocol that restores the connection after signal loss, built into Windows, Android and iOS.", "IKEv2/IPsec — современный стабильный протокол, восстанавливает соединение после потери сигнала, встроен в Windows, Android и iOS."), true},
	}
}

func (u *ui) profileByName(name string) *config.Profile {
	for _, p := range u.profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// wgProfileFor — профиль WireGuard, поставленный с этого сервера (nil, если не ставился).
func (u *ui) wgProfileFor(srv *config.Profile) *config.Profile {
	for _, p := range u.profiles {
		if p.Protocol == "wireguard" && p.ServerRef == srv.Name {
			return p
		}
	}
	return nil
}

func (u *ui) uniqueProfileName(base string) string {
	name, n := base, 2
	for u.profileExists(name) {
		name = fmt.Sprintf("%s %d", base, n)
		n++
	}
	return name
}

// renderProtocolList — список протоколов под сведениями о сервере (как в референсе).
func (m *manager) renderProtocolList() {
	m.body.Add(kit.SectionLabel(L("Other protocols", "Другие протоколы")))
	for _, pi := range protoList() {
		pi := pi
		var trailing fyne.CanvasObject
		var tap func()
		switch {
		case !pi.ready:
			trailing = kit.Text(L("soon", "скоро"), 12, kit.ColMuted, false)
			tap = func() {
				kit.ShowInfo(pi.title, L("Installation and connection for this protocol will be added in one of the next versions.", "Установка и подключение по этому протоколу будут добавлены в одной из следующих версий."), m.w)
			}
		case pi.id == "amneziawg" && m.u.awgProfileFor(m.p) != nil:
			awg := m.u.awgProfileFor(m.p)
			trailing = widget.NewIcon(theme.NavigateNextIcon())
			tap = func() { m.u.openAWGManage(awg) }
		case pi.id == "amneziawg":
			trailing = widget.NewIcon(theme.DownloadIcon())
			tap = func() { m.u.openAWGInstall(m) }
		case pi.id == "openvpn" && m.u.ovProfileFor(m.p) != nil:
			ov := m.u.ovProfileFor(m.p)
			trailing = widget.NewIcon(theme.NavigateNextIcon())
			tap = func() { m.u.openOVManage(ov) }
		case pi.id == "openvpn":
			trailing = widget.NewIcon(theme.DownloadIcon())
			tap = func() { m.u.openOVInstall(m) }
		case pi.id == "ipsec":
			trailing, tap = m.ipsecRow()
		case m.u.wgProfileFor(m.p) != nil:
			wg := m.u.wgProfileFor(m.p)
			trailing = widget.NewIcon(theme.NavigateNextIcon())
			tap = func() { m.u.openWGManage(wg) }
		default:
			trailing = widget.NewIcon(theme.DownloadIcon())
			tap = func() { m.u.openWGInstall(m) }
		}
		m.body.Add(kit.ListRow(pi.title, pi.desc, trailing, tap))
	}
}

// ---------- установка WireGuard ----------

func serverHost(p *config.Profile) string {
	if p.IPv4 != "" {
		return p.IPv4
	}
	return p.IPv6
}

func (u *ui) openWGInstall(m *manager) {
	w := u.newSubPage(L("Install WireGuard", "Установка WireGuard"))
	port := widget.NewEntry()
	port.SetText(strconv.Itoa(20000 + rand.IntN(40000)))
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
		install.Disable()
		port.Disable()
		fail := func(err error) {
			barBox.Hide()
			bar.Stop()
			install.Enable()
			port.Enable()
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
				cfg, err := m.sess.InstallWireGuard(deploy.WGInstall{Port: n}, serverHost(m.p), func(f string, a ...any) {
					line := fmt.Sprintf(f, a...)
					fyne.Do(func() { setStatus(line) })
				})
				fyne.Do(func() {
					if err != nil {
						fail(err)
						return
					}
					p := u.saveWGProfile(m.p, "", cfg)
					if p == nil {
						fail(errors.New(L("failed to save profile", "не удалось сохранить профиль")))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.showWGShare(L("WireGuard client", "Клиент WireGuard"), cfg)
				})
			}()
		}, fail)
	}
	body := kit.PageBox(
		note(L("WireGuard is a popular VPN protocol with high performance, speed and low power consumption. It is installed on the server over SSH; afterwards you can connect to it from this app or with any WireGuard client.", "WireGuard — популярный VPN-протокол с высокой производительностью, скоростью и низким энергопотреблением. Устанавливается на сервер по SSH; затем к нему можно подключиться из этого приложения или любым клиентом WireGuard.")),
		kit.Lead(kit.Text(L("Port (UDP)", "Порт (UDP)"), 12, kit.ColMuted, false)), port,
		status, barBox,
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, install, nil, nil, scrolled(body))))
	w.Show()
}

// saveWGProfile сохраняет профиль WireGuard для сервера srv; label — суффикс имени (пусто — «WireGuard»).
func (u *ui) saveWGProfile(srv *config.Profile, label string, cfg *wgconf.Config) *config.Profile {
	base := srv.Name + " · WireGuard"
	if label != "" {
		base += " · " + label
	}
	p := &config.Profile{Name: u.uniqueProfileName(base), Protocol: "wireguard", WG: cfg, ServerRef: srv.Name}
	if h, _, err := cfg.EndpointHostPort(); err == nil {
		if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil && ip.To4() == nil {
			p.IPv6 = ip.String()
		} else {
			p.IPv4 = h
		}
	}
	if err := saveProfile(p); err != nil {
		kit.ShowError(err, u.win)
		return nil
	}
	u.reload()
	return p
}

// ---------- управление установленным WireGuard ----------

func (u *ui) openWGManage(p *config.Profile) {
	w := u.newSubPage("WireGuard")
	srv := u.profileByName(p.ServerRef)
	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	needServer := func(f func(s *deploy.Session, srv *config.Profile)) func() {
		return func() {
			if srv == nil || srv.SSH == nil {
				kit.ShowError(errors.New(L("the profile of the server WireGuard was installed from was not found: management unavailable", "профиль сервера, с которого ставился WireGuard, не найден: управление недоступно")), w)
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
	rows := kit.PageBox(
		kit.ListRow(L("This device's configuration", "Конфигурация этого устройства"), L("QR code and .conf file for the WireGuard app", "QR-код и файл .conf для приложения WireGuard"), arrow(), func() { u.showWGShare(p.Name, p.WG) }),
		kit.ListRow(L("Add client", "Добавить клиента"), L("Issue a new key and configuration for another device", "Выдать новый ключ и конфигурацию для другого устройства"), arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			name := widget.NewEntry()
			name.SetText(L("Client", "Клиент"))
			kit.ShowForm(L("New WireGuard client", "Новый клиент WireGuard"), L("Create", "Создать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("Name", "Имя"), name)}, func(ok bool) {
				if !ok {
					return
				}
				go func() {
					cfg, err := s.WireGuardAddClient(name.Text, serverHost(srv))
					fyne.Do(func() {
						if err != nil {
							kit.ShowError(err, w)
							return
						}
						u.showWGShare(fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), strings.TrimSpace(name.Text)), cfg)
					})
				}()
			}, w)
		})),
		kit.ListRowColored(L("Uninstall WireGuard from server", "Удалить WireGuard с сервера"), L("Stop the service and delete the settings and all clients on the server", "Остановить службу и удалить настройки и всех клиентов на сервере"), "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			kit.ShowConfirm(L("Uninstall WireGuard", "Удалить WireGuard"), L("WireGuard will be stopped on the server; all its clients will stop connecting. Continue?", "WireGuard будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveWireGuard(); err != nil {
					kit.ShowError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				kit.ShowInfo(L("Done", "Готово"), L("WireGuard has been removed from the server and the profile deleted from the app.", "WireGuard удалён с сервера, профиль удалён из приложения."), u.win)
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

func (u *ui) deleteProfile(p *config.Profile) {
	if p.Name == u.active {
		kit.ShowError(errors.New(L("disconnect the VPN first", "сначала отключите VPN")), u.win)
		return
	}
	if path, err := config.ProfilePath(p.Name); err == nil {
		_ = os.Remove(path)
	}
	u.reload()
}

// showWGShare показывает конфигурацию WireGuard: QR-код, текст, копирование и сохранение .conf.
func (u *ui) showWGShare(title string, cfg *wgconf.Config) {
	w := u.newSubPage(title)
	text := cfg.String()
	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.Wrapping = fyne.TextWrapBreak
	entry.SetMinRowsVisible(8)
	copyBtn := widget.NewButtonWithIcon(L("Copy", "Скопировать"), theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	saveBtn := widget.NewButtonWithIcon(L(".conf file…", "Файл .conf…"), theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write([]byte(text)); err != nil {
				kit.ShowError(err, w)
			}
		}, w)
		fd.SetFileName("veil-wireguard.conf")
		fd.Show()
	})
	body := kit.PageBox(
		qrImage(text, ""),
		note(L("Scan the QR code in the WireGuard app or save the file. The configuration contains a private key: do not show it to others.", "Отсканируйте QR-код в приложении WireGuard или сохраните файл. В конфигурации приватный ключ: не показывайте её посторонним.")),
		entry,
		container.NewGridWithColumns(2, copyBtn, saveBtn),
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}

// ---------- импорт: ссылка Veil или обычный .conf WireGuard ----------

func decodeAny(text string) (*config.Profile, error) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "veil://") && !strings.Contains(t, "[Interface]") && ovpnconf.Looks(t) {
		cfg, err := ovpnconf.Parse(t)
		if err != nil {
			return nil, err
		}
		p := &config.Profile{Name: "OpenVPN", Protocol: "openvpn", OVPN: cfg}
		setProfileHost(p, cfg)
		if h, _, _, err := cfg.Remote(); err == nil {
			p.Name = "OpenVPN " + h
		}
		return p, nil
	}
	if strings.Contains(t, "[Interface]") {
		cfg, err := wgconf.ParseAWG(t)
		if err != nil {
			return nil, err
		}
		proto, title := "wireguard", "WireGuard"
		if !cfg.AWG.Empty() {
			proto, title = "amneziawg", "AmneziaWG"
		} else {
			cfg.AWG = nil
		}
		p := &config.Profile{Name: title, Protocol: proto, WG: cfg}
		if h, _, err := cfg.EndpointHostPort(); err == nil {
			if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil && ip.To4() == nil {
				p.IPv6 = ip.String()
			} else {
				p.IPv4 = h
				p.Name = title + " " + h
			}
		}
		return p, nil
	}
	return config.DecodeLink(t)
}
