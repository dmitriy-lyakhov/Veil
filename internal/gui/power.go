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
	powerSize  = 190
	ringWidth  = 3
	glowExtra  = 30
	pulseCycle = 900 * time.Millisecond
)

// powerButton — большое кольцо с мягким свечением и подписью внутри (как в референсе).
type powerButton struct {
	widget.BaseWidget
	OnTapped func()
	state    powerState
	disabled bool

	glow2 *canvas.Circle
	glow1 *canvas.Circle
	ring  *canvas.Circle
	face  *canvas.Circle
	label *canvas.Text
	pulse *fyne.Animation
}

func newPowerButton(tap func()) *powerButton {
	p := &powerButton{OnTapped: tap}
	p.glow2 = canvas.NewCircle(color.Transparent)
	p.glow1 = canvas.NewCircle(color.Transparent)
	p.ring = canvas.NewCircle(color.Transparent)
	p.ring.StrokeWidth = ringWidth
	p.face = canvas.NewCircle(colBackground)
	p.label = text(L("Connect", "Подключить"), 20, colText, false)
	p.label.Alignment = fyne.TextAlignCenter
	p.pulse = fyne.NewAnimation(pulseCycle, func(f float32) {
		a := uint8(70 + 185*f)
		p.ring.StrokeColor = color.NRGBA{colAccent.R, colAccent.G, colAccent.B, a}
		p.glow1.FillColor = color.NRGBA{colAccent.R, colAccent.G, colAccent.B, uint8(10 + 30*f)}
		p.glow1.Refresh()
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
}

func (p *powerButton) SetDisabled(d bool) {
	p.disabled = d
	p.apply()
}

func rgba(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func (p *powerButton) apply() {
	p.pulse.Stop()
	switch p.state {
	case powerOn:
		p.ring.StrokeColor = colAccent
		p.glow1.FillColor = rgba(colAccent, 0x30)
		p.glow2.FillColor = rgba(colAccent, 0x14)
		p.label.Text, p.label.Color = L("Disconnect", "Отключить"), colText
	case powerBusy:
		p.glow2.FillColor = rgba(colAccent, 0x0a)
		p.label.Text, p.label.Color = L("Connecting…", "Подключение…"), colMuted
		p.pulse.Start()
	default:
		p.ring.StrokeColor = colText
		p.glow1.FillColor = rgba(colText, 0x26)
		p.glow2.FillColor = rgba(colText, 0x0e)
		p.label.Text, p.label.Color = L("Connect", "Подключить"), colText
		if p.disabled {
			p.ring.StrokeColor = colIdle
			p.glow1.FillColor = color.Transparent
			p.glow2.FillColor = color.Transparent
			p.label.Color = colMuted
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

func (p *powerButton) CreateRenderer() fyne.WidgetRenderer {
	return &powerRenderer{p: p}
}

type powerRenderer struct{ p *powerButton }

func (r *powerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(powerSize+glowExtra*2, powerSize+glowExtra*2)
}

func (r *powerRenderer) Layout(size fyne.Size) {
	d := min(size.Width, size.Height) - glowExtra*2
	cx, cy := size.Width/2, size.Height/2
	place := func(o fyne.CanvasObject, diameter float32) {
		o.Resize(fyne.NewSize(diameter, diameter))
		o.Move(fyne.NewPos(cx-diameter/2, cy-diameter/2))
	}
	place(r.p.glow2, d+glowExtra*2)
	place(r.p.glow1, d+glowExtra)
	place(r.p.ring, d)
	place(r.p.face, d-ringWidth*2)
	ls := r.p.label.MinSize()
	r.p.label.Resize(ls)
	r.p.label.Move(fyne.NewPos(cx-ls.Width/2, cy-ls.Height/2))
}

func (r *powerRenderer) Refresh() {
	r.Layout(r.p.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *powerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.glow2, r.p.glow1, r.p.ring, r.p.face, r.p.label}
}

func (r *powerRenderer) Destroy() { r.p.pulse.Stop() }
