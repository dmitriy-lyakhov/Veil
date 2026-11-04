package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// flatTheme убирает у поля ввода рамку, фон и внутренний отступ: оформление даёт карточка вокруг.
type flatTheme struct{ veilTheme }

func (f flatTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameInputBackground, theme.ColorNameInputBorder:
		return color.Transparent
	}
	return f.veilTheme.Color(n, v)
}

func (f flatTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInnerPadding, theme.SizeNameInputBorder:
		return 0
	case theme.SizeNameText:
		return 16
	}
	return f.veilTheme.Size(n)
}

func (f flatTheme) Font(s fyne.TextStyle) fyne.Resource {
	s.Bold = true
	return f.veilTheme.Font(s)
}

// fieldCard — поле формы по дизайну: карточка от 64 px, подпись 12 px сверху, значение 16 px/600.
func fieldCard(label string, value fyne.CanvasObject, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.SetMinSize(fyne.NewSize(1, 64))
	body := container.New(&vstackLayout{gap: 4}, text(label, 12, colMuted, false), value)
	var inner fyne.CanvasObject = container.NewBorder(nil, nil, nil, trailing, container.NewVBox(layout.NewSpacer(), body, layout.NewSpacer()))
	if trailing == nil {
		inner = container.NewVBox(layout.NewSpacer(), body, layout.NewSpacer())
	}
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16), inner))
	if onTap == nil {
		return card
	}
	return newRowTap(card, onTap)
}

// fieldEntry — карточка с полем ввода.
func fieldEntry(label string, e *widget.Entry) fyne.CanvasObject {
	return withGap(fieldCard(label, container.NewThemeOverride(e, flatTheme{}), nil, nil))
}

// fieldSelect — карточка выбора из списка: по нажатию раскрывается меню под полем.
func (u *ui) fieldSelect(label, current string, options []string, changed func(string)) fyne.CanvasObject {
	val := text(current, 16, colText, true)
	var card fyne.CanvasObject
	card = fieldCard(label, val, container.NewCenter(iconImg("down", colMuted, 22)), func() {
		items := make([]*fyne.MenuItem, len(options))
		for i, o := range options {
			o := o
			items[i] = fyne.NewMenuItem(o, func() {
				val.Text = o
				val.Refresh()
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

// withGap добавляет 4 px под полем: вместе с зазором контейнера получается шаг 12 px, как в дизайне.
func withGap(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&vstackLayout{}, o, spacerH(4))
}

// bigButton — кнопка 52 px: главная (акцент) или обычная на карточке.
func bigButton(label string, icon string, primary bool, onTap func()) fyne.CanvasObject {
	var bg, hv color.Color = colCard, colCardHover
	var fg color.Color = colText
	if primary {
		bg, hv, fg = colAccent, mix(colAccent, colText, 0.12), colAccentDark
	}
	row := container.NewHBox()
	if icon != "" {
		row.Add(container.NewCenter(iconImg(icon, fg, 20)))
	}
	row.Add(text(label, 16, fg, true))
	return container.New(&fillMin{h: 52}, newTapCardR(container.NewCenter(row), bg, hv, 14, 1, onTap))
}

// fillMin растягивает содержимое на всю область и задаёт минимальную высоту.
type fillMin struct{ h float32 }

func (f *fillMin) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Resize(s)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (f *fillMin) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objs {
		w = max(w, o.MinSize().Width)
	}
	return fyne.NewSize(w, f.h)
}

func spacerH(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

func infoBG() fyne.CanvasObject {
	r := canvas.NewRectangle(colCard)
	r.CornerRadius = 14
	return r
}

// infoNote — пояснение в карточке (13 px, как на страницах DNS и «Новый сервер»).
func infoNote(s string) fyne.CanvasObject {
	return container.NewStack(infoBG(), container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16), prose(s)))
}

// dangerButton — кнопка 52 px на карточке с красным текстом (удаление, очистка).
func dangerButton(label string, onTap func()) fyne.CanvasObject {
	return container.New(&fillMin{h: 52}, newTapCardR(container.NewCenter(text(label, 16, colAccentInk, true)), colCard, colCardHover, 14, 1, onTap))
}

// infoRow — строка «подпись — значение» высотой 52 px; при нажатии копирует значение.
func infoRow(label, value string, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.SetMinSize(fyne.NewSize(1, 52))
	val := text(value, 14, colText, false)
	val.Alignment = fyne.TextAlignTrailing
	row := container.NewBorder(nil, nil, container.NewCenter(text(label, 14, colMuted, false)), container.NewCenter(val))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), row))
	if onTap == nil {
		return card
	}
	return newRowTap(card, onTap)
}

// optionRow — строка выбора значения справа: по нажатию раскрывается меню.
func (u *ui) optionRow(label, current string, options []string, changed func(string)) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.SetMinSize(fyne.NewSize(1, 52))
	val := text(current, 14, colText, false)
	row := container.NewBorder(nil, nil, container.NewCenter(text(label, 14, colMuted, false)),
		container.NewHBox(container.NewCenter(val), container.NewCenter(iconImg("down", colMuted, 20))))
	card := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), row))
	return newRowTap(card, func() {
		items := make([]*fyne.MenuItem, len(options))
		for i, o := range options {
			o := o
			items[i] = fyne.NewMenuItem(o, func() {
				val.Text = o
				val.Refresh()
				changed(o)
			})
		}
		pop := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(card)
		pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+card.Size().Height+6))
	})
}
