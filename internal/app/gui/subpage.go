package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
)

const subPageTime = 280 * time.Millisecond

// subPage — вложенная страница поверх главного окна вместо отдельного окна ОС.
// Реализует fyne.Window (через встроенное главное окно), поэтому годится как
// родитель для диалогов; SetContent/Show/Close/Resize/SetTitle переопределены.
type subPage struct {
	fyne.Window
	title  *widget.RichText
	body   *fyne.Container
	root   *fyne.Container
	lay    *slideLayout
	anim   *fyne.Animation
	shown  bool
	right  *fyne.Container
	titleW *kit.FitText
	subs   *subFrame
	u      *ui

	// OnClosed вызывается при закрытии страницы (отмена фоновых задач).
	OnClosed func()
}

type slideLayout struct{ off float32 }

// Layout: на узком окне страница выезжает на весь экран, на широком — только справа от боковой панели.
func (l *slideLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if size.Width >= wideBreakpoint && len(objs) > 1 {
		w := size.Width - sidebarWidth
		objs[1].Show()
		objs[1].Resize(fyne.NewSize(sidebarWidth, size.Height))
		objs[1].Move(fyne.NewPos(0, 0))
		objs[0].Resize(fyne.NewSize(w, size.Height))
		objs[0].Move(fyne.NewPos(sidebarWidth+l.off*w, 0))
		return
	}
	if len(objs) > 1 {
		objs[1].Hide()
	}
	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(l.off*size.Width, 0))
}

func (l *slideLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

func (u *ui) newSubPage(title string) *subPage {
	p := &subPage{Window: u.win, lay: &slideLayout{off: 1}, u: u}
	p.title = widget.NewRichText(&widget.TextSegment{Text: title, Style: widget.RichTextStyle{
		ColorName: theme.ColorNameForeground, SizeName: theme.SizeNameSubHeadingText, Alignment: fyne.TextAlignCenter, TextStyle: fyne.TextStyle{Bold: true}}})
	// длинный заголовок обрезается многоточием, а не заезжает под кнопку «назад»
	p.title.Wrapping = fyne.TextWrapOff
	p.title.Truncation = fyne.TextTruncateEllipsis
	p.titleW = kit.NewFitText(title, 24, kit.ColText, true)
	back := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), p.Close)
	back.Importance = widget.LowImportance
	line := canvas.NewRectangle(kit.ColLine)
	p.right = container.NewStack()
	p.body = container.NewStack()
	bg := canvas.NewRectangle(kit.ColBackground)
	pill := u.statusPill()
	sidebar, items := u.sidebar(func(i int) {
		u.closeSubPages()
		u.showPage(i)
	})
	for _, n := range items {
		n.SetActive(n.e.index == u.curPage)
	}
	p.subs = &subFrame{bg: bg, back: back, title: p.title, titleW: p.titleW, right: p.right, pill: pill, line: line, body: p.body}
	content := container.New(p.subs, bg, line, p.body, back, p.title, p.titleW, p.right, pill)
	p.root = container.New(p.lay, content, sidebar)
	return p
}

// subFrame — шапка и содержимое вложенной страницы: на широком окне (дизайн десктопа) заголовок 24 px
// слева от «назад», плашка статуса справа, колонка до 560 px; на узком — заголовок по центру.
type subFrame struct {
	bg, back, title, right, pill, line, body fyne.CanvasObject
	titleW                                   *kit.FitText
}

func (f *subFrame) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(300, 240) }

