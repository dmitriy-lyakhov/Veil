package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
)

const (
	methodPassword = "Пароль"
	methodKey      = "SSH-ключ"
	transportAuto  = "Авто: HTTP/3 → HTTP/2"
	transportH2    = "Только HTTP/2 (TCP)"
	maxImportBytes = 64 << 10
)

var siteLangs = map[string]string{"Русский": "ru", "English": "en"}

func (u *ui) serversPage() fyne.CanvasObject {
	add := widget.NewButtonWithIcon("Добавить", theme.ContentAddIcon(), u.showAddChooser)
	add.Importance = widget.HighImportance
	u.serverList = container.NewVBox()
	header := container.NewBorder(nil, nil, nil, add, text("Серверы", 22, colText, true))
	return container.NewPadded(container.NewBorder(header, nil, nil, nil, container.NewVScroll(u.serverList)))
}

func (u *ui) rebuildServers() {
	if u.serverList == nil {
		return
	}
	u.serverList.RemoveAll()
	if len(u.profiles) == 0 {
		empty := widget.NewLabel("Пока нет серверов. Разверните Veil на своём сервере по SSH или импортируйте файл конфигурации.")
		empty.Wrapping = fyne.TextWrapWord
		u.serverList.Add(card(empty))
		return
	}
	for _, p := range u.profiles {
		u.serverList.Add(u.serverCard(p))
	}
	u.serverList.Refresh()
}

func (u *ui) serverCard(p *config.Profile) fyne.CanvasObject {
	dot := canvas.NewCircle(colIdle)
	if p.Name == u.selected {
		dot.FillColor = colAccent
	}
	dotBox := container.NewGridWrap(fyne.NewSize(12, 12), dot)
	name := text(p.Name, 16, colText, true)
	sub := text(addresses(p), 12, colMuted, false)
	lines := container.NewVBox(name, sub)
	if p.Name == u.active {
		state := "подключение…"
		if u.connected {
			state = "подключено"
		}
		lines.Add(text("● "+state, 12, colOK, false))
	}
	info := widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() { u.showDetails(p) })
	info.Importance = widget.LowImportance
	actions := container.NewHBox(info)
	if managed(p) {
		gear := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() { u.openManager(p) })
		gear.Importance = widget.LowImportance
		actions = container.NewHBox(gear, info)
	}
	body := container.NewBorder(nil, nil, container.NewCenter(dotBox), container.NewCenter(actions), lines)
	return newTapCard(body, colCard, colCardHover, func() { u.selectProfile(p.Name) })
}

func (u *ui) showAddChooser() {
	var d dialog.Dialog
	option := func(icon fyne.Resource, title, desc string, action func()) fyne.CanvasObject {
		t := text(title, 16, colText, true)
		l := widget.NewLabel(desc)
		l.Wrapping = fyne.TextWrapWord
		return newTapCard(container.NewBorder(nil, nil, widget.NewIcon(icon), nil, container.NewVBox(t, l)), colCard2, colCard2Hover, func() {
			d.Hide()
			action()
		})
	}
	content := container.NewVBox(
		option(theme.ComputerIcon(), "Свой сервер по SSH", "IP-адрес, логин и пароль или SSH-ключ. Приложение само установит Veil и сайт-прикрытие.", u.showDeployForm),
		option(theme.FileIcon(), "Файл или ссылка", "Импорт конфигурации .veil или ссылки veil://, выданной в админ-панели.", u.showImport),
		option(theme.MailForwardIcon(), "Через ближайший вход", "Подключение через свои серверы: выход задаётся явно, вход выбирается ближайший при каждом подключении.", u.openMultiEntryBuilder),
	)
	d = dialog.NewCustom("Добавить сервер", "Отмена", container.NewGridWrap(fyne.NewSize(360, 380), content), u.win)
	d.Show()
}

