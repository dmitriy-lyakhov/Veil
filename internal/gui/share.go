package gui

import (
	"encoding/base64"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	qrcode "github.com/skip2/go-qrcode"

	"veil/internal/config"
	"veil/internal/server"
)

const (
	qrDisplay = 230
	qrPixels  = 512
)

func qrImage(link, dataURI string) fyne.CanvasObject {
	var png []byte
	if b64, ok := strings.CutPrefix(dataURI, "data:image/png;base64,"); ok {
		png, _ = base64.StdEncoding.DecodeString(b64)
	}
	if len(png) == 0 {
		png, _ = qrcode.Encode(link, qrcode.Medium, qrPixels)
	}
	img := canvas.NewImageFromResource(fyne.NewStaticResource("qr.png", png))
	img.FillMode = canvas.ImageFillContain
	bg := canvas.NewRectangle(colText)
	bg.CornerRadius = 14
	return container.NewCenter(container.NewGridWrap(fyne.NewSize(qrDisplay, qrDisplay), container.NewStack(bg, container.NewPadded(img))))
}

func (u *ui) showShare(parent fyne.Window, title string, r server.LinkResponse, note string, installable bool) {
	link := widget.NewMultiLineEntry()
	link.SetText(r.Link)
	link.Wrapping = fyne.TextWrapBreak
	link.SetMinRowsVisible(3)
	hint := widget.NewLabel(note)
	hint.Wrapping = fyne.TextWrapWord
	rows := container.NewVBox(qrImage(r.Link, r.QR))
	if r.Via != "" {
		v := widget.NewLabel("Маршрут: " + r.Via)
		v.Wrapping = fyne.TextWrapWord
		rows.Add(v)
	}
	rows.Add(hint)
	rows.Add(link)
	copyBtn := widget.NewButtonWithIcon("Скопировать", theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(r.Link)
	})
	saveBtn := widget.NewButtonWithIcon("Файл…", theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			defer w.Close()
			if _, err := w.Write([]byte(r.Link + "\n")); err != nil {
				dialog.ShowError(err, parent)
			}
		}, parent)
		name := r.File
		if name == "" {
			name = config.SafeFileName(r.Name) + ".txt"
		}
		fd.SetFileName(name)
		fd.Show()
	})
	buttons := container.NewGridWithColumns(2, copyBtn, saveBtn)
	rows.Add(buttons)
	if installable {
		install := widget.NewButtonWithIcon("Добавить на это устройство", theme.ContentAddIcon(), func() {
			p, err := config.DecodeLink(r.Link)
			if err != nil {
				dialog.ShowError(err, parent)
				return
			}
			base := p.Name
			for i := 2; u.profileExists(p.Name); i++ {
				p.Name = fmt.Sprintf("%s %d", base, i)
			}
			if err := saveProfile(p); err != nil {
				dialog.ShowError(err, parent)
				return
			}
			u.reload()
			u.selectProfile(p.Name)
			dialog.ShowInformation("Готово", "Сервер «"+p.Name+"» добавлен в приложение.", parent)
		})
		install.Importance = widget.HighImportance
		rows.Add(install)
	}
	d := dialog.NewCustom(title, "Закрыть", container.NewGridWrap(fyne.NewSize(380, 560), container.NewVScroll(rows)), parent)
	d.Show()
}
