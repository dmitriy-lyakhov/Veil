package gui

import (
	"fyne.io/fyne/v2/layout"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	wideBreakpoint = 720 // от этой ширины вместо нижней панели — боковая
	sidebarWidth   = 232
	bottomBarH     = 56
)

// navEntry — пункт навигации: иконка и подпись.
type navEntry struct {
	icon  string
	en    string
	ru    string
	index int // индекс страницы в u.pages
}

var navEntries = []navEntry{
	{"home", "Connect", "Подключение", 0},
	{"share", "Share", "Доступ", 1},
	{"sliders", "Settings", "Настройки", 2},
	{"plus", "Add server", "Добавить", 3},
}

// navItem — кнопка навигации; в узком виде — иконка над подписью, в широком — в строку.
type navItem struct {
	widget.BaseWidget
	e      navEntry
	wide   bool
	active bool
	hover  bool
	tap    func()
}

func newNavItem(e navEntry, wide bool, tap func()) *navItem {
	n := &navItem{e: e, wide: wide, tap: tap}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navItem) Tapped(*fyne.PointEvent) { n.tap() }
func (n *navItem) Cursor() desktop.Cursor  { return desktop.PointerCursor }
func (n *navItem) MouseIn(*desktop.MouseEvent) {
	n.hover = true
	n.Refresh()
}
func (n *navItem) MouseMoved(*desktop.MouseEvent) {}
func (n *navItem) MouseOut() {
	n.hover = false
	n.Refresh()
}

func (n *navItem) SetActive(a bool) {
	if n.active != a {
		n.active = a
		n.Refresh()
	}
}

func (n *navItem) CreateRenderer() fyne.WidgetRenderer {
	r := &navRenderer{n: n, bg: canvas.NewRectangle(color.Transparent), bar: canvas.NewRectangle(color.Transparent)}
	r.bg.CornerRadius = 11
	r.lbl = canvas.NewText("", colText)
	r.lbl.TextSize = 15
	r.lbl.TextStyle = fyne.TextStyle{Bold: true}
	r.ico = canvas.NewImageFromResource(nil)
	r.ico.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type navRenderer struct {
	n   *navItem
	bg  *canvas.Rectangle
	bar *canvas.Rectangle
	lbl *canvas.Text
	ico *canvas.Image
}

func (r *navRenderer) MinSize() fyne.Size {
	if r.n.wide {
		return fyne.NewSize(120, 48)
	}
	return fyne.NewSize(56, bottomBarH-8)
}

func (r *navRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	ls := r.lbl.MinSize()
	r.lbl.Resize(ls)
	const is = 22
	r.ico.Resize(fyne.NewSize(is, is))
	if r.n.wide {
		r.lbl.Show()
		r.bar.Resize(fyne.NewSize(0, 0))
		r.ico.Move(fyne.NewPos(12, (s.Height-is)/2))
		r.lbl.Move(fyne.NewPos(12+is+12, (s.Height-ls.Height)/2))
		return
	}
	r.bar.Resize(fyne.NewSize(24, 3))
	r.bar.Move(fyne.NewPos((s.Width-24)/2, 0))
	r.lbl.Hide()
	r.ico.Move(fyne.NewPos((s.Width-is)/2, (s.Height-is)/2))
}

func (r *navRenderer) Refresh() {
	fg := colMuted
	r.bg.FillColor = color.Transparent
	r.bar.FillColor = color.Transparent
	if r.n.active {
		fg = colText
		if r.n.wide {
			r.bg.FillColor = colCard2
		} else {
			r.bar.FillColor = colAccent
		}
	} else if r.n.hover {
		fg = colText
		if r.n.wide {
			r.bg.FillColor = colCardHover
		}
	}
	r.lbl.Text = L(r.n.e.en, r.n.e.ru)
	r.lbl.Color = fg
	r.ico.Resource = iconRes(r.n.e.icon, fg)
	r.Layout(r.n.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *navRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.bar, r.ico, r.lbl}
}
func (r *navRenderer) Destroy() {}

// shellLayout кладёт содержимое и навигацию: до 720 px — нижняя панель, от 720 px — боковая.
type shellLayout struct {
	content, bottom, side fyne.CanvasObject
}

func (l *shellLayout) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	if s.Width >= wideBreakpoint {
		l.bottom.Hide()
		l.side.Show()
		l.side.Resize(fyne.NewSize(sidebarWidth, s.Height))
		l.side.Move(fyne.NewPos(0, 0))
		l.content.Resize(fyne.NewSize(s.Width-sidebarWidth, s.Height))
		l.content.Move(fyne.NewPos(sidebarWidth, 0))
		return
	}
	l.side.Hide()
	l.bottom.Show()
	l.bottom.Resize(fyne.NewSize(s.Width, bottomBarH))
	l.bottom.Move(fyne.NewPos(0, s.Height-bottomBarH))
	l.content.Resize(fyne.NewSize(s.Width, s.Height-bottomBarH))
	l.content.Move(fyne.NewPos(0, 0))
}

func (l *shellLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(winMinWidth, winMinHeight)
}

