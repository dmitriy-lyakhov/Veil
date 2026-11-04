package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const subPageTime = 280 * time.Millisecond

// subPage — вложенная страница поверх главного окна вместо отдельного окна ОС.
// Реализует fyne.Window (через встроенное главное окно), поэтому годится как
// родитель для диалогов; SetContent/Show/Close/Resize/SetTitle переопределены.
type subPage struct {
	fyne.Window
	title *canvas.Text
	body  *fyne.Container
	root  *fyne.Container
	lay   *slideLayout
	anim  *fyne.Animation
	shown bool
	right *fyne.Container

	// OnClosed вызывается при закрытии страницы (отмена фоновых задач).
	OnClosed func()
}

type slideLayout struct{ off float32 }

func (l *slideLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(l.off*size.Width, 0))
}

func (l *slideLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

func (u *ui) newSubPage(title string) *subPage {
	p := &subPage{Window: u.win, lay: &slideLayout{off: 1}}
	p.title = text(title, 17, colText, true)
	back := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), p.Close)
	back.Importance = widget.LowImportance
	line := canvas.NewRectangle(colLine)
	line.SetMinSize(fyne.NewSize(1, 1))
	p.right = container.NewStack()
	p.title.Alignment = fyne.TextAlignCenter
	// кнопка «назад» и правое действие одинаковой ширины, чтобы заголовок был ровно по центру
	side := fyne.NewSize(44, 40)
	left := container.NewGridWrap(side, back)
	rightBox := container.NewGridWrap(side, p.right)
	header := container.NewVBox(container.NewPadded(container.NewBorder(nil, nil, left, rightBox, container.NewCenter(p.title))), line)
	p.body = container.NewStack()
	bg := canvas.NewRectangle(colBackground)
	p.root = container.New(p.lay, container.NewStack(bg, container.NewBorder(header, nil, nil, nil, p.body)))
	return p
}

func (p *subPage) SetTitle(s string) { p.title.Text = s; p.title.Refresh() }

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
			fyne.Do(func() { removeOverlay(p.Window.Canvas(), p.root) })
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
	c := p.Window.Canvas()
	p.lay.off = 1
	p.root.Resize(c.Size())
	c.Overlays().Add(p.root)
	time.AfterFunc(firstFrameDelay, func() {
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
