package gui

import (
	"image/color"
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	modalMinWidth  = 340
	modalSlide     = 28
	modalInTime    = 240 * time.Millisecond
	modalOutTime   = 160 * time.Millisecond
	pageAnimTime   = 260 * time.Millisecond
	pageSlide      = 36
	hoverAnimTime  = 140 * time.Millisecond
	modalDimAlpha  = 0x99
	messageLineLen = 34
)

// lead сдвигает заголовок на внутренний отступ виджета-подписи, чтобы
// заголовок и текст под ним начинались на одной вертикали.
func lead(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), o)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// ---------- модальные окна с анимацией ----------

type modal interface {
	Show()
	Hide()
}

type modalLayout struct{ off, offX float32 }

func (l *modalLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(size)
	card := objs[1]
	ms := card.MinSize()
	w := ms.Width
	if w < modalMinWidth {
		w = modalMinWidth
	}
	if w > size.Width-24 {
		w = size.Width - 24
	}
	h := ms.Height
	if h > size.Height-24 {
		h = size.Height - 24
	}
	card.Resize(fyne.NewSize(w, h))
	card.Move(fyne.NewPos((size.Width-w)/2+l.offX, (size.Height-h)/2+l.off))
}

func (l *modalLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

type modalCard struct {
	win     fyne.Window
	root    *fyne.Container
	lay     *modalLayout
	dim     *canvas.Rectangle
	curtain *canvas.Rectangle
	buttons *fyne.Container
	anim    *fyne.Animation
	shown   bool
}

func newModal(win fyne.Window, title string, content fyne.CanvasObject) *modalCard {
	m := &modalCard{win: win, lay: &modalLayout{}}
	m.dim = canvas.NewRectangle(withAlpha(color.NRGBA{}, 0))
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 18
	bg.StrokeColor = colBorder
	bg.StrokeWidth = 1
	m.curtain = canvas.NewRectangle(colCard)
	m.curtain.CornerRadius = 18

	m.buttons = container.NewHBox(layout.NewSpacer())
	heading := container.New(layout.NewCustomPaddedLayout(10, 10, theme.InnerPadding(), 0), text(title, 17, colText, true))
	body := container.NewVBox(heading, content, m.buttons)
	card := container.NewStack(bg, container.NewPadded(container.NewPadded(body)), m.curtain)
	m.root = container.New(m.lay, m.dim, withShadow(card, 18))
	return m
}

func (m *modalCard) run(in bool) {
	if m.anim != nil {
		m.anim.Stop()
	}
	d := modalInTime
	if !in {
		d = modalOutTime
	}
	m.anim = fyne.NewAnimation(d, func(p float32) {
		k := p
		if !in {
			k = 1 - p
		}
		m.dim.FillColor = withAlpha(color.NRGBA{}, uint8(float32(modalDimAlpha)*k))
		m.curtain.FillColor = withAlpha(color.NRGBA{0x1a, 0x1b, 0x21, 0xff}, uint8(255*(1-k)))
		m.lay.off = modalSlide * (1 - k)
		m.relayout()
		if !in && p >= 1 {
			fyne.Do(func() { removeOverlay(m.win.Canvas(), m.root) })
		}
	})
	m.anim.Curve = fyne.AnimationEaseOut
	m.anim.Start()
}

func (m *modalCard) Show() {
	if m.shown {
		return
	}
	m.shown = true
	c := m.win.Canvas()
	m.dim.FillColor = withAlpha(color.NRGBA{}, 0)
	m.curtain.FillColor = colCard
	m.lay.off = modalSlide
	m.root.Resize(c.Size())
	c.Overlays().Add(m.root)
	// первый показ создаёт отрисовку всего содержимого и занимает кадр-другой: анимацию запускаем
	// чуть позже, пока окно ещё невидимо, иначе начало движения «проскакивает»
	afterFrame(func() {
		fyne.Do(func() {
			if m.shown {
				m.run(true)
			}
		})
	})
}

func (m *modalCard) Hide() {
	if !m.shown {
		return
	}
	m.shown = false
	m.run(false)
}

func (m *modalCard) addButtons(dismiss, confirm string, cb func(bool)) {
	m.buttons.Add(widget.NewButton(dismiss, func() {
		m.Hide()
		if cb != nil {
			cb(false)
		}
	}))
	if confirm != "" {
		yes := widget.NewButton(confirm, func() {
			m.Hide()
			if cb != nil {
				cb(true)
			}
		})
		yes.Importance = widget.HighImportance
		m.buttons.Add(yes)
	}
}

func newCustom(title, dismiss string, content fyne.CanvasObject, win fyne.Window) modal {
	m := newModal(win, title, content)
	m.addButtons(dismiss, "", nil)
	return m
}

func newCustomConfirm(title, confirm, dismiss string, content fyne.CanvasObject, cb func(bool), win fyne.Window) modal {
	m := newModal(win, title, content)
	m.addButtons(dismiss, confirm, cb)
	return m
}

func newCustomNoButtons(title string, content fyne.CanvasObject, win fyne.Window) modal {
	return newModal(win, title, content)
}

func showCustom(title, dismiss string, content fyne.CanvasObject, win fyne.Window) {
	newCustom(title, dismiss, content, win).Show()
}

func messageContent(s string) fyne.CanvasObject {
	lines := 0
	for _, p := range strings.Split(s, "\n") {
		n := (utf8.RuneCountInString(p) + messageLineLen - 1) / messageLineLen
		if n < 1 {
			n = 1
		}
		lines += n
	}
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	return container.NewGridWrap(fyne.NewSize(modalMinWidth-24, float32(lines)*22+12), l)
}

func showInfo(title, msg string, win fyne.Window) {
	newCustom(title, "OK", messageContent(msg), win).Show()
}

func showError(err error, win fyne.Window) {
	newCustom("Ошибка", "Закрыть", messageContent(err.Error()), win).Show()
}

func showConfirm(title, msg string, cb func(bool), win fyne.Window) {
	newCustomConfirm(title, "Да", "Нет", messageContent(msg), cb, win).Show()
}

func showForm(title, confirm, dismiss string, items []*widget.FormItem, cb func(bool), win fyne.Window) {
	newCustomConfirm(title, confirm, dismiss, widget.NewForm(items...), cb, win).Show()
}

// ---------- переходы между страницами ----------

func (u *ui) animatePage(in fyne.CanvasObject, from, to int) {
	// переход без анимации: страница сразу на месте, занавеса нет
	for _, o := range u.pages.Objects[:4] {
		o.Move(fyne.NewPos(0, 0))
	}
	u.curtainA = 0
	u.curtain.FillColor = withAlpha(colBackground, 0)
	u.curtain.Refresh()
}

// ---------- плавная подсветка карточек ----------

func (c *tapCard) fadeTo(to color.Color) {
	if c.anim != nil {
		c.anim.Stop()
	}
	c.anim = canvas.NewColorRGBAAnimation(c.bg.FillColor, to, hoverAnimTime, func(col color.Color) {
		c.bg.FillColor = col
		c.bg.Refresh()
	})
	c.anim.Start()
}

// tightVBox — вертикальный стек с заданным зазором (может быть отрицательным).
type tightVBox struct{ gap, afterFirst float32 }

func (l tightVBox) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	var y float32
	first := true
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := heightAtWidth(o, size.Width)
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		g := l.gap
		if first && l.afterFirst != 0 {
			g = l.afterFirst
		}
		first = false
		y += h + g
	}
}

