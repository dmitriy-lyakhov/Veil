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
	p := theme.InnerPadding()
	flushT := container.New(layout.NewCustomPaddedLayout(-p, -p, -p, -p), t)
	if sub == "" {
		return flushT
	}
	return container.New(tightVBox{gap: 5}, flushT, proseFlush(sub))
}

// listRow — строка списка в стиле референса: заголовок, описание, элемент справа, разделитель снизу.
func listRow(title, sub string, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	return listRowColored(title, sub, theme.ColorNameForeground, trailing, onTap)
}

func listRowColored(title, sub string, titleColor fyne.ThemeColorName, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	return listRowOf(rowText(title, sub, titleColor), trailing, onTap)
}

func listRowOf(content, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	return listRowH(content, trailing, onTap, 64)
}

// listRowH — строка списка заданной минимальной высоты (дизайн: строки страниц 64 px, пункты «Настроек» 56 px).
func listRowH(content, trailing fyne.CanvasObject, onTap func(), h float32) fyne.CanvasObject {
	var right fyne.CanvasObject
	if trailing != nil {
		right = vcenter(trailing)
	}
	body := container.NewBorder(nil, nil, nil, right, vcenter(content))
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	bg.SetMinSize(fyne.NewSize(1, h))
	cardRow := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10.4, 10.4, 16, 16), body))
	return newRowTap(cardRow, onTap)
}

// navRow — строка-переход с шевроном.
func navRow(title, sub string, onTap func()) fyne.CanvasObject {
	h := float32(64)
	if sub == "" {
		h = 56
	}
	return listRowH(rowText(title, sub, theme.ColorNameForeground), container.NewCenter(iconImg("right", colMuted, 22)), onTap, h)
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

// evenCols — колонки равной ширины без промежутков (переключатель дизайна: кнопки по 276 px в рамке 560).
type evenCols struct{}

func (evenCols) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	if len(objs) == 0 {
		return
	}
	w := s.Width / float32(len(objs))
	for i, o := range objs {
		o.Resize(fyne.NewSize(w, s.Height))
		o.Move(fyne.NewPos(w*float32(i), 0))
	}
}

func (evenCols) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		m := o.MinSize()
		w = max(w, m.Width)
		h = max(h, m.Height)
	}
	return fyne.NewSize(w*float32(len(objs)), h)
}

func newSegmented(options []string, selected int, changed func(int)) fyne.CanvasObject {
	btns := make([]*tapCard, len(options))
	grid := container.New(evenCols{})
	apply := func(sel int) {
		for i, b := range btns {
			if i == sel {
				b.base, b.hover = colCard2, colCard2Hover
			} else {
				b.base, b.hover = color.Transparent, colCardHover
			}
			b.bg.FillColor = b.base
			b.bg.Refresh()
		}
	}
	for i, o := range options {
		i := i
		t := newFitText(o, 14, colText, true)
		t.Alignment = fyne.TextAlignCenter
		btns[i] = newTapCardR(vcenter(t), color.Transparent, colCardHover, 12, 1, func() {
			apply(i)
			changed(i)
		})
		btns[i].bg.StrokeWidth = 0
		grid.Add(btns[i])
	}
	apply(selected)
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 16
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	bg.SetMinSize(fyne.NewSize(1, 52))
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), grid))
}

// sectionLabel — подпись раздела на странице: 12 px/600, приглушённая, отступ 4 px слева и 10 px сверху (дизайн: section).
func sectionLabel(s string) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(10, 0, 4, 0), text(s, 12, colMuted, true))
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
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	handle := canvas.NewRectangle(colIdle)
	handle.CornerRadius = 2
	handle.SetMinSize(fyne.NewSize(40, 4))
	top := container.NewCenter(container.NewGridWrap(fyne.NewSize(40, 4), handle))
	body := container.NewVBox(top, content, canvas.NewRectangle(color.Transparent))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, sheetBleed, 8, 8), body))
	s.root = container.New(s.lay, s.dim, withShadow(card, 22))
	c := win.Canvas()
	s.root.Resize(c.Size())
	c.Overlays().Add(s.root)
	s.shown = true
	afterFrame(func() {
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

// hrow — горизонтальный ряд с заданным промежутком (HBox из Fyne берёт зазор из темы).
type hrow struct{ gap float32 }

func (l hrow) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	var x float32
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		o.Resize(fyne.NewSize(m.Width, s.Height))
		o.Move(fyne.NewPos(x, 0))
		x += m.Width + l.gap
	}
}

func (l hrow) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w += m.Width
		h = max(h, m.Height)
		n++
	}
	if n > 1 {
		w += l.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

// pageButton — кнопка 52 px внутри страницы: подпись 15 px/600 (дизайн: btn), главная — акцентная.
func pageButton(label, icon string, primary bool, onTap func()) fyne.CanvasObject {
	var bg, hv color.Color = colCard, colCardHover
	var fg color.Color = colText
	if primary {
		bg, hv, fg = colAccent, mix(colAccent, colText, 0.12), colAccentDark
	}
	return container.New(&fillMin{h: 52}, newTapCardR(iconLabelRow(icon, label, 15, fg), bg, hv, 14, 1, onTap))
}

// pageButtonDanger — кнопка страницы с красной подписью.
func pageButtonDanger(label, icon string, onTap func()) fyne.CanvasObject {
	return container.New(&fillMin{h: 52}, newTapCardR(iconLabelRow(icon, label, 15, colAccentInk), colCard, colCardHover, 14, 1, onTap))
}

// ghostButton — вторичное действие под главной кнопкой («Отмена»): без фона, 44 px, подпись 14 px/600 приглушённая.
func ghostButton(label string, onTap func()) fyne.CanvasObject {
	t := newFitText(label, 14, colMuted, true)
	t.Alignment = fyne.TextAlignCenter
	c := newTapCardR(vcenter(t), color.Transparent, colCardHover, 14, 1, onTap)
	c.bg.StrokeWidth = 0
	return container.New(&fillMin{h: 44}, c)
}

// navRowIcon — строка-переход со значком в плитке 44 px слева (дизайн: «Другие способы подключения»).
func navRowIcon(icon, title, sub string, onTap func()) fyne.CanvasObject {
	tileBG := canvas.NewRectangle(colCard2)
	tileBG.CornerRadius = 11
	tile := container.NewGridWrap(fyne.NewSize(44, 44), container.NewStack(tileBG, container.NewCenter(iconImg(icon, colText, 22))))
	text := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 6), container.NewCenter(tile)), nil, vcenter(rowText(title, sub, theme.ColorNameForeground)))
	return listRowOf(text, container.NewCenter(iconImg("right", colMuted, 22)), onTap)
}
