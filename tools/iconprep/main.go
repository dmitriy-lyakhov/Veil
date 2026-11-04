// iconprep: рисует значок приложения (знак Veil на тёмной плитке) в PNG.
// Запуск: go run ./tools/iconprep internal/gui/icon.png 256
// Для macOS: go run ./tools/iconprep packaging/macos/icon.png 1024 100
package main

import (
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><path d="M22 0H74C86.15 0 96 9.85 96 22V74C96 86.15 86.15 96 74 96H22C9.85 96 0 86.15 0 74V22C0 9.85 9.85 0 22 0Z" fill="#0B0B0C"/><path d="M25.25 27h14l15.75 40.25h-14z" fill="#FF393D"/><path d="M70.75 27h-14l-15.75 40.25h14z" fill="#6B6F78"/><path d="M48 49.4L41 67.25h14z" fill="#D9DBE0"/></svg>`

func main() {
	out := os.Args[1]
	size := 256
	if len(os.Args) > 2 {
		size, _ = strconv.Atoi(os.Args[2])
	}
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		panic(err)
	}
	margin := 0
	if len(os.Args) > 3 {
		margin, _ = strconv.Atoi(os.Args[3])
	}
	inner := float64(size - 2*margin)
	icon.SetTarget(float64(margin), float64(margin), inner, inner)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
