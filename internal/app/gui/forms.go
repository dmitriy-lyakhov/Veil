package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
)

// flatTheme убирает у поля ввода рамку, фон и внутренний отступ: оформление даёт карточка вокруг.
type flatTheme struct{ kit.VeilTheme }

func (f flatTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameInputBackground, theme.ColorNameInputBorder:
		return color.Transparent
	}
	return f.VeilTheme.Color(n, v)
}

func (f flatTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInnerPadding, theme.SizeNameInputBorder:
		return 0
	case theme.SizeNameText:
		return 16
	}
	return f.VeilTheme.Size(n)
}

func (f flatTheme) Font(s fyne.TextStyle) fyne.Resource {
	s.Bold = true
	return f.VeilTheme.Font(s)
}

// fieldCard — поле формы по дизайну: карточка от 64 px, подпись 12 px сверху, значение 16 px/600.
func fieldCard(label string, value fyne.CanvasObject, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(kit.ColCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
	bg.SetMinSize(fyne.NewSize(1, 64))
	body := container.New(&vstackLayout{gap: 4}, kit.Text(label, 12, kit.ColMuted, false), value)
	var inner fyne.CanvasObject = container.NewBorder(nil, nil, nil, trailing, container.NewVBox(layout.NewSpacer(), body, layout.NewSpacer()))
	if trailing == nil {
		inner = container.NewVBox(layout.NewSpacer(), body, layout.NewSpacer())
	}
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16), inner))
	if onTap == nil {
		return card
	}
	return kit.NewRowTap(card, onTap)
}

// fieldEntry — карточка с полем ввода.
func fieldEntry(label string, e *widget.Entry) fyne.CanvasObject {
	return withGap(fieldCard(label, container.NewThemeOverride(e, flatTheme{}), nil, nil))
}

// fieldSelect — карточка выбора из списка: по нажатию раскрывается меню под полем.
func (u *ui) fieldSelect(label, current string, options []string, changed func(string)) fyne.CanvasObject {
	val := kit.NewFitText(current, 16, kit.ColText, true)
	var card fyne.CanvasObject
	card = fieldCard(label, val, container.NewCenter(kit.IconImg("down", kit.ColMuted, 22)), func() {
		items := make([]*fyne.MenuItem, len(options))
		for i, o := range options {
			o := o
			items[i] = fyne.NewMenuItem(o, func() {
				val.SetText(o)
				changed(o)
			})
		}
		pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(card)
		pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+card.Size().Height+6))
		pop.Resize(fyne.NewSize(card.Size().Width, pop.MinSize().Height))
	})
	return withGap(card)
}

func (u *ui) showMenuAt(anchor fyne.CanvasObject, options []string, pick func(string)) {
	items := make([]*fyne.MenuItem, len(options))
	for i, o := range options {
		o := o
		items[i] = fyne.NewMenuItem(o, func() { pick(o) })
	}
	pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+anchor.Size().Height+6))
	pop.Resize(fyne.NewSize(anchor.Size().Width, pop.MinSize().Height))
}

// withGap оставлена для совместимости: шаг между полями даёт контейнер страницы (pageGap).
func withGap(o fyne.CanvasObject) fyne.CanvasObject { return o }

// bigButton — кнопка 52 px: главная (акцент) или обычная на карточке.
func bigButton(label string, icon string, primary bool, onTap func()) fyne.CanvasObject {
	var bg, hv color.Color = kit.ColCard, kit.ColCardHover
	var fg color.Color = kit.ColText
	if primary {
		bg, hv, fg = kit.ColAccent, kit.Mix(kit.ColAccent, kit.ColText, 0.12), kit.ColAccentDark
	}
	return container.New(&kit.FillMin{H: 52}, kit.NewTapCardR(kit.IconLabelRow(icon, label, 16, fg), bg, hv, 14, 1, onTap))
}

func spacerH(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

func infoBG() fyne.CanvasObject {
	r := canvas.NewRectangle(kit.ColCard)
	r.CornerRadius = 14
	r.StrokeColor, r.StrokeWidth = kit.ColBorder, 1
	return r
}

// infoNote — пояснение в карточке со значком «info» (13 px, как на страницах DNS и «Резервная копия»).
func infoNote(s string) fyne.CanvasObject { return noteBox(kit.IconImg("info", kit.ColMuted, 22), s) }

// dangerButton — кнопка 52 px на карточке с красным текстом (удаление, очистка).
func dangerButton(label string, onTap func()) fyne.CanvasObject {
	return container.New(&kit.FillMin{H: 52}, kit.NewTapCardR(kit.IconLabelRow("", label, 16, kit.ColAccentInk), kit.ColCard, kit.ColCardHover, 14, 1, onTap))
}

// infoRow — строка «подпись — значение» высотой 52 px; при нажатии копирует значение.
func infoRow(label, value string, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(kit.ColCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
	bg.SetMinSize(fyne.NewSize(1, 52))
	val := kit.NewFitText(value, 14, kit.ColText, true)
	val.Alignment = fyne.TextAlignTrailing
	row := container.NewBorder(nil, nil, container.NewCenter(kit.Text(label, 14, kit.ColMuted, false)), nil, kit.Vcenter(val))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), row))
	if onTap == nil {
		return card
	}
	return kit.NewRowTap(card, onTap)
}

// optionRow — строка выбора значения справа: по нажатию раскрывается меню.
func (u *ui) optionRow(label, current string, options []string, changed func(string)) fyne.CanvasObject {
	bg := canvas.NewRectangle(kit.ColCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = kit.ColBorder, 1
	bg.SetMinSize(fyne.NewSize(1, 52))
	val := kit.NewFitText(current, 14, kit.ColText, false)
	val.Alignment = fyne.TextAlignTrailing
	row := container.NewBorder(nil, nil, container.NewCenter(kit.Text(label, 14, kit.ColMuted, false)),
		container.NewCenter(kit.IconImg("down", kit.ColMuted, 20)), kit.Vcenter(val))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), row))
	return kit.NewRowTap(card, func() {
		items := make([]*fyne.MenuItem, len(options))
		for i, o := range options {
			o := o
			items[i] = fyne.NewMenuItem(o, func() {
				val.SetText(o)
				changed(o)
			})
		}
		pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(card)
		pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+card.Size().Height+6))
	})
}

// bigButtonDanger — кнопка 52 px на карточке с красным текстом и значком (дизайн: btn 'danger').
func bigButtonDanger(label, icon string, onTap func()) fyne.CanvasObject {
	return container.New(&kit.FillMin{H: 52}, kit.NewTapCardR(kit.IconLabelRow(icon, label, 16, kit.ColAccentInk), kit.ColCard, kit.ColCardHover, 14, 1, onTap))
}

// menuRow — строка страницы (64 px): заголовок, значение под ним, шеврон «вниз»; по нажатию раскрывается меню выбора.
func (u *ui) menuRow(title, current string, options []string, changed func(string)) fyne.CanvasObject {
	var row fyne.CanvasObject
	row = kit.ListRow(title, current, container.NewCenter(kit.IconImg("down", kit.ColMuted, 22)), func() {
		items := make([]*fyne.MenuItem, len(options))
		for i, o := range options {
			o := o
			items[i] = fyne.NewMenuItem(o, func() { changed(o) })
		}
		pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(row)
		pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+row.Size().Height+6))
	})
	return row
}
