package gui

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/wgconf"
)

// ---------- AmneziaWG: установка, управление, настройки подключения ----------

// awgProfileFor — профиль AmneziaWG, поставленный с этого сервера (nil, если не ставился).
func (u *ui) awgProfileFor(srv *config.Profile) *config.Profile {
	for _, p := range u.profiles {
		if p.Protocol == "amneziawg" && p.ServerRef == srv.Name {
			return p
		}
	}
	return nil
}

func numEntry(v int) *widget.Entry {
	e := widget.NewEntry()
	if v != 0 {
		e.SetText(strconv.Itoa(v))
	}
	return e
}

// atoiField разбирает необязательное целое поле формы (пусто — 0).
func atoiField(name, s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf(L("%s: a whole number not less than zero is required", "%s: нужно целое число не меньше нуля"), name)
	}
	return n, nil
}

func (u *ui) openAWGInstall(m *manager) {
	w := u.newSubPage(L("Install AmneziaWG", "Установка AmneziaWG"))
	def := deploy.DefaultAWG()
	port := widget.NewEntry()
	port.SetText(strconv.Itoa(20000 + rand.IntN(40000)))
	subnet := widget.NewEntry()
	subnet.SetText(deploy.AWGDefaultSubnet)
	jc, jmin, jmax := numEntry(def.Jc), numEntry(def.Jmin), numEntry(def.Jmax)
	s1, s2 := numEntry(def.S1), numEntry(def.S2)
	fields := []fyne.Disableable{port, subnet, jc, jmin, jmax, s1, s2}
	setEnabled := func(on bool) {
		for _, f := range fields {
			if on {
				f.Enable()
			} else {
				f.Disable()
			}
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
	install := widget.NewButton(L("Install", "Установить"), nil)
	install.Importance = widget.HighImportance
	install.OnTapped = func() {
		o := deploy.AWGInstall{Subnet: strings.TrimSpace(subnet.Text), AWG: wgconf.AWG{H1: def.H1, H2: def.H2, H3: def.H3, H4: def.H4}}
		var err error
		if o.Port, err = atoiField(L("Port", "Порт"), port.Text); err != nil || o.Port == 0 {
			showError(errors.New(L("port must be a number from 1 to 65535", "порт должен быть числом от 1 до 65535")), w)
			return
		}
		for _, f := range []struct {
			name string
			e    *widget.Entry
			dst  *int
		}{{"Jc", jc, &o.AWG.Jc}, {"Jmin", jmin, &o.AWG.Jmin}, {"Jmax", jmax, &o.AWG.Jmax}, {"S1", s1, &o.AWG.S1}, {"S2", s2, &o.AWG.S2}} {
			if *f.dst, err = atoiField(f.name, f.e.Text); err != nil {
				showError(err, w)
				return
			}
		}
		o2, _, err := deploy.ValidateAWGInstall(o)
		if err != nil {
			showError(err, w)
			return
		}
		install.Disable()
		setEnabled(false)
		fail := func(err error) {
			barBox.Hide()
			bar.Stop()
			install.Enable()
			setEnabled(true)
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
				cfg, err := m.sess.InstallAmneziaWG(o2, serverHost(m.p), func(f string, a ...any) {
					line := fmt.Sprintf(f, a...)
					fyne.Do(func() { setStatus(line) })
				})
				fyne.Do(func() {
					if err != nil {
						fail(err)
						return
					}
					if u.saveAWGProfile(m.p, "", cfg) == nil {
						fail(errors.New(L("failed to save profile", "не удалось сохранить профиль")))
						return
					}
					bar.Stop()
					w.Close()
					m.render()
					u.showAWGShare(L("AmneziaWG client", "Клиент AmneziaWG"), cfg)
				})
			}()
		}, fail)
	}
	form := container.NewVBox(
		note(L("AmneziaWG is a WireGuard-based protocol with traffic obfuscation. It is installed on the server over SSH (Ubuntu or Debian, from the Amnezia repository; a kernel module is built, which takes a few minutes). Afterwards you can connect to it from this app or with the AmneziaWG app.", "AmneziaWG — протокол на основе WireGuard с маскировкой трафика. Устанавливается на сервер по SSH (Ubuntu или Debian, из репозитория Amnezia; собирается модуль ядра, это занимает несколько минут). Затем к нему можно подключиться из этого приложения или приложением AmneziaWG.")),
		lead(text(L("Subnet", "Подсеть"), 12, colMuted, false)), subnet,
		lead(text(L("Port (UDP)", "Порт (UDP)"), 12, colMuted, false)), port,
		lead(text(L("Jc — number of junk packets", "Jc — число мусорных пакетов"), 12, colMuted, false)), jc,
		lead(text(L("Jmin — minimum junk packet size", "Jmin — минимальный размер мусорного пакета"), 12, colMuted, false)), jmin,
		lead(text(L("Jmax — maximum junk packet size", "Jmax — максимальный размер мусорного пакета"), 12, colMuted, false)), jmax,
		lead(text(L("S1 — padding added to the first packet size", "S1 — добавка к размеру первого пакета"), 12, colMuted, false)), s1,
		lead(text(L("S2 — padding added to the reply packet size", "S2 — добавка к размеру ответного пакета"), 12, colMuted, false)), s2,
		note(L("Headers H1–H4 are generated randomly. The parameters are the same on the server and the clients.", "Заголовки H1–H4 генерируются случайно. Параметры одинаковы у сервера и клиентов.")),
		status, barBox,
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, install, nil, nil, scrolled(form))))
	w.Show()
}

