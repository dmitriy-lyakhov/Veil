// iconprep: обрезает прозрачные поля у иконки и уменьшает её (для заголовка окна Windows).
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func main() {
	in, _ := os.Open(os.Args[1])
	img, err := png.Decode(in)
	if err != nil {
		panic(err)
	}
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x2000 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	fmt.Println("bbox", minX, minY, maxX, maxY)
	crop := image.Rect(minX, minY, maxX+1, maxY+1)
	side := max(crop.Dx(), crop.Dy())
	dst := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, image.Rect(crop.Min.X, crop.Min.Y, crop.Min.X+side, crop.Min.Y+side), draw.Over, nil)
	out, _ := os.Create(os.Args[2])
	defer out.Close()
	_ = png.Encode(out, dst)
}
