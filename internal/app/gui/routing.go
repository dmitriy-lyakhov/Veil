package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/routing"
)

func groupTitle(g string) string {
	titles := map[string]string{
		"ai": L("AI", "Нейросети"), "anime": L("Anime", "Аниме"), "art": L("Art", "Искусство"), "casino": L("Casino", "Казино"), "discord": "Discord",
		"education": L("Education", "Образование"), "finance": L("Finance", "Финансы"), "games": L("Games", "Игры"), "hosting": L("Hosting", "Хостинг"),
		"jetbrains": "JetBrains", "messengers": L("Messengers", "Мессенджеры"), "music": L("Music", "Музыка"), "news": L("News", "Новости"),
		"porn": "18+", "shop": L("Shops", "Магазины"), "socials": L("Social media", "Соцсети"), "tools": L("Tools", "Инструменты"),
		"torrent": L("Torrents", "Торренты"), "video": L("Video", "Видео"), "youtube": "YouTube", "other": L("Other", "Прочее"),
	}
	if t, ok := titles[g]; ok {
		return t
	}
	return g
}

// ---------- каталог iplist.opencck.org ----------

func (u *ui) openCatalog(target *routing.List, added func()) {
	cachePath, _ := routing.CatalogPath()
	w := u.newSubPage(L("Address catalog", "Каталог адресов"))
	ctx, cancel := context.WithCancel(context.Background())
	w.OnClosed = cancel

	// длинный адрес не помещается в однострочное поле (оно прокручивается и рисует полосу),
	// поэтому поле многострочное с переносом
	urlEntry := widget.NewMultiLineEntry()
	urlEntry.Wrapping = fyne.TextWrapBreak
	urlEntry.SetMinRowsVisible(2)
	urlEntry.SetText(routing.DefaultCatalogURL)
	status := kit.Prose("")
	setStatus := func(s string) {
		status.Segments[0].(*widget.TextSegment).Text = s
		status.Refresh()
	}
	bar := kit.NewThinBar()
	barBox := container.NewPadded(bar)
	barBox.Hide()
	filter := widget.NewEntry()
	filter.SetPlaceHolder(L("Search by name or domain", "Поиск по названию или домену"))
	groupsBox := kit.NewVBoxFit()
	add := widget.NewButtonWithIcon(L("Add selected", "Добавить выбранное"), theme.ConfirmIcon(), nil)
	add.Importance = widget.HighImportance
	add.Disable()
	load := widget.NewButtonWithIcon(L("Load catalog", "Загрузить каталог"), theme.DownloadIcon(), nil)
	selectAll := widget.NewButtonWithIcon(L("Select all", "Выбрать все"), theme.CheckButtonCheckedIcon(), nil)
	selectAll.Importance = widget.LowImportance
	selectAll.Disable()

	selected := map[string]routing.Service{}
	var cat *routing.Catalog
	var updateSelect func()
	updateAdd := func() {
		if n := len(selected); n > 0 {
			add.SetText(fmt.Sprintf(L("Add selected (%d)", "Добавить выбранное (%d)"), n))
			add.Enable()
		} else {
			add.SetText(L("Add selected", "Добавить выбранное"))
			add.Disable()
		}
		if updateSelect != nil {
			updateSelect()
		}
	}
	matches := func(s routing.Service, q string) bool {
		if q == "" || strings.Contains(strings.ToLower(s.Name), q) {
			return true
		}
		for _, d := range s.Domains {
			if strings.Contains(d, q) {
				return true
			}
		}
		return false
	}
	// сервисы, которые сейчас видны с учётом поиска
	visible := func() []routing.Service {
		if cat == nil {
			return nil
		}
		q := strings.ToLower(strings.TrimSpace(filter.Text))
		var out []routing.Service
		for _, s := range cat.Services {
			if matches(s, q) {
				out = append(out, s)
			}
		}
		return out
	}
	var render func()
	updateSelect = func() {
		vis := visible()
		if len(vis) == 0 {
			selectAll.Disable()
			selectAll.SetText(L("Select all", "Выбрать все"))
			return
		}
		selectAll.Enable()
		all := true
		for _, s := range vis {
			if _, ok := selected[s.Name]; !ok {
				all = false
				break
			}
		}
		if all {
			selectAll.SetText(L("Deselect all", "Снять выбор"))
		} else {
			selectAll.SetText(L("Select all", "Выбрать все"))
		}
	}
	selectAll.OnTapped = func() {
		vis := visible()
		all := len(vis) > 0
		for _, s := range vis {
			if _, ok := selected[s.Name]; !ok {
				all = false
				break
			}
		}
		for _, s := range vis {
			if all {
				delete(selected, s.Name)
			} else {
				selected[s.Name] = s
			}
		}
		render()
		updateAdd()
	}
	render = func() {
		groupsBox.RemoveAll()
		if cat == nil {
			updateSelect()
			return
		}
		q := strings.ToLower(strings.TrimSpace(filter.Text))
		names, byGroup := cat.Groups()
		sort.Slice(names, func(i, j int) bool { return groupTitle(names[i]) < groupTitle(names[j]) })
		var items []*widget.AccordionItem
		for _, g := range names {
			var vis []routing.Service
			for _, s := range byGroup[g] {
				if matches(s, q) {
					vis = append(vis, s)
				}
			}
			if len(vis) == 0 {
				continue
			}
			labels := make([]string, len(vis))
			byLabel := map[string]routing.Service{}
			var sel []string
			for i, s := range vis {
				labels[i] = fmt.Sprintf(L("%s · %d subnets, %d domains", "%s · %d подсетей, %d доменов"), s.Name, len(s.Prefixes), len(s.Domains))
				byLabel[labels[i]] = s
				if _, ok := selected[s.Name]; ok {
					sel = append(sel, labels[i])
				}
			}
			cg := widget.NewCheckGroup(labels, nil)
			cg.SetSelected(sel)
			cg.OnChanged = func(now []string) {
				on := map[string]bool{}
				for _, l := range now {
					on[l] = true
				}
				for l, s := range byLabel {
					if on[l] {
						selected[s.Name] = s
					} else {
						delete(selected, s.Name)
					}
				}
				updateAdd()
			}
			it := widget.NewAccordionItem(fmt.Sprintf("%s (%d)", groupTitle(g), len(vis)), cg)
			it.Open = q != ""
			items = append(items, it)
		}
		if len(items) == 0 {
			groupsBox.Add(kit.RichText(L("Nothing found", "Ничего не найдено"), true, false))
		} else {
			groupsBox.Add(widget.NewAccordion(items...))
		}
		groupsBox.Refresh()
		updateSelect()
	}
	filter.OnChanged = func(string) { render() }

	describe := func() {
		if cat == nil {
			setStatus(L("The catalog is not loaded. Press \"Load catalog\": it is about 45 MB and may take several minutes (the list is large and served slowly). It is stored on this device.", "Каталог не загружен. Нажмите «Загрузить каталог»: это около 45 МБ, скачивание может занять несколько минут (список большой и отдаётся медленно). Он сохраняется на этом устройстве."))
			return
		}
		setStatus(fmt.Sprintf(L("Loaded %s: %d services. Addresses are merged into subnets. Reload to update.", "Загружен %s: %d сервисов. Адреса объединены в подсети. Обновить можно повторной загрузкой."), cat.Fetched.Format("02.01.2006 15:04"), len(cat.Services)))
	}
	if c, err := routing.LoadCatalog(cachePath); err == nil {
		cat = c
		urlEntry.SetText(c.URL)
	}
	describe()
	render()

	load.OnTapped = func() {
		load.Disable()
		barBox.Show()
		bar.Start()
		setStatus(L("Downloading catalog…", "Скачиваем каталог…"))
		url := strings.Join(strings.Fields(urlEntry.Text), "")
		go func() {
			start := time.Now()
			c, err := routing.FetchCatalog(ctx, url, func(read, total int64) {
				msg := L("Downloaded ", "Скачано ") + humanBytes(uint64(read))
				if total > 0 {
					msg += L(" of ", " из ") + humanBytes(uint64(total))
				}
				msg += fmt.Sprintf(L(" · %d s", " · %d с"), int(time.Since(start).Seconds()))
				fyne.Do(func() { setStatus(msg) })
			})
			if err == nil {
				err = c.Save(cachePath)
			}
			fyne.Do(func() {
				bar.Stop()
				barBox.Hide()
				load.Enable()
				if err != nil {
					if ctx.Err() == nil {
						setStatus(L("Failed to load the catalog: ", "Не удалось загрузить каталог: ") + err.Error())
					}
					return
				}
				cat = c
				describe()
				render()
			})
		}()
	}
	add.OnTapped = func() {
		var ss []routing.Service
		for _, s := range selected {
			ss = append(ss, s)
		}
		sort.Slice(ss, func(i, j int) bool { return ss[i].Name < ss[j].Name })
		target.Imports = append(target.Imports, routing.ImportFrom("opencck", ss))
		added()
		w.Close()
	}

	body := kit.NewVBoxFit()
	body.Add(status)
	body.Add(barBox)
	body.Add(kit.Lead(kit.Text(L("Catalog address", "Адрес каталога"), 12, kit.ColMuted, false)))
	body.Add(urlEntry)
	body.Add(load)
	body.Add(filter)
	body.Add(selectAll)
	body.Add(groupsBox)
	body.Add(add)
	w.SetContent(container.NewPadded(container.NewVScroll(body)))
	w.Show()
}
