package gui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/helper"
)

const (
	appID        = "net.veil.client"
	appVersion   = "0.4.0"
	pollInterval = time.Second
	readyTimeout = 3 * time.Minute
	prefSelected = "selected"
	winWidth     = 420
	winHeight    = 780
)

type ui struct {
	app      fyne.App
	win      fyne.Window
	pages    *fyne.Container
	navBtns  []*widget.Button
	profiles []*config.Profile
	selected string

	power   *powerButton
	status  *canvas.Text
	sub     *canvas.Text
	rx, tx  *canvas.Text
	srvName *canvas.Text
	srvAddr *canvas.Text
	recheck *widget.Button

	serverList *fyne.Container
	logPath    *widget.Label

	sessions  map[string]*deploy.Session
	handle    *helper.Handle
	active    string
	connected bool
	since     time.Time
	stop      chan struct{}
}

func Run() {
	a := app.NewWithID(appID)
	a.Settings().SetTheme(veilTheme{})
	if icon := bundledIcon(); icon != nil {
		a.SetIcon(icon)
	}
	u := &ui{app: a, win: a.NewWindow("Veil"), selected: a.Preferences().String(prefSelected), sessions: map[string]*deploy.Session{}}
	u.pages = container.NewStack(u.homePage(), u.serversPage(), u.settingsPage())
	u.win.SetContent(container.NewBorder(nil, u.navBar(theme.HomeIcon(), theme.StorageIcon(), theme.SettingsIcon()), nil, nil, u.pages))
	u.showPage(0)
	u.reload()
	u.win.SetCloseIntercept(func() {
		for name := range u.sessions {
			u.dropSession(name)
		}
		u.disconnect()
		u.win.Close()
	})
	u.win.Resize(fyne.NewSize(winWidth, winHeight))
	u.win.ShowAndRun()
}

const (
	navButtonSize = 52
	navGap        = 28
)

func (u *ui) navBar(icons ...fyne.Resource) fyne.CanvasObject {
	row := container.NewHBox(layout.NewSpacer())
	for i, ic := range icons {
		if i > 0 {
			gap := canvas.NewRectangle(color.Transparent)
			gap.SetMinSize(fyne.NewSize(navGap, 1))
			row.Add(gap)
		}
		b := widget.NewButtonWithIcon("", ic, func() { u.showPage(i) })
		u.navBtns = append(u.navBtns, b)
		row.Add(container.NewGridWrap(fyne.NewSize(navButtonSize, navButtonSize), b))
	}
	row.Add(layout.NewSpacer())
	line := canvas.NewRectangle(colLine)
	line.SetMinSize(fyne.NewSize(1, 1))
	return container.NewVBox(line, container.NewPadded(row))
}

func (u *ui) showPage(i int) {
	for n, o := range u.pages.Objects {
		if n == i {
			o.Show()
		} else {
			o.Hide()
		}
	}
	for n, b := range u.navBtns {
		if n == i {
			b.Importance = widget.HighImportance
		} else {
			b.Importance = widget.LowImportance
		}
		b.Refresh()
	}
	u.pages.Refresh()
}

func bundledIcon() fyne.Resource {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "..", "Resources", "icon.png"))
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource("icon.png", b)
}

func centered(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(layout.NewSpacer(), o, layout.NewSpacer())
}

