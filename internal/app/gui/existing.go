package gui

// «Сервер уже настроен» (дизайн Existing): протокол на сервере уже был — ничего не переустанавливалось,
// приложение выдало этому устройству нового клиента. Слева — что работает на сервере, справа — клиент
// с QR-кодом или файлом и кнопками «Скопировать», «Сохранить файл», «Подключиться».

import (
	"encoding/base64"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	qrcode "github.com/skip2/go-qrcode"

	"veil/internal/app/gui/kit"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/vpnconf/ipsecconf"
)

// containerWhat — что за контейнер, по метке протокола.
func containerWhat(proto string) string {
	switch proto {
	case "admin":
		return L("admin panel", "админ-панель")
	case "":
		return ""
	}
	return protoLabel(proto)
}

// containerSub — «образ · порты · состояние» для строки контейнера.
func containerSub(c deploy.ContainerInfo) string {
	var parts []string
	if c.Image != "" {
		parts = append(parts, c.Image)
	}
	ports := strings.TrimSpace(c.Ports)
	if ports == "" {
		ports = L("no ports", "без портов")
	}
	parts = append(parts, ports)
	if c.Status != "" {
		parts = append(parts, c.Status)
	}
	return strings.Join(parts, " · ")
}

// clientFile — текст конфигурации клиента, имя файла и подпись вкладки «файл».
type clientFile struct {
	text, fileName, tab string
	data                []byte // что сохранять (по умолчанию — text)
}

func clientFileOf(proto string, prof *config.Profile) clientFile {
	switch {
	case prof.WG != nil:
		name := "veil-wireguard.conf"
		if proto == "amneziawg" {
			name = "veil-amneziawg.conf"
		}
		return clientFile{text: prof.WG.String(), fileName: name, tab: L(".conf file", "Файл .conf")}
	case prof.OVPN != nil:
		return clientFile{text: prof.OVPN.String(), fileName: "veil-openvpn.ovpn", tab: L(".ovpn file", "Файл .ovpn")}
	case prof.IPsec != nil:
		f := clientFile{text: prof.IPsec.Instructions(), fileName: "veil-ca.crt", tab: L("Instructions", "Инструкция"), data: []byte(prof.IPsec.CAPEM)}
		if prof.IPsec.Auth == ipsecconf.AuthCert {
			if raw, err := base64.StdEncoding.DecodeString(prof.IPsec.P12); err == nil {
				f.fileName, f.data = "veil-client.p12", raw
			}
		}
		return f
	}
	return clientFile{}
}

// qrFits — помещается ли текст в QR-код (большой .ovpn с сертификатами — нет).
func qrFits(text string) bool {
	_, err := qrcode.New(text, qrcode.Medium)
	return err == nil && text != ""
}

func (u *ui) openExisting(form *subPage, job installJob, prof *config.Profile, res *deploy.ProtocolResult) *subPage {
	w := u.newSubPage(fmt.Sprintf(L("Server already set up · %s", "Сервер уже настроен · %s"), job.req.Name))
	w.setMaxWidth(wideColumns)
	label := protoLabel(job.proto)

	// слева: контейнеры на сервере, свой — с красной обводкой
	left := kit.NewVBoxFit()
	left.Add(kit.SectionLabel(L("Already running on the server", "На сервере уже работает")))
	ownName, ownSub := res.Container, label+" · "+L("a new client was issued", "выдан новый клиент")
	if ownName == "" {
		ownName = label
		ownSub = L("system service", "системная служба") + " · " + L("a new client was issued", "выдан новый клиент")
	}
	left.Add(ctrRow(ownName, ownSub, true, true))
	for _, c := range res.Others {
		title := c.Name
		if what := containerWhat(c.Protocol); what != "" {
			title += " · " + what
		}
		left.Add(ctrRow(title, containerSub(c), c.Running(), false))
	}
	left.Add(infoNote(L("The protocol is already installed — nothing is reinstalled: the app only issues a new client for this device and connects.",
		"Протокол уже стоит — ничего не переустанавливается: приложение только выдаёт этому устройству нового клиента и подключается.")))

	// справа: новый клиент
	file := clientFileOf(job.proto, prof)
	sub := L("Installed as a system service earlier", "Был установлен системной службой раньше")
	if res.Container != "" {
		sub = fmt.Sprintf(L("Container %s was already running", "Контейнер %s уже был запущен"), res.Container)
	}
	head := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 10), container.NewCenter(kit.IconImg("check", kit.ColOK, 22))), nil,
		container.New(kit.TightVBox{Gap: 2},
			kit.NewFlowText(fmt.Sprintf(L("%s: new client “%s”", "%s: новый клиент «%s»"), label, prof.Name), 18, kit.ColText, true, 24),
			kit.NewFlowText(sub, 13, kit.ColMuted, false, 18)))

	view := container.NewStack()
	textView := func() fyne.CanvasObject {
		bg := canvas0()
		return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 14, 14), kit.NewFlowText(file.text, 12, kit.ColText, false, 18)))
	}
	showQR := func(on bool) {
		if on {
			view.Objects = []fyne.CanvasObject{qrImage(file.text, "")}
		} else {
			view.Objects = []fyne.CanvasObject{textView()}
		}
		view.Refresh()
	}
	right := kit.NewVBoxFit()
	right.Add(head)
	if qrFits(file.text) && prof.IPsec == nil {
		right.Add(kit.NewSegmented([]string{L("QR code", "QR-код"), file.tab}, 0, func(i int) { showQR(i == 0) }))
		showQR(true)
	} else {
		showQR(false)
	}
	right.Add(view)

	save := func() {
		data := file.data
		if data == nil {
			data = []byte(file.text)
		}
		fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			if _, err := wr.Write(data); err != nil {
				kit.ShowError(err, w)
			}
		}, w)
		fd.SetFileName(file.fileName)
		fd.Show()
	}
	right.Add(newAutoGrid(150,
		bigButton(L("Copy", "Скопировать"), "copy", false, func() { u.app.Clipboard().SetContent(file.text) }),
		bigButton(L("Save file", "Сохранить файл"), "download", false, save),
	))
	right.Add(bigButton(L("Connect", "Подключиться"), "", true, func() {
		u.reload()
		u.selectProfile(prof.Name)
		w.Close()
		if form != nil {
			form.Close()
		}
		u.showPage(0)
		if u.handle == nil {
			u.toggle()
		}
	}))
	card := container.NewStack(cardBG(16), container.New(layout.NewCustomPaddedLayout(20, 20, 20, 20), right))

	w.SetContent(container.NewPadded(scrolled(newSplit(300, 0, 640, left, card))))
	w.Show()
	return w
}

// ctrRow — строка контейнера: точка состояния, имя и подпись; свой — с красной обводкой.
func ctrRow(title, sub string, ok, ring bool) fyne.CanvasObject {
	body := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), statusDot(ok)), nil,
		kit.Vcenter(kit.RowText(title, sub, theme.ColorNameForeground)))
	row, bg := padCard(body, 8, 14, 56)
	if ring {
		bg.StrokeColor, bg.StrokeWidth = kit.ColAccent, 2
	}
	return row
}

// canvas0 — тёмная подложка для текста конфигурации (как поле в дизайне).
func canvas0() fyne.CanvasObject {
	r := cardBG(14)
	r.FillColor = kit.ColBackground
	return r
}
