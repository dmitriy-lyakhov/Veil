package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/server"
)

// shareState — раздел «Поделиться»: клиенты (пользователи) выдаются и управляются здесь,
// а не на странице сервера. Вкладки «Подключение» и «Пользователи», выбор сервера сверху.
type shareState struct {
	u      *ui
	tab    int
	server string
	filter string
	names  []string
	body   *fyne.Container
	mgrs   map[string]*manager
	name   string // введённое имя нового клиента (сохраняется между перерисовками)
	role   string // "user" или "admin": что можно клиенту на сервере
	list   *fyne.Container
}

func (u *ui) sharePage() fyne.CanvasObject {
	s := &shareState{u: u, mgrs: map[string]*manager{}, name: L("New client", "Новый клиент")}
	u.share = s
	seg := newSegmented([]string{L("Connection", "Подключение"), L("Users", "Пользователи")}, 0, func(i int) {
		s.tab = i
		s.render()
	})
	s.body = newVBoxFit()
	top := container.NewVBox(seg)
	s.refreshServers()
	return u.tabPage(L("Share VPN access", "Доступ к VPN"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(s.body)))
}

// managedNames — серверы, которыми можно управлять из приложения.
func (u *ui) managedNames() []string {
	var out []string
	for _, p := range u.profiles {
		if managed(p) {
			out = append(out, p.Name)
		}
	}
	return out
}

func (s *shareState) refreshServers() {
	names := s.u.managedNames()
	s.names = names
	found := false
	for _, n := range names {
		if n == s.server {
			found = true
		}
	}
	if !found {
		s.server = ""
		if len(names) > 0 {
			s.server = names[0]
			for _, n := range names {
				if n == s.u.selected {
					s.server = n
				}
			}
		}
	}
	s.render()
}

func (s *shareState) manager() *manager {
	for _, p := range s.u.profiles {
		if p.Name != s.server {
			continue
		}
		m := s.mgrs[p.Name]
		if m == nil {
			m = &manager{u: s.u, p: p, w: s.u.win, status: newStatusLine(), body: newVBoxFit(), clientsOnly: true}
			m.onLoaded = s.render
			s.mgrs[p.Name] = m
		}
		m.p = p
		if m.sess == nil {
			if cached, ok := s.u.sessions[p.Name]; ok {
				m.sess = cached
			}
		}
		return m
	}
	return nil
}

func (s *shareState) render() {
	if s.body == nil {
		return
	}
	s.body.RemoveAll()
	m := s.manager()
	if m == nil {
		s.body.Add(note(L("Access for other people and devices is issued here. This needs a server installed from this app: add one with the \"+\" button.", "Здесь выдаются доступы для других людей и устройств. Для этого нужен сервер, установленный из этого приложения: добавьте его кнопкой «+».")))
		s.body.Refresh()
		return
	}
	if s.tab == 0 {
		s.renderConnection(m)
	} else {
		s.renderUsers(m)
	}
	s.body.Refresh()
}

func (s *shareState) renderConnection(m *manager) {
	s.body.Add(prose(L("Share VPN access without the ability to manage the server", "Доступ к VPN без права управлять сервером")))
	name := widget.NewEntry()
	name.SetText(s.name)
	name.OnChanged = func(v string) { s.name = v }
	fmtApp := L("For the Veil app", "Для приложения Veil")
	share := bigButton(L("Share", "Поделиться"), "share", true, func() {
		n := strings.TrimSpace(s.name)
		if n == "" {
			showError(fmt.Errorf("%s", L("enter a user name", "укажите имя пользователя")), s.u.win)
			return
		}
		m.ensureSession(func() {
			var r server.LinkResponse
			m.call("POST", "/clients", map[string]any{"name": n, "note": "", "route": []string{}, "role": s.roleValue()}, &r, func(err error) {
				if err != nil {
					showError(err, s.u.win)
					return
				}
				s.u.showShare(s.u.win, L("Client \""+r.Name+"\"", "Клиент «"+r.Name+"»"), r, L("Scan the QR in the Veil app on the client device, or send the file or link. Shown once: the config contains a private key.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте файл или ссылку. Показывается один раз: в конфиге приватный ключ."), true, infoRow(L("Role", "Роль"), roleName(s.roleValue()), nil))
				s.name = L("New client", "Новый клиент")
				m.refresh()
				s.render()
			})
		}, nil)
	})
	route := bigButton(L("Set route via cascade…", "Задать маршрут через каскад…"), "", false, func() {
		m.ensureSession(m.addClient, nil)
	})
	s.body.Add(fieldEntry(L("User name", "Имя пользователя"), name))
	s.body.Add(s.u.fieldSelect(L("Server", "Сервер"), s.server, s.names, func(v string) {
		s.server = v
		s.render()
	}))
	s.body.Add(s.u.fieldSelect(L("Connection format", "Формат подключения"), fmtApp, []string{fmtApp}, func(string) {}))
	s.body.Add(roleBlock(s.role, func(v string) { s.role = v }))
	cascade := bigButton(L("Share a cascade…", "Поделиться каскадом…"), "", false, func() { s.u.openCascadeBuilder(true) })
	s.body.Add(spacerH(8))
	s.body.Add(share)
	s.body.Add(route)
	s.body.Add(cascade)
}

func (s *shareState) renderUsers(m *manager) {
	if !m.loaded {
		if m.sess != nil {
			m.refresh()
			s.body.Add(note(L("Loading users…", "Загружаем пользователей…")))
			return
		}
		s.body.Add(note(L("The user list is stored on the server. Sign in over SSH to open it.", "Список пользователей хранится на сервере. Чтобы его открыть, войдите по SSH.")))
		btn := widget.NewButtonWithIcon(L("Sign in via SSH", "Войти по SSH"), theme.LoginIcon(), func() {
			m.ensureSession(m.refresh, nil)
		})
		btn.Importance = widget.HighImportance
		s.body.Add(btn)
		return
	}
	search := widget.NewEntry()
	search.SetPlaceHolder(L("Search", "Поиск"))
	search.SetText(s.filter)
	search.OnChanged = func(v string) {
		s.filter = v
		s.renderList(m)
	}
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), m.refresh)
	refresh.Importance = widget.LowImportance
	s.body.Add(lead(text(L("Users", "Пользователи"), 20, colText, false)))
	s.body.Add(container.NewBorder(nil, nil, nil, refresh, search))
	s.list = newVBoxFit()
	s.body.Add(s.list)
	s.renderList(m)
}

func (s *shareState) renderList(m *manager) {
	if s.list == nil {
		return
	}
	s.list.RemoveAll()
	q := strings.ToLower(strings.TrimSpace(s.filter))
	shown, transit := 0, 0
	for _, c := range m.state.Clients {
		if c.RelayOf != "" {
			transit++
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name), q) && !strings.Contains(strings.ToLower(c.Note), q) {
			continue
		}
		s.list.Add(m.clientCard(c))
		shown++
	}
	if shown == 0 {
		msg := L("No users yet.", "Пользователей пока нет.")
		if q != "" {
			msg = L("Nothing found.", "Ничего не найдено.")
		}
		s.list.Add(note(msg))
	}
	if transit > 0 {
		s.list.Add(note(L(fmt.Sprintf("Transit clients from other cascade nodes: %d", transit), fmt.Sprintf("Транзитных клиентов от других узлов каскада: %d", transit))))
	}
	s.list.Refresh()
}

func normRole(r string) string {
	if r == "admin" {
		return "admin"
	}
	return "user"
}

func (s *shareState) roleValue() string { return normRole(s.role) }

