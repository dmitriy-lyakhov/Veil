package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/assets"
)

// Шрифты: Montserrat из internal/assets (SIL Open Font License).
var (
	resMedium   = fyne.NewStaticResource("Montserrat-Medium.ttf", assets.FontMedium)
	resSemiBold = fyne.NewStaticResource("Montserrat-SemiBold.ttf", assets.FontSemiBold)
	resBold     = fyne.NewStaticResource("Montserrat-Bold.ttf", assets.FontBold)
)

// Цвета дизайн-системы Veil (токены из макета). Переменные, потому что тема переключается на лету:
// applyPalette перезаписывает их, после чего интерфейс пересобирается.
var (
	ColBackground color.NRGBA // surface
	ColCard       color.NRGBA // surface-raised
	ColCard2      color.NRGBA // surface-high
	ColLine       color.NRGBA // разделители
	ColBorder     color.NRGBA // граница карточек и полей
	colBorderHot  color.NRGBA // граница под курсором
	colShadow     color.NRGBA // тень диалогов и панелей
	colMenu       color.NRGBA // фон выпадающих списков
	ColText       color.NRGBA // ink
	ColMuted      color.NRGBA // ink-muted
	ColAccent     color.NRGBA // accent
	ColAccentDark color.NRGBA // on-accent: текст и значок на акцентной заливке
	ColAccentInk  color.NRGBA // accent-ink: акцент как текст или тонкий значок
	ColOK         color.NRGBA // success-ink
	ColBad        color.NRGBA // ошибка (accent-ink)
	ColIdle       color.NRGBA // выключенное кольцо, дорожка
	ColCardHover  color.NRGBA
	ColCard2Hover color.NRGBA
	ColBar        color.NRGBA // заголовок окна / панели
	colScrim      color.NRGBA
	PaletteDark   bool
)

func nrgba(hex uint32) color.NRGBA {
	return color.NRGBA{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 0xff}
}

// mix смешивает два цвета: t=0 — a, t=1 — b.
func Mix(a, b color.NRGBA, t float32) color.NRGBA {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

func init() { ApplyPalette(true) }

func ApplyPalette(dark bool) {
	PaletteDark = dark
	ColAccent = nrgba(0xFF393D)
	ColAccentDark = nrgba(0x0B0B0C)
	if dark {
		ColBackground, ColCard, ColCard2 = nrgba(0x0B0B0C), nrgba(0x1B1C1F), nrgba(0x2A2B30)
		ColText, ColMuted = nrgba(0xFFFFFF), nrgba(0xA8AAB2)
		ColAccentInk, ColOK = nrgba(0xFF393D), nrgba(0x7DE0A0)
		ColBar = nrgba(0x1B1C1F)
		colScrim = color.NRGBA{0x0B, 0x0B, 0x0C, 0xC7}
		colShadow = color.NRGBA{0, 0, 0, 0x2e}
		colMenu = nrgba(0x34353B)
	} else {
		ColBackground, ColCard, ColCard2 = nrgba(0xFFFFFF), nrgba(0xF2F3F5), nrgba(0xE4E6EA)
		ColText, ColMuted = nrgba(0x0B0B0C), nrgba(0x5B6068)
		ColAccentInk, ColOK = nrgba(0xDB1E32), nrgba(0x177A44)
		ColBar = nrgba(0xF2F3F5)
		colScrim = color.NRGBA{0x0B, 0x0B, 0x0C, 0x8C}
		colShadow = color.NRGBA{0x0B, 0x0B, 0x0C, 0x33}
		colMenu = nrgba(0xFFFFFF)
	}
	ColLine = ColCard2
	ColBorder = Mix(ColCard2, ColText, 0.07)
	colBorderHot = Mix(ColCard2, ColText, 0.28)
	ColBad = ColAccentInk
	ColIdle = ColCard2
	ColCardHover = Mix(ColCard, ColText, 0.04)
	ColCard2Hover = Mix(ColCard2, ColText, 0.06)
}

type VeilTheme struct{}

func (VeilTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return ColBackground
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameHeaderBackground:
		return ColCard2
	case theme.ColorNameMenuBackground:
		return colMenu
	case theme.ColorNameOverlayBackground:
		return ColCard
	case theme.ColorNameDisabledButton:
		return ColCard
	case theme.ColorNameForeground:
		return ColText
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return ColMuted
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return ColAccent
	case theme.ColorNameFocus:
		return ColAccentInk
	case theme.ColorNameForegroundOnPrimary:
		return ColAccentDark
	case theme.ColorNameInputBorder:
		return ColBorder
	case theme.ColorNameSeparator:
		return ColLine
	case theme.ColorNameSelection:
		return color.NRGBA{ColAccent.R, ColAccent.G, ColAccent.B, 0x55}
	case theme.ColorNameHover:
		return color.NRGBA{ColText.R, ColText.G, ColText.B, 0x10}
	case theme.ColorNamePressed:
		return color.NRGBA{ColText.R, ColText.G, ColText.B, 0x1a}
	case theme.ColorNameSuccess:
		return ColOK
	case theme.ColorNameError:
		return ColAccentInk
	case theme.ColorNameShadow:
		if PaletteDark {
			return color.NRGBA{0, 0, 0, 0x99}
		}
		return color.NRGBA{0x0B, 0x0B, 0x0C, 0x40}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

// Font: Montserrat 500 для текста, 600 для заголовков и строк (полужирный стиль Fyne);
// моноширинный стиль занят под цифры, которые тикают: жирное начертание.
func (VeilTheme) Font(s fyne.TextStyle) fyne.Resource {
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

func (VeilTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	if name, ok := themeIconMap[n]; ok {
		if r := IconRes(name, ColText); r != nil {
			return r
		}
	}
	return theme.DefaultTheme().Icon(n)
}

func (VeilTheme) Size(n fyne.ThemeSizeName) float32 {
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

func Card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColCard)
	bg.CornerRadius = 18
	bg.StrokeColor, bg.StrokeWidth = ColBorder, 1
	return container.NewStack(bg, container.NewPadded(container.NewPadded(content)))
}

func Text(s string, size float32, c color.Color, bold bool) *canvas.Text {
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

func NewTapCard(content fyne.CanvasObject, base, hover color.Color, onTap func()) *tapCard {
	bg := canvas.NewRectangle(base)
	bg.CornerRadius = 18
	bg.StrokeColor, bg.StrokeWidth = ColBorder, 1
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
	c.bg.StrokeColor = ColBorder
	c.fadeTo(c.base)
}

func (c *tapCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

// cardTight — компактная карточка для списков (меньше полей, чем у card).
func cardTight(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = ColBorder, 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(4, 4, 10, 6), content))
}

// newTapCardR — нажимаемая карточка с заданным радиусом и внутренним отступом (по дизайну).
func NewTapCardR(content fyne.CanvasObject, base, hover color.Color, radius, pad float32, onTap func()) *tapCard {
	c := NewTapCard(content, base, hover, onTap)
	c.bg.CornerRadius = radius
	c.pad = pad
	return c
}
