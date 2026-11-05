package gui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

// statTile — плитка статистики: подпись, значение с единицей и пояснение.
type statTile struct {
	label, val, unit, delta *canvas.Text
	obj                     fyne.CanvasObject
}

func newStatTile(label, unit string, tap func()) *statTile {
	t := &statTile{
		label: text(label, 12, colMuted, false),
		val:   text("—", 22, colText, true),
		unit:  text(unit, 12, colMuted, false),
		delta: text("", 12, colOK, false),
	}
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	body := container.NewVBox(t.label, container.NewHBox(t.val, t.unit), t.delta)
	t.obj = container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), body))
	if tap != nil {
		t.obj = newRowTap(t.obj, tap)
	}
	return t
}

func (t *statTile) set(val, delta string) {
	t.val.Text, t.delta.Text = val, delta
	t.val.Refresh()
	t.delta.Refresh()
}

// statsLayout — сетка плиток: 2 столбца, а при большой ширине 4. Запоминает ширину последней раскладки,
// чтобы MinSize считал высоту по тому же числу столбцов, что и Layout.
type statsLayout struct{ lastW float32 }

func (*statsLayout) cols(w float32) int {
	if w >= 440 {
		return 4
	}
	return 2
}

func (l *statsLayout) gap(w float32) float32 {
	if l.cols(w) == 4 {
		return 8
	}
	return 12
}

func (l *statsLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	l.lastW = s.Width
	c, g := l.cols(s.Width), l.gap(s.Width)
	cw := (s.Width - g*float32(c-1)) / float32(c)
	var rowH, y float32
	for i, o := range objs {
		col := i % c
		if col == 0 && i > 0 {
			y += rowH + g
			rowH = 0
		}
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(cw, h))
		o.Move(fyne.NewPos(float32(col)*(cw+g), y))
		rowH = max(rowH, h)
	}
}

func (l *statsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	c := l.cols(l.lastW)
	g := l.gap(l.lastW)
	var h, rowH float32
	for i, o := range objs {
		if i%c == 0 && i > 0 {
			h += rowH + g
			rowH = 0
		}
		rowH = max(rowH, o.MinSize().Height)
	}
	return fyne.NewSize(200, h+rowH)
}

// homeLayout раскладывает главный экран: на узком окне кольцо сверху и карточка сервера внизу,
// на широком (от 720 px окна) — заголовок со статусом, кольцо слева и колонка сервера со статистикой справа.
type homeLayout struct {
	header, hero, side, bottom fyne.CanvasObject
	power                      *powerButton
	hl                         *headerLeft
}

// headerLeft показывает слева в шапке главной: заголовок на широком окне, логотип на узком.
type headerLeft struct {
	title, logo fyne.CanvasObject
	wide        bool
}

func (h *headerLeft) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	o := h.logo
	if h.wide {
		o = h.title
		h.logo.Hide()
	} else {
		h.title.Hide()
	}
	o.Show()
	m := o.MinSize()
	o.Resize(m)
	o.Move(fyne.NewPos(0, (s.Height-m.Height)/2))
}

func (h *headerLeft) MinSize([]fyne.CanvasObject) fyne.Size {
	if h.wide {
		return h.title.MinSize()
	}
	return h.logo.MinSize()
}

func (l *homeLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(300, 320) }