func (u *ui) showDeployForm() {
	w := u.app.NewWindow("Новый сервер")
	name := widget.NewEntry()
	name.SetPlaceHolder("Например: Амстердам")
	ipv6 := widget.NewEntry()
	ipv6.SetPlaceHolder("2001:db8::10")
	ipv4 := widget.NewEntry()
	ipv4.SetPlaceHolder("203.0.113.10")
	port := widget.NewEntry()
	port.SetText("22")
	user := widget.NewEntry()
	user.SetText("root")
	password := widget.NewPasswordEntry()
	keyPath := widget.NewEntry()
	keyPath.SetPlaceHolder("~/.ssh/id_ed25519")
	passphrase := widget.NewPasswordEntry()
	pick := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err == nil && r != nil {
				keyPath.SetText(r.URI().Path())
				r.Close()
			}
		}, w).Show()
	})
	lang := widget.NewSelect([]string{"Русский", "English"}, nil)
	lang.SetSelected("Русский")

	authForm := widget.NewForm()
	method := widget.NewRadioGroup([]string{methodPassword, methodKey}, func(s string) {
		if s == methodKey {
			authForm.Items = []*widget.FormItem{
				widget.NewFormItem("SSH-ключ", container.NewBorder(nil, nil, nil, pick, keyPath)),
				widget.NewFormItem("Пароль ключа", passphrase),
			}
		} else {
			authForm.Items = []*widget.FormItem{widget.NewFormItem("Пароль", password)}
		}
		authForm.Refresh()
	})
	method.Horizontal = true
	method.SetSelected(methodPassword)

	form := widget.NewForm(
		widget.NewFormItem("Название", name),
		widget.NewFormItem("IPv6", ipv6),
		widget.NewFormItem("IPv4", ipv4),
		widget.NewFormItem("SSH-порт", port),
		widget.NewFormItem("Пользователь", user),
		widget.NewFormItem("Язык сайта", lang),
	)
	logView := widget.NewMultiLineEntry()
	logView.Wrapping = fyne.TextWrapWord
	logView.SetMinRowsVisible(9)
	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	appendLog := func(s string) {
		logView.SetText(strings.TrimLeft(logView.Text+"\n"+s, "\n"))
		logView.CursorRow = strings.Count(logView.Text, "\n")
		logView.Refresh()
	}
	var install *widget.Button
	install = widget.NewButtonWithIcon("Установить и подключить", theme.UploadIcon(), func() {
		req, err := buildRequest(name.Text, ipv4.Text, ipv6.Text, port.Text, user.Text, method.Selected, password.Text, keyPath.Text, passphrase.Text)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		req.SiteLang = siteLangs[lang.Selected]
		if u.profileExists(req.Name) {
			dialog.ShowError(fmt.Errorf("сервер «%s» уже есть — выберите другое название", req.Name), w)
			return
		}
		install.Disable()
		progress.Show()
		progress.Start()
		logView.SetText("")
		go func() {
			logf := func(format string, args ...any) {
				line := fmt.Sprintf(format, args...)
				fyne.Do(func() { appendLog(line) })
			}
			prof, err := deploy.Deploy(context.Background(), req, logf)
			if err == nil {
				err = saveProfile(prof)
			}
			fyne.Do(func() {
				progress.Stop()
				progress.Hide()
				if err != nil {
					appendLog("Ошибка: " + err.Error())
					install.Enable()
					return
				}
				appendLog("Готово. Профиль сохранён.")
				u.reload()
				u.selectProfile(prof.Name)
				w.Close()
				u.showPage(0)
				if u.handle == nil {
					u.toggle()
				}
			})
		}()
	})
	install.Importance = widget.HighImportance
	hint := widget.NewLabel("Приложение подключится по SSH, установит Veil (порт 443, TCP и UDP) и сгенерирует сайт-прикрытие. Пароль SSH не сохраняется.")
	hint.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewVScroll(container.NewVBox(hint, form, method, authForm, install, progress, logView))))
	w.Resize(fyne.NewSize(winWidth+40, 700))
	w.Show()
}

func (u *ui) showImport() {
	w := u.app.NewWindow("Импорт конфигурации")
	link := widget.NewMultiLineEntry()
	link.SetPlaceHolder("veil://…")
	link.Wrapping = fyne.TextWrapBreak
	link.SetMinRowsVisible(5)
	name := widget.NewEntry()
	name.SetPlaceHolder("Название (необязательно)")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	preview := func() {
		p, err := config.DecodeLink(link.Text)
		switch {
		case strings.TrimSpace(link.Text) == "":
			status.SetText("")
		case err != nil:
			status.SetText("⚠ " + err.Error())
		default:
			status.SetText("✓ Сервер " + addresses(p))
			if name.Text == "" {
				name.SetText(p.Name)
			}
		}
	}
	link.OnChanged = func(string) { preview() }
	fileBtn := widget.NewButtonWithIcon("Открыть файл…", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			b, err := io.ReadAll(io.LimitReader(r, maxImportBytes))
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			link.SetText(strings.TrimSpace(string(b)))
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{config.FileExt, ".json", ".txt"}))
		fd.Show()
	})
	pasteBtn := widget.NewButtonWithIcon("Вставить", theme.ContentPasteIcon(), func() {
		link.SetText(u.app.Clipboard().Content())
	})
	add := widget.NewButtonWithIcon("Добавить сервер", theme.ConfirmIcon(), func() {
		p, err := config.DecodeLink(link.Text)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if n := strings.TrimSpace(name.Text); n != "" {
			p.Name = n
		}
		if u.profileExists(p.Name) {
			dialog.ShowError(fmt.Errorf("сервер «%s» уже есть — задайте другое название", p.Name), w)
			return
		}
		if err := saveProfile(p); err != nil {
			dialog.ShowError(err, w)
			return
		}
		u.reload()
		u.selectProfile(p.Name)
		w.Close()
		u.showPage(0)
	})
	add.Importance = widget.HighImportance
	hint := widget.NewLabel("Вставьте ссылку veil://… или откройте файл .veil, который выдала админ-панель вашего сервера.")
	hint.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewVBox(hint, container.NewGridWithColumns(2, fileBtn, pasteBtn), link, status, name, add)))
	w.Resize(fyne.NewSize(winWidth+40, 460))
	w.Show()
}

