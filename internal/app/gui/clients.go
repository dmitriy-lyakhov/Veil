package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/server"
)

// clientRoute — строка маршрута клиента для списка и страницы клиента.
func clientRoute(c server.ClientInfo) string {
	route := L("exit on this server", "выход на этом сервере")
	if len(c.Route) > 0 {
		route = "→ " + strings.Join(c.Route, " → ")
	}
	if c.Auto != nil {
		route = L("auto ", "авто ") + route
		if c.Auto.CostMs > 0 {
			route += fmt.Sprintf(L(" · %.0f ms", " · %.0f мс"), c.Auto.CostMs)
		}
	}
	if c.Disabled {
		route = L("blocked · ", "заблокирован · ") + route
	}
	return route
}

func clientTraffic(c server.ClientInfo) string {
	return fmt.Sprintf("↓ %s  ↑ %s", humanBytes(c.TxBytes), humanBytes(c.RxBytes))
}

func newEntryText(s string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(s)
	return e
}

// clientRow — строка списка клиентов с шевроном (дизайн «Доступ → Пользователи»): имя (+значок «Админ»), маршрут и трафик.
// Нажатие открывает страницу клиента.
func (m *manager) clientRow(c server.ClientInfo) fyne.CanvasObject {
	var title fyne.CanvasObject = kit.NewFitText(c.Name, 16, kit.ColText, true)
	if m.roles[c.Name] == "admin" {
		title = container.NewBorder(nil, nil, nil, container.NewCenter(roleBadge()), title)
	}
	sub := clientRoute(c)
	if c.Note != "" {
		sub = c.Note + "\n" + sub
	}
	sub += "\n" + clientTraffic(c)
	content := container.New(kit.TightVBox{Gap: 5}, title, kit.ProseFlush(sub))
	return kit.ListRowOf(content, container.NewCenter(kit.IconImg("right", kit.ColMuted, 22)), func() { m.openClient(c.Name) })
}

func (m *manager) clientByName(name string) (server.ClientInfo, bool) {
	for _, c := range m.state.Clients {
		if c.Name == name {
			return c, true
		}
	}
	return server.ClientInfo{}, false
}

// openClient — страница «Клиент» (дизайн «clientDetail»): роль, маршрут, трафик, заметка и действия.
func (m *manager) openClient(name string) {
	c0, ok := m.clientByName(name)
	if !ok {
		return
	}
	w := m.u.newSubPage(c0.Name)
	body := kit.NewVBoxFit()
	var render func()
	// после изменения на сервере перечитываем состояние и перерисовываем страницу (или закрываем, если клиента больше нет)
	after := func(rename string) func(error) {
		return func(err error) {
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			if rename != "" {
				name = rename
			}
			m.refreshThen(func() {
				if _, ok := m.clientByName(name); !ok {
					w.Close()
					return
				}
				render()
			})
		}
	}
	render = func() {
		c, ok := m.clientByName(name)
		if !ok {
			return
		}
		w.SetTitle(c.Name)
		body.RemoveAll()
		cur := normRole(m.roles[c.Name])
		next := "admin"
		toggleTitle, toggleSub := L("Make an administrator", "Сделать администратором"), L("Full access to the admin panel of this server", "Полный доступ к админ-панели этого сервера")
		if cur == "admin" {
			next = "user"
			toggleTitle, toggleSub = L("Make a user", "Сделать пользователем"), L("Admin panel access will be removed", "Доступ к админ-панели будет закрыт")
		}
		body.Add(infoRow(L("Role", "Роль"), roleName(cur), nil))
		body.Add(infoRow(L("Route", "Маршрут"), clientRoute(c), nil))
		body.Add(infoRow(L("Traffic", "Трафик"), clientTraffic(c), nil))
		if c.Note != "" {
			body.Add(infoRow(L("Note", "Заметка"), c.Note, nil))
		}
		arrow := func() fyne.CanvasObject { return container.NewCenter(kit.IconImg("right", kit.ColMuted, 22)) }
		// смена роли — отдельное действие со своим подтверждением
		body.Add(kit.ListRow(toggleTitle, toggleSub, arrow(), func() {
			kit.ShowConfirm(toggleTitle, fmt.Sprintf(L("“%s”: %s. Continue?", "«%s»: %s. Продолжить?"), c.Name, toggleSub), func(ok bool) {
				if ok {
					m.call("POST", "/clients/update", map[string]any{"name": c.Name, "role": next}, nil, after(""))
				}
			}, w)
		}))
		body.Add(kit.ListRow(L("New config", "Новый конфиг"), L("The old one stops working", "Старый перестанет работать"), arrow(), func() {
			kit.ShowConfirm(L("New config", "Новый конфиг"), fmt.Sprintf(L("Issue a new config for “%s”? The old one will stop working.", "Выпустить новый конфиг для «%s»? Старый перестанет работать."), c.Name), func(ok bool) {
				if !ok {
					return
				}
				var r server.LinkResponse
				m.call("POST", "/clients/reissue", map[string]string{"name": c.Name}, &r, func(err error) {
					if err != nil {
						kit.ShowError(err, w)
						return
					}
					m.u.showShare(w, fmt.Sprintf(L("New config “%s”", "Новый конфиг «%s»"), c.Name), r, L("The old config no longer works. The link is shown once.", "Старый конфиг больше не работает. Ссылка показывается один раз."), true)
					m.refresh()
				})
			}, w)
		}))
		body.Add(kit.ListRow(L("Name and note", "Имя и заметка"), "", arrow(), func() { m.openClientEdit(c, func(newName string) { after(newName)(nil) }) }))
		block, blockTitle := !c.Disabled, L("Block", "Заблокировать")
		if c.Disabled {
			blockTitle = L("Unblock", "Разблокировать")
		}
		body.Add(kit.ListRow(blockTitle, L("A blocked client fails transport authentication immediately", "Заблокированный клиент не проходит аутентификацию сразу"), arrow(), func() {
			m.call("POST", "/clients/update", map[string]any{"name": c.Name, "disabled": block}, nil, after(""))
		}))
		body.Add(kit.ListRowColored(L("Delete", "Удалить"), L("The client connections will be terminated", "Его подключения будут разорваны"), theme.ColorNameError, arrow(), func() {
			kit.ShowConfirm(L("Delete client", "Удалить клиента"), fmt.Sprintf(L("Delete “%s”? Its connections will be dropped.", "Удалить «%s»? Его подключения будут разорваны."), c.Name), func(ok bool) {
				if !ok {
					return
				}
				m.call("POST", "/clients/delete", map[string]string{"name": c.Name}, nil, func(err error) {
					if err != nil {
						kit.ShowError(err, w)
						return
					}
					w.Close()
					m.refresh()
				})
			}, w)
		}))
		body.Refresh()
	}
	render()
	pageWithFooter(w, body)
	w.Show()
}

