package gui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Шрифты: Montserrat (SIL Open Font License, см. fonts/OFL.txt).
var (
	//go:embed fonts/Montserrat-Medium.ttf
	fontMedium []byte
	//go:embed fonts/Montserrat-SemiBold.ttf
	fontSemiBold []byte
	//go:embed fonts/Montserrat-Bold.ttf
	fontBold []byte

	resMedium   = fyne.NewStaticResource("Montserrat-Medium.ttf", fontMedium)
	resSemiBold = fyne.NewStaticResource("Montserrat-SemiBold.ttf", fontSemiBold)
	resBold     = fyne.NewStaticResource("Montserrat-Bold.ttf", fontBold)
)

// Цвета дизайн-системы Veil (токены из макета). Переменные, потому что тема переключается на лету:
// applyPalette перезаписывает их, после чего интерфейс пересобирается.
var (
	colBackground color.NRGBA // surface
	colCard       color.NRGBA // surface-raised
	colCard2      color.NRGBA // surface-high
	colLine       color.NRGBA // разделители
	colBorder     color.NRGBA // граница карточек и полей
	colBorderHot  color.NRGBA // граница под курсором
	colShadow     color.NRGBA // тень диалогов и панелей
	colMenu       color.NRGBA // фон выпадающих списков
	colText       color.NRGBA // ink
	colMuted      color.NRGBA // ink-muted
	colAccent     color.NRGBA // accent
	colAccentDark color.NRGBA // on-accent: текст и значок на акцентной заливке
	colAccentInk  color.NRGBA // accent-ink: акцент как текст или тонкий значок
	colOK         color.NRGBA // success-ink
	colBad        color.NRGBA // ошибка (accent-ink)
	colIdle       color.NRGBA // выключенное кольцо, дорожка
	colCardHover  color.NRGBA
	colCard2Hover color.NRGBA
	colBar        color.NRGBA // заголовок окна / панели
	colScrim      color.NRGBA
	paletteDark   bool
)

func nrgba(hex uint32) color.NRGBA {
	return color.NRGBA{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 0xff}
}

// mix смешивает два цвета: t=0 — a, t=1 — b.
func mix(a, b color.NRGBA, t float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

func init() { applyPalette(true) }

func applyPalette(dark bool) {
	paletteDark = dark
	colAccent = nrgba(0xFF393D)
	colAccentDark = nrgba(0x0B0B0C)
	if dark {
		colBackground, colCard, colCard2 = nrgba(0x0B0B0C), nrgba(0x1B1C1F), nrgba(0x2A2B30)
		colText, colMuted = nrgba(0xFFFFFF), nrgba(0xA8AAB2)
		colAccentInk, colOK = nrgba(0xFF393D), nrgba(0x7DE0A0)
		colBar = nrgba(0x1B1C1F)
		colScrim = color.NRGBA{0x0B, 0x0B, 0x0C, 0xC7}
		colShadow = color.NRGBA{0, 0, 0, 0x2e}
		colMenu = nrgba(0x34353B)
	} else {
		colBackground, colCard, colCard2 = nrgba(0xFFFFFF), nrgba(0xF2F3F5), nrgba(0xE4E6EA)
		colText, colMuted = nrgba(0x0B0B0C), nrgba(0x5B6068)
		colAccentInk, colOK = nrgba(0xDB1E32), nrgba(0x177A44)
		colBar = nrgba(0xF2F3F5)
		colScrim = color.NRGBA{0x0B, 0x0B, 0x0C, 0x8C}
		colShadow = color.NRGBA{0x0B, 0x0B, 0x0C, 0x33}
		colMenu = nrgba(0xFFFFFF)
	}
	colLine = colCard2
	colBorder = mix(colCard2, colText, 0.07)
	colBorderHot = mix(colCard2, colText, 0.28)
	colBad = colAccentInk
	colIdle = colCard2
	colCardHover = mix(colCard, colText, 0.04)
	colCard2Hover = mix(colCard2, colText, 0.06)
}

type veilTheme struct{}

func (veilTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameHeaderBackground:
		return colCard2
	case theme.ColorNameMenuBackground:
		return colMenu
	case theme.ColorNameOverlayBackground:
		return colCard
	case theme.ColorNameDisabledButton:
		return colCard
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return colAccent
	case theme.ColorNameFocus:
		return colAccentInk
	case theme.ColorNameForegroundOnPrimary:
		return colAccentDark
	case theme.ColorNameInputBorder:
		return colBorder
	case theme.ColorNameSeparator:
		return colLine
	case theme.ColorNameSelection:
		return color.NRGBA{colAccent.R, colAccent.G, colAccent.B, 0x55}
	case theme.ColorNameHover:
		return color.NRGBA{colText.R, colText.G, colText.B, 0x10}
	case theme.ColorNamePressed:
		return color.NRGBA{colText.R, colText.G, colText.B, 0x1a}
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameError:
		return colAccentInk
	case theme.ColorNameShadow:
		if paletteDark {
			return color.NRGBA{0, 0, 0, 0x99}
		}
		return color.NRGBA{0x0B, 0x0B, 0x0C, 0x40}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

// Font: Montserrat 500 для текста, 600 для заголовков и строк (полужирный стиль Fyne);
// моноширинный стиль занят под цифры, которые тикают: жирное начертание.
func (veilTheme) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Monospace:
		return resBold
	case s.Bold:
		return resSemiBold
	}
	return resMedium
}

