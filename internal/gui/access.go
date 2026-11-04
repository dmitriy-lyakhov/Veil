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
	sel    *widget.Select
	body   *fyne.Container
	mgrs   map[string]*manager
	name   string // введённое имя нового клиента (сохраняется между перерисовками)
	list   *fyne.Container
}

func (u *ui) sharePage() fyne.CanvasObject {
	s := &shareState{u: u, mgrs: map[string]*manager{}, name: L("New client", "Новый клиент")}
	u.share = s
	title := text(L("Share access", "Поделиться доступом"), 20, colText, false)
	title.Alignment = fyne.TextAlignCenter
	seg := newSegmented([]string{L("Connection", "Подключение"), L("Users", "Пользователи")}, 0, func(i int) {
		s.tab = i
		s.render()
	})
	s.sel = widget.NewSelect(nil, func(v string) {
		s.server = v
		s.render()
	})
	s.body = newVBoxFit()
	top := container.NewVBox(container.NewPadded(title), seg, lead(text(L("Server", "Сервер"), 12, colMuted, false)), s.sel)
	s.refreshServers()
	return container.NewPadded(container.NewBorder(top, nil, nil, nil, container.NewVScroll(s.body)))
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
	s.sel.Options = names
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
	s.sel.Selected = s.server
	s.sel.Refresh()
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
	s.body.Add(note(L("Share VPN access without the ability to manage the server.", "Поделитесь доступом к VPN без возможности управлять сервером.")))
	name := widget.NewEntry()
	name.SetText(s.name)
	name.OnChanged = func(v string) { s.name = v }
	fmtApp := L("For the Veil app", "Для приложения Veil")
	format := widget.NewSelect([]string{fmtApp}, nil)
	format.SetSelected(fmtApp)
	share := widget.NewButtonWithIcon(L("Share", "Поделиться"), theme.MailForwardIcon(), func() {
		n := strings.TrimSpace(s.name)
		if n == "" {
			showError(fmt.Errorf("%s", L("enter a user name", "укажите имя пользователя")), s.u.win)
			return
		}
		m.ensureSession(func() {
			var r server.LinkResponse
			m.call("POST", "/clients", map[string]any{"name": n, "note": "", "route": []string{}}, &r, func(err error) {
				if err != nil {
					showError(err, s.u.win)
					return
				}
				s.u.showShare(s.u.win, L("Client \""+r.Name+"\"", "Клиент «"+r.Name+"»"), r, L("Scan the QR in the Veil app on the client device, or send the file or link. Shown once: the config contains a private key.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте файл или ссылку. Показывается один раз: в конфиге приватный ключ."), true)
				s.name = L("New client", "Новый клиент")
				m.refresh()
				s.render()
			})
		}, nil)
	})
	share.Importance = widget.HighImportance
	route := widget.NewButtonWithIcon(L("Set route via cascade…", "Задать маршрут через каскад…"), theme.ListIcon(), func() {
		m.ensureSession(m.addClient, nil)
	})
	route.Importance = widget.LowImportance
	s.body.Add(lead(text(L("User name", "Имя пользователя"), 12, colMuted, false)))
	s.body.Add(name)
	s.body.Add(lead(text(L("Connection format", "Формат подключения"), 12, colMuted, false)))
	s.body.Add(format)
	s.body.Add(share)
	s.body.Add(route)
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
