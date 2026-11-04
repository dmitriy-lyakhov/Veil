package gui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type powerState int

const (
	powerOff powerState = iota
	powerBusy
	powerOn
)

const (
	ringWidth  = 4
	pulseCycle = 900 * time.Millisecond
)

// powerButton — кольцо подключения по дизайну: тонкое кольцо цвета текста; при подключении
// заливается акцентом, внутри щит, подпись и таймер.
type powerButton struct {
	widget.BaseWidget
	OnTapped func()
	OnState  func(powerState)
	state    powerState
	disabled bool
	diameter float32
	timerTxt string

	ring   *canvas.Circle
	shield *canvas.Image
	label  *canvas.Text
	timer  *canvas.Text
	pulse  *fyne.Animation
}

func newPowerButton(tap func()) *powerButton {
	p := &powerButton{OnTapped: tap, diameter: 196}
	p.ring = canvas.NewCircle(color.Transparent)
	p.ring.StrokeWidth = ringWidth
	p.shield = canvas.NewImageFromResource(iconRes("shieldok", colAccentDark))
	p.shield.FillMode = canvas.ImageFillContain
	p.label = text(L("Connect", "Подключить"), 22, colText, true)
	p.timer = text("", 15, colAccentDark, true)
	p.pulse = fyne.NewAnimation(pulseCycle, func(f float32) {
		p.ring.StrokeColor = mix(colIdle, colAccent, f)
		p.ring.Refresh()
	})
	p.pulse.AutoReverse = true
	p.pulse.RepeatCount = fyne.AnimationRepeatForever
	p.ExtendBaseWidget(p)
	p.apply()
	return p
}

func (p *powerButton) SetState(s powerState) {
	if s == p.state {
		return
	}
	p.state = s
	p.apply()
	if p.OnState != nil {
		p.OnState(s)
	}
}

func (p *powerButton) SetDisabled(d bool) {
	p.disabled = d
	p.apply()
}

// SetDiameter меняет размер кольца: 232 на широком экране, 196 на узком, 168 на средней ширине.
func (p *powerButton) SetDiameter(d float32) {
	if p.diameter != d {
		p.diameter = d
		p.Refresh()
	}
}

// SetTimer задаёт время подключения внутри кольца.
func (p *powerButton) SetTimer(s string) {
	if p.timerTxt != s {
		p.timerTxt = s
		p.timer.Text = s
		p.timer.Refresh()
		p.Refresh()
	}
}

func rgba(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func (p *powerButton) apply() {
	p.pulse.Stop()
	switch p.state {
	case powerOn:
		p.ring.StrokeColor, p.ring.FillColor = colAccent, colAccent
		p.label.Text, p.label.Color = L("Connected", "Подключено"), colAccentDark
		p.timer.Color = colAccentDark
		p.shield.Resource = iconRes("shieldok", colAccentDark)
	case powerBusy:
		p.ring.StrokeColor, p.ring.FillColor = colIdle, color.Transparent
		p.label.Text, p.label.Color = L("Connecting", "Подключение"), colText
		p.pulse.Start()
	default:
		p.ring.StrokeColor, p.ring.FillColor = colText, color.Transparent
		p.label.Text, p.label.Color = L("Connect", "Подключить"), colText
		if p.disabled {
			p.ring.StrokeColor, p.label.Color = colIdle, colMuted
		}
	}
	p.Refresh()
}

func (p *powerButton) Tapped(*fyne.PointEvent) {
	if !p.disabled && p.OnTapped != nil {
		p.OnTapped()
	}
}

func (p *powerButton) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (p *powerButton) CreateRenderer() fyne.WidgetRenderer { return &powerRenderer{p: p} }

type powerRenderer struct{ p *powerButton }

func (r *powerRenderer) MinSize() fyne.Size { return fyne.NewSize(r.p.diameter, r.p.diameter) }

func (r *powerRenderer) Layout(size fyne.Size) {
	p := r.p
	d := min(size.Width, size.Height)
	cx, cy := size.Width/2, size.Height/2
	p.ring.Resize(fyne.NewSize(d, d))
	p.ring.Move(fyne.NewPos(cx-d/2, cy-d/2))
	// размер подписи подгоняется под кольцо, как в дизайне
	fs := float32(22)
	if d >= 220 {
		fs = 24
	}
	if n := float32(len([]rune(p.label.Text))); n > 0 {
		if m := d * 0.62 / (n * 0.64); m < fs {
			fs = max(11, m)
		}
	}
	p.label.TextSize = fs
	ls := p.label.MinSize()
	shield := float32(38)
	if d < 160 {
		shield = 28
	}
	on := p.state == powerOn
	h := ls.Height
	if on {
		h += shield + 6
		if p.timerTxt != "" {
			h += p.timer.MinSize().Height + 6
		}
	}
	y := cy - h/2
	if on {
		p.shield.Show()
		p.shield.Resize(fyne.NewSize(shield, shield))
		p.shield.Move(fyne.NewPos(cx-shield/2, y))
		y += shield + 6
	} else {
		p.shield.Hide()
	}
	p.label.Resize(ls)
	p.label.Move(fyne.NewPos(cx-ls.Width/2, y))
	if on && p.timerTxt != "" {
		ts := p.timer.MinSize()
		p.timer.Show()
		p.timer.Resize(ts)
		p.timer.Move(fyne.NewPos(cx-ts.Width/2, y+ls.Height+6))
	} else {
		p.timer.Hide()
	}
}

func (r *powerRenderer) Refresh() {
	r.Layout(r.p.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *powerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.ring, r.p.shield, r.p.label, r.p.timer}
}

func (r *powerRenderer) Destroy() { r.p.pulse.Stop() }
