package gui

import (
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/routing"
)

func (u *ui) openSplitApps() {
	path, err := routing.Path()
	if err != nil {
		kit.ShowError(err, u.win)
		return
	}
	cfg, err := routing.Load(path)
	if err != nil {
		kit.ShowError(err, u.win)
		return
	}
	w := u.newSubPage(L("App split tunneling", "Раздельный доступ: приложения"))
	persist := func() {
		if err := cfg.Save(path); err != nil {
			kit.ShowError(err, w)
		}
	}

	items := kit.NewVBoxFit()
	var rebuild func()
	rebuild = func() {
		items.RemoveAll()
		if len(cfg.Apps) == 0 {
			items.Add(note(L("No apps yet. Add an app by name.", "Приложений пока нет. Добавьте приложение по названию.")))
		}
		for i, name := range cfg.Apps {
			i := i
			items.Add(entryRow(name, "", func() {
				cfg.RemoveApp(i)
				persist()
				rebuild()
			}))
		}
		items.Refresh()
	}

	enable, _ := kit.SwitchRowFit(L("App split tunneling", "Раздельный доступ по приложениям"), "", cfg.AppsEnabled, func(on bool) {
		cfg.AppsEnabled = on
		persist()
		u.refreshSplitLink()
	})
	desc := note(L("Apps from the list should not have access via VPN", "Приложения из списка не должны работать через VPN"))
	warn := note(L("Only the \"Apps from the list should not have access via VPN\" mode is available on this platform. The list is saved, but the tunnel does not filter traffic by process yet, so it is not applied.", "На этой платформе доступен только режим «Приложения из списка не работают через VPN». Список сохраняется, но туннель пока не разделяет трафик по процессам, поэтому правила не применяются."))

	entry := widget.NewEntry()
	entry.SetPlaceHolder(L("application name", "название приложения"))
	add := func() {
		if strings.TrimSpace(entry.Text) == "" {
			return
		}
		cfg.AddApp(entry.Text)
		entry.SetText("")
		persist()
		rebuild()
	}
	entry.OnSubmitted = func(string) { add() }
	rebuild()
	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), add)
	addBtn.Importance = widget.HighImportance

	top := container.New(kit.VboxFit{}, enable, desc, warn)
	bottom := container.NewBorder(nil, nil, nil, addBtn, entry)
	w.SetContent(container.NewPadded(container.NewBorder(top, bottom, nil, nil, container.NewVScroll(items))))
	w.Show()
}

func (u *ui) routingConfig() *routing.Config {
	path, err := routing.Path()
	if err != nil {
		return nil
	}
	cfg, err := routing.Load(path)
	if err != nil {
		return nil
	}
	return cfg
}
