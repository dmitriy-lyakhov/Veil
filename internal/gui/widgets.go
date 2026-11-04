package gui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---------- переключатель ----------

type switchWidget struct {
	widget.BaseWidget
	on       bool
	disabled bool
	onChange func(bool)

	track *canvas.Rectangle
	knob  *canvas.Circle
	pos   float32 // 0 — выключен, 1 — включён
	anim  *fyne.Animation
}

const (
	switchW = 46
	switchH = 26
)

func newSwitch(on bool, changed func(bool)) *switchWidget {
	s := &switchWidget{on: on, onChange: changed}
	s.track = canvas.NewRectangle(colCard2)
	s.track.CornerRadius = switchH / 2
	s.track.StrokeWidth = 1
	s.knob = canvas.NewCircle(colMuted)
	if on {
		s.pos = 1
	}
	s.ExtendBaseWidget(s)
	s.paint()
	return s
}

func (s *switchWidget) paint() {
	t := s.pos
	mix := func(a, b uint8) uint8 { return uint8(float32(a)*(1-t) + float32(b)*t) }
	off, onc := colCard2, colAccent
	s.track.FillColor = color.NRGBA{mix(off.R, onc.R), mix(off.G, onc.G), mix(off.B, onc.B), 0xff}
	s.track.StrokeColor = color.NRGBA{mix(colLine.R, onc.R), mix(colLine.G, onc.G), mix(colLine.B, onc.B), 0xff}
	k1, k2 := colMuted, colAccentDark
	s.knob.FillColor = color.NRGBA{mix(k1.R, k2.R), mix(k1.G, k2.G), mix(k1.B, k2.B), 0xff}
	if s.disabled {
		s.track.FillColor = colCard
		s.knob.FillColor = colIdle
	}
}

func (s *switchWidget) On() bool { return s.on }

// SetOn меняет состояние без вызова обработчика.
func (s *switchWidget) SetOn(on bool) {
	if s.on == on {
		return
	}
	s.on = on
	s.animate()
}

func (s *switchWidget) SetDisabled(d bool) {
	s.disabled = d
	s.paint()
	s.Refresh()
}

func (s *switchWidget) animate() {
	if s.anim != nil {
		s.anim.Stop()
	}
	from, to := s.pos, float32(0)
	if s.on {
		to = 1
	}
	s.anim = fyne.NewAnimation(140*time.Millisecond, func(p float32) {
		s.pos = from + (to-from)*p
		s.paint()
		s.Refresh()
	})
	s.anim.Start()
}

func (s *switchWidget) Tapped(*fyne.PointEvent) {
	if s.disabled {
		return
	}
	s.on = !s.on
	s.animate()
	if s.onChange != nil {
		s.onChange(s.on)
	}
}

func (s *switchWidget) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (s *switchWidget) CreateRenderer() fyne.WidgetRenderer { return &switchRenderer{s: s} }

type switchRenderer struct{ s *switchWidget }

func (r *switchRenderer) MinSize() fyne.Size { return fyne.NewSize(switchW, switchH) }

func (r *switchRenderer) Layout(size fyne.Size) {
	r.s.track.Resize(size)
	d := size.Height - 8
	r.s.knob.Resize(fyne.NewSize(d, d))
	r.s.knob.Move(fyne.NewPos(4+r.s.pos*(size.Width-size.Height), 4))
}

func (r *switchRenderer) Refresh() {
	r.Layout(r.s.Size())
	r.s.track.Refresh()
	r.s.knob.Refresh()
}

func (r *switchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.s.track, r.s.knob}
}

func (r *switchRenderer) Destroy() {}

// ---------- строки списков ----------

// rowTap — нажимаемая прозрачная область с подсветкой при наведении.
type rowTap struct {
	widget.BaseWidget
	content fyne.CanvasObject
	bg      *canvas.Rectangle
	onTap   func()
	anim    *fyne.Animation
}

func newRowTap(content fyne.CanvasObject, onTap func()) *rowTap {
	r := &rowTap{content: content, bg: canvas.NewRectangle(color.Transparent), onTap: onTap}
	r.bg.CornerRadius = 12
	r.ExtendBaseWidget(r)
	return r
}