func (l tightVBox) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	first := true
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		g := l.gap
		if first && l.afterFirst != 0 {
			g = l.afterFirst
		}
		first = false
		h += ms.Height + g
	}
	// зазор после последнего элемента не нужен
	return fyne.NewSize(w, h-l.gap)
}

// removeOverlay убирает только указанный оверлей. Штатный Overlays().Remove
// удаляет ещё и все оверлеи выше него — например, страницу, открытую пока
// модальное окно закрывалось.
func removeOverlay(c fyne.Canvas, o fyne.CanvasObject) {
	list := c.Overlays().List()
	idx := -1
	for i, x := range list {
		if x == o {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	above := append([]fyne.CanvasObject(nil), list[idx+1:]...)
	c.Overlays().Remove(o)
	for _, a := range above {
		c.Overlays().Add(a)
	}
}

func heightAtWidth(o fyne.CanvasObject, w float32) float32 {
	o.Resize(fyne.NewSize(w, o.Size().Height))
	return o.MinSize().Height
}

// vboxFit — как VBox, но не требует ширины по содержимому: всё укладывается
// в доступную ширину, а не раздвигает прокручиваемую область за край окна.
type vboxFit struct{}

func (vboxFit) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	var y float32
	pad := theme.Padding()
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := heightAtWidth(o, size.Width)
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + pad
	}
}

