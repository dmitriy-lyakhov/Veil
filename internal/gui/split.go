package gui

import (
	"fmt"
	"io"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/routing"
)

const maxListFile = 8 << 20

// openSplit — страница раздельного туннелирования по сайтам и адресам (как в референсе):
// переключатель, режим «Через VPN / Напрямую», список записей и поле добавления.
func (u *ui) openSplit() {
	path, err := routing.Path()
	if err != nil {
		showError(err, u.win)
		return
	}
	cfg, err := routing.Load(path)
	if err != nil {
		showError(err, u.win)
		return
	}
	w := u.newSubPage(L("Split tunneling", "Раздельное туннелирование"))
	mode := 0 // 0 — через VPN, 1 — напрямую
	list := func() *routing.List {
		if mode == 0 {
			return &cfg.Via
		}
		return &cfg.Direct
	}
	persist := func() {
		if err := cfg.Save(path); err != nil {
			showError(err, w)
		}
		u.refreshSplitLink()
	}

	hint := prose("")
	setHint := func() {
		t := L("Only what is listed here goes through the tunnel. If the list is empty, all traffic goes through the VPN.", "В туннель идёт только то, что перечислено здесь. Если список пуст, через VPN идёт весь трафик.")
		if mode == 1 {
			t = L("These sites and addresses always connect directly, bypassing the VPN.", "Эти сайты и адреса всегда открываются напрямую, мимо VPN.")
		}
		hint.Segments[0].(*widget.TextSegment).Text = t
		hint.Refresh()
	}
	items := newVBoxFit()
	var rebuild func()
	rebuild = func() {
		items.RemoveAll()
		l := list()
		lines := manualLines(l.Manual)
		for i, ln := range lines {
			i, ln := i, ln
			items.Add(entryRow(ln, "", func() {
				lines = append(lines[:i], lines[i+1:]...)
				l.Manual = strings.Join(lines, "\n")
				persist()
				rebuild()
			}))
		}
		for i, im := range l.Imports {
			i, im := i, im
			sub := fmt.Sprintf(L("%s · %d subnets, %d domains", "%s · %d подсетей, %d доменов"), im.Source, len(im.Prefixes), len(im.Domains))
			items.Add(entryRow(im.Title, sub, func() {
				l.Imports = append(l.Imports[:i], l.Imports[i+1:]...)
				persist()
				rebuild()
			}))
		}
		if len(lines) == 0 && len(l.Imports) == 0 {
			items.Add(note(L("The list is empty.", "Список пуст.")))
		}
		items.Refresh()
	}

	enableRow, _ := switchRow(L("Split tunneling", "Раздельное туннелирование"), "", cfg.Enabled, func(on bool) {
		cfg.Enabled = on
		persist()
	})
	seg := newSegmented([]string{L("Via VPN", "Через VPN"), L("Direct", "Напрямую")}, 0, func(i int) {
		mode = i
		setHint()
		rebuild()
	})
	setHint()
	rebuild()

	entry := widget.NewEntry()
	entry.SetPlaceHolder(L("site or IP", "сайт или IP"))
	add := func() {
		raw := strings.TrimSpace(entry.Text)
		if raw == "" {
			return
		}
		prefixes, domains, errs := routing.ParseLines(raw)
		if len(errs) > 0 || len(prefixes)+len(domains) == 0 {
			msg := L("invalid entry", "некорректная запись")
			if len(errs) > 0 {
				msg = errs[0]
			}
			showError(fmt.Errorf("%s", msg), w)
			return
		}
		l := list()
		for _, p := range prefixes {
			l.Manual = appendLine(l.Manual, p.String())
		}
		for _, d := range domains {
			l.Manual = appendLine(l.Manual, d)
		}
		entry.SetText("")
		persist()
		rebuild()
	}
	entry.OnSubmitted = func(string) { add() }
	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), add)
	addBtn.Importance = widget.HighImportance

	var menuBtn *widget.Button
	menuBtn = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem(L("Add from the opencck catalog", "Добавить из каталога opencck"), func() {
				u.openCatalog(list(), func() {
					persist()
					rebuild()
				})
			}),
			fyne.NewMenuItem(L("Add sites from file", "Добавить сайты из файла"), func() { u.importList(w, list(), false, func() { persist(); rebuild() }) }),
			fyne.NewMenuItem(L("Replace list from file", "Заменить список из файла"), func() { u.importList(w, list(), true, func() { persist(); rebuild() }) }),
			fyne.NewMenuItem(L("Save list to file", "Сохранить список в файл"), func() { u.exportList(w, *list()) }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(L("Clear list", "Очистить список"), func() {
				showConfirm(L("Clear list", "Очистить список"), L("Delete all entries from this list?", "Удалить все записи из этого списка?"), func(ok bool) {
					if ok {
						*list() = routing.List{}
						persist()
						rebuild()
					}
				}, w)
			}),
		)
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(menuBtn)
		pos.Y -= 4 * 44
		widget.ShowPopUpMenuAtPosition(menu, u.win.Canvas(), pos)
	})
	menuBtn.Importance = widget.LowImportance

	top := container.NewVBox(enableRow, seg, hint)
	bottom := container.NewBorder(nil, nil, nil, container.NewHBox(menuBtn), container.NewBorder(nil, nil, nil, addBtn, entry))
	w.SetContent(container.NewPadded(container.NewBorder(top, bottom, nil, nil, container.NewVScroll(items))))
	w.Show()
}

func manualLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func appendLine(text, line string) string {
	for _, ln := range manualLines(text) {
		if ln == line {
			return text
		}
	}
	if strings.TrimSpace(text) == "" {
		return line
	}
	return strings.TrimRight(text, "\r\n") + "\n" + line
}

// entryRow — строка списка: запись и кнопка удаления.
func entryRow(title, sub string, remove func()) fyne.CanvasObject {
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), remove)
	del.Importance = widget.LowImportance
	return listRow(title, sub, del, nil)
}

func (u *ui) importList(parent fyne.Window, l *routing.List, replace bool, done func()) {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		b, err := io.ReadAll(io.LimitReader(r, maxListFile))
		if err != nil {
			showError(err, parent)
			return
		}
		prefixes, domains, errs := routing.ParseLines(string(b))
		if replace {
			*l = routing.List{}
		}
		for _, p := range prefixes {
			l.Manual = appendLine(l.Manual, p.String())
		}
		for _, d := range domains {
			l.Manual = appendLine(l.Manual, d)
		}
		done()
		msg := fmt.Sprintf(L("Entries added: %d.", "Добавлено записей: %d."), len(prefixes)+len(domains))
		if len(errs) > 0 {
			msg += fmt.Sprintf(L(" Lines skipped due to errors: %d.", " Пропущено строк с ошибками: %d."), len(errs))
		}
		showInfo(L("Import", "Импорт"), msg, parent)
	}, parent)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".lst", ".list", ".csv"}))
	fd.Show()
}

func (u *ui) exportList(parent fyne.Window, l routing.List) {
	fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
		if err != nil || wr == nil {
			return
		}
		defer wr.Close()
		var sb strings.Builder
		for _, ln := range manualLines(l.Manual) {
			sb.WriteString(ln + "\n")
		}
		for _, im := range l.Imports {
			for _, d := range im.Domains {
				sb.WriteString(d + "\n")
			}
			for _, p := range im.Prefixes {
				sb.WriteString(p + "\n")
			}
		}
		if _, err := wr.Write([]byte(sb.String())); err != nil {
			showError(err, parent)
		}
	}, parent)
	fd.SetFileName("veil-sites.txt")
	fd.Show()
}