func (r *rowTap) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(r.bg, r.content))
}

func (r *rowTap) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

func (r *rowTap) fade(to color.Color) {
	if r.anim != nil {
		r.anim.Stop()
	}
	r.anim = canvas.NewColorRGBAAnimation(r.bg.FillColor, to, hoverAnimTime, func(c color.Color) {
		r.bg.FillColor = c
		r.bg.Refresh()
	})
	r.anim.Start()
}

func (r *rowTap) MouseIn(*desktop.MouseEvent)    { r.fade(color.NRGBA{0xff, 0xff, 0xff, 0x0c}) }
func (r *rowTap) MouseMoved(*desktop.MouseEvent) {}
func (r *rowTap) MouseOut()                      { r.fade(color.Transparent) }
func (r *rowTap) Cursor() desktop.Cursor {
	if r.onTap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// rowText — заголовок и необязательное описание строки (с переносом).
func rowText(title, sub string, titleColor fyne.ThemeColorName) fyne.CanvasObject {
	t := widget.NewRichText(&widget.TextSegment{Text: title, Style: widget.RichTextStyle{
		ColorName: titleColor, SizeName: theme.SizeNameSubHeadingText, TextStyle: fyne.TextStyle{Bold: true}}})
	t.Wrapping = fyne.TextWrapWord
	if sub == "" {
		return t
	}
	return container.New(tightVBox{gap: -18}, t, prose(sub))
}

// listRow — строка списка в стиле референса: заголовок, описание, элемент справа, разделитель снизу.
func listRow(title, sub string, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	return listRowColored(title, sub, theme.ColorNameForeground, trailing, onTap)
}

func listRowColored(title, sub string, titleColor fyne.ThemeColorName, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	return listRowOf(rowText(title, sub, titleColor), trailing, onTap)
}

func listRowOf(content, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	var right fyne.CanvasObject
	if trailing != nil {
		right = vcenter(trailing)
	}
	body := container.NewBorder(nil, nil, nil, right, vcenter(content))
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.SetMinSize(fyne.NewSize(1, 56))
	cardRow := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), body))
	return newRowTap(cardRow, onTap)
}

// navRow — строка-переход с шевроном.
func navRow(title, sub string, onTap func()) fyne.CanvasObject {
	return listRow(title, sub, container.NewCenter(iconImg("right", colMuted, 22)), onTap)
}

// switchRow — строка с переключателем; нажатие по строке тоже переключает.
func switchRow(title, sub string, on bool, changed func(bool)) (fyne.CanvasObject, *switchWidget) {
	sw := newSwitch(on, changed)
	row := listRow(title, sub, sw, func() { sw.Tapped(nil) })
	return row, sw
}

func switchRowFit(title, sub string, on bool, changed func(bool)) (fyne.CanvasObject, *switchWidget) {
	sw := newSwitch(on, changed)
	return listRowOf(rowFit(title, sub, colText), sw, func() { sw.Tapped(nil) }), sw
}

// ---------- сегментный переключатель ----------

func newSegmented(options []string, selected int, changed func(int)) fyne.CanvasObject {
	btns := make([]*widget.Button, len(options))
	grid := container.NewGridWithColumns(len(options))
	apply := func(sel int) {
		for i, b := range btns {
			if i == sel {
				b.Importance = widget.MediumImportance
			} else {
				b.Importance = widget.LowImportance
			}
			b.Refresh()
		}
	}
	for i, o := range options {
		i := i
		btns[i] = widget.NewButton(o, func() {
			apply(i)
			changed(i)
		})
		grid.Add(btns[i])
	}
	apply(selected)
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 16
	bg.SetMinSize(fyne.NewSize(1, 52))
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), grid))
}

func sectionLabel(s string) fyne.CanvasObject {
	return lead(text(s, 13, colMuted, false))
}

// ---------- нижняя панель ----------

type tapRect struct {
	widget.BaseWidget
	rect  *canvas.Rectangle
	onTap func()
}

func newTapRect(c color.Color, onTap func()) *tapRect {
	t := &tapRect{rect: canvas.NewRectangle(c), onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapRect) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.rect) }
func (t *tapRect) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

type sheetLayout struct{ off float32 } // off — доля высоты, на которую панель опущена (0..1)