func (vboxFit) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h += o.MinSize().Height
		n++
	}
	if n > 1 {
		h += float32(n-1) * theme.Padding()
	}
	return fyne.NewSize(0, h)
}

func newVBoxFit() *fyne.Container { return container.New(vboxFit{}) }

// richText — текст с переносом (в т.ч. длинных слов без пробелов).
func richText(s string, muted, bold bool) *widget.RichText {
	style := widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText}
	if !muted {
		style = widget.RichTextStyle{ColorName: theme.ColorNameForeground, SizeName: theme.SizeNameText, TextStyle: fyne.TextStyle{Bold: bold}}
	}
	rt := widget.NewRichText(&widget.TextSegment{Text: s, Style: style})
	rt.Wrapping = fyne.TextWrapBreak
	return rt
}

// shake — короткое покачивание карточки по горизонтали (ошибка ввода).
func (m *modalCard) shake() {
	a := fyne.NewAnimation(420*time.Millisecond, func(p float32) {
		m.lay.offX = float32(math.Sin(float64(p)*6*math.Pi)) * 12 * (1 - p)
		if p >= 1 {
			m.lay.offX = 0
		}
		m.relayout()
	})
	a.Start()
}

// thinBar — бесконечный индикатор прогресса малой высоты (стандартный слишком толстый).
type thinBar struct{ widget.ProgressBarInfinite }

const thinBarHeight = 4

func newThinBar() *thinBar {
	b := &thinBar{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *thinBar) MinSize() fyne.Size { return fyne.NewSize(40, thinBarHeight) }

// prose — приглушённый текст с переносом по словам (описания и подсказки);
// richText переносит и посреди слова, что нужно для имён и адресов без пробелов.
func prose(s string) *widget.RichText {
	rt := richText(s, true, false)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

const firstFrameDelay = 50 * time.Millisecond

// relayout передвигает окно по текущему состоянию анимации. В отличие от Refresh контейнера,
// не перерисовывает всё содержимое на каждом кадре, а только перекладывает слои.
func (m *modalCard) relayout() {
	m.lay.Layout(m.root.Objects, m.root.Size())
	m.dim.Refresh()
	m.curtain.Refresh()
}

// framePending — число отложенных «первых кадров» анимаций, которые ещё не отработали.
// Тесты дожидаются нуля перед сменой тестового приложения: иначе отложенный вызов
// попадает в момент сброса кэшей темы и падает с nil pointer.
var framePending atomic.Int32

// afterFrame запускает f через firstFrameDelay и учитывает её в framePending.
// afterFrameHook, если задан (в тестах), заменяет отложенный запуск: Fyne не рассчитан на параллельную
// работу с интерфейсом из таймеров и теста, а тестовый драйвер выполняет fyne.Do сразу, в горутине таймера.
var afterFrameHook func(func())

func afterFrame(f func()) {
	if afterFrameHook != nil {
		afterFrameHook(f)
		return
	}
	framePending.Add(1)
	time.AfterFunc(firstFrameDelay, func() {
		defer framePending.Add(-1)
		f()
	})
}
