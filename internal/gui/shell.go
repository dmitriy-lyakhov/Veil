package gui

import (
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
	bottomBarH     = 64
)

// navEntry — пункт навигации: иконка и подпись.
type navEntry struct {
	icon  string
	en    string
	ru    string
	index int // индекс страницы в u.pages
}

var navEntries = []navEntry{
	{"home", "Home", "Главная", 0},
	{"share", "Share", "Поделиться", 1},
	{"sliders", "Settings", "Настройки", 2},
	{"server", "Servers", "Серверы", 3},
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
	r.bg.CornerRadius = 12
	r.lbl = canvas.NewText("", colText)
	r.lbl.TextSize = 13
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
		return fyne.NewSize(120, 44)
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
		r.bar.Resize(fyne.NewSize(3, 22))
		r.bar.Move(fyne.NewPos(0, (s.Height-22)/2))
		r.ico.Move(fyne.NewPos(16, (s.Height-is)/2))
		r.lbl.Move(fyne.NewPos(16+is+12, (s.Height-ls.Height)/2))
		return
	}
	r.bar.Resize(fyne.NewSize(24, 3))
	r.bar.Move(fyne.NewPos((s.Width-24)/2, 0))
	gap := float32(3)
	top := (s.Height - is - gap - ls.Height) / 2
	r.ico.Move(fyne.NewPos((s.Width-is)/2, top))
	r.lbl.Move(fyne.NewPos((s.Width-ls.Width)/2, top+is+gap))
}

func (r *navRenderer) Refresh() {
	fg := colMuted
	r.bg.FillColor = color.Transparent
	r.bar.FillColor = color.Transparent
	if r.n.active {
		fg = colText
		r.bar.FillColor = colAccent
		if r.n.wide {
			r.bg.FillColor = colCard
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
	var bottom, side []fyne.CanvasObject
	var wideItems []*navItem
	for _, e := range navEntries {
		idx := e.index
		nb := newNavItem(e, false, func() { u.showPage(idx) })
		nw := newNavItem(e, true, func() { u.showPage(idx) })
		u.navItems = append(u.navItems, nb)
		wideItems = append(wideItems, nw)
		bottom = append(bottom, nb)
		side = append(side, nw)
	}
	u.navItems = append(u.navItems, wideItems...)

	line := func() *canvas.Rectangle {
		r := canvas.NewRectangle(colLine)
		r.SetMinSize(fyne.NewSize(1, 1))
		return r
	}
	bbar := container.NewStack(canvas.NewRectangle(colBar), container.NewBorder(line(), nil, nil, nil, container.NewGridWithColumns(len(bottom), bottom...)))

	brand := container.NewHBox(markImg(28), canvas.NewText("Veil", colText))
	if t, ok := brand.Objects[1].(*canvas.Text); ok {
		t.TextSize = 20
		t.TextStyle = fyne.TextStyle{Bold: true}
	}
	sbox := container.NewVBox(append([]fyne.CanvasObject{container.NewPadded(brand)}, side...)...)
	sbar := container.NewStack(canvas.NewRectangle(colBar), container.NewBorder(nil, nil, nil, line(), container.NewPadded(sbox)))

	l := &shellLayout{content: u.pages, bottom: bbar, side: sbar}
	return container.New(l, u.pages, bbar, sbar)
}

// setActiveNav подсвечивает пункт навигации текущей страницы.
func (u *ui) setActiveNav(page int) {
	for _, n := range u.navItems {
		n.SetActive(n.e.index == page)
	}
}