func (u *ui) homePage() fyne.CanvasObject {
	brand := text("Veil", 20, colText, true)
	tagline := text("HTTPS-маскировка · IPv6 first", 12, colMuted, false)
	u.power = newPowerButton(u.toggle)
	u.status = text("Отключено", 26, colText, true)
	u.status.Alignment = fyne.TextAlignCenter
	u.sub = text("Выберите сервер и нажмите кнопку", 13, colMuted, false)
	u.sub.Alignment = fyne.TextAlignCenter

	u.rx = text("0 Б", 18, colText, true)
	u.tx = text("0 Б", 18, colText, true)
	stat := func(label string, v *canvas.Text) fyne.CanvasObject {
		return card(container.NewVBox(text(label, 12, colMuted, false), v))
	}
	stats := container.NewGridWithColumns(2, stat("↓ Получено", u.rx), stat("↑ Отправлено", u.tx))

	u.srvName = text("Сервер не выбран", 16, colText, true)
	u.srvAddr = text("Добавьте сервер на вкладке «Серверы»", 12, colMuted, false)
	chevron := widget.NewIcon(theme.NavigateNextIcon())
	server := newTapCard(container.NewBorder(nil, nil, widget.NewIcon(theme.StorageIcon()), chevron,
		container.NewVBox(u.srvName, u.srvAddr)), colCard, colCardHover, func() { u.showPage(1) })

	u.recheck = widget.NewButtonWithIcon("Перепроверить ближайший вход", theme.ViewRefreshIcon(), func() {
		if p := u.current(); p != nil {
			u.recheckEntries(p)
		}
	})
	u.recheck.Hide()
	return container.NewPadded(container.NewBorder(
		container.NewVBox(brand, tagline),
		container.NewVBox(u.recheck, stats, server),
		nil, nil,
		container.NewVBox(layout.NewSpacer(), centered(u.power), u.status, u.sub, layout.NewSpacer()),
	))
}

func (u *ui) reload() {
	ps, err := config.ListProfiles()
	if err != nil {
		u.toast("Ошибка: " + err.Error())
	}
	u.profiles = ps
	if u.current() == nil && len(ps) > 0 {
		u.selected = ps[0].Name
	}
	u.app.Preferences().SetString(prefSelected, u.selected)
	u.rebuildServers()
	u.refreshHome()
}

func (u *ui) current() *config.Profile {
	for _, p := range u.profiles {
		if p.Name == u.selected {
			return p
		}
	}
	return nil
}

func (u *ui) selectProfile(name string) {
	u.selected = name
	u.app.Preferences().SetString(prefSelected, name)
	u.rebuildServers()
	u.refreshHome()
}

func addresses(p *config.Profile) string {
	if len(p.Entries) > 0 {
		names := make([]string, len(p.Entries))
		for i, e := range p.Entries {
			names[i] = e.Name
		}
		return fmt.Sprintf("входы: %s → выход: %s", strings.Join(names, ", "), p.ExitName)
	}
	var parts []string
	if p.IPv6 != "" {
		parts = append(parts, p.IPv6)
	}
	if p.IPv4 != "" {
		parts = append(parts, p.IPv4)
	}
	return strings.Join(parts, " · ")
}

func (u *ui) refreshHome() {
	p := u.current()
	if p == nil {
		u.srvName.Text, u.srvAddr.Text = "Сервер не выбран", "Добавьте сервер на вкладке «Серверы»"
	} else {
		u.srvName.Text, u.srvAddr.Text = p.Name, addresses(p)
	}
	u.srvName.Refresh()
	u.srvAddr.Refresh()
	if p != nil && len(p.Entries) > 0 {
		u.recheck.Show()
	} else {
		u.recheck.Hide()
	}
	if u.handle == nil {
		u.power.SetDisabled(p == nil)
		u.power.SetState(powerOff)
	}
}

func (u *ui) setStatus(title, sub string, subColor fyne.ThemeColorName) {
	u.status.Text = title
	u.sub.Text = sub
	u.sub.Color = colMuted
	if subColor == theme.ColorNameError {
		u.sub.Color = colBad
	}
	u.status.Refresh()
	u.sub.Refresh()
}

func (u *ui) toast(msg string) {
	u.setStatus(u.status.Text, msg, theme.ColorNameError)
}

func (u *ui) toggle() {
	if u.handle != nil {
		u.disconnect()
		return
	}
	p := u.current()
	if p == nil {
		u.showPage(1)
		return
	}
	path, err := config.ProfilePath(p.Name)
	if err != nil {
		u.toast(err.Error())
		return
	}
	u.power.SetState(powerBusy)
	u.setStatus("Подключение…", "Подтвердите права администратора", "")
	name := p.Name
	go func() {
		h, err := helper.Launch(path)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), readyTimeout)
			err = h.WaitReady(ctx)
			cancel()
		}
		fyne.Do(func() {
			if err != nil {
				u.power.SetState(powerOff)
				u.setStatus("Отключено", err.Error(), theme.ColorNameError)
				return
			}
			u.handle, u.active, u.connected = h, name, false
			u.stop = make(chan struct{})
			u.rebuildServers()
			go u.poll(h, u.stop)
		})
	}()
}

