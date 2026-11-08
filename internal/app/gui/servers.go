package gui

import (
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

	"veil/internal/app/gui/kit"
	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
)

const maxImportBytes = 64 << 10

func methodPassword() string { return L("Password", "Пароль") }
func methodKey() string      { return L("SSH key", "SSH-ключ") }
func transportAuto() string  { return L("Auto: HTTP/3 → HTTP/2", "Авто: HTTP/3 → HTTP/2") }
func transportH2() string    { return L("HTTP/2 only (TCP)", "Только HTTP/2 (TCP)") }

var siteLangs = map[string]string{"Русский": "ru", "English": "en"}

func (u *ui) serversPage() fyne.CanvasObject {
	u.serverList = kit.NewVBoxFit()
	return container.NewVScroll(u.serverList)
}

func (u *ui) rebuildServers() {
	if u.serverList == nil {
		return
	}
	u.serverList.RemoveAll()
	if len(u.profiles) == 0 {
		empty := widget.NewLabel(L("No servers yet. Deploy Veil on your own server over SSH or import a configuration file.", "Пока нет серверов. Разверните Veil на своём сервере по SSH или импортируйте файл конфигурации."))
		empty.Wrapping = fyne.TextWrapWord
		u.serverList.Add(kit.Card(empty))
		return
	}
	for _, p := range u.profiles {
		u.serverList.Add(u.serverCard(p))
	}
	u.serverList.Refresh()
}

func (u *ui) serverCard(p *config.Profile) fyne.CanvasObject {
	dot := canvas.NewCircle(kit.ColIdle)
	if p.Name == u.selected {
		dot.FillColor = kit.ColAccent
	}
	dotBox := container.NewGridWrap(fyne.NewSize(12, 12), dot)
	sub := addresses(p)
	if p.Name == u.active {
		state := L("connecting…", "подключение…")
		if u.connected {
			state = L("connected", "подключено")
		}
		sub = state + " · " + sub
	}
	info := widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() { u.showDetails(p) })
	info.Importance = widget.LowImportance
	actions := container.NewHBox(info)
	if managed(p) {
		gear := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() { u.openManager(p) })
		gear.Importance = widget.LowImportance
		actions = container.NewHBox(gear, info)
	}
	body := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 4), container.NewCenter(dotBox)), nil, kit.Vcenter(kit.RowText(p.Name, sub, theme.ColorNameForeground)))
	return kit.ListRowOf(body, container.NewCenter(actions), func() { u.selectProfile(p.Name) })
}

func (u *ui) showAddChooser() {
	var d kit.Modal
	option := func(icon fyne.Resource, title, desc string, action func()) fyne.CanvasObject {
		t := kit.Text(title, 16, kit.ColText, true)
		l := widget.NewLabel(desc)
		l.Wrapping = fyne.TextWrapWord
		return kit.NewTapCard(container.NewBorder(nil, nil, widget.NewIcon(icon), nil, container.New(kit.TightVBox{Gap: -4}, kit.Lead(t), l)), kit.ColCard2, kit.ColCard2Hover, func() {
			d.Hide()
			action()
		})
	}
	content := kit.PageBox(
		option(theme.ComputerIcon(), L("Your own server over SSH", "Свой сервер по SSH"), L("IP address, login and password or SSH key. The app will install Veil and the cover site itself.", "IP-адрес, логин и пароль или SSH-ключ. Приложение само установит Veil и сайт-прикрытие."), u.showDeployForm),
		option(theme.FileIcon(), L("File or link", "Файл или ссылка"), L("Import a .veil configuration or a veil:// link issued by the admin panel.", "Импорт конфигурации .veil или ссылки veil://, выданной в админ-панели."), u.showImport),
		option(theme.MailForwardIcon(), L("Via the nearest entry", "Через ближайший вход"), L("Connect through your own servers: the exit is set explicitly, the nearest entry is chosen on every connection.", "Подключение через свои серверы: выход задаётся явно, вход выбирается ближайший при каждом подключении."), u.openMultiEntryBuilder),
	)
	d = kit.NewCustom(L("Add server", "Добавить сервер"), L("Cancel", "Отмена"), container.NewGridWrap(fyne.NewSize(360, 352), content), u.win)
	d.Show()
}