var themeIconMap = map[fyne.ThemeIconName]string{
	theme.IconNameMoveDown:           "down",
	theme.IconNameArrowDropDown:      "down",
	theme.IconNameNavigateNext:       "right",
	theme.IconNameNavigateBack:       "back",
	theme.IconNameContentAdd:         "plus",
	theme.IconNameContentRemove:      "close",
	theme.IconNameCancel:             "close",
	theme.IconNameWindowClose:        "close",
	theme.IconNameCheckButtonChecked: "check",
	theme.IconNameConfirm:            "check",
	theme.IconNameSearch:             "search",
	theme.IconNameContentCopy:        "copy",
	theme.IconNameContentPaste:       "copy",
	theme.IconNameDelete:             "trash",
	theme.IconNameDocumentCreate:     "edit",
	theme.IconNameViewRefresh:        "refresh",
	theme.IconNameInfo:               "info",
	theme.IconNameWarning:            "warn",
	theme.IconNameError:              "warn",
	theme.IconNameDownload:           "download",
	theme.IconNameUpload:             "download",
	theme.IconNameFile:               "file",
	theme.IconNameSettings:           "sliders",
	theme.IconNameHome:               "home",
	theme.IconNameAccount:            "user",
	theme.IconNameMailForward:        "share",
}

func (veilTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	if name, ok := themeIconMap[n]; ok {
		if r := iconRes(name, colText); r != nil {
			return r
		}
	}
	return theme.DefaultTheme().Icon(n)
}

func (veilTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius, theme.SizeNameSelectionRadius:
		return 14 // radius-md
	case theme.SizeNameDialogRadius, theme.SizeNameCardRadius, theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return 16 // radius-lg
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNamePadding:
		return 8 // space-2
	case theme.SizeNameInnerPadding:
		return 12 // space-3
	case theme.SizeNameText:
		return 15
	case theme.SizeNameCaptionText:
		return 13
	case theme.SizeNameSubHeadingText:
		return 16 // body: заголовки строк
	case theme.SizeNameHeadingText:
		return 20 // heading
	}
	return theme.DefaultTheme().Size(n)
}

func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 18
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	return container.NewStack(bg, container.NewPadded(container.NewPadded(content)))
}

func text(s string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle.Bold = bold
	return t
}

type tapCard struct {
	pad float32
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
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	c := &tapCard{content: content, bg: bg, base: base, hover: hover, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapCard) CreateRenderer() fyne.WidgetRenderer {
	if c.pad > 0 {
		return widget.NewSimpleRenderer(container.NewStack(c.bg, container.New(layout.NewCustomPaddedLayout(c.pad, c.pad, c.pad, c.pad), c.content)))
	}
	return widget.NewSimpleRenderer(container.NewStack(c.bg, container.NewPadded(container.NewPadded(c.content))))
}

func (c *tapCard) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapCard) MouseIn(*desktop.MouseEvent) {
	c.bg.StrokeColor = colBorderHot
	c.fadeTo(c.hover)
}

func (c *tapCard) MouseMoved(*desktop.MouseEvent) {}

func (c *tapCard) MouseOut() {
	c.bg.StrokeColor = colBorder
	c.fadeTo(c.base)
}

func (c *tapCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

// cardTight — компактная карточка для списков (меньше полей, чем у card).
func cardTight(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 10, 6), content))
}

// newTapCardR — нажимаемая карточка с заданным радиусом и внутренним отступом (по дизайну).
func newTapCardR(content fyne.CanvasObject, base, hover color.Color, radius, pad float32, onTap func()) *tapCard {
	c := newTapCard(content, base, hover, onTap)
	c.bg.CornerRadius = radius
	c.pad = pad
	return c
}