func (u *ui) showDetails(p *config.Profile) {
	var d dialog.Dialog
	rows := container.NewVBox(text(p.Name, 18, colText, true), text(addresses(p), 12, colMuted, false))
	if p.Via != "" {
		v := widget.NewLabel("Каскад: " + p.Via)
		v.Wrapping = fyne.TextWrapWord
		rows.Add(v)
	}
	if managed(p) {
		mg := widget.NewButtonWithIcon("Управление сервером", theme.SettingsIcon(), func() {
			d.Hide()
			u.openManager(p)
		})
		mg.Importance = widget.HighImportance
		rows.Add(mg)
	}

	mode := widget.NewRadioGroup([]string{transportAuto, transportH2}, nil)
	if len(p.Transports) == 1 && p.Transports[0] == "h2" {
		mode.SetSelected(transportH2)
	} else {
		mode.SetSelected(transportAuto)
	}
	mode.OnChanged = func(s string) {
		p.Transports = nil
		if s == transportH2 {
			p.Transports = []string{"h2"}
		}
		if err := saveProfile(p); err != nil {
			dialog.ShowError(err, u.win)
		}
	}
	rows.Add(widget.NewSeparator())
	rows.Add(text("Транспорт", 14, colText, true))
	rows.Add(mode)

	if p.AdminURL != "" {
		rows.Add(widget.NewSeparator())
		rows.Add(text("Админ-панель", 14, colText, true))
		copyRow := func(label, value string) fyne.CanvasObject {
			btn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(value) })
			btn.Importance = widget.LowImportance
			return container.NewBorder(nil, nil, text(label, 12, colMuted, false), btn, widget.NewLabel(value))
		}
		rows.Add(copyRow("Адрес", p.AdminURL))
		if p.AdminUser != "" {
			rows.Add(copyRow("Логин", p.AdminUser))
		}
		if p.AdminPassword != "" {
			rows.Add(copyRow("Пароль", p.AdminPassword))
		}
		openBtn := widget.NewButtonWithIcon("Открыть в браузере", theme.ComputerIcon(), func() {
			if target, err := url.Parse(p.AdminURL); err == nil {
				_ = u.app.OpenURL(target)
			}
		})
		note := "Панель доступна только через VPN: сначала подключитесь к этому серверу. Браузер предупредит о самоподписанном сертификате — это ожидаемо."
		if p.Name != u.active || !u.connected {
			openBtn.Disable()
		}
		hint := widget.NewLabel(note)
		hint.Wrapping = fyne.TextWrapWord
		rows.Add(openBtn)
		rows.Add(hint)
	}

	remove := widget.NewButtonWithIcon("Удалить профиль", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Удалить профиль", "Удалить «"+p.Name+"» с этого устройства? Сервер не изменится.", func(ok bool) {
			if !ok {
				return
			}
			if path, err := config.ProfilePath(p.Name); err == nil {
				_ = os.Remove(path)
			}
			d.Hide()
			u.reload()
		}, u.win)
	})
	remove.Importance = widget.DangerImportance
	if p.Name == u.active {
		remove.Disable()
	}
	rows.Add(layout.NewSpacer())
	rows.Add(remove)
	d = dialog.NewCustom("Сервер", "Закрыть", container.NewGridWrap(fyne.NewSize(360, 480), container.NewVScroll(rows)), u.win)
	d.Show()
}

func (u *ui) profileExists(name string) bool {
	path, err := config.ProfilePath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func saveProfile(p *config.Profile) error {
	path, err := config.ProfilePath(p.Name)
	if err != nil {
		return err
	}
	return config.Save(path, p)
}

func buildRequest(name, v4, v6, port, user, method, password, keyPath, passphrase string) (deploy.Request, error) {
	v4, v6 = strings.TrimSpace(v4), strings.Trim(strings.TrimSpace(v6), "[]")
	if v4 == "" && v6 == "" {
		return deploy.Request{}, errors.New("укажите хотя бы один адрес сервера: IPv6 или IPv4")
	}
	if v4 != "" {
		if a, err := netip.ParseAddr(v4); err != nil || !a.Is4() {
			return deploy.Request{}, fmt.Errorf("«%s» — не IPv4-адрес", v4)
		}
	}
	if v6 != "" {
		if a, err := netip.ParseAddr(v6); err != nil || !a.Is6() {
			return deploy.Request{}, fmt.Errorf("«%s» — не IPv6-адрес", v6)
		}
	}
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || p <= 0 || p > 65535 {
		return deploy.Request{}, errors.New("некорректный SSH-порт")
	}
	req := deploy.Request{Name: strings.TrimSpace(name), IPv4: v4, IPv6: v6, SSHPort: p, User: strings.TrimSpace(user)}
	if req.Name == "" {
		req.Name = v6
		if req.Name == "" {
			req.Name = v4
		}
	}
	if method == methodKey {
		if strings.TrimSpace(keyPath) == "" {
			return req, errors.New("укажите путь к SSH-ключу")
		}
		req.KeyPath, req.KeyPassphrase = strings.TrimSpace(keyPath), passphrase
	} else {
		if password == "" {
			return req, errors.New("укажите пароль SSH")
		}
		req.Password = password
	}
	return req, nil
}
