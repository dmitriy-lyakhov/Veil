package gui

import (
	"context"
	_ "embed"
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

	"veil/internal/app/gui/i18n"
	"veil/internal/app/gui/kit"
	"veil/internal/app/gui/platform"
	"veil/internal/app/helper"
	"veil/internal/app/settings"
	"veil/internal/client"
	"veil/internal/client/dialer"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/transport/netpin"
)

const (
	appID        = "net.veil.client"
	appVersion   = "1.0.0"
	pollInterval = time.Second
	readyTimeout = 3 * time.Minute
	prefSelected = "selected"
	winWidth     = 1000
	winHeight    = 700
	winMinWidth  = 360
	winMinHeight = 560
)

type ui struct {
	app      fyne.App
	win      fyne.Window
	pages    *fyne.Container
	navItems []*navItem
	profiles []*config.Profile
	selected string

	power   *kit.PowerButton
	status  *canvas.Text
	sub     *canvas.Text
	rx, tx  *canvas.Text
	srvName *kit.FitText
	srvAddr *kit.FitText
	recheck *widget.Button

	serverList *fyne.Container
	logPath    *widget.Label

	sessions   map[string]*deploy.Session
	handle     *helper.Handle
	active     string
	connected  bool
	since      time.Time
	adminHosts string // клиент записал «admin» в hosts ("ok"), не смог ("failed") или не требовалось ("")
	stop       chan struct{}

	curtain   *canvas.Rectangle
	pageAnim  *fyne.Animation
	pageShown bool
	curPage   int
	curtainA  float32
	routeLbl  *widget.Label

	traffic            *canvas.Text
	splitTxt           *canvas.Text
	srvNameW, srvAddrW *kit.FitText
	pills              []pillRef
	subpages           []*subPage
	stats              [4]*statTile
	statAt             time.Time
	rxPrev, txPrev     uint64
	statOK             bool
	tr                 trafficState
	trayReady          bool
	serversView        fyne.CanvasObject
	st                 *settings.Settings
	share              *shareState
}

func (u *ui) buildPages() {
	u.pills = nil
	u.curtain = canvas.NewRectangle(color.Transparent)
	u.serversView = u.serversPage()
	u.pages = container.NewStack(u.homePage(), u.sharePage(), u.settingsPage(), u.addPage(), u.curtain)
}

func Run() {
	release, first := platform.AcquireSingleInstance()
	if !first {
		platform.ActivateExisting()
		return
	}
	defer release()
	a := app.NewWithID(appID)
	a.Settings().SetTheme(kit.VeilTheme{})
	if icon := bundledIcon(); icon != nil {
		a.SetIcon(icon)
	}
	u := &ui{app: a, win: a.NewWindow("Veil"), selected: a.Preferences().String(prefSelected), sessions: map[string]*deploy.Session{}}
	u.loadSettings()
	kit.ApplyPalette(u.st.Dark())
	i18n.Lang = u.st.Language()
	u.buildPages()
	u.win.SetPadded(false)
	u.win.SetContent(u.buildShell())
	u.showPage(0)
	u.reload()
	u.win.SetCloseIntercept(u.closeWindow)
	u.afterStart()
	u.win.Resize(fyne.NewSize(winWidth, winHeight))
	u.win.ShowAndRun()
}

func (u *ui) showPage(i int) {
	pages := u.pages.Objects[:4]
	if u.pageShown && i == u.curPage {
		return
	}
	prev := u.curPage
	first := !u.pageShown
	for n, o := range pages {
		if n == i {
			o.Show()
		} else {
			o.Hide()
		}
	}
	u.pageShown, u.curPage = true, i
	u.setActiveNav(i)
	if !first {
		u.animatePage(pages[i], prev, i)
	}
}

//go:embed icon.png
var embeddedIcon []byte

func bundledIcon() fyne.Resource {
	if exe, err := os.Executable(); err == nil {
		if b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "..", "Resources", "icon.png")); err == nil {
			return fyne.NewStaticResource("icon.png", b)
		}
	}
	return fyne.NewStaticResource("icon.png", embeddedIcon)
}

func centered(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(layout.NewSpacer(), o, layout.NewSpacer())
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
	if u.share != nil {
		u.share.refreshServers()
	}
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
	name, addr := L("No server selected", "Сервер не выбран"), L("Add a server with the + button", "Добавьте сервер кнопкой «+»")
	if p != nil {
		name, addr = p.Name, addresses(p)
	}
	u.srvName.SetText(name)
	u.srvAddr.SetText(addr)
	u.srvNameW.SetText(name)
	u.srvAddrW.SetText(addr)
	if p != nil && len(p.Entries) > 0 {
		u.recheck.Show()
	} else {
		u.recheck.Hide()
	}
	if u.handle == nil {
		u.power.SetDisabled(p == nil)
		u.power.SetState(kit.PowerOff)
	}
}