// protocolProfile — профиль обычного протокола, поставленного прямо из формы: SSH хранится в нём самом,
// и он же служит «сервером» для управления (ServerRef указывает на себя).
func protocolProfile(req deploy.Request, proto string, res *deploy.ProtocolResult) *config.Profile {
	return &config.Profile{
		Name: req.Name, Protocol: proto, IPv4: req.IPv4, IPv6: req.IPv6, SSH: res.SSH, ServerRef: req.Name,
		WG: res.WG, OVPN: res.OVPN, IPsec: res.IPsec,
	}
}

func (u *ui) showImport() {
	w := u.newSubPage(L("Import configuration", "Импорт конфигурации"))
	link := widget.NewMultiLineEntry()
	link.SetPlaceHolder("veil://…")
	link.Wrapping = fyne.TextWrapBreak
	link.SetMinRowsVisible(5)
	name := widget.NewEntry()
	name.SetPlaceHolder(L("Name (optional)", "Название (необязательно)"))
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	preview := func() {
		p, err := decodeAny(link.Text)
		switch {
		case strings.TrimSpace(link.Text) == "":
			status.SetText("")
		case err != nil:
			status.SetText("⚠ " + err.Error())
		default:
			status.SetText(L("✓ Server ", "✓ Сервер ") + addresses(p))
			if name.Text == "" {
				name.SetText(p.Name)
			}
		}
	}
	link.OnChanged = func(string) { preview() }
	fileBtn := widget.NewButtonWithIcon(L("Open file…", "Открыть файл…"), theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			b, err := io.ReadAll(io.LimitReader(r, maxImportBytes))
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			link.SetText(strings.TrimSpace(string(b)))
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{config.FileExt, ".json", ".txt", ".conf", ".ovpn"}))
		fd.Show()
	})
	pasteBtn := widget.NewButtonWithIcon(L("Paste", "Вставить"), theme.ContentPasteIcon(), func() {
		link.SetText(u.app.Clipboard().Content())
	})
	add := widget.NewButtonWithIcon(L("Add server", "Добавить сервер"), theme.ConfirmIcon(), func() {
		p, err := decodeAny(link.Text)
		if err != nil {
			kit.ShowError(err, w)
			return
		}
		if n := strings.TrimSpace(name.Text); n != "" {
			p.Name = n
		}
		if u.profileExists(p.Name) {
			kit.ShowError(fmt.Errorf(L("server “%s” already exists — set a different name", "сервер «%s» уже есть — задайте другое название"), p.Name), w)
			return
		}
		if err := saveProfile(p); err != nil {
			kit.ShowError(err, w)
			return
		}
		u.reload()
		u.selectProfile(p.Name)
		w.Close()
		u.showPage(0)
	})
	add.Importance = widget.HighImportance
	hint := widget.NewLabel(L("Paste a veil://… link or open a .veil file issued by your server's admin panel.", "Вставьте ссылку veil://… или откройте файл .veil, который выдала админ-панель вашего сервера."))
	hint.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(kit.PageBox(hint, container.NewGridWithColumns(2, fileBtn, pasteBtn), link, status, name, add)))
	w.Resize(fyne.NewSize(winWidth+40, 460))
	w.Show()
}