const sheetBleed = 24 // фон панели продолжается ниже края окна, чтобы нижние углы не были видны

func (l *sheetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(size)
	card := objs[1]
	h := card.MinSize().Height + sheetBleed
	if max := size.Height * 0.88; h > max {
		h = max
	}
	card.Resize(fyne.NewSize(size.Width, h))
	card.Move(fyne.NewPos(0, size.Height-h+sheetBleed+l.off*(h)))
}

func (l *sheetLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

type sheet struct {
	win   fyne.Window
	root  *fyne.Container
	lay   *sheetLayout
	dim   *tapRect
	anim  *fyne.Animation
	shown bool
}

// showSheet открывает нижнюю панель с содержимым; закрывается по нажатию на затемнение.
func showSheet(win fyne.Window, content fyne.CanvasObject) *sheet {
	s := &sheet{win: win, lay: &sheetLayout{off: 1}}
	s.dim = newTapRect(withAlpha(color.NRGBA{}, 0), s.Close)
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 22
	handle := canvas.NewRectangle(colIdle)
	handle.CornerRadius = 2
	handle.SetMinSize(fyne.NewSize(40, 4))
	top := container.NewCenter(container.NewGridWrap(fyne.NewSize(40, 4), handle))
	body := container.NewVBox(top, content, canvas.NewRectangle(color.Transparent))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, sheetBleed, 8, 8), body))
	s.root = container.New(s.lay, s.dim, card)
	c := win.Canvas()
	s.root.Resize(c.Size())
	c.Overlays().Add(s.root)
	s.shown = true
	time.AfterFunc(firstFrameDelay, func() {
		fyne.Do(func() {
			if s.shown {
				s.run(true)
			}
		})
	})
	return s
}

func (s *sheet) run(in bool) {
	if s.anim != nil {
		s.anim.Stop()
	}
	from, to, d := s.lay.off, float32(0), modalInTime
	if !in {
		to, d = 1, modalOutTime
	}
	s.anim = fyne.NewAnimation(d, func(p float32) {
		s.lay.off = from + (to-from)*p
		s.dim.rect.FillColor = withAlpha(color.NRGBA{}, uint8(float32(modalDimAlpha)*(1-s.lay.off)))
		s.lay.Layout(s.root.Objects, s.root.Size())
		s.dim.rect.Refresh()
		if !in && p >= 1 {
			fyne.Do(func() { removeOverlay(s.win.Canvas(), s.root) })
		}
	})
	s.anim.Curve = fyne.AnimationEaseOut
	s.anim.Start()
}

func (s *sheet) Close() {
	if !s.shown {
		return
	}
	s.shown = false
	s.run(false)
}

// scrollMax — прокрутка, которая занимает высоту содержимого, но не больше max.
func scrollMax(content fyne.CanvasObject, max float32) fyne.CanvasObject {
	sc := container.NewVScroll(content)
	return container.New(&scrollMaxLayout{content: content, max: max}, sc)
}

type scrollMaxLayout struct {
	content fyne.CanvasObject
	max     float32
}

func (l *scrollMaxLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(size)
}

func (l *scrollMaxLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	h := l.content.MinSize().Height
	if h > l.max {
		h = l.max
	}
	return fyne.NewSize(0, h)
}

// vcenter центрирует элемент по вертикали, оставляя ему всю ширину (в отличие от NewCenter,
// который ужимает элемент до минимального размера и сдвигает его к середине).
func vcenter(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(layout.NewSpacer(), o, layout.NewSpacer())
}

// statusLine — строка состояния, которая не занимает места, пока в ней нет текста.
type statusLine struct{ Label *widget.Label }

func newStatusLine() *statusLine {
	l := widget.NewLabel("")
	l.Wrapping = fyne.TextWrapWord
	l.Hide()
	return &statusLine{Label: l}
}

func (s *statusLine) SetText(t string) {
	s.Label.SetText(t)
	if t == "" {
		s.Label.Hide()
	} else {
		s.Label.Show()
	}
}

// layoutPadLeft — отступ слева на внутренний отступ виджета (для полосы под подписями).
func layoutPadLeft() fyne.Layout {
	return layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0)
}