// saveAWGProfile сохраняет профиль AmneziaWG для сервера srv; label — суффикс имени (пусто — «AmneziaWG»).
func (u *ui) saveAWGProfile(srv *config.Profile, label string, cfg *wgconf.Config) *config.Profile {
	base := srv.Name + " · AmneziaWG"
	if label != "" {
		base += " · " + label
	}
	p := &config.Profile{Name: u.uniqueProfileName(base), Protocol: "amneziawg", WG: cfg, ServerRef: srv.Name}
	if h, _, err := cfg.EndpointHostPort(); err == nil {
		if ip := net.ParseIP(strings.Trim(h, "[]")); ip != nil && ip.To4() == nil {
			p.IPv6 = ip.String()
		} else {
			p.IPv4 = h
		}
	}
	if err := saveProfile(p); err != nil {
		showError(err, u.win)
		return nil
	}
	u.reload()
	return p
}

func (u *ui) openAWGManage(p *config.Profile) {
	w := u.newSubPage("AmneziaWG")
	srv := u.profileByName(p.ServerRef)
	arrow := func() fyne.CanvasObject { return widget.NewIcon(theme.NavigateNextIcon()) }
	needServer := func(f func(s *deploy.Session, srv *config.Profile)) func() {
		return func() {
			if srv == nil || srv.SSH == nil {
				showError(errors.New(L("the profile of the server AmneziaWG was installed from was not found: management unavailable", "профиль сервера, с которого ставился AmneziaWG, не найден: управление недоступно")), w)
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
		listRow(L("Connection settings", "Настройки подключения"), L("MTU, Jc, Jmin, Jmax, signature packets I1–I3", "MTU, Jc, Jmin, Jmax, сигнатурные пакеты I1–I3"), arrow(), func() { u.openAWGConn(p) }),
		listRow(L("This device's configuration", "Конфигурация этого устройства"), L("QR code and .conf file for the AmneziaWG app", "QR-код и файл .conf для приложения AmneziaWG"), arrow(), func() { u.showAWGShare(p.Name, p.WG) }),
		listRow(L("Add client", "Добавить клиента"), L("Issue a new key and configuration for another device", "Выдать новый ключ и конфигурацию для другого устройства"), arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			name := widget.NewEntry()
			name.SetText(L("Client", "Клиент"))
			showForm(L("New AmneziaWG client", "Новый клиент AmneziaWG"), L("Create", "Создать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("Name", "Имя"), name)}, func(ok bool) {
				if !ok {
					return
				}
				go func() {
					cfg, err := s.AmneziaWGAddClient(name.Text, serverHost(srv))
					fyne.Do(func() {
						if err != nil {
							showError(err, w)
							return
						}
						u.showAWGShare(fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), strings.TrimSpace(name.Text)), cfg)
					})
				}()
			}, w)
		})),
		listRowColored(L("Uninstall AmneziaWG from server", "Удалить AmneziaWG с сервера"), L("Stop the service and delete the settings and all clients on the server", "Остановить службу и удалить настройки и всех клиентов на сервере"), "error", arrow(), needServer(func(s *deploy.Session, srv *config.Profile) {
			showConfirm(L("Uninstall AmneziaWG", "Удалить AmneziaWG"), L("AmneziaWG will be stopped on the server; all its clients will stop connecting. Continue?", "AmneziaWG будет остановлен на сервере, все его клиенты перестанут подключаться. Продолжить?"), func(ok bool) {
				if !ok {
					return
				}
				if err := s.RemoveAmneziaWG(); err != nil {
					showError(err, w)
					return
				}
				u.deleteProfile(p)
				w.Close()
				showInfo(L("Done", "Готово"), L("AmneziaWG has been removed from the server and the profile deleted from the app.", "AmneziaWG удалён с сервера, профиль удалён из приложения."), u.win)
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

// openAWGConn — настройки подключения: MTU, мусорные пакеты Jc/Jmin/Jmax и сигнатурные пакеты I1–I3.
// Эти значения действуют только на стороне клиента; S1, S2 и H1–H4 должны совпадать с сервером и здесь не меняются.
func (u *ui) openAWGConn(p *config.Profile) {
	w := u.newSubPage(L("Connection settings", "Настройки подключения"))
	cfg := p.WG
	if cfg.AWG == nil {
		cfg.AWG = &wgconf.AWG{}
	}
	a := cfg.AWG
	mtu := numEntry(cfg.MTU)
	jc, jmin, jmax := numEntry(a.Jc), numEntry(a.Jmin), numEntry(a.Jmax)
	var is [3]*widget.Entry
	for i := range is {
		is[i] = widget.NewMultiLineEntry()
		is[i].Wrapping = fyne.TextWrapBreak
		is[i].SetMinRowsVisible(2)
		is[i].SetText(a.I[i])
	}
	save := widget.NewButton(L("Save", "Сохранить"), nil)
	save.Importance = widget.HighImportance
	save.OnTapped = func() {
		next := *cfg
		na := *a
		next.AWG = &na
		var err error
		for _, f := range []struct {
			name string
			e    *widget.Entry
			dst  *int
		}{{"MTU", mtu, &next.MTU}, {"Jc", jc, &na.Jc}, {"Jmin", jmin, &na.Jmin}, {"Jmax", jmax, &na.Jmax}} {
			if *f.dst, err = atoiField(f.name, f.e.Text); err != nil {
				showError(err, w)
				return
			}
		}
		if next.MTU != 0 && (next.MTU < 576 || next.MTU > 1500) {
			showError(errors.New(L("MTU must be between 576 and 1500", "MTU должен быть от 576 до 1500")), w)
			return
		}
		for i := range is {
			na.I[i] = strings.TrimSpace(strings.ReplaceAll(is[i].Text, "\n", ""))
		}
		if err := na.Validate(); err != nil {
			showError(err, w)
			return
		}
		if p.Name == u.active {
			showError(errors.New(L("disconnect the VPN first, then change the settings", "сначала отключите VPN, затем меняйте настройки")), w)
			return
		}
		np := *p
		np.WG = &next
		if err := saveProfile(&np); err != nil {
			showError(err, w)
			return
		}
		u.reload()
		w.Close()
	}
	form := container.NewVBox(
		note(L("The parameters apply on this device. If the server does not respond, restore the values issued by the server.", "Параметры действуют на этом устройстве. Если сервер не отвечает, верните значения, выданные сервером.")),
		lead(text("MTU", 12, colMuted, false)), mtu,
		lead(text("Jc", 12, colMuted, false)), jc,
		lead(text("Jmin", 12, colMuted, false)), jmin,
		lead(text("Jmax", 12, colMuted, false)), jmax,
		lead(text("I1", 12, colMuted, false)), is[0],
		lead(text("I2", 12, colMuted, false)), is[1],
		lead(text("I3", 12, colMuted, false)), is[2],
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, save, nil, nil, scrolled(form))))
	w.Show()
}

// showAWGShare показывает конфигурацию AmneziaWG: QR-код, текст, копирование и сохранение .conf.
func (u *ui) showAWGShare(title string, cfg *wgconf.Config) {
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
				showError(err, w)
			}
		}, w)
		fd.SetFileName("veil-amneziawg.conf")
		fd.Show()
	})
	body := container.NewVBox(
		qrImage(text, ""),
		note(L("Open the .conf file in the AmneziaWG app (import from file). The configuration contains a private key: do not show it to others.", "Откройте файл .conf в приложении AmneziaWG (импорт из файла). В конфигурации приватный ключ: не показывайте её посторонним.")),
		entry,
		container.NewGridWithColumns(2, copyBtn, saveBtn),
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}
