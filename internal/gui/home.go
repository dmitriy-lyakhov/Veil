package gui

import (
	"fmt"
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
	u.power.OnState = func(powerState) { u.refreshPill() }
	u.status = text(L("Disconnected", "Отключено"), 16, colText, true)
	u.status.Alignment = fyne.TextAlignCenter
	u.sub = text(L("Pick a server and press the button", "Выберите сервер и нажмите кнопку"), 13, colMuted, false)
	u.sub.Alignment = fyne.TextAlignCenter
	u.traffic = text("", 13, colMuted, false)
	u.traffic.Alignment = fyne.TextAlignCenter

	u.splitTxt = text("", 13, colText, false)
	splitIcon := iconImg("split", colText, 18)
	splitChev := iconImg("down", colMuted, 18)
	split := centered(newTapCardR(container.NewHBox(container.NewCenter(splitIcon), u.splitTxt, container.NewCenter(splitChev)), colCard, colCardHover, 14, 12, u.openSplitSheet))
	u.refreshSplitLink()

	u.recheck = widget.NewButtonWithIcon(L("Recheck the nearest entry", "Перепроверить ближайший вход"), theme.ViewRefreshIcon(), func() {
		if p := u.current(); p != nil {
			u.recheckEntries(p)
		}
	})
	u.recheck.Hide()
	hero := container.NewVBox(layout.NewSpacer(), centered(u.power), u.status, u.sub, u.traffic, centered(u.recheck), split, layout.NewSpacer())

	// узкий вид: карточка сервера внизу по центру
	u.srvName = newFitText(L("No server selected", "Сервер не выбран"), 28, colText, true)
	u.srvAddr = newFitText(L("Add a server with the + button", "Добавьте сервер кнопкой «+»"), 13, colMuted, false)
	u.srvAddr.Alignment = fyne.TextAlignCenter
	bottom := newTapCardR(container.NewVBox(nameWithChevron(u.srvName, container.NewCenter(iconImg("down", colMuted, 22)), true), u.srvAddr),
		colCard, colCardHover, 16, 16, u.openServerSheet)

	// широкий вид: карточка слева и плитки статистики
	u.srvNameW = newFitText(L("No server selected", "Сервер не выбран"), 20, colText, true)
	u.srvAddrW = newFitText(L("Add a server with the + button", "Добавьте сервер кнопкой «+»"), 13, colMuted, false)
	srvW := newTapCardR(container.NewVBox(nameWithChevron(u.srvNameW, container.NewCenter(iconImg("down", colMuted, 22)), false), u.srvAddrW),
		colCard, colCardHover, 14, 16, u.openServerSheet)
	u.stats = [4]*statTile{
		newStatTile(L("Download", "Загрузка"), L("MB/s", "МБ/с")),
		newStatTile(L("Upload", "Отдача"), L("MB/s", "МБ/с")),
		newStatTile(L("Latency", "Задержка"), L("ms", "мс")),
		newStatTile(L("Session", "Сессия"), ""),
	}
	grid := container.New(&statsLayout{}, u.stats[0].obj, u.stats[1].obj, u.stats[2].obj, u.stats[3].obj)
	side := container.NewVBox(srvW, grid)

	title := text(L("Connect", "Подключение"), 24, colText, true)
	logoName := text("Veil", 20, colText, true)
	logo := container.NewHBox(markImg(28), container.NewCenter(logoName))
	hl := &headerLeft{title: title, logo: logo}
	header := container.New(layout.NewCustomPaddedLayout(0, 0, 24, 24),
		container.NewBorder(nil, nil, container.New(hl, title, logo), container.NewCenter(u.statusPill()), nil))
	return container.New(&homeLayout{header: header, hero: hero, side: side, bottom: bottom, power: u.power, hl: hl}, header, hero, side, bottom)
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
	u.splitTxt.Text = L("Split access", "Раздельный доступ")
	if path, err := routing.Path(); err == nil {
		if cfg, err := routing.Load(path); err == nil && cfg.Enabled && !cfg.Empty() {
			d1, p1 := cfg.Via.Count()
			d2, p2 := cfg.Direct.Count()
			u.splitTxt.Text += fmt.Sprintf(" · %d", d1+p1+d2+p2)
		}
	}
	u.splitTxt.Refresh()
}

func (u *ui) openSplitSheet() {
	var s *sheet
	state := L("Off", "Отключено")
	if u.splitEnabled() {
		state = L("On", "Включено")
	}
	desc := prose(L("Route some sites and addresses through the VPN and the rest directly", "Позволяет направлять через VPN одни сайты и адреса, а другие — напрямую"))
	content := container.NewVBox(
		lead(text(L("Split tunneling", "Раздельное туннелирование"), 20, colText, false)),
		desc,
		navRow(L("By sites and addresses", "По сайтам и адресам"), state, func() {
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
		title := newFitText(p.Name, 22, colText, false)
		title.Alignment = fyne.TextAlignCenter
		addr := newFitText(addresses(p), 13, colMuted, false)
		addr.Alignment = fyne.TextAlignCenter
		head.Add(title)
		head.Add(addr)
	}
	head.Add(sectionLabel(L("Servers", "Серверы")))
	if len(u.profiles) == 0 {
		box.Add(richText(L("No servers yet. Press + below to add one.", "Серверов пока нет. Нажмите «+» внизу, чтобы добавить."), true, false))
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
			container.NewCenter(gear), vcenter(rowFit(p.Name, addresses(p), colText)))
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
