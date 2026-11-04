package gui

import (
	"fmt"
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
	powerSize  = 210
	ringWidth  = 6
	glowExtra  = 26
	iconScale  = 0.32
	pulseCycle = 900 * time.Millisecond
)

const powerSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="2.2" stroke-linecap="round"><path d="M12 3.5v8"/><path d="M6.6 6.9a7.6 7.6 0 1 0 10.8 0"/></svg>`

func powerIcon(c color.NRGBA) fyne.Resource {
	hex := fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	return fyne.NewStaticResource("power-"+hex[1:]+".svg", []byte(fmt.Sprintf(powerSVG, hex)))
}

type powerButton struct {
	widget.BaseWidget
	OnTapped func()
	state    powerState
	disabled bool

	glow  *canvas.Circle
	ring  *canvas.Circle
	face  *canvas.Circle
	icon  *canvas.Image
	pulse *fyne.Animation
}

func newPowerButton(tap func()) *powerButton {
	p := &powerButton{OnTapped: tap}
	p.glow = canvas.NewCircle(color.Transparent)
	p.ring = canvas.NewCircle(color.Transparent)
	p.ring.StrokeWidth = ringWidth
	p.face = canvas.NewCircle(colCard)
	p.icon = canvas.NewImageFromResource(powerIcon(colMuted))
	p.icon.FillMode = canvas.ImageFillContain
	p.pulse = fyne.NewAnimation(pulseCycle, func(f float32) {
		a := uint8(40 + 160*f)
		p.ring.StrokeColor = color.NRGBA{colAccent.R, colAccent.G, colAccent.B, a}
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

func (p *powerButton) apply() {
	p.pulse.Stop()
	switch p.state {
	case powerOn:
		p.ring.StrokeColor = colAccent
		p.face.FillColor = colAccent
		p.glow.FillColor = color.NRGBA{colAccent.R, colAccent.G, colAccent.B, 0x2a}
		p.icon.Resource = powerIcon(colAccentDark)
	case powerBusy:
		p.face.FillColor = colCard
		p.glow.FillColor = color.NRGBA{colAccent.R, colAccent.G, colAccent.B, 0x12}
		p.icon.Resource = powerIcon(colAccent)
		p.pulse.Start()
	default:
		p.ring.StrokeColor = colIdle
		p.face.FillColor = colCard
		p.glow.FillColor = color.Transparent
		ic := colText
		if p.disabled {
			ic = colMuted
		}
		p.icon.Resource = powerIcon(ic)
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
	place(r.p.glow, d+glowExtra*2)
	place(r.p.ring, d)
	place(r.p.face, d-ringWidth*4)
	place(r.p.icon, d*iconScale)
}

func (r *powerRenderer) Refresh() {
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *powerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.glow, r.p.ring, r.p.face, r.p.icon}
}

func (r *powerRenderer) Destroy() { r.p.pulse.Stop() }
