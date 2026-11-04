package gui

import (
	"errors"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"veil/internal/front"
)

func frontNone() string  { return L("No masquerading", "Без маскировки") }
func frontOther() string { return L("Other domain…", "Свой домен…") }

// frontPicker — выбор сайта, под который маскируется вход: готовые варианты и свой домен.
type frontPicker struct {
	sel    *widget.Select
	custom *widget.Entry
	box    *fyne.Container
}

func newFrontPicker(current string) *frontPicker {
	p := &frontPicker{custom: widget.NewEntry()}
	p.custom.SetPlaceHolder("example.ru")
	opts := append([]string{frontNone()}, front.Presets...)
	opts = append(opts, frontOther())
	p.sel = widget.NewSelect(opts, func(string) { p.syncCustom() })
	p.box = container.NewVBox(p.sel, p.custom)
	switch {
	case current == "":
		p.sel.SetSelected(frontNone())
	case slices.Contains(front.Presets, current):
		p.sel.SetSelected(current)
	default:
		p.sel.SetSelected(frontOther())
		p.custom.SetText(current)
	}
	p.syncCustom()
	return p
}

func (p *frontPicker) syncCustom() {
	if p.sel.Selected == frontOther() {
		p.custom.Show()
	} else {
		p.custom.Hide()
	}
}

// SNI возвращает выбранный домен; пустая строка — без маскировки.
func (p *frontPicker) SNI() (string, error) {
	switch p.sel.Selected {
	case "", frontNone():
		return "", nil
	case frontOther():
		sni, err := front.NormalizeSNI(p.custom.Text)
		if err == nil && sni == "" {
			err = errors.New(L("enter the site domain, for example ya.ru", "укажите домен сайта, например ya.ru"))
		}
		return sni, err
	}
	return p.sel.Selected, nil
}

func (p *frontPicker) View() fyne.CanvasObject { return p.box }

func frontWarning(sni string) string {
	return L(
		"Masquerade as "+sni+"?\n\nClient configs issued earlier stop working: issue them again (they have no key). QUIC (UDP/443) is turned off. Cascade keys issued by this server must be issued again. Reconnect after applying.",
		"Включить маскировку под "+sni+"?\n\nКонфиги клиентов, выданные раньше, перестанут работать: выпустите их заново (в них нет ключа). QUIC (UDP/443) будет отключён. Ключи каскада, выданные этим сервером, тоже нужно выдать заново. После применения переподключитесь.",
	)
}