// openClientEdit — страница «Имя и заметка»: переименование клиента и заметка.
func (m *manager) openClientEdit(c server.ClientInfo, saved func(newName string)) {
	w := m.u.newSubPage(L("Name and note", "Имя и заметка"))
	name, note := newEntryText(c.Name), newEntryText(c.Note)
	body := kit.NewVBoxFit()
	body.Add(fieldEntry(L("Name", "Имя"), name))
	body.Add(fieldEntry(L("Note", "Заметка"), note))
	save := bigButton(L("Save", "Сохранить"), "", true, func() {
		m.call("POST", "/clients/update", map[string]any{"name": c.Name, "new_name": name.Text, "note": note.Text}, nil, func(err error) {
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			w.Close()
			saved(name.Text)
		})
	})
	cancel := kit.GhostButton(L("Cancel", "Отмена"), w.Close)
	pageWithFooter(w, body, save, cancel)
	w.Show()
}

// setRole меняет роль клиента на сервере (clients/update с полем role).
func (m *manager) setRole(name, role string) {
	m.call("POST", "/clients/update", map[string]any{"name": name, "role": role}, nil, func(err error) {
		if err != nil {
			kit.ShowError(err, m.w)
			return
		}
		m.refresh()
	})
}

// addClient — страница «Новый клиент» (дизайн «clientNew»): имя, заметка, роль, маршрут через каскад; «Создать» закреплена внизу.
func (m *manager) addClient() {
	w := m.u.newSubPage(L("New client", "Новый клиент"))
	role := "user"
	name := newEntryText("")
	name.SetPlaceHolder(L("For example: Anna's phone", "Например: телефон Ани"))
	note := newEntryText("")
	routeBox := kit.NewVBoxFit()
	var route []string
	var rebuild func()
	rebuild = func() {
		routeBox.RemoveAll()
		first := exitHere()
		if len(route) > 0 {
			first = exitLast()
		}
		for i, h := range route {
			routeBox.Add(kit.ListRow(fmt.Sprintf("%d. %s", i+1, h), "", nil, nil))
		}
		routeBox.Add(kit.ListRow(first, "", nil, nil))
		if len(route) > 0 {
			routeBox.Add(kit.PageButton(L("Remove the last node", "Убрать последний узел"), "back", false, func() {
				route = route[:len(route)-1]
				rebuild()
			}))
		}
		loading := kit.Text(L("Loading options…", "Загрузка вариантов…"), 12, kit.ColMuted, false)
		routeBox.Add(loading)
		routeBox.Refresh()
		var resp struct {
			Upstreams []string `json:"upstreams"`
		}
		m.call("POST", "/explore", map[string]any{"route": route}, &resp, func(err error) {
			routeBox.Remove(loading)
			if err != nil {
				routeBox.Add(kit.Text(err.Error(), 12, kit.ColBad, false))
				routeBox.Refresh()
				return
			}
			var options []string
			for _, o := range resp.Upstreams {
				if !containsString(route, o) {
					options = append(options, o)
				}
			}
			if len(options) > 0 {
				routeBox.Add(m.u.menuRow(L("Next through…", "Дальше через…"), "", options, func(v string) {
					route = append(route, v)
					rebuild()
				}))
			}
			routeBox.Refresh()
		})
	}
	rebuild()
	body := kit.NewVBoxFit()
	body.Add(fieldEntry(L("Name", "Имя"), name))
	body.Add(fieldEntry(L("Note", "Заметка"), note))
	body.Add(roleBlock(role, func(v string) { role = v }))
	body.Add(kit.SectionLabel(L("Route", "Маршрут")))
	body.Add(routeBox)
	create := bigButton(L("Create", "Создать"), "", true, func() {
		var r server.LinkResponse
		m.call("POST", "/clients", map[string]any{"name": name.Text, "note": note.Text, "route": route, "role": role}, &r, func(err error) {
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			w.Close()
			m.u.showShare(m.w, fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), r.Name), r, L("Scan the QR in the Veil app on the client device, or send the link or file.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте ссылку или файл."), true, infoRow(L("Role", "Роль"), roleName(role), nil))
			m.refresh()
		})
	})
	cancel := kit.GhostButton(L("Cancel", "Отмена"), w.Close)
	pageWithFooter(w, body, create, cancel)
	w.Show()
}
