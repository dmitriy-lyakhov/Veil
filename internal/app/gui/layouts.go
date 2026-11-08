package gui

// Раскладки и мелкие элементы для страниц в две колонки (установка сервера, связи, контейнеры).
// Все раскладки здесь «высота по ширине»: Layout запоминает ширину, MinSize считает высоту для неё
// (так же, как flowText и vboxFit в kit), поэтому на узком окне колонки встают друг под друга,
// а длинные строки переносятся, а не вылезают за край.

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
)

// wideColumns — ширина колонки содержимого для страниц в две колонки на широком окне.
const wideColumns = 940

// textSetter — текст с переносом, который можно поменять (kit.NewFlowText, kit.ProseFlush).
type textSetter interface {
	fyne.CanvasObject
	SetText(string)
}

func flow(s string, size float32, c color.Color, bold bool) textSetter {
	return kit.NewFlowText(s, size, c, bold, size*1.4)
}

// ---------- две колонки ----------

// splitCols — две колонки: левая фиксированной ширины (left > 0) или доля ratio, правая — остаток.
// Уже minWide колонки встают одна под другой.
type splitCols struct {
	left    float32
	ratio   float32
	gap     float32
	minWide float32
	w       float32
}

func newSplit(left, ratio, minWide float32, a, b fyne.CanvasObject) *fyne.Container {
	return container.New(&splitCols{left: left, ratio: ratio, gap: 20, minWide: minWide}, a, b)
}

func (l *splitCols) widths(w float32) (float32, float32) {
	lw := l.left
	if lw <= 0 {
		r := l.ratio
		if r <= 0 {
			r = 0.5
		}
		lw = (w - l.gap) * r
	}
	return lw, w - l.gap - lw
}

func (l *splitCols) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	l.w = s.Width
	if s.Width >= l.minWide {
		lw, rw := l.widths(s.Width)
		h0 := kit.HeightAtWidth(objs[0], lw)
		h1 := kit.HeightAtWidth(objs[1], rw)
		objs[0].Move(fyne.NewPos(0, 0))
		objs[0].Resize(fyne.NewSize(lw, h0))
		objs[1].Move(fyne.NewPos(lw+l.gap, 0))
		objs[1].Resize(fyne.NewSize(rw, h1))
		return
	}
	h0 := kit.HeightAtWidth(objs[0], s.Width)
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(s.Width, h0))
	h1 := kit.HeightAtWidth(objs[1], s.Width)
	objs[1].Move(fyne.NewPos(0, h0+l.gap))
	objs[1].Resize(fyne.NewSize(s.Width, h1))
}

func (l *splitCols) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h0, h1 := objs[0].MinSize().Height, objs[1].MinSize().Height
	if l.w >= l.minWide && l.w > 0 {
		return fyne.NewSize(0, max(h0, h1))
	}
	return fyne.NewSize(0, h0+h1+l.gap)
}

// ---------- сетка, которая переносит ячейки ----------

// autoGrid — ячейки равной ширины: столько колонок, сколько помещается при ширине ячейки не меньше minCell.
type autoGrid struct {
	minCell float32
	gap     float32
	w       float32
}

func newAutoGrid(minCell float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(&autoGrid{minCell: minCell, gap: 8}, objs...)
}

func visibleObjs(objs []fyne.CanvasObject) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, o := range objs {
		if o.Visible() {
			out = append(out, o)
		}
	}
	return out
}

func (l *autoGrid) cols(w float32, n int) int {
	if n == 0 {
		return 1
	}
	c := int(math.Floor(float64((w + l.gap) / (l.minCell + l.gap))))
	return max(1, min(n, c))
}

func (l *autoGrid) Layout(all []fyne.CanvasObject, s fyne.Size) {
	l.w = s.Width
	objs := visibleObjs(all)
	cols := l.cols(s.Width, len(objs))
	cw := (s.Width - l.gap*float32(cols-1)) / float32(cols)
	var y float32
	for i := 0; i < len(objs); i += cols {
		var rh float32
		for j := i; j < min(i+cols, len(objs)); j++ {
			rh = max(rh, kit.HeightAtWidth(objs[j], cw))
		}
		for j := i; j < min(i+cols, len(objs)); j++ {
			objs[j].Move(fyne.NewPos(float32(j-i)*(cw+l.gap), y))
			objs[j].Resize(fyne.NewSize(cw, rh))
		}
		y += rh + l.gap
	}
}