func (u *ui) showDetails(p *config.Profile) {
	var w *subPage
	w = u.newSubPage(p.Name)
	rows := kit.PageBox(infoRow(L("Address", "Адрес"), addresses(p), nil))
	if p.Role == "admin" || p.Role == "user" {
		rows.Add(infoRow(L("Role on the server", "Роль на сервере"), roleName(p.Role), nil))
	}
	if p.Via != "" {
		rows.Add(infoRow(L("Cascade", "Каскад"), p.Via, nil))
	}
	cur := transportAuto()
	if len(p.Transports) == 1 && p.Transports[0] == "h2" {
		cur = transportH2()
	}
	rows.Add(u.optionRow(L("Transport", "Транспорт"), cur, []string{transportAuto(), transportH2()}, func(s string) {
		p.Transports = nil
		if s == transportH2() {
			p.Transports = []string{"h2"}
		}
		if err := saveProfile(p); err != nil {
			kit.ShowError(err, u.win)
		}
	}))

	if p.AdminURL != "" {
		rows.Add(kit.SectionLabel(L("Admin panel", "Админ-панель")))
		// имя «admin» клиент прописывает в hosts при подключении; без прав на запись — запасной адрес по IP
		aliasURL := adminAliasURL(p)
		hostsFailed := p.Name == u.active && u.connected && u.adminHosts == client.AdminHostsFailed
		openURL := aliasURL
		if openURL == "" || hostsFailed {
			openURL = p.AdminURL
		}
		openAdmin := func() {
			if p.Name != u.active || !u.connected {
				kit.ShowError(errors.New(L("connect to this server first", "сначала подключитесь к этому серверу")), u.win)
				return
			}
			if target, err := url.Parse(openURL); err == nil {
				_ = u.app.OpenURL(target)
			}
		}
		shown := openURL
		if aliasURL != "" && !hostsFailed {
			shown = aliasURL
		}
		rows.Add(kit.NavRow(L("Open the admin panel", "Открыть админку"), shown, openAdmin))
		if hostsFailed {
			rows.Add(noteIcon("warn", fmt.Sprintf(L("No write access to the hosts file: “%s” does not open. Use the address by IP: %s.", "Нет прав на запись в hosts, поэтому адрес «%s» не откроется. Используйте адрес по IP: %s."), aliasURL, p.AdminURL), kit.ColAccentInk))
		}
		rows.Add(infoRow(L("Address", "Адрес"), p.AdminURL, func() { u.app.Clipboard().SetContent(p.AdminURL) }))
		if p.AdminUser != "" {
			rows.Add(infoRow(L("Login", "Логин"), p.AdminUser, func() { u.app.Clipboard().SetContent(p.AdminUser) }))
		}
		if p.AdminPassword != "" {
			rows.Add(infoRow(L("Password", "Пароль"), strings.Repeat("•", 12), func() { u.app.Clipboard().SetContent(p.AdminPassword) }))
		}
		rows.Add(infoNote(L("The panel is only available over VPN: connect to this server first. The browser will warn about a self-signed certificate — this is expected.", "Панель доступна только через VPN: сначала подключитесь к этому серверу. Браузер предупредит о самоподписанном сертификате — это ожидаемо.")))
	}
	if managed(p) {
		rows.Add(bigButton(L("Server management", "Управление сервером"), "", true, func() {
			w.Close()
			u.openManager(p)
		}))
	}
	rows.Add(dangerButton(L("Delete profile", "Удалить профиль"), func() {
		if p.Name == u.active {
			kit.ShowError(errors.New(L("disconnect first", "сначала отключитесь")), u.win)
			return
		}
		kit.ShowConfirm(L("Delete profile", "Удалить профиль"), fmt.Sprintf(L("Delete “%s” from this device? The server will not change.", "Удалить «%s» с этого устройства? Сервер не изменится."), p.Name), func(ok bool) {
			if !ok {
				return
			}
			if path, err := config.ProfilePath(p.Name); err == nil {
				_ = os.Remove(path)
			}
			w.Close()
			u.reload()
		}, u.win)
	}))
	w.SetContent(container.NewPadded(scrolled(rows)))
	w.Show()
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
		return deploy.Request{}, errors.New(L("specify at least one server address: IPv6 or IPv4", "укажите хотя бы один адрес сервера: IPv6 или IPv4"))
	}
	if v4 != "" {
		if a, err := netip.ParseAddr(v4); err != nil || !a.Is4() {
			return deploy.Request{}, fmt.Errorf(L("“%s” is not an IPv4 address", "«%s» — не IPv4-адрес"), v4)
		}
	}
	if v6 != "" {
		if a, err := netip.ParseAddr(v6); err != nil || !a.Is6() {
			return deploy.Request{}, fmt.Errorf(L("“%s” is not an IPv6 address", "«%s» — не IPv6-адрес"), v6)
		}
	}
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || p <= 0 || p > 65535 {
		return deploy.Request{}, errors.New(L("invalid SSH port", "некорректный SSH-порт"))
	}
	req := deploy.Request{Name: strings.TrimSpace(name), IPv4: v4, IPv6: v6, SSHPort: p, User: strings.TrimSpace(user)}
	if req.Name == "" {
		req.Name = v6
		if req.Name == "" {
			req.Name = v4
		}
	}
	if method == methodKey() {
		if strings.TrimSpace(keyPath) == "" {
			return req, errors.New(L("specify the path to the SSH key", "укажите путь к SSH-ключу"))
		}
		req.KeyPath, req.KeyPassphrase = strings.TrimSpace(keyPath), passphrase
	} else {
		if password == "" {
			return req, errors.New(L("specify the SSH password", "укажите пароль SSH"))
		}
		req.Password = password
	}
	return req, nil
}
