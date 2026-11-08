package gui

// «Свой сервер по SSH» (дизайн Main): слева выбор протокола и админ-панели, справа адрес и вход по SSH.

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
)

// protoChoice — протокол в форме установки: ключ deploy, подпись, описание и контейнер на сервере.
type protoChoice struct {
	key, name string
	en, ru    string
	ctr       string
}

func deployProtos() []protoChoice {
	return []protoChoice{
		{"veil", "Veil", "HTTPS on 443, cover site, cascades", "HTTPS на 443, сайт-прикрытие, каскады", "veil"},
		{"wireguard", "WireGuard", "Fast and simple, UDP", "Быстрый и простой, UDP", "veil-wireguard"},
		{"amneziawg", "AmneziaWG", "WireGuard with obfuscation against DPI", "WireGuard с обфускацией от DPI", "veil-amneziawg"},
		{"openvpn", "OpenVPN", "Works with any OpenVPN client", "Подходит к любым клиентам OpenVPN", "veil-openvpn"},
		{"ipsec", "IPsec/IKEv2", "Built into Windows, macOS and iOS", "Встроен в Windows, macOS и iOS", "veil-ipsec"},
	}
}

func protoLabel(key string) string {
	for _, p := range deployProtos() {
		if p.key == key {
			return p.name
		}
	}
	return key
}

// choiceRow — строка выбора (радиокнопка): красная точка и красная обводка у выбранной.
func choiceRow(title, sub string, on bool, tap func()) (fyne.CanvasObject, func(bool)) {
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeWidth = 2
	dot := canvas.NewCircle(color.Transparent)
	radio := container.NewCenter(container.NewGridWrap(fyne.NewSize(20, 20), container.NewStack(ring, container.New(layout.NewCustomPaddedLayout(5, 5, 5, 5), dot))))
	text := kit.RowText(title, sub, theme.ColorNameForeground)
	body := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), radio), nil, kit.Vcenter(text))
	card, bg := padCard(body, 8, 14, 56)
	mark := func(on bool) {
		if on {
			ring.StrokeColor, dot.FillColor = kit.ColAccent, kit.ColAccent
			bg.StrokeColor, bg.StrokeWidth = kit.ColAccent, 2
		} else {
			ring.StrokeColor, dot.FillColor = kit.ColIdle, color.Transparent
			bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
		}
		ring.Refresh()
		dot.Refresh()
		bg.Refresh()
	}
	mark(on)
	return kit.NewRowTap(card, tap), mark
}

// switchRowText — строка с переключателем, подпись под заголовком которой можно менять.
func switchRowText(title, sub string, on bool, changed func(bool)) (fyne.CanvasObject, *kit.SwitchWidget, textSetter) {
	subT := kit.ProseFlush(sub).(textSetter)
	sw := kit.NewSwitch(on, changed)
	content := container.New(kit.TightVBox{Gap: 5}, kit.RowText(title, "", theme.ColorNameForeground), subT)
	return kit.ListRowOf(content, sw, func() { sw.Tapped(nil) }), sw, subT
}

func adminPanelSub(on bool) string {
	if on {
		return L("A separate veil-admin container. Opens only over VPN: https://10.66.0.1/admin", "Отдельный контейнер veil-admin. Открывается только через VPN: https://10.66.0.1/admin")
	}
	return L("Not installed. /admin answers 404; it can be turned on later on the server page.", "Не ставить. /admin будет отвечать 404; включить можно позже на странице сервера.")
}

func (u *ui) showDeployForm() { u.deployForm(0) }