func (u *ui) setStatus(title, sub string, subColor fyne.ThemeColorName) {
	u.status.Text = title
	u.sub.Text = sub
	u.sub.Color = kit.ColMuted
	if subColor == theme.ColorNameError {
		u.sub.Color = kit.ColBad
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
		u.showPage(3)
		return
	}
	path, err := config.ProfilePath(p.Name)
	if err != nil {
		u.toast(err.Error())
		return
	}
	if err := checkOpenVPNInstalled(p); err != nil {
		kit.ShowError(err, u.win)
		return
	}
	u.logf("подключение к «%s»", p.Name)
	u.power.SetState(kit.PowerBusy)
	// открытые SSH-сессии установлены без привязки к физическому интерфейсу:	// после подъёма туннеля их пакеты ушли бы внутрь него	for n := range u.sessions {		u.dropSession(n)	}
	u.setStatus(L("Connecting…", "Подключение…"), L("Confirm administrator rights", "Подтвердите права администратора"), "")
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
				u.power.SetState(kit.PowerOff)
				u.setStatus(L("Disconnected", "Отключено"), err.Error(), theme.ColorNameError)
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
	u.adminHosts = r.AdminHosts
	netpin.Set(r.PinV4, r.PinV6)
	switch r.State {
	case client.StateConnecting:
		u.power.SetState(kit.PowerBusy)
		u.setStatus(L("Connecting…", "Подключение…"), L("Finding the best route: IPv6 → IPv4, HTTP/3 → HTTP/2", "Ищем лучший маршрут: IPv6 → IPv4, HTTP/3 → HTTP/2"), "")
	case client.StateConnected:
		if !u.connected {
			u.connected, u.since = true, time.Now()
			u.logf("подключено: %s %s", r.Transport, r.Family)
			u.rebuildServers()
		}
		u.power.SetState(kit.PowerOn)
		elapsed := time.Since(u.since).Truncate(time.Second)
		u.power.SetTimer(clock(elapsed))
		u.updateStats(r.RxBytes, r.TxBytes, clock(elapsed), r.LatencyMS)
		sub := fmt.Sprintf("%s · %s · %s", transportShort(r.Transport), r.Family, clock(elapsed))
		if r.Entry != "" {
			sub = fmt.Sprintf(L("entry %s (%.0f ms) → exit %s\n%s", "вход %s (%.0f мс) → выход %s\n%s"), r.Entry, r.EntryRTT, r.Exit, sub)
			if len(r.Skipped) > 0 {
				sub += "\n" + L("Entries unavailable: ", "Недоступны входы: ") + strings.Join(r.Skipped, ", ")
			}
		}
		u.setStatus(L("Connected", "Подключено"), sub, "")
	case client.StateReconnecting:
		u.power.SetState(kit.PowerBusy)
		if r.Entry != "" {
			// каскад из нескольких входов: текущий перестал отвечать, клиент переходит на следующий
			sub := fmt.Sprintf(L("Entry “%s” is not responding, switching…", "Вход «%s» не отвечает, переключаемся…"), r.Entry)
			if next := nextEntry(u.activeProfile(), r.Entry, r.Skipped); next != "" {
				sub = fmt.Sprintf(L("Entry “%s” is not responding, switching to “%s”…", "Вход «%s» не отвечает, переключаемся на «%s»…"), r.Entry, next)
			}
			u.setStatus(L("Switching…", "Переключение…"), sub, "")
			break
		}
		u.setStatus(L("Reconnecting…", "Переподключение…"), r.LastError, "")
	}
	u.traffic.Text = "↓ " + humanBytes(r.RxBytes) + "     ↑ " + humanBytes(r.TxBytes)
	u.traffic.Refresh()
}

func (u *ui) disconnect() {
	if u.handle == nil {
		return
	}
	_ = u.handle.Stop()
	u.finish("")
}

func (u *ui) finish(errMsg string) {
	u.logf("отключено: %s", errMsg)
	if u.stop != nil {
		close(u.stop)
		u.stop = nil
	}
	u.handle, u.active, u.connected, u.adminHosts = nil, "", false, ""
	u.traffic.Text = ""
	u.traffic.Refresh()
	netpin.Set(0, 0)
	u.power.SetTimer("")
	u.resetStats()
	u.power.SetState(kit.PowerOff)
	if errMsg != "" {
		u.setStatus(L("Disconnected", "Отключено"), errMsg, theme.ColorNameError)
	} else {
		u.setStatus(L("Disconnected", "Отключено"), L("Press the button to connect", "Нажмите кнопку, чтобы подключиться"), "")
	}
	u.rebuildServers()
	u.refreshHome()
}

func transportShort(kind string) string {
	switch dialer.Kind(kind) {
	case dialer.KindH3:
		return "HTTP/3 · QUIC"
	case dialer.KindH2:
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

// activeProfile — профиль, к которому сейчас подключены (или идёт подключение).
func (u *ui) activeProfile() *config.Profile {
	for _, p := range u.profiles {
		if p.Name == u.active {
			return p
		}
	}
	return nil
}