func (u *ui) poll(h *helper.Handle, stop chan struct{}) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	failures := 0
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		r, err := h.Status()
		if err != nil {
			failures++
			if failures < 3 {
				continue
			}
			r = &helper.Report{Error: "помощник туннеля завершился, журнал: " + h.Log}
		}
		failures = 0
		fyne.Do(func() { u.showReport(r) })
		if r.Error != "" || r.State == client.StateStopped {
			fyne.Do(func() { u.finish(r.Error) })
			return
		}
	}
}

func (u *ui) showReport(r *helper.Report) {
	switch r.State {
	case client.StateConnecting:
		u.power.SetState(powerBusy)
		u.setStatus("Подключение…", "Ищем лучший маршрут: IPv6 → IPv4, HTTP/3 → HTTP/2", "")
	case client.StateConnected:
		if !u.connected {
			u.connected, u.since = true, time.Now()
			u.rebuildServers()
		}
		u.power.SetState(powerOn)
		elapsed := time.Since(u.since).Truncate(time.Second)
		sub := fmt.Sprintf("%s · %s · %s", transportShort(r.Transport), r.Family, clock(elapsed))
		if r.Entry != "" {
			sub = fmt.Sprintf("вход %s (%.0f мс) → выход %s\n%s", r.Entry, r.EntryRTT, r.Exit, sub)
		}
		u.setStatus("Подключено", sub, "")
	case client.StateReconnecting:
		u.power.SetState(powerBusy)
		u.setStatus("Переподключение…", r.LastError, "")
	}
	u.rx.Text, u.tx.Text = humanBytes(r.RxBytes), humanBytes(r.TxBytes)
	u.rx.Refresh()
	u.tx.Refresh()
}

func (u *ui) disconnect() {
	if u.handle == nil {
		return
	}
	_ = u.handle.Stop()
	u.finish("")
}

func (u *ui) finish(errMsg string) {
	if u.stop != nil {
		close(u.stop)
		u.stop = nil
	}
	u.handle, u.active, u.connected = nil, "", false
	u.power.SetState(powerOff)
	if errMsg != "" {
		u.setStatus("Отключено", errMsg, theme.ColorNameError)
	} else {
		u.setStatus("Отключено", "Нажмите кнопку, чтобы подключиться", "")
	}
	u.rebuildServers()
	u.refreshHome()
}

func (u *ui) settingsPage() fyne.CanvasObject {
	u.logPath = widget.NewLabel("")
	u.logPath.Wrapping = fyne.TextWrapBreak
	if dir, err := config.ProfilesDir(); err == nil {
		u.logPath.SetText("Профили: " + dir)
	}
	about := widget.NewLabel("Ядро WireGuard внутри HTTPS на порту 443. Основной транспорт — HTTP/3 (QUIC) с отпечатком Chrome, запасной — HTTP/2 поверх TLS 1.3. Сначала IPv6, затем IPv4.")
	about.Wrapping = fyne.TextWrapWord
	return container.NewPadded(container.NewVBox(
		text("Настройки", 22, colText, true),
		card(container.NewVBox(text("О приложении", 15, colText, true), text("Veil "+appVersion, 13, colMuted, false), about)),
		card(container.NewVBox(text("Данные", 15, colText, true), u.logPath)),
	))
}

func transportShort(kind string) string {
	switch client.Kind(kind) {
	case client.KindH3:
		return "HTTP/3 · QUIC"
	case client.KindH2:
		return "HTTP/2 · TLS"
	}
	return kind
}

func clock(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d Б", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cБ", float64(n)/float64(div), []rune("КМГТ")[exp])
}