func (l *autoGrid) MinSize(all []fyne.CanvasObject) fyne.Size {
	objs := visibleObjs(all)
	if len(objs) == 0 {
		return fyne.NewSize(0, 0)
	}
	cols := len(objs)
	if l.w > 0 {
		cols = l.cols(l.w, len(objs))
	} else {
		cols = 1
	}
	var h float32
	rows := 0
	for i := 0; i < len(objs); i += cols {
		var rh float32
		for j := i; j < min(i+cols, len(objs)); j++ {
			rh = max(rh, objs[j].MinSize().Height)
		}
		h += rh
		rows++
	}
	return fyne.NewSize(0, h+l.gap*float32(rows-1))
}

// ---------- ряд, который переносится (чипы подсетей) ----------

type flowRow struct {
	gap float32
	w   float32
}

func (l *flowRow) place(objs []fyne.CanvasObject, w float32, apply bool) float32 {
	var x, y, rh float32
	for _, o := range visibleObjs(objs) {
		m := o.MinSize()
		ow := min(m.Width, w)
		if x > 0 && x+ow > w {
			x = 0
			y += rh + l.gap
			rh = 0
		}
		if apply {
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(ow, m.Height))
		}
		x += ow + l.gap
		rh = max(rh, m.Height)
	}
	return y + rh
}

func (l *flowRow) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	l.w = s.Width
	l.place(objs, s.Width, true)
}

func (l *flowRow) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w := l.w
	if w <= 0 {
		w = 1 // до первой раскладки — по одному в строке
	}
	return fyne.NewSize(0, l.place(objs, w, false))
}

// ---------- строка с кнопками справа (или снизу на узком окне) ----------

// sideActions — содержимое слева и кнопки справа; если кнопкам с текстом не хватает места
// (текст уже minText), кнопки уходят под текст.
type sideActions struct {
	minText float32
	gap     float32
	w       float32
}

func (l *sideActions) side(w float32, actions fyne.CanvasObject) bool {
	return w-actions.MinSize().Width-l.gap >= l.minText
}

func (l *sideActions) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	l.w = s.Width
	text, actions := objs[0], objs[1]
	am := actions.MinSize()
	if l.side(s.Width, actions) {
		tw := s.Width - am.Width - l.gap
		th := kit.HeightAtWidth(text, tw)
		text.Resize(fyne.NewSize(tw, th))
		text.Move(fyne.NewPos(0, (s.Height-th)/2))
		actions.Resize(am)
		actions.Move(fyne.NewPos(s.Width-am.Width, (s.Height-am.Height)/2))
		return
	}
	th := kit.HeightAtWidth(text, s.Width)
	text.Resize(fyne.NewSize(s.Width, th))
	text.Move(fyne.NewPos(0, 0))
	actions.Resize(fyne.NewSize(min(am.Width, s.Width), am.Height))
	actions.Move(fyne.NewPos(0, th+l.gap))
}

func (l *sideActions) MinSize(objs []fyne.CanvasObject) fyne.Size {
	tm, am := objs[0].MinSize(), objs[1].MinSize()
	if l.w > 0 && l.side(l.w, objs[1]) {
		return fyne.NewSize(0, max(tm.Height, am.Height))
	}
	return fyne.NewSize(0, tm.Height+l.gap+am.Height)
}

// ---------- карточки и кнопки ----------

// cardBG — фон карточки дизайна: surface-raised, скругление r, тонкая граница.
func cardBG(r float32) *canvas.Rectangle {
	bg := canvas.NewRectangle(kit.ColCard)
	bg.CornerRadius = r
	bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
	return bg
}

// padCard — карточка с полями (сверху/снизу v, по бокам h) и минимальной высотой minH.
func padCard(content fyne.CanvasObject, v, h, minH float32) (fyne.CanvasObject, *canvas.Rectangle) {
	bg := cardBG(14)
	bg.SetMinSize(fyne.NewSize(1, minH))
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(v, v, h, h), content)), bg
}

// noteText — карточка-пояснение (как noteBox) с текстом, который можно менять.
func noteText(icon string, t textSetter) fyne.CanvasObject {
	objs := []fyne.CanvasObject{t}
	if icon != "" {
		objs = append([]fyne.CanvasObject{container.NewGridWrap(fyne.NewSize(22, 22), kit.IconImg(icon, kit.ColMuted, 22))}, objs...)
	}
	return container.NewStack(infoBG(), container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), container.New(noteLayout{icon: icon != ""}, objs...)))
}

