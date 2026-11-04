package gui

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Набор иконок дизайн-системы: сетка 24 px, штрих 2 px, квадратные окончания, острые углы.
var iconPaths = map[string]string{
	"home":     "M3 11l9-8 9 8M5 9.5V21h5v-6h4v6h5V9.5",
	"share":    "M17 3h4v4h-4zM3 10h4v4H3zM17 17h4v4h-4zM7 11l10-5M7 13l10 5",
	"sliders":  "M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0M14 4v4M8 10v4M16 16v4",
	"plus":     "M12 4v16M4 12h16",
	"down":     "M5 9l7 7 7-7",
	"right":    "M9 5l7 7-7 7",
	"back":     "M15 5l-7 7 7 7",
	"download": "M12 3v13M6 11l6 6 6-6M4 21h16",
	"check":    "M4 12l5 5 11-11",
	"search":   "M7 3h6l4 4v6l-4 4H7l-4-4V7zM16 16l5 5",
	"split":    "M4 20L20 4M11 4h9v9",
	"shield":   "M12 2l8 3v7l-8 10-8-10V5z",
	"shieldok": "M12 2l8 3v7l-8 10-8-10V5zM8 12l3 3 5-6",
	"close":    "M5 5l14 14M19 5L5 19",
	"server":   "M3 4h18v6H3zM3 14h18v6H3zM7 7h2M7 17h2",
	"qr":       "M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2v2h-2zM18 18h2v2h-2z",
	"info":     "M8 3h8l5 5v8l-5 5H8l-5-5V8zM12 11v6M12 7.5h.01",
	"warn":     "M12 3l10 18H2zM12 10v5M12 18h.01",
	"lock":     "M6 11h12v10H6zM8 11V7h8v4",
	"file":     "M6 2h9l5 5v15H6zM15 2v5h5",
	"camera":   "M3 7h4l2-3h6l2 3h4v13H3zM9 13h6v4H9z",
	"copy":     "M8 8h12v12H8zM4 16V4h12",
	"chart":    "M4 20V10M10 20V4M16 20v-8M22 20H2",
	"trash":    "M4 6h16M9 6V3h6v3M6 6v15h12V6M10 10v7M14 10v7",
	"edit":     "M4 20h4L20 8l-4-4L4 16zM14 6l4 4",
	"bell":     "M6 17V10a6 6 0 0112 0v7l2 2H4zM10 21h4",
	"refresh":  "M20 12a8 8 0 11-3-6M20 4v6h-6",
	"link":     "M10 14l4-4M8 12l-3 3a3 3 0 004 4l3-3M16 12l3-3a3 3 0 00-4-4l-3 3",
	"user":     "M9 3h6v6H9zM4 21v-6h16v6",
	"globe":    "M3 12h18M12 3v18M5 5h14v14H5z",
	"empty":    "M3 8l9-5 9 5v9l-9 5-9-5zM3 8l9 5 9-5M12 13v9",
}

// Знак Veil: два слоя вуали (красный и серый) и светлый треугольник на пересечении.
const (
	markSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M3 4h8l9 23h-8z" fill="#FF393D"/><path d="M29 4h-8l-9 23h8z" fill="#6B6F78"/><path d="M16 16.8L12 27h8z" fill="#D9DBE0"/></svg>`
)

var (
	iconMu    sync.Mutex
	iconCache = map[string]fyne.Resource{}
)

func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// iconRes возвращает SVG-иконку нужного цвета (тема меняется — цвет задаёт вызывающий).
func iconRes(name string, c color.Color) fyne.Resource {
	d, ok := iconPaths[name]
	if !ok {
		return nil
	}
	hex := hexOf(c)
	key := name + hex
	iconMu.Lock()
	defer iconMu.Unlock()
	if r, ok := iconCache[key]; ok {
		return r
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="%s" stroke-width="2" stroke-linecap="square" stroke-linejoin="miter"><path d="%s"/></svg>`, hex, d)
	r := fyne.NewStaticResource("veil-"+name+"-"+hex[1:]+".svg", []byte(svg))
	iconCache[key] = r
	return r
}

// iconImg — иконка фиксированного размера.
func iconImg(name string, c color.Color, size float32) *canvas.Image {
	img := canvas.NewImageFromResource(iconRes(name, c))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}

var markRes = fyne.NewStaticResource("veil-mark.svg", []byte(markSVG))

// markImg — знак Veil.
func markImg(size float32) *canvas.Image {
	img := canvas.NewImageFromResource(markRes)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	return img
}
