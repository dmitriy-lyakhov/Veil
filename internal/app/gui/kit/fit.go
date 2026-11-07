package kit

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	fitMinWidth    = 96
	fitChevronGap  = 8
	fitEllipsis    = "…"
	FitRowTitleSz  = 16
	fitRowSubSz    = 12
	FitRowSubShift = 2
)

func fitString(s string, width, size float32, style fyne.TextStyle) string {
	if width <= 0 {
		return ""
	}
	if fyne.MeasureText(s, size, style).Width <= width {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fyne.MeasureText(string(runes[:mid])+fitEllipsis, size, style).Width <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return fitEllipsis
	}
	return strings.TrimRight(string(runes[:lo]), " ") + fitEllipsis
}

type FitText struct {
	widget.BaseWidget
	full      string
	size      float32
	col       color.Color
	style     fyne.TextStyle
	Alignment fyne.TextAlign
}

func NewFitText(s string, size float32, c color.Color, bold bool) *FitText {
	f := &FitText{full: s, size: size, col: c, style: fyne.TextStyle{Bold: bold}}
	f.ExtendBaseWidget(f)
	return f
}

func (f *FitText) SetText(s string) {
	f.full = s
	f.Refresh()
}

func (f *FitText) Text() string { return f.full }

func (f *FitText) SetColor(c color.Color) {
	f.col = c
	f.Refresh()
}

func (f *FitText) NaturalWidth() float32 {
	return fyne.MeasureText(f.full, f.size, f.style).Width
}

func (f *FitText) CreateRenderer() fyne.WidgetRenderer {
	t := canvas.NewText(f.full, f.col)
	t.TextSize = f.size
	t.TextStyle = f.style
	return &fitRenderer{f: f, t: t}
}

type fitRenderer struct {
	f *FitText
	t *canvas.Text
}

func (r *fitRenderer) MinSize() fyne.Size {
	n := fyne.MeasureText(r.f.full, r.f.size, r.f.style)
	return fyne.NewSize(min(n.Width, fitMinWidth), n.Height)
}

func (r *fitRenderer) Layout(s fyne.Size) {
	r.t.Text = fitString(r.f.full, s.Width, r.f.size, r.f.style)
	r.t.Color = r.f.col
	r.t.Alignment = r.f.Alignment
	h := fyne.MeasureText("Ag", r.f.size, r.f.style).Height
	r.t.Resize(fyne.NewSize(s.Width, h))
	r.t.Move(fyne.NewPos(0, (s.Height-h)/2))
}

func (r *fitRenderer) Refresh() {
	r.Layout(r.f.Size())
	r.t.Refresh()
}

func (r *fitRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.t} }

func (r *fitRenderer) Destroy() {}

type nameChevron struct{ centered bool }

func (l nameChevron) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	name, chev := objs[0], objs[1]
	cs := chev.MinSize()
	avail := max(0, s.Width-cs.Width-fitChevronGap)
	nw := avail
	if f, ok := name.(*FitText); ok {
		nw = min(avail, f.NaturalWidth())
	}
	total := nw + fitChevronGap + cs.Width
	x := float32(0)
	if l.centered {
		x = max(0, (s.Width-total)/2)
	}
	nh := name.MinSize().Height
	name.Resize(fyne.NewSize(nw, nh))
	name.Move(fyne.NewPos(x, (s.Height-nh)/2))
	chev.Resize(cs)
	chev.Move(fyne.NewPos(x+nw+fitChevronGap, (s.Height-cs.Height)/2))
}

func (l nameChevron) MinSize(objs []fyne.CanvasObject) fyne.Size {
	n, c := objs[0].MinSize(), objs[1].MinSize()
	return fyne.NewSize(n.Width+fitChevronGap+c.Width, max(n.Height, c.Height))
}

func NameWithChevron(name fyne.CanvasObject, chevron fyne.CanvasObject, centered bool) fyne.CanvasObject {
	return container.New(nameChevron{centered: centered}, name, chevron)
}

type iconLabel struct{}

func (iconLabel) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	icon, label := objs[0], objs[1]
	is := icon.MinSize()
	avail := max(0, s.Width-is.Width-fitChevronGap)
	lw := avail
	if f, ok := label.(*FitText); ok {
		lw = min(avail, f.NaturalWidth())
	}
	x := max(0, (s.Width-(is.Width+fitChevronGap+lw))/2)
	lh := label.MinSize().Height
	icon.Resize(is)
	icon.Move(fyne.NewPos(x, (s.Height-is.Height)/2))
	label.Resize(fyne.NewSize(lw, lh))
	label.Move(fyne.NewPos(x+is.Width+fitChevronGap, (s.Height-lh)/2))
}

func (iconLabel) MinSize(objs []fyne.CanvasObject) fyne.Size {
	i, l := objs[0].MinSize(), objs[1].MinSize()
	return fyne.NewSize(i.Width+fitChevronGap+l.Width, max(i.Height, l.Height))
}

func IconLabelRow(icon, label string, size float32, fg color.Color) fyne.CanvasObject {
	t := NewFitText(label, size, fg, true)
	t.Alignment = fyne.TextAlignCenter
	if icon == "" {
		return t
	}
	return container.New(iconLabel{}, container.NewCenter(IconImg(icon, fg, 20)), t)
}

func RowFit(title, sub string, titleColor color.Color) fyne.CanvasObject {
	t := NewFitText(title, FitRowTitleSz, titleColor, true)
	if sub == "" {
		return t
	}
	s := NewFitText(sub, fitRowSubSz, ColMuted, false)
	return container.New(TightVBox{Gap: FitRowSubShift}, t, s)
}
