package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	colBackground = color.NRGBA{0x0f, 0x10, 0x14, 0xff}
	colCard       = color.NRGBA{0x1a, 0x1b, 0x21, 0xff}
	colCard2      = color.NRGBA{0x24, 0x25, 0x2d, 0xff}
	colLine       = color.NRGBA{0x2e, 0x2f, 0x39, 0xff}
	colText       = color.NRGBA{0xec, 0xec, 0xf1, 0xff}
	colMuted      = color.NRGBA{0x8b, 0x8d, 0x98, 0xff}
	colAccent     = color.NRGBA{0x63, 0x66, 0xf1, 0xff}
	colAccentDark = color.NRGBA{0xff, 0xff, 0xff, 0xff} // текст и ползунок на акцентном фоне
	colOK         = color.NRGBA{0x4a, 0xde, 0x80, 0xff}
	colBad        = color.NRGBA{0xf8, 0x71, 0x71, 0xff}
	colIdle       = color.NRGBA{0x3a, 0x3c, 0x47, 0xff}
	colCardHover  = color.NRGBA{0x22, 0x23, 0x2b, 0xff}
	colCard2Hover = color.NRGBA{0x2e, 0x30, 0x3a, 0xff}
)

type veilTheme struct{}

func (veilTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameHeaderBackground:
		return colCard2
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return colCard
	case theme.ColorNameDisabledButton:
		return colCard
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return colAccent
	case theme.ColorNameForegroundOnPrimary:
		return colAccentDark
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colLine
	case theme.ColorNameSelection:
		return color.NRGBA{colAccent.R, colAccent.G, colAccent.B, 0x55}
	case theme.ColorNameHover:
		return color.NRGBA{0xff, 0xff, 0xff, 0x0f}
	case theme.ColorNamePressed:
		return color.NRGBA{0xff, 0xff, 0xff, 0x1a}
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameError:
		return colBad
	case theme.ColorNameShadow:
		return color.NRGBA{0, 0, 0, 0x66}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (veilTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (veilTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (veilTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius, theme.SizeNameSelectionRadius:
		return 12
	case theme.SizeNameDialogRadius, theme.SizeNameCardRadius, theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return 18
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 17
	}
	return theme.DefaultTheme().Size(n)
}

func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 18
	return container.NewStack(bg, container.NewPadded(container.NewPadded(content)))
}

func text(s string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle.Bold = bold
	return t
}

type tapCard struct {
	widget.BaseWidget
	content fyne.CanvasObject
	bg      *canvas.Rectangle
	base    color.Color
	hover   color.Color
	onTap   func()
	anim    *fyne.Animation
}

func newTapCard(content fyne.CanvasObject, base, hover color.Color, onTap func()) *tapCard {
	bg := canvas.NewRectangle(base)
	bg.CornerRadius = 18
	c := &tapCard{content: content, bg: bg, base: base, hover: hover, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapCard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(c.bg, container.NewPadded(container.NewPadded(c.content))))
}

func (c *tapCard) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapCard) MouseIn(*desktop.MouseEvent) { c.fadeTo(c.hover) }

func (c *tapCard) MouseMoved(*desktop.MouseEvent) {}

func (c *tapCard) MouseOut() { c.fadeTo(c.base) }

func (c *tapCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

// cardTight — компактная карточка для списков (меньше полей, чем у card).
func cardTight(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 10, 6), content))
}
