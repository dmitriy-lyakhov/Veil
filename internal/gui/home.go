package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/routing"
)

// homePage — главный экран как в референсе: кольцо подключения, ссылка на раздельное
// туннелирование и внизу карточка выбранного сервера (по нажатию открывается выбор).
func (u *ui) homePage() fyne.CanvasObject {
	u.power = newPowerButton(u.toggle)
	u.status = text("Отключено", 20, colText, true)
	u.status.Alignment = fyne.TextAlignCenter
	u.sub = text("Выберите сервер и нажмите кнопку", 13, colMuted, false)
	u.sub.Alignment = fyne.TextAlignCenter
	u.traffic = text("", 12, colMuted, false)
	u.traffic.Alignment = fyne.TextAlignCenter

	u.splitTxt = text("", 13, colMuted, false)
	split := newRowTap(container.NewPadded(container.NewHBox(layout.NewSpacer(),
		widget.NewIcon(theme.MailForwardIcon()), u.splitTxt, widget.NewIcon(theme.MenuDropDownIcon()), layout.NewSpacer())), u.openSplitSheet)
	u.refreshSplitLink()

	u.srvName = text("Сервер не выбран", 22, colText, false)
	u.srvAddr = text("Добавьте сервер кнопкой «+»", 13, colMuted, false)
	u.srvAddr.Alignment = fyne.TextAlignCenter
	srvTitle := container.NewHBox(layout.NewSpacer(), u.srvName, widget.NewIcon(theme.MenuDropDownIcon()), layout.NewSpacer())
	server := newTapCard(container.NewVBox(srvTitle, u.srvAddr), colCard, colCardHover, u.openServerSheet)

	u.recheck = widget.NewButtonWithIcon("Перепроверить ближайший вход", theme.ViewRefreshIcon(), func() {
		if p := u.current(); p != nil {
			u.recheckEntries(p)
		}
	})
	u.recheck.Hide()
	return container.NewPadded(container.NewBorder(
		nil,
		container.NewVBox(u.recheck, split, server),
		nil, nil,
		container.NewVBox(layout.NewSpacer(), centered(u.power), u.status, u.sub, u.traffic, layout.NewSpacer()),
	))
}

func (u *ui) splitEnabled() bool {
	path, err := routing.Path()
	if err != nil {
		return false
	}
	cfg, err := routing.Load(path)
	return err == nil && cfg.Enabled && !cfg.Empty()
}

func (u *ui) refreshSplitLink() {
	if u.splitTxt == nil {
		return
	}
	if u.splitEnabled() {
		u.splitTxt.Text = "Раздельное туннелирование включено"
	} else {
		u.splitTxt.Text = "Раздельное туннелирование отключено"
	}
	u.splitTxt.Refresh()
}

func (u *ui) openSplitSheet() {
	var s *sheet
	state := "Отключено"
	if u.splitEnabled() {
		state = "Включено"
	}
	desc := prose("Позволяет направлять через VPN одни сайты и адреса, а другие — напрямую")
	content := container.NewVBox(
		lead(text("Раздельное туннелирование", 20, colText, false)),
		desc,
		navRow("По сайтам и адресам", state, func() {
			s.Close()
			u.openSplit()
		}),
	)
	s = showSheet(u.win, content)
}

// openServerSheet — нижняя панель выбора сервера: радиокнопки и шестерёнка управления.
func (u *ui) openServerSheet() {
	var s *sheet
	// заголовок вне прокрутки: иначе при длинном списке он уезжает вверх и под ним появляется тень прокрутки
	head := container.NewVBox()
	box := container.NewVBox()
	if p := u.current(); p != nil {
		title := text(p.Name, 22, colText, false)
		title.Alignment = fyne.TextAlignCenter
		addr := text(addresses(p), 13, colMuted, false)
		addr.Alignment = fyne.TextAlignCenter
		head.Add(title)
		head.Add(addr)
	}
	head.Add(sectionLabel("Серверы"))
	if len(u.profiles) == 0 {
		box.Add(richText("Серверов пока нет. Нажмите «+» внизу, чтобы добавить.", true, false))
	}
	for _, p := range u.profiles {
		p := p
		icon := theme.RadioButtonIcon()
		if p.Name == u.selected {
			icon = theme.RadioButtonCheckedIcon()
		}
		gear := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
			s.Close()
			u.openServer(p)
		})
		gear.Importance = widget.LowImportance
		row := container.NewBorder(nil, nil, container.NewCenter(widget.NewIcon(icon)),
			container.NewCenter(gear), vcenter(rowText(p.Name, addresses(p), "foreground")))
		line := canvas.NewRectangle(colLine)
		line.SetMinSize(fyne.NewSize(1, 1))
		tap := newRowTap(container.NewPadded(row), func() {
			u.selectProfile(p.Name)
			s.Close()
		})
		box.Add(container.NewVBox(tap, line))
	}
	s = showSheet(u.win, container.NewBorder(head, nil, nil, nil, scrollMax(box, 300)))
}

// openServer открывает страницу сервера: управление для установленных из приложения, сведения для остальных.
func (u *ui) openServer(p *config.Profile) {
	if p.Protocol == "amneziawg" {
		u.openAWGManage(p)
		return
	}
	if p.Protocol == "openvpn" {
		u.openOVManage(p)
		return
	}
	if p.Protocol == "ipsec" {
		u.openIPsecManage(p)
		return
	}
	if !p.IsVeil() {
		u.openWGManage(p)
		return
	}
	if managed(p) {
		u.openManager(p)
		return
	}
	u.showDetails(p)
}