// smallButton — компактная кнопка 36 px с подписью 12 px (действия в строках списка).
func smallButton(label string, onTap func()) fyne.CanvasObject {
	t := kit.Text(label, 12, kit.ColText, true)
	t.Alignment = fyne.TextAlignCenter
	return container.New(&kit.FillMin{H: 36}, kit.NewTapCardR(container.New(layout.NewCustomPaddedLayout(0, 0, 6, 6), kit.Vcenter(t)), kit.ColCard2, kit.ColCard2Hover, 11, 1, onTap))
}

// saveButton — главная кнопка, которая доступна только при несохранённых изменениях.
type saveButton struct {
	box     *fyne.Container
	label   string
	onTap   func()
	enabled bool
}

func newSaveButton(label string, onTap func()) *saveButton {
	b := &saveButton{box: container.NewStack(), label: label, onTap: onTap}
	b.render()
	return b
}

func (b *saveButton) render() {
	if b.enabled {
		b.box.Objects = []fyne.CanvasObject{kit.PageButton(b.label, "", true, b.onTap)}
	} else {
		t := kit.Text(b.label, 15, kit.ColMuted, true)
		t.Alignment = fyne.TextAlignCenter
		bg := canvas.NewRectangle(kit.ColCard2)
		bg.CornerRadius = 14
		b.box.Objects = []fyne.CanvasObject{container.New(&kit.FillMin{H: 52}, container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 18, 18), kit.Vcenter(t))))}
	}
	b.box.Refresh()
}

func (b *saveButton) SetEnabled(on bool) {
	if b.enabled == on && len(b.box.Objects) > 0 {
		return
	}
	b.enabled = on
	b.render()
}

func (b *saveButton) Object() fyne.CanvasObject { return b.box }

// hintAndButton — подсказка слева (переносится) и кнопка справа; на узком окне кнопка под подсказкой.
func hintAndButton(hint fyne.CanvasObject, btn fyne.CanvasObject) fyne.CanvasObject {
	w := container.New(&kit.FillMin{H: 52}, btn)
	sized := container.New(&minWidth{w: 150}, w)
	return container.New(&sideActions{minText: 140, gap: 12}, hint, sized)
}

// minWidth — задаёт содержимому минимальную ширину.
type minWidth struct{ w float32 }

func (l *minWidth) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(s)
	}
}

func (l *minWidth) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		m := o.MinSize()
		w, h = max(w, m.Width), max(h, m.Height)
	}
	return fyne.NewSize(max(w, l.w), h)
}

// ---------- переключатель вкладок ----------

// tabItem — вкладка сегментного переключателя: подпись 13 px/600, выбранная — на фоне surface-high.
type tabItem struct {
	widget.BaseWidget
	bg    *canvas.Rectangle
	text  *kit.FitText
	onTap func()
	on    bool
}

func newTabItem(label string, onTap func()) *tabItem {
	t := &tabItem{bg: canvas.NewRectangle(color.Transparent), text: kit.NewFitText(label, 13, kit.ColMuted, true), onTap: onTap}
	t.bg.CornerRadius = 9
	t.text.Alignment = fyne.TextAlignCenter
	t.ExtendBaseWidget(t)
	return t
}

func (t *tabItem) setOn(on bool) {
	t.on = on
	if on {
		t.bg.FillColor = kit.ColCard2
		t.text.SetColor(kit.ColText)
	} else {
		t.bg.FillColor = color.Transparent
		t.text.SetColor(kit.ColMuted)
	}
	t.bg.Refresh()
}

func (t *tabItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(t.bg, container.New(layout.NewCustomPaddedLayout(0, 0, 6, 6), kit.Vcenter(t.text))))
}

func (t *tabItem) MinSize() fyne.Size { return fyne.NewSize(t.natural()+12, 32) }

func (t *tabItem) natural() float32 { return t.text.NaturalWidth() }

func (t *tabItem) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tabItem) Cursor() desktop.Cursor { return desktop.PointerCursor }

// tabGrid — вкладки равной ширины в одну строку; если подписи не помещаются, вкладки
// переносятся на вторую строку (на узком окне пять вкладок не влезают в одну).
type tabGrid struct {
	gap   float32
	w     float32
	rows  int
	onFix func() // вызывается, когда после раскладки число строк изменилось
}

func (l *tabGrid) cols(w float32, objs []fyne.CanvasObject) int {
	var need float32
	for _, o := range objs {
		need = max(need, o.MinSize().Width)
	}
	n := len(objs)
	for c := n; c > 1; c-- {
		if (w-l.gap*float32(c-1))/float32(c) >= need {
			return c
		}
	}
	return 1
}

