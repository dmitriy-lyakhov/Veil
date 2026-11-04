package gui

import (
	"errors"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"veil/internal/front"
)

func frontNone() string  { return L("No masquerading", "Без маскировки") }
func frontOther() string { return L("Other domain…", "Свой домен…") }

type frontPicker struct {
	choice    string
	custom    *widget.Entry
	customRow fyne.CanvasObject
	view      *fyne.Container
}

func newFrontPicker(u *ui, current string) *frontPicker {
	p := &frontPicker{custom: widget.NewEntry()}
	p.custom.SetPlaceHolder("example.ru")
	opts := append([]string{frontNone()}, front.Presets...)
	opts = append(opts, frontOther())
	switch {
	case current == "":
		p.choice = frontNone()
	case slices.Contains(front.Presets, current):
		p.choice = current
	default:
		p.choice = frontOther()
		p.custom.SetText(current)
	}
	field := u.fieldSelect(L("Masquerade as", "Маскировка под"), p.choice, opts, func(s string) {
		p.choice = s
		p.syncCustom()
	})
	p.customRow = fieldEntry(L("Site domain", "Домен сайта"), p.custom)
	p.view = newVBoxFit()
	p.view.Add(field)
	p.view.Add(p.customRow)
	p.syncCustom()
	return p
}

func (p *frontPicker) syncCustom() {
	if p.choice == frontOther() {
		p.customRow.Show()
	} else {
		p.customRow.Hide()
	}
	p.view.Refresh()
}

func (p *frontPicker) SNI() (string, error) {
	switch p.choice {
	case "", frontNone():
		return "", nil
	case frontOther():
		sni, err := front.NormalizeSNI(p.custom.Text)
		if err == nil && sni == "" {
			err = errors.New(L("enter the site domain, for example ya.ru", "укажите домен сайта, например ya.ru"))
		}
		return sni, err
	}
	return p.choice, nil
}

func (p *frontPicker) View() fyne.CanvasObject { return p.view }

func frontWarning(sni string) string {
	return L(
		"Masquerade as "+sni+"?\n\nClient configs issued earlier stop working: issue them again (they have no key). QUIC (UDP/443) is turned off. Cascade keys issued by this server must be issued again. Reconnect after applying.",
		"Включить маскировку под "+sni+"?\n\nКонфиги клиентов, выданные раньше, перестанут работать: выпустите их заново (в них нет ключа). QUIC (UDP/443) будет отключён. Ключи каскада, выданные этим сервером, тоже нужно выдать заново. После применения переподключитесь.",
	)
}
