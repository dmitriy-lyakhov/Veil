// iconprep: рисует значки приложения в PNG.
//
//	go run ./tools/iconprep internal/gui/icon.png 256            значок приложения (знак на тёмной плитке)
//	go run ./tools/iconprep internal/gui/tray_on.png 64 tray-on   значок в трее при подключении (красный слой)
//	go run ./tools/iconprep internal/gui/tray_off.png 64 tray-off значок в трее при отключении (полностью серый)
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

const (
	appSVG   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="22" fill="#0B0B0C"/><path d="M25.25 27h14l15.75 40.25h-14z" fill="#FF393D"/><path d="M70.75 27h-14l-15.75 40.25h14z" fill="#6B6F78"/><path d="M48 49.4L41 67.25h14z" fill="#D9DBE0"/></svg>`
	trayOnSV = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M3 4h8l9 23h-8z" fill="#FF393D"/><path d="M29 4h-8l-9 23h8z" fill="#8A8E98"/><path d="M16 16.8L12 27h8z" fill="#D9DBE0"/></svg>`
	trayOff  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M3 4h8l9 23h-8z" fill="#6B6F78"/><path d="M29 4h-8l-9 23h8z" fill="#8A8E98"/><path d="M16 16.8L12 27h8z" fill="#B5B8C0"/></svg>`
)

func main() {
	out := os.Args[1]
	size := 256
	if len(os.Args) > 2 {
		size, _ = strconv.Atoi(os.Args[2])
	}
	svg := appSVG
	if len(os.Args) > 3 {
		switch os.Args[3] {
		case "tray-on":
			svg = trayOnSV
		case "tray-off":
			svg = trayOff
		}
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
