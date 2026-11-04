// iconprep: рисует значок приложения (знак Veil на тёмной плитке) в PNG.
// Запуск: go run ./tools/iconprep internal/gui/icon.png 256
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

const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="22" fill="#0B0B0C"/><path d="M25.25 27h14l15.75 40.25h-14z" fill="#FF393D"/><path d="M70.75 27h-14l-15.75 40.25h14z" fill="#6B6F78"/><path d="M48 49.4L41 67.25h14z" fill="#D9DBE0"/></svg>`

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
	icon.SetTarget(0, 0, float64(size), float64(size))
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