func (f *subFrame) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	f.bg.Resize(s)
	f.bg.Move(fyne.NewPos(0, 0))
	bs := fyne.NewSize(44, 40)
	rs := f.right.MinSize()
	if rs.Width < 44 {
		rs.Width = 44
	}
	rs.Height = 40
	if s.Width >= wideBreakpoint-sidebarWidth {
		const hdr = 72
		f.line.Hide()
		f.title.Hide()
		f.titleW.Show()
		f.pill.Show()
		ps := f.pill.MinSize()
		f.pill.Resize(ps)
		f.pill.Move(fyne.NewPos(s.Width-24-ps.Width, 18))
		f.back.Resize(bs)
		f.back.Move(fyne.NewPos(16, (hdr-bs.Height)/2))
		ts := f.titleW.MinSize()
		avail := s.Width - 24 - ps.Width - 8 - rs.Width - 8 - 68
		f.titleW.Resize(fyne.NewSize(max(0, min(f.titleW.NaturalWidth(), avail)), ts.Height))
		f.titleW.Move(fyne.NewPos(68, 28))
		f.right.Resize(rs)
		f.right.Move(fyne.NewPos(s.Width-24-ps.Width-8-rs.Width, (hdr-rs.Height)/2))
		w := min(560, s.Width-48)
		f.body.Resize(fyne.NewSize(w+16, s.Height-92))
		f.body.Move(fyne.NewPos(16, 92))
		return
	}
	const hdr = 56
	f.titleW.Hide()
	f.pill.Hide()
	f.title.Show()
	f.line.Show()
	f.line.Resize(fyne.NewSize(s.Width, 1))
	f.line.Move(fyne.NewPos(0, hdr))
	f.back.Resize(bs)
	f.back.Move(fyne.NewPos(8, (hdr-bs.Height)/2))
	f.right.Resize(rs)
	f.right.Move(fyne.NewPos(s.Width-8-rs.Width, (hdr-rs.Height)/2))
	f.title.Resize(fyne.NewSize(s.Width-2*60, f.title.MinSize().Height))
	f.title.Move(fyne.NewPos(60, (hdr-f.title.MinSize().Height)/2))
	f.body.Resize(fyne.NewSize(s.Width-16, s.Height-hdr-1-8))
	f.body.Move(fyne.NewPos(8, hdr+1+8))
}

func (p *subPage) SetTitle(s string) {
	p.titleW.SetText(s)
	p.title.Segments[0].(*widget.TextSegment).Text = s
	p.title.Refresh()
}

func (p *subPage) SetContent(o fyne.CanvasObject) {
	p.body.Objects = []fyne.CanvasObject{o}
	p.body.Refresh()
}

func (p *subPage) Resize(fyne.Size) {}

func (p *subPage) run(in bool) {
	if p.anim != nil {
		p.anim.Stop()
	}
	from, to := p.lay.off, float32(0)
	if !in {
		to = 1
	}
	p.anim = fyne.NewAnimation(subPageTime, func(t float32) {
		p.lay.off = from + (to-from)*t
		p.lay.Layout(p.root.Objects, p.root.Size())
		if !in && t >= 1 {
			fyne.Do(func() { kit.RemoveOverlay(p.Window.Canvas(), p.root) })
		}
	})
	p.anim.Curve = fyne.AnimationEaseOut
	p.anim.Start()
}

func (p *subPage) Show() {
	if p.shown {
		return
	}
	p.shown = true
	p.u.subpages = append(p.u.subpages, p)
	c := p.Window.Canvas()
	p.lay.off = 1
	p.root.Resize(c.Size())
	c.Overlays().Add(p.root)
	kit.AfterFrame(func() {
		fyne.Do(func() {
			if p.shown {
				p.run(true)
			}
		})
	})
}

func (p *subPage) Close() {
	if !p.shown {
		return
	}
	p.shown = false
	p.u.dropSubPage(p)
	if p.OnClosed != nil {
		p.OnClosed()
	}
	p.run(false)
}

// SetRight ставит элемент справа в шапке (например, кнопку меню); nil убирает.
func (p *subPage) SetRight(o fyne.CanvasObject) {
	if o == nil {
		p.right.Objects = nil
	} else {
		p.right.Objects = []fyne.CanvasObject{o}
	}
	p.right.Refresh()
}

func (u *ui) dropSubPage(p *subPage) {
	for i, s := range u.subpages {
		if s == p {
			u.subpages = append(u.subpages[:i], u.subpages[i+1:]...)
			return
		}
	}
}

// closeSubPages закрывает все открытые вложенные страницы (переход по боковой панели).
func (u *ui) closeSubPages() {
	for len(u.subpages) > 0 {
		u.subpages[len(u.subpages)-1].Close()
	}
}

// animatePage — переход между страницами (сейчас без анимации): страница сразу на месте, занавеса нет.
func (u *ui) animatePage(in fyne.CanvasObject, from, to int) {
	for _, o := range u.pages.Objects[:4] {
		o.Move(fyne.NewPos(0, 0))
	}
	u.curtainA = 0
	u.curtain.FillColor = kit.WithAlpha(kit.ColBackground, 0)
	u.curtain.Refresh()
}
