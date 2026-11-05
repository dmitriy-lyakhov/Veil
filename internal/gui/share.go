package gui

import (
	"encoding/base64"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	qrcode "github.com/skip2/go-qrcode"

	"veil/internal/config"
	"veil/internal/server"
)

const (
	qrDisplay = 220
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
	bg.CornerRadius = 16
	qr := container.NewCenter(container.NewGridWrap(fyne.NewSize(qrDisplay, qrDisplay), container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 10, 10), img))))
	return container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), qr)
}

// showShare — страница результата выдачи доступа (дизайн «shareResult»): QR, строки «маршрут/роль/ссылка», кнопки.
// extra — дополнительные строки под маршрутом (например, «Роль»).
func (u *ui) showShare(parent fyne.Window, title string, r server.LinkResponse, note string, installable bool, extra ...fyne.CanvasObject) {
	u.showShareX(parent, title, r, note, installable, extra, nil)
}

// pageWithFooter — тело страницы с прокруткой и кнопки, закреплённые внизу (не уходят за прокрутку, как «Сохранить» на DNS).
func pageWithFooter(w *subPage, body fyne.CanvasObject, footer ...fyne.CanvasObject) {
	if len(footer) == 0 {
		w.SetContent(container.NewPadded(scrolled(body)))
		return
	}
	w.SetContent(container.New(layout.NewCustomPaddedLayout(8, 2, 8, 8), container.NewBorder(nil, newVBoxFit2(footer...), nil, nil, scrolled(body))))
}

// newVBoxFit2 — вертикальный ряд кнопок с общим шагом страницы.
func newVBoxFit2(objs ...fyne.CanvasObject) fyne.CanvasObject {
	c := container.New(vboxFit{gap: 4})
	for _, o := range objs {
		c.Add(o)
	}
	return container.NewVBox(spacerH(6), c)
}

// shortLink — ссылка для строки «Ссылка»: начало и многоточие, полная копируется по нажатию.
func shortLink(s string) string {
	r := []rune(s)
	if len(r) <= 22 {
		return s
	}
	return string(r[:20]) + "…"
}

// showShareX — то же, плюс after: кнопки, закреплённые внизу страницы (например, «Отозвать у всех входов»).
func (u *ui) showShareX(parent fyne.Window, title string, r server.LinkResponse, note string, installable bool, extra, after []fyne.CanvasObject) *subPage {
	w := u.newSubPage(title)
	rows := newVBoxFit()
	rows.Add(noteIcon("qr", note, colMuted))
	rows.Add(qrImage(r.Link, r.QR))
	switch {
	case r.Via != "":
		rows.Add(infoRow(L("Route", "Маршрут"), r.Via, nil))
	case len(after) == 0 && installable:
		rows.Add(infoRow(L("Route", "Маршрут"), L("exit on this server", "выход на этом сервере"), nil))
	}
	for _, e := range extra {
		rows.Add(e)
	}
	rows.Add(infoRow(L("Link", "Ссылка"), shortLink(r.Link), func() { u.app.Clipboard().SetContent(r.Link) }))
	rows.Add(pageButton(L("Copy", "Скопировать"), "copy", true, func() {
		u.app.Clipboard().SetContent(r.Link)
	}))
	rows.Add(pageButton(L("File…", "Файл…"), "download", false, func() {
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write([]byte(r.Link + "\n")); err != nil {
				showError(err, w)
			}
		}, w)
		name := r.File
		if name == "" {
			name = config.SafeFileName(r.Name) + ".txt"
		}
		fd.SetFileName(name)
		fd.Show()
	}))
	if installable {
		rows.Add(pageButton(L("Add to this device", "Добавить на это устройство"), "plus", false, func() {
			p, err := config.DecodeLink(r.Link)
			if err != nil {
				showError(err, w)
				return
			}
			base := p.Name
			for i := 2; u.profileExists(p.Name); i++ {
				p.Name = fmt.Sprintf("%s %d", base, i)
			}
			if err := saveProfile(p); err != nil {
				showError(err, w)
				return
			}
			u.reload()
			u.selectProfile(p.Name)
			showInfo(L("Done", "Готово"), L("Server \""+p.Name+"\" was added to the app.", "Сервер «"+p.Name+"» добавлен в приложение."), w)
		}))
	}
	rows.Add(noteIcon("lock", L("The link and file contain a private key and are shown once.", "Ссылка и файл содержат приватный ключ и показываются один раз."), colMuted))
	pageWithFooter(w, rows, after...)
	w.Show()
	return w
}