// buildShell собирает окно: страницы + адаптивная навигация.
func (u *ui) buildShell() fyne.CanvasObject {
	u.navItems = nil
	var bottom []fyne.CanvasObject
	for _, e := range navEntries {
		idx := e.index
		nb := newNavItem(e, false, func() { u.showPage(idx) })
		u.navItems = append(u.navItems, nb)
		bottom = append(bottom, nb)
	}
	sbar, wideItems := u.sidebar(func(i int) { u.showPage(i) })
	u.navItems = append(u.navItems, wideItems...)

	line := func() *canvas.Rectangle {
		r := canvas.NewRectangle(colLine)
		r.SetMinSize(fyne.NewSize(1, 1))
		return r
	}
	bbar := container.NewStack(canvas.NewRectangle(colBar), container.NewBorder(line(), nil, nil, nil, container.NewGridWithColumns(len(bottom), bottom...)))

	l := &shellLayout{content: u.pages, bottom: bbar, side: sbar}
	return container.New(l, u.pages, bbar, sbar)
}

// setActiveNav подсвечивает пункт навигации текущей страницы.
func (u *ui) setActiveNav(page int) {
	for _, n := range u.navItems {
		n.SetActive(n.e.index == page)
	}
}

// vstackLayout складывает элементы столбиком без зазоров темы (отступы задаются явно).
type vstackLayout struct{ gap float32 }

func (l *vstackLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	var y float32
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(s.Width, h))
		o.Move(fyne.NewPos(0, y))
		y += h + l.gap
	}
}

func (l *vstackLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = max(w, m.Width)
		h += m.Height
		n++
	}
	if n > 1 {
		h += l.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

// pageFrame — рамка вкладки: на широком экране заголовок слева и плашка статуса справа (72 px),
// содержимое колонкой до 560 px; на узком — заголовок по центру.
type pageFrame struct {
	title, pill, body fyne.CanvasObject
	top               float32
}

func (l *pageFrame) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(300, 240) }

func (l *pageFrame) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	ts := l.title.MinSize()
	if s.Width >= wideBreakpoint-sidebarWidth {
		const hdr = 72
		ps := l.pill.MinSize()
		l.pill.Show()
		l.pill.Resize(ps)
		l.pill.Move(fyne.NewPos(s.Width-24-ps.Width, 18)) // плашка: 18 px сверху, высота 36
		l.title.Resize(ts)
		l.title.Move(fyne.NewPos(24, 28)) // заголовок 24 px: верх на 28 px
		w := min(560, s.Width-48)
		l.body.Resize(fyne.NewSize(w, s.Height-l.top-24))
		l.body.Move(fyne.NewPos(24, l.top))
		return
	}
	l.pill.Hide()
	l.title.Resize(ts)
	l.title.Move(fyne.NewPos((s.Width-ts.Width)/2, (56-ts.Height)/2))
	l.body.Resize(fyne.NewSize(s.Width-32, s.Height-60-12))
	l.body.Move(fyne.NewPos(16, 60))
}

// tabPage собирает страницу вкладки по рамке выше.
func (u *ui) tabPage(title string, body fyne.CanvasObject) fyne.CanvasObject {
	return u.tabPageAt(title, body, 100)
}

func (u *ui) tabPageAt(title string, body fyne.CanvasObject, top float32) fyne.CanvasObject {
	t := text(title, 24, colText, true)
	pill := u.statusPill()
	return container.New(&pageFrame{title: t, pill: pill, body: body, top: top}, t, pill, body)
}

// fixedHeight задаёт строке фиксированную высоту, содержимое выравнивается по центру.
type fixedHeight struct{ h float32 }

func (f *fixedHeight) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		m := o.MinSize()
		o.Resize(fyne.NewSize(s.Width, m.Height))
		o.Move(fyne.NewPos(0, (s.Height-m.Height)/2))
	}
}

func (f *fixedHeight) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objs {
		w = max(w, o.MinSize().Width)
	}
	return fyne.NewSize(w, f.h)
}

// sidebar строит боковую панель (232 px): знак, название и пункты навигации.
func (u *ui) sidebar(tap func(int)) (fyne.CanvasObject, []*navItem) {
	var items []*navItem
	var objs []fyne.CanvasObject
	for _, e := range navEntries {
		idx := e.index
		n := newNavItem(e, true, func() { tap(idx) })
		items = append(items, n)
		objs = append(objs, n)
	}
	line := canvas.NewRectangle(colLine)
	line.SetMinSize(fyne.NewSize(1, 1))
	name := canvas.NewText("Veil", colText)
	name.TextSize = 20
	name.TextStyle = fyne.TextStyle{Bold: true}
	brand := container.NewHBox(markImg(28), name)
	nav := container.New(&vstackLayout{gap: 6}, objs...)
	brandRow := container.New(layout.NewCustomPaddedLayout(0, 0, 12, 0), brand)
	sbox := container.New(&vstackLayout{gap: 28}, container.New(&fixedHeight{h: 32}, brandRow), nav)
	return container.NewStack(canvas.NewRectangle(colBar), container.NewBorder(nil, nil, nil, line, container.New(layout.NewCustomPaddedLayout(24, 24, 16, 16), sbox))), items
}