func (l *homeLayout) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	wide := s.Width >= wideBreakpoint-sidebarWidth
	l.hl.wide = wide
	if !wide {
		l.header.Show()
		l.header.Resize(fyne.NewSize(s.Width, 56))
		l.header.Move(fyne.NewPos(0, 0))
		l.side.Hide()
		l.bottom.Show()
		bh := l.bottom.MinSize().Height
		l.bottom.Resize(fyne.NewSize(s.Width, bh))
		l.bottom.Move(fyne.NewPos(0, s.Height-bh))
		avail := s.Height - bh - 56
		l.hero.Resize(fyne.NewSize(s.Width, avail))
		for _, d := range []float32{196, 168, 140, 120, 100, 84} {
			l.power.SetDiameter(d)
			if l.hero.MinSize().Height <= avail {
				break
			}
		}
		l.hero.Resize(fyne.NewSize(s.Width, avail))
		l.hero.Move(fyne.NewPos(0, 56))
		return
	}
	const hdr = 72
	narrow := s.Width+sidebarWidth < 900
	l.bottom.Hide()
	l.header.Show()
	l.side.Show()
	l.header.Resize(fyne.NewSize(s.Width, hdr))
	l.header.Move(fyne.NewPos(0, 0))
	if narrow {
		sw := s.Width - 48
		l.side.Resize(fyne.NewSize(sw, l.side.MinSize().Height))
		sh := l.side.MinSize().Height
		l.side.Resize(fyne.NewSize(sw, sh))
		l.side.Move(fyne.NewPos(24, s.Height-16-sh))
		avail := s.Height - 16 - sh - hdr
		l.hero.Resize(fyne.NewSize(s.Width, avail))
		for _, d := range []float32{168, 140, 120, 100, 84} {
			l.power.SetDiameter(d)
			if l.hero.MinSize().Height <= avail {
				break
			}
		}
		l.hero.Resize(fyne.NewSize(s.Width, avail))
		l.hero.Move(fyne.NewPos(0, hdr))
		return
	}
	const sw = 300
	sh := l.side.MinSize().Height
	l.side.Resize(fyne.NewSize(sw, sh))
	l.side.Move(fyne.NewPos(s.Width-24-sw, hdr+(s.Height-hdr-24-sh)/2))
	l.power.SetDiameter(232)
	l.hero.Resize(fyne.NewSize(s.Width-24-sw-24, s.Height-hdr-24))
	l.hero.Move(fyne.NewPos(0, hdr))
}

// statusPill — плашка «● Подключено» в правом верхнем углу.
func (u *ui) statusPill() fyne.CanvasObject {
	dotC := canvas.NewCircle(color.Transparent)
	dotC.StrokeWidth = 2
	txtC := text("", 13, colText, true)
	u.pills = append(u.pills, pillRef{dotC, txtC})
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 20
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	dot := container.NewGridWrap(fyne.NewSize(10, 10), dotC)
	row := container.NewHBox(container.NewCenter(dot), txtC)
	u.refreshPill()
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(8, 8, 14, 14), row))
}

func (u *ui) refreshPill() {
	for _, p := range u.pills {
		switch {
		case u.power != nil && u.power.state == powerOn:
			p.dot.FillColor, p.dot.StrokeColor = colOK, colOK
			p.txt.Text = L("Connected", "Подключено")
		case u.power != nil && u.power.state == powerBusy:
			p.dot.FillColor, p.dot.StrokeColor = color.Transparent, colAccent
			p.txt.Text = L("Connecting…", "Подключение…")
		default:
			p.dot.FillColor, p.dot.StrokeColor = color.Transparent, colMuted
			p.txt.Text = L("Disconnected", "Отключено")
		}
		p.dot.Refresh()
		p.txt.Refresh()
	}
}

type pillRef struct {
	dot *canvas.Circle
	txt *canvas.Text
}

// updateStats обновляет плитки статистики по отчёту о подключении.
func (u *ui) updateStats(rx, tx uint64, session string, latency float64) {
	now := timeNow()
	t := &u.tr
	if t.store == nil {
		t.store = loadTrafficStore()
	}
	if !u.statAt.IsZero() && now.After(u.statAt) && u.statOK {
		dt := now.Sub(u.statAt).Seconds()
		delta := func(cur, prev uint64) uint64 {
			if cur < prev {
				return 0
			}
			return cur - prev
		}
		dr, dtx := delta(rx, u.rxPrev), delta(tx, u.txPrev)
		t.rxRate, t.txRate = float64(dr)/dt, float64(dtx)/dt
		t.push(t.rxRate)
		t.store.add(dr, dtx, now)
		u.stats[0].set(fmtMB(t.rxRate/(1<<20)), L("▼ incoming", "▼ входящий"))
		u.stats[1].set(fmtMB(t.txRate/(1<<20)), L("▲ outgoing", "▲ исходящий"))
	}
	u.statAt, u.rxPrev, u.txPrev, u.statOK = now, rx, tx, true
	t.rx, t.tx, t.session, t.latency = rx, tx, session, latency
	if latency > 0 {
		u.stats[2].set(fmt.Sprintf("%.0f", latency), L("▲ stable", "▲ стабильно"))
		if int(latency) != t.trayLat {
			t.trayLat = int(latency)
			u.updateTray()
		}
	}
	u.stats[3].set(session, "")
	if t.refresh != nil {
		t.refresh()
	}
}

func (u *ui) resetStats() {
	u.tr.reset()
	u.statOK = false
	for _, t := range u.stats {
		if t != nil {
			t.set("—", "")
		}
	}
}