// deployForm открывает форму с выбранным протоколом sel (индекс в deployProtos).
func (u *ui) deployForm(sel int) *subPage {
	w := u.newSubPage(L("Your own server over SSH", "Свой сервер по SSH"))
	w.setMaxWidth(wideColumns)
	protos := deployProtos()
	name := widget.NewEntry()
	name.SetPlaceHolder(L("For example: Amsterdam", "Например: Амстердам"))
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
	langName := "Русский"
	if L("en", "ru") == "en" {
		langName = "English"
	}
	method := methodPassword()
	frontPick := newFrontPicker(u, "")
	admin := true
	var running *installView // идущая установка: повторное нажатие не запускает вторую

	// пояснение под полями: что будет на сервере
	info := kit.NewFlowText("", 13, kit.ColText, false, 18.85)
	updateInfo := func() {
		p := protos[sel]
		ctr := p.ctr
		if p.key == "veil" && admin {
			ctr += L(", the panel in veil-admin", ", панель — в veil-admin")
		}
		info.SetText(fmt.Sprintf(L("The app will check the system and install Docker if it is missing. %s will run in the %s container (where Docker is unavailable, as a system service). If the protocol is already on the server, only a new client is issued. The SSH password is not stored.",
			"Приложение проверит систему и, если Docker нет, установит его. %s запустится в контейнере %s (где Docker недоступен — системной службой). Если протокол на сервере уже есть, просто выдаст нового клиента. Пароль SSH не сохраняется."), p.name, ctr))
	}

	// под списком протоколов: админ-панель и сайт-прикрытие (только Veil) или пояснение
	var adminSub textSetter
	var adminRow fyne.CanvasObject
	adminRow, _, adminSub = switchRowText(L("Admin panel", "Админ-панель"), adminPanelSub(admin), admin, func(on bool) {
		admin = on
		adminSub.SetText(adminPanelSub(on))
		updateInfo()
	})
	langField := u.fieldSelect(L("Cover site language", "Язык сайта-прикрытия"), langName, []string{"Русский", "English"}, func(s string) { langName = s })
	otherText := kit.NewFlowText("", 13, kit.ColText, false, 18.85)
	otherNote := noteText("info", otherText)
	extra := kit.NewVBoxFit()
	renderExtra := func() {
		extra.RemoveAll()
		p := protos[sel]
		if p.key == "veil" {
			extra.Add(adminRow)
			extra.Add(kit.SectionLabel(L("Cover site", "Сайт-прикрытие")))
			extra.Add(langField)
			extra.Add(frontPick.View())
		} else {
			otherText.SetText(fmt.Sprintf(L("Only Veil has a web admin panel. Manage %s clients from the app: server page → “Protocol”.",
				"Веб-админ-панель есть только у Veil. Клиентами %s управляйте из приложения: страница сервера → «Протокол»."), p.name))
			extra.Add(otherNote)
		}
		extra.Refresh()
	}

	left := kit.NewVBoxFit()
	left.Add(kit.SectionLabel(L("Protocol", "Протокол")))
	var marks []func(bool)
	for i, p := range protos {
		i := i
		row, mark := choiceRow(p.name, L(p.en, p.ru), i == sel, func() {
			sel = i
			for j, m := range marks {
				m(j == sel)
			}
			renderExtra()
			updateInfo()
			left.Refresh()
		})
		marks = append(marks, mark)
		left.Add(row)
	}
	left.Add(extra)

	authBox := kit.NewVBoxFit()
	renderAuth := func() {
		authBox.RemoveAll()
		if method == methodKey() {
			authBox.Add(fieldEntry(L("SSH key", "SSH-ключ"), keyPath))
			authBox.Add(bigButton(L("Choose key file…", "Выбрать файл ключа…"), "file", false, func() {
				dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
					if err == nil && r != nil {
						keyPath.SetText(r.URI().Path())
						r.Close()
					}
				}, w).Show()
			}))
			authBox.Add(fieldEntry(L("Key passphrase", "Пароль ключа"), passphrase))
		} else {
			authBox.Add(fieldEntry(L("Password", "Пароль"), password))
		}
		authBox.Refresh()
	}
	renderAuth()
	segment := kit.NewSegmented([]string{methodPassword(), methodKey()}, 0, func(i int) {
		method = []string{methodPassword(), methodKey()}[i]
		renderAuth()
	})

	install := bigButton(L("Install and connect", "Установить и подключить"), "download", true, func() {
		req, err := buildRequest(name.Text, ipv4.Text, ipv6.Text, port.Text, user.Text, method, password.Text, keyPath.Text, passphrase.Text)
		if err != nil {
			kit.ShowError(err, w)
			return
		}
		p := protos[sel]
		if p.key == "veil" {
			req.SiteLang = siteLangs[langName]
			sni, err := frontPick.SNI()
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			req.FrontSNI = sni
			req.AdminPanel = admin
		}
		if u.profileExists(req.Name) {
			kit.ShowError(fmt.Errorf(L("server “%s” already exists — choose a different name", "сервер «%s» уже есть — выберите другое название"), req.Name), w)
			return
		}
		if running != nil && !running.finished && !running.failed {
			running.w.Show()
			return
		}
		running = u.openInstall(w, installJob{req: req, proto: p.key, admin: p.key == "veil" && admin})
	})

	right := kit.NewVBoxFit()
	right.Add(fieldEntry(L("Name", "Название"), name))
	right.Add(container.New(&splitCols{ratio: 0.5, gap: 10}, fieldEntry("IPv6", ipv6), fieldEntry("IPv4", ipv4)))
	right.Add(container.New(&splitCols{ratio: 0.42, gap: 10}, fieldEntry(L("SSH port", "SSH-порт"), port), fieldEntry(L("User", "Пользователь"), user)))
	right.Add(segment)
	right.Add(authBox)
	right.Add(noteText("info", info))
	right.Add(install)

	renderExtra()
	updateInfo()
	w.SetContent(container.NewPadded(scrolled(newSplit(0, 0.5, 520, left, right))))
	w.Show()
	return w
}
