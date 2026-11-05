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

// extra — дополнительные строки под маршрутом (например, «Роль»).
func (u *ui) showShare(parent fyne.Window, title string, r server.LinkResponse, note string, installable bool, extra ...fyne.CanvasObject) {
	u.showShareX(parent, title, r, note, installable, extra, nil)
}

// showShareX — то же, плюс строки after: в самом низу окна, под кнопками (например, «Отозвать»).
func (u *ui) showShareX(parent fyne.Window, title string, r server.LinkResponse, note string, installable bool, extra, after []fyne.CanvasObject) {
	link := widget.NewMultiLineEntry()
	link.SetText(r.Link)
	link.Wrapping = fyne.TextWrapBreak
	link.SetMinRowsVisible(3)
	hint := widget.NewLabel(note)
	hint.Wrapping = fyne.TextWrapWord
	rows := container.NewVBox(qrImage(r.Link, r.QR))
	if r.Via != "" {
		v := widget.NewLabel(L("Route: ", "Маршрут: ") + r.Via)
		v.Wrapping = fyne.TextWrapWord
		rows.Add(v)
	}
	for _, e := range extra {
		rows.Add(e)
	}
	rows.Add(hint)
	rows.Add(link)
	copyBtn := widget.NewButtonWithIcon(L("Copy", "Скопировать"), theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(r.Link)
	})
	saveBtn := widget.NewButtonWithIcon(L("File…", "Файл…"), theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			defer w.Close()
			if _, err := w.Write([]byte(r.Link + "\n")); err != nil {
				showError(err, parent)
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
		install := widget.NewButtonWithIcon(L("Add to this device", "Добавить на это устройство"), theme.ContentAddIcon(), func() {
			p, err := config.DecodeLink(r.Link)
			if err != nil {
				showError(err, parent)
				return
			}
			base := p.Name
			for i := 2; u.profileExists(p.Name); i++ {
				p.Name = fmt.Sprintf("%s %d", base, i)
			}
			if err := saveProfile(p); err != nil {
				showError(err, parent)
				return
			}
			u.reload()
			u.selectProfile(p.Name)
			showInfo(L("Done", "Готово"), L("Server \""+p.Name+"\" was added to the app.", "Сервер «"+p.Name+"» добавлен в приложение."), parent)
		})
		install.Importance = widget.HighImportance
		rows.Add(install)
	}
	for _, a := range after {
		rows.Add(a)
	}
	d := newCustom(title, L("Close", "Закрыть"), container.NewGridWrap(fyne.NewSize(380, 560), container.NewVScroll(rows)), parent)
	d.Show()
}