func (l *tabGrid) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	l.w = s.Width
	if len(objs) == 0 {
		return
	}
	cols := l.cols(s.Width, objs)
	rows := (len(objs) + cols - 1) / cols
	rh := (s.Height - l.gap*float32(rows-1)) / float32(rows)
	for i, o := range objs {
		r, c := i/cols, i%cols
		inRow := min(cols, len(objs)-r*cols)
		cw := (s.Width - l.gap*float32(inRow-1)) / float32(inRow)
		o.Move(fyne.NewPos(float32(c)*(cw+l.gap), float32(r)*(rh+l.gap)))
		o.Resize(fyne.NewSize(cw, rh))
	}
	if rows != l.rows && l.onFix != nil {
		l.rows = rows
		fix := l.onFix
		kit.AfterFrame(func() { fyne.Do(fix) })
	}
	l.rows = rows
}

func (l *tabGrid) MinSize(objs []fyne.CanvasObject) fyne.Size {
	rows := 1
	if l.w > 0 && len(objs) > 0 {
		cols := l.cols(l.w, objs)
		rows = (len(objs) + cols - 1) / cols
	}
	return fyne.NewSize(0, float32(rows)*32+l.gap*float32(rows-1))
}

// tabBar — сегментный переключатель вкладок страницы сервера (дизайн: 40 px, поле 4 px, выбранная — surface-high).
type tabBar struct {
	items []*tabItem
	grid  *tabGrid
	obj   fyne.CanvasObject
}

func newTabBar(labels []string, sel int, pick func(int)) *tabBar {
	b := &tabBar{grid: &tabGrid{gap: 4}}
	c := container.New(b.grid)
	for i, l := range labels {
		i := i
		it := newTabItem(l, func() {
			b.set(i)
			pick(i)
		})
		b.items = append(b.items, it)
		c.Add(it)
	}
	b.set(sel)
	bg := canvas.NewRectangle(kit.ColCard)
	bg.CornerRadius = 12
	bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
	b.obj = container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4), c))
	return b
}

func (b *tabBar) set(sel int) {
	for i, it := range b.items {
		it.setOn(i == sel)
	}
}

// ---------- значки состояния ----------

// statusDot — точка 8 px: зелёная — работает, серая — нет.
func statusDot(ok bool) fyne.CanvasObject {
	c := kit.ColIdle
	if ok {
		c = kit.ColOK
	}
	d := canvas.NewCircle(c)
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(8, 8), d))
}

// pulse — мигающая акцентная точка («идёт»): анимация останавливается вызовом stop.
func pulse(size float32) (fyne.CanvasObject, func()) {
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeColor, ring.StrokeWidth = kit.ColCard2, 2.5
	dot := canvas.NewCircle(kit.ColAccent)
	inner := container.New(layout.NewCustomPaddedLayout(size/4, size/4, size/4, size/4), dot)
	obj := container.NewCenter(container.NewGridWrap(fyne.NewSize(size, size), container.NewStack(ring, inner)))
	a := fyne.NewAnimation(900*time.Millisecond, func(p float32) {
		k := float32(0.35 + 0.65*math.Abs(math.Sin(float64(p)*math.Pi)))
		dot.FillColor = kit.WithAlpha(kit.ColAccent, uint8(255*k))
		dot.Refresh()
	})
	a.RepeatCount = fyne.AnimationRepeatForever
	a.Start()
	return obj, a.Stop
}

// fracBar — тонкая полоса прогресса с долей заполнения (дизайн: 6 px, дорожка surface-high, заполнение — акцент).
type fracBar struct {
	track, fill *canvas.Rectangle
	frac        float32
	obj         *fyne.Container
}

func newFracBar() *fracBar {
	b := &fracBar{track: canvas.NewRectangle(kit.ColCard2), fill: canvas.NewRectangle(kit.ColAccent)}
	b.track.CornerRadius, b.fill.CornerRadius = 3, 3
	b.obj = container.New(b, b.track, b.fill)
	return b
}

func (b *fracBar) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	b.track.Move(fyne.NewPos(0, 0))
	b.track.Resize(s)
	b.fill.Move(fyne.NewPos(0, 0))
	b.fill.Resize(fyne.NewSize(s.Width*b.frac, s.Height))
}

func (b *fracBar) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(10, 6) }

func (b *fracBar) set(f float32) {
	b.frac = max(0, min(1, f))
	b.obj.Refresh()
}
