package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

const (
	shadowLayers = 4
	shadowSpread = 3
	shadowDrop   = 4
)

type shadowLayout struct{}

func (shadowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	n := len(objs) - 1
	for i := 0; i < n; i++ {
		g := float32(shadowSpread * (n - i))
		objs[i].Move(fyne.NewPos(-g, -g+shadowDrop))
		objs[i].Resize(fyne.NewSize(s.Width+2*g, s.Height+2*g))
	}
	objs[n].Move(fyne.NewPos(0, 0))
	objs[n].Resize(s)
}

func (shadowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[len(objs)-1].MinSize()
}

func withShadow(content fyne.CanvasObject, radius float32) fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, shadowLayers+1)
	for i := 0; i < shadowLayers; i++ {
		r := canvas.NewRectangle(color.NRGBA{colShadow.R, colShadow.G, colShadow.B, colShadow.A / shadowLayers / 2})
		r.CornerRadius = radius + float32(shadowSpread*(shadowLayers-i))
		objs = append(objs, r)
	}
	objs = append(objs, content)
	return container.New(shadowLayout{}, objs...)
}
