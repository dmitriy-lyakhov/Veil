package gui

import (
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

func newStatTile(label, unit string) *statTile {
	t := &statTile{
		label: text(label, 12, colMuted, false),
		val:   text("—", 22, colText, true),
		unit:  text(unit, 12, colMuted, false),
		delta: text("", 12, colOK, false),
	}
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	body := container.NewVBox(t.label, container.NewHBox(t.val, t.unit), t.delta)
	t.obj = container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), body))
	return t
}

func (t *statTile) set(val, delta string) {
	t.val.Text, t.delta.Text = val, delta
	t.val.Refresh()
	t.delta.Refresh()
}

// statsLayout — сетка плиток: 2 столбца, а при большой ширине 4.
type statsLayout struct{}

func (statsLayout) cols(w float32) int {
	if w >= 480 {
		return 4
	}
	return 2
}

func (l statsLayout) gap(w float32) float32 {
	if l.cols(w) == 4 {
		return 8
	}
	return 12
}

func (l statsLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
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

func (l statsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	c := 2
	if len(objs) > 0 && len(objs) <= 4 {
		c = 2
	}
	g := l.gap(0)
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
}

func (l *homeLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(300, 320) }

func (l *homeLayout) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	wide := s.Width >= wideBreakpoint-sidebarWidth
	if !wide {
		l.header.Hide()
		l.side.Hide()
		l.bottom.Show()
		bh := l.bottom.MinSize().Height
		l.bottom.Resize(fyne.NewSize(s.Width, bh))
		l.bottom.Move(fyne.NewPos(0, s.Height-bh))
		d := float32(196)
		if s.Height-bh < 420 {
			d = 150
		}
		l.power.SetDiameter(d)
		l.hero.Resize(fyne.NewSize(s.Width, s.Height-bh))
		l.hero.Move(fyne.NewPos(0, 0))
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
		l.power.SetDiameter(168)
		l.hero.Resize(fyne.NewSize(s.Width, s.Height-16-sh-hdr))
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
func (u *ui) updateStats(rx, tx uint64, session string) {
	now := timeNow()
	if !u.statAt.IsZero() && now.After(u.statAt) && u.statOK {
		dt := now.Sub(u.statAt).Seconds()
		rate := func(cur, prev uint64) string {
			if cur < prev {
				return "0.0"
			}
			return fmtMB(float64(cur-prev) / dt / (1 << 20))
		}
		u.stats[0].set(rate(rx, u.rxPrev), L("▼ incoming", "▼ входящий"))
		u.stats[1].set(rate(tx, u.txPrev), L("▲ outgoing", "▲ исходящий"))
	}
	u.statAt, u.rxPrev, u.txPrev, u.statOK = now, rx, tx, true
	u.stats[3].set(session, "")
}

func (u *ui) resetStats() {
	u.statOK = false
	for _, t := range u.stats {
		if t != nil {
			t.set("—", "")
		}
	}
}
