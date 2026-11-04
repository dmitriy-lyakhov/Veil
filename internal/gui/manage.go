package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

const sshOpenTimeout = 40 * time.Second

func exitHere() string { return L("Exit on this server", "Выход на этом сервере") }
func exitLast() string {
	return L("Exit on the last node", "Выход на последнем узле")
}

func nodeName(p *config.Profile) string {
	return strings.ReplaceAll(strings.TrimSpace(p.Name), "/", "-")
}

func managed(p *config.Profile) bool { return p.SSH != nil && p.IsVeil() }

func (u *ui) withSession(parent fyne.Window, p *config.Profile, done func(*deploy.Session, error)) {
	if s, ok := u.sessions[p.Name]; ok {
		done(s, nil)
		return
	}
	open := func(password string) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), sshOpenTimeout)
			defer cancel()
			s, err := deploy.Open(ctx, p, password, "")
			fyne.Do(func() {
				if err == nil {
					u.sessions[p.Name] = s
				}
				done(s, err)
			})
		}()
	}
	if !deploy.NeedsPassword(p) {
		open("")
		return
	}
	u.askSSHPassword(parent, p, func(s *deploy.Session, err error) {
		if err == nil {
			u.sessions[p.Name] = s
		}
		done(s, err)
	})
}

func isAuthError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unable to authenticate")
}

// askSSHPassword — окно ввода пароля SSH с проверкой прямо в нём: пока идёт
// подключение, показывается индикатор; при неверном пароле окно остаётся
// открытым, карточка трясётся и появляется сообщение под полем.
func (u *ui) askSSHPassword(parent fyne.Window, p *config.Profile, done func(*deploy.Session, error)) {
	pw := widget.NewPasswordEntry()
	errLbl := canvas.NewText("", colBad)
	errLbl.TextSize = 12
	errBox := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), errLbl)
	errBox.Hide()
	progress := newThinBar()
	barBox := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), progress)
	barBox.Hide()
	content := container.NewVBox(widget.NewForm(widget.NewFormItem(L("SSH password", "Пароль SSH"), pw)), errBox, barBox)

	m := newModal(parent, fmt.Sprintf(L("Sign in to “%s”", "Вход на «%s»"), p.Name), content)
	cancel := widget.NewButton(L("Cancel", "Отмена"), nil)
	login := widget.NewButton(L("Sign in", "Войти"), nil)
	login.Importance = widget.HighImportance
	m.buttons.Add(cancel)
	m.buttons.Add(login)

	fail := func(msg string) {
		errLbl.Text = msg
		errLbl.Refresh()
		errBox.Show()
		progress.Stop()
		barBox.Hide()
		pw.Enable()
		login.Enable()
		parent.Canvas().Focus(pw)
		m.shake()
	}
	var submit func()
	submit = func() {
		if pw.Text == "" {
			fail(L("Enter the password", "Введите пароль"))
			return
		}
		errBox.Hide()
		pw.Disable()
		login.Disable()
		barBox.Show()
		progress.Start()
		pass := pw.Text
		go func() {
			ctx, stop := context.WithTimeout(context.Background(), sshOpenTimeout)
			defer stop()
			s, err := deploy.Open(ctx, p, pass, "")
			fyne.Do(func() {
				switch {
				case err == nil:
					m.Hide()
					done(s, nil)
				case isAuthError(err):
					pw.SetText("")
					fail(L("Wrong password", "Неверный пароль"))
				default:
					fail(L("Failed to connect: ", "Не удалось подключиться: ") + shortErr(err))
				}
			})
		}()
	}
	login.OnTapped = submit
	pw.OnSubmitted = func(string) { submit() }
	cancel.OnTapped = func() {
		m.Hide()
		done(nil, errors.New("отменено"))
	}
	m.Show()
	parent.Canvas().Focus(pw)
}

func shortErr(err error) string {
	s := err.Error()
	if r := []rune(s); len(r) > 90 {
		s = string(r[:90]) + "…"
	}
	return s
}

func (u *ui) dropSession(name string) {
	if s, ok := u.sessions[name]; ok {
		s.Close()
		delete(u.sessions, name)
	}
}

type manager struct {
	u       *ui
	p       *config.Profile
	w       fyne.Window
	sess    *deploy.Session
	state   server.State
	body    *fyne.Container
	status  *statusLine
	tabs    []*widget.Button
	section int

	loaded      bool   // состояние сервера загружено (был вход по SSH)
	clientsOnly bool   // менеджер встроен в раздел «Поделиться»: рисует только клиентов
	onLoaded    func() // вызывается после обновления данных (для встроенного режима)
}

func (u *ui) openManager(p *config.Profile) {
	m := &manager{u: u, p: p, w: u.newSubPage(p.Name)}
	m.status = newStatusLine()
	m.body = newVBoxFit()
	seg := container.NewGridWithColumns(4)
	for i, title := range []string{L("Protocol", "Протокол"), L("Services", "Сервисы"), L("Cascade", "Каскад"), L("Management", "Управление")} {
		b := widget.NewButton(title, func() { m.show(i) })
		m.tabs = append(m.tabs, b)
		seg.Add(b)
	}
	header := container.NewVBox(lead(text(sshSummary(p), 14, colMuted, false)), seg, m.status.Label)
	m.w.SetContent(container.NewPadded(container.NewBorder(header, nil, nil, nil, container.NewVScroll(m.body))))
	m.w.Resize(fyne.NewSize(winWidth+80, 720))
	m.w.Show()
	m.highlight()
	// вход по SSH не требуется для просмотра страницы: пароль спросим, когда он понадобится
	if s, ok := u.sessions[p.Name]; ok {
		m.sess = s
		m.refresh()
	} else {
		m.render()
	}
}

// ensureSession выполняет ok, когда есть SSH-сессия; при необходимости спрашивает пароль.
func (m *manager) ensureSession(ok func(), fail func(error)) {
	if m.sess != nil {
		ok()
		return
	}
	m.u.withSession(m.w, m.p, func(s *deploy.Session, err error) {
		if err != nil {
			if fail != nil {
				fail(err)
			}
			return
		}
		m.sess = s
		ok()
	})
}

func (m *manager) highlight() {
	for i, b := range m.tabs {
		if i == m.section {
			b.Importance = widget.HighImportance
		} else {
			b.Importance = widget.MediumImportance
		}
		b.Refresh()
	}
}

func (m *manager) show(i int) {
	m.section = i
	m.highlight()
	m.render()
}

func (m *manager) call(method, path string, in, out any, done func(error)) {
	if m.sess == nil {
		m.ensureSession(func() { m.call(method, path, in, out, done) }, done)
		return
	}
	m.status.SetText(L("Working…", "Выполняется…"))
	go func() {
		err := m.sess.Call(method, path, in, out)
		fyne.Do(func() {
			if err != nil && !errors.Is(err, deploy.ErrOutdated) && strings.Contains(err.Error(), "ssh") {
				m.u.dropSession(m.p.Name)
			}
			if err != nil {
				m.status.SetText(L("Error: ", "Ошибка: ") + err.Error())
			} else {
				m.status.SetText("")
			}
			done(err)
		})
	}()
}

func (m *manager) refresh() {
	var st server.State
	m.call("GET", "/state", nil, &st, func(err error) {
		if err != nil {
			if err.Error() == "отменено" {
				m.render()
				return
			}
			m.renderError(err)
			return
		}
		m.state = st
		m.loaded = true
		m.status.SetText(fmt.Sprintf(L("Cover site: %s · clients %d, online %d", "Сайт-прикрытие: %s · клиентов %d, онлайн %d"), st.Server.Company, st.Server.Clients, st.Server.Online))
		m.render()
	})
}

func (m *manager) renderError(err error) {
	if m.clientsOnly {
		showError(err, m.w)
		return
	}
	m.body.RemoveAll()
	msg := widget.NewLabel(err.Error())
	msg.Wrapping = fyne.TextWrapWord
	m.body.Add(card(msg))
	if errors.Is(err, deploy.ErrOutdated) {
		m.body.Add(m.updateButton(""))
	}
	m.body.Refresh()
}

func (m *manager) render() {
	if m.clientsOnly {
		if m.onLoaded != nil {
			m.onLoaded()
		}
		return
	}
	m.body.RemoveAll()
	switch m.section {
	case 0:
		m.renderProtocol()
	case 1:
		if m.loaded {
			m.renderServer()
		} else {
			m.renderLogin(L("The cover site, domain and admin panel password settings are stored on the server. Sign in over SSH to open them.", "Настройки сайта-прикрытия, домена и пароля админ-панели хранятся на сервере. Чтобы их открыть, войдите по SSH."))
		}
	case 2:
		if m.loaded {
			m.renderCascade()
		} else {
			m.renderLogin(L("The cascade is configured on the server. Sign in over SSH to open it.", "Каскад настраивается на сервере. Чтобы его открыть, войдите по SSH."))
		}
	default:
		m.renderManagement()
	}
	m.body.Refresh()
}

func iconButton(icon fyne.Resource, tap func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tap)
	b.Importance = widget.LowImportance
	return b
}

func (m *manager) confirm(title, msg string, yes func()) {
	showConfirm(title, msg, func(ok bool) {
		if ok {
			yes()
		}
	}, m.w)
}

func (m *manager) renderClients() {
	add := widget.NewButtonWithIcon(L("New client", "Новый клиент"), theme.ContentAddIcon(), m.addClient)
	add.Importance = widget.HighImportance
	m.body.Add(container.NewGridWithColumns(2, add, widget.NewButtonWithIcon(L("Refresh", "Обновить"), theme.ViewRefreshIcon(), m.refresh)))
	transit := 0
	for _, c := range m.state.Clients {
		if c.RelayOf != "" {
			transit++
			continue
		}
		m.body.Add(m.clientCard(c))
	}
	if len(m.state.Clients)-transit == 0 {
		m.body.Add(card(widget.NewLabel(L("No clients yet.", "Клиентов пока нет."))))
	}
	if transit > 0 {
		m.body.Add(text(fmt.Sprintf(L("Transit clients from other cascade nodes: %d", "Транзитных клиентов от других узлов каскада: %d"), transit), 12, colMuted, false))
	}
}

func (m *manager) clientCard(c server.ClientInfo) fyne.CanvasObject {
	dot := canvas.NewCircle(colIdle)
	switch {
	case c.Disabled:
		dot.FillColor = colBad
	case c.Online:
		dot.FillColor = colOK
	}
	lines := container.New(tightVBox{gap: -7, afterFirst: -12}, richText(c.Name, false, true))
	if c.Note != "" {
		lines.Add(richText(c.Note, true, false))
	}
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
	lines.Add(richText(route, true, false))
	lines.Add(richText(fmt.Sprintf("↓ %s  ↑ %s", humanBytes(c.TxBytes), humanBytes(c.RxBytes)), true, false))

	edit := iconButton(theme.DocumentCreateIcon(), func() { m.editClient(c) })
	lockIcon := theme.VisibilityOffIcon()
	if c.Disabled {
		lockIcon = theme.VisibilityIcon()
	}
	lock := iconButton(lockIcon, func() {
		next := !c.Disabled
		m.call("POST", "/clients/update", map[string]any{"name": c.Name, "disabled": next}, nil, func(err error) {
			if err == nil {
				m.refresh()
			}
		})
	})
	reissue := iconButton(theme.ViewRefreshIcon(), func() {
		m.confirm(L("New config", "Новый конфиг"), fmt.Sprintf(L("Issue a new config for “%s”? The old one will stop working.", "Выпустить новый конфиг для «%s»? Старый перестанет работать."), c.Name), func() {
			var r server.LinkResponse
			m.call("POST", "/clients/reissue", map[string]string{"name": c.Name}, &r, func(err error) {
				if err == nil {
					m.u.showShare(m.w, fmt.Sprintf(L("New config “%s”", "Новый конфиг «%s»"), c.Name), r, L("The old config no longer works. The link is shown once.", "Старый конфиг больше не работает. Ссылка показывается один раз."), true)
					m.refresh()
				}
			})
		})
	})
	del := iconButton(theme.DeleteIcon(), func() {
		m.confirm(L("Delete client", "Удалить клиента"), fmt.Sprintf(L("Delete “%s”? Its connections will be dropped.", "Удалить «%s»? Его подключения будут разорваны."), c.Name), func() {
			m.call("POST", "/clients/delete", map[string]string{"name": c.Name}, nil, func(err error) {
				if err == nil {
					m.refresh()
				}
			})
		})
	})
	sq := func(b fyne.CanvasObject) fyne.CanvasObject { return container.NewGridWrap(fyne.NewSize(34, 34), b) }
	actions := container.NewHBox(sq(edit), sq(lock), sq(reissue), sq(del))
	return cardTight(container.NewBorder(nil, nil, container.NewCenter(container.NewGridWrap(fyne.NewSize(10, 10), dot)), container.NewCenter(actions), lines))
}

func (m *manager) editClient(c server.ClientInfo) {
	name := widget.NewEntry()
	name.SetText(c.Name)
	note := widget.NewEntry()
	note.SetText(c.Note)
	showForm(L("Client", "Клиент"), L("Save", "Сохранить"), L("Cancel", "Отмена"), []*widget.FormItem{
		widget.NewFormItem(L("Name", "Имя"), name), widget.NewFormItem(L("Note", "Заметка"), note),
	}, func(ok bool) {
		if !ok {
			return
		}
		m.call("POST", "/clients/update", map[string]any{"name": c.Name, "new_name": name.Text, "note": note.Text}, nil, func(err error) {
			if err == nil {
				m.refresh()
			}
		})
	}, m.w)
}

func (m *manager) addClient() {
	name := widget.NewEntry()
	name.SetPlaceHolder(L("For example: Anna's phone", "Например: телефон Ани"))
	note := widget.NewEntry()
	routeBox := container.NewVBox()
	var route []string
	var rebuild func()
	rebuild = func() {
		routeBox.RemoveAll()
		for i, h := range route {
			routeBox.Add(text(fmt.Sprintf("%d. %s", i+1, h), 13, colText, false))
		}
		if len(route) > 0 {
			routeBox.Add(widget.NewButton(L("Remove last node", "Убрать последний узел"), func() {
				route = route[:len(route)-1]
				rebuild()
			}))
		}
		loading := text(L("Loading options…", "Загрузка вариантов…"), 12, colMuted, false)
		routeBox.Add(loading)
		var resp struct {
			Upstreams []string `json:"upstreams"`
		}
		m.call("POST", "/explore", map[string]any{"route": route}, &resp, func(err error) {
			routeBox.Remove(loading)
			if err != nil {
				routeBox.Add(text(err.Error(), 12, colBad, false))
				return
			}
			hopPrefix := L("then via ", "дальше через ")
			first := exitHere()
			if len(route) > 0 {
				first = exitLast()
			}
			options := []string{first}
			for _, o := range resp.Upstreams {
				if !containsString(route, o) {
					options = append(options, hopPrefix+o)
				}
			}
			sel := widget.NewSelect(options, nil)
			sel.SetSelected(first)
			sel.OnChanged = func(v string) {
				if hop, ok := strings.CutPrefix(v, hopPrefix); ok {
					route = append(route, hop)
					rebuild()
				}
			}
			routeBox.Add(sel)
		})
	}
	rebuild()
	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(L("Name", "Имя"), name), widget.NewFormItem(L("Note", "Заметка"), note)),
		text(L("Route", "Маршрут"), 14, colText, true), routeBox,
	)
	d := newCustomConfirm(L("New client", "Новый клиент"), L("Create", "Создать"), L("Cancel", "Отмена"), container.NewGridWrap(fyne.NewSize(380, 360), container.NewVScroll(content)), func(ok bool) {
		if !ok {
			return
		}
		var r server.LinkResponse
		m.call("POST", "/clients", map[string]any{"name": name.Text, "note": note.Text, "route": route}, &r, func(err error) {
			if err != nil {
				showError(err, m.w)
				return
			}
			m.u.showShare(m.w, fmt.Sprintf(L("Client “%s”", "Клиент «%s»"), r.Name), r, L("Scan the QR in the Veil app on the client's device, or send the file or link. Shown once: the config contains a private key.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте файл или ссылку. Показывается один раз: в конфиге приватный ключ."), true)
			m.refresh()
		})
	}, m.w)
	d.Show()
}

func containsString(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

func (m *manager) renderCascade() {
	m.body.Add(text(L("Next nodes", "Следующие узлы"), 15, colText, true))
	if len(m.state.Upstreams) == 0 {
		m.body.Add(text(L("None — this server sends traffic to the internet itself.", "Нет — этот сервер выпускает трафик в интернет сам."), 12, colMuted, false))
	}
	for _, up := range m.state.Upstreams {
		del := iconButton(theme.DeleteIcon(), func() {
			m.confirm(L("Disconnect node", "Отключить узел"), fmt.Sprintf(L("Disconnect “%s”?", "Отключить «%s»?"), up.Name), func() {
				m.call("POST", "/upstreams/delete", map[string]string{"name": up.Name}, nil, func(err error) {
					if err == nil {
						m.refresh()
					}
				})
			})
		})
		info := container.NewVBox(text(up.Name, 14, colText, true), text(fmt.Sprintf(L("%s · clients %d", "%s · клиентов %d"), up.Company, up.Clients), 12, colMuted, false))
		m.body.Add(card(container.NewBorder(nil, nil, nil, del, info)))
	}
	link := widget.NewButtonWithIcon(L("Link to your own server…", "Связать со своим сервером…"), theme.MailForwardIcon(), m.linkOwnServer)
	link.Importance = widget.HighImportance
	m.body.Add(link)
	m.body.Add(widget.NewButtonWithIcon(L("Build a cascade from several servers…", "Построить каскад из нескольких серверов…"), theme.ListIcon(), func() { m.u.openChainBuilder(m.p) }))
	m.body.Add(widget.NewButtonWithIcon(L("Connection via the nearest entry…", "Подключение через ближайший вход…"), theme.MailForwardIcon(), m.u.openMultiEntryBuilder))
	if len(m.state.Latency) > 0 {
		m.body.Add(text(L("Latency to neighbours", "Задержки до соседей"), 15, colText, true))
		for _, l := range m.state.Latency {
			line := fmt.Sprintf(L("%s — %.1f ms", "%s — %.1f мс"), l.Name, l.RTTms)
			col := colText
			if !l.OK {
				line, col = l.Name+L(" — no connection", " — нет связи"), colBad
			}
			m.body.Add(text(line, 13, col, false))
		}
		opt := widget.NewButtonWithIcon(L("Rebuild auto routes now", "Перестроить автомаршруты сейчас"), theme.ViewRefreshIcon(), func() {
			var resp struct {
				Events []server.RerouteEvent `json:"events"`
			}
			m.call("POST", "/cascade/optimize", map[string]any{}, &resp, func(err error) {
				if err != nil {
					return
				}
				msg := L("Current routes are optimal (or the new path is not yet confirmed — two measurements in a row are needed).", "Текущие маршруты оптимальны (или новый путь ещё не подтвердился — нужно два замера подряд).")
				if len(resp.Events) > 0 {
					var lines []string
					for _, e := range resp.Events {
						if e.Error != "" {
							lines = append(lines, e.Client+": "+e.Error)
						} else {
							lines = append(lines, fmt.Sprintf(L("%s: %s → %s (%.0f → %.0f ms)", "%s: %s → %s (%.0f → %.0f мс)"), e.Client, e.From, e.To, e.OldMs, e.NewMs))
						}
					}
					msg = strings.Join(lines, "\n")
				}
				showInfo(L("Auto routes", "Автомаршруты"), msg, m.w)
				m.refresh()
			})
		})
		m.body.Add(opt)
	}
	m.body.Add(widget.NewButtonWithIcon(L("Add node by cascade key…", "Добавить узел по ключу каскада…"), theme.ContentPasteIcon(), m.addUpstreamByKey))

	m.body.Add(widget.NewSeparator())
	m.body.Add(text(L("Issued cascade keys", "Выданные ключи каскада"), 15, colText, true))
	if len(m.state.Relays) == 0 {
		m.body.Add(text(L("No keys have been issued.", "Ключи не выдавались."), 12, colMuted, false))
	}
	for _, rl := range m.state.Relays {
		del := iconButton(theme.DeleteIcon(), func() {
			m.confirm(L("Revoke key", "Отозвать ключ"), fmt.Sprintf(L("Revoke the key “%s”? Clients going through this node will be deleted.", "Отозвать ключ «%s»? Клиенты, идущие через этот узел, будут удалены."), rl.Name), func() {
				m.call("POST", "/relays/delete", map[string]string{"name": rl.Name}, nil, func(err error) {
					if err == nil {
						m.refresh()
					}
				})
			})
		})
		state := L("offline", "не на связи")
		if rl.Online {
			state = L("online", "на связи")
		}
		info := container.NewVBox(text(rl.Name, 14, colText, true), text(fmt.Sprintf(L("%s · clients %d", "%s · клиентов %d"), state, rl.Clients), 12, colMuted, false))
		m.body.Add(card(container.NewBorder(nil, nil, nil, del, info)))
	}
	m.body.Add(widget.NewButtonWithIcon(L("Issue cascade key…", "Выдать ключ каскада…"), theme.ContentAddIcon(), m.issueRelay))
}

func (m *manager) otherManaged() []*config.Profile {
	var out []*config.Profile
	for _, p := range m.u.profiles {
		if managed(p) && p.Name != m.p.Name {
			out = append(out, p)
		}
	}
	return out
}

func (m *manager) linkOwnServer() {
	others := m.otherManaged()
	if len(others) == 0 {
		showInfo(L("Cascade", "Каскад"), L("There are no other servers of yours in the app. Add another server over SSH.", "Других своих серверов в приложении нет. Добавьте ещё один сервер по SSH."), m.w)
		return
	}
	names := make([]string, len(others))
	for i, p := range others {
		names[i] = p.Name
	}
	sel := widget.NewSelect(names, nil)
	sel.SetSelected(names[0])
	showForm(L("Next node", "Следующий узел"), L("Link", "Связать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("Server", "Сервер"), sel)}, func(ok bool) {
		if !ok {
			return
		}
		for _, p := range others {
			if p.Name == sel.Selected {
				m.status.SetText(fmt.Sprintf(L("Linking “%s” → “%s”…", "Связываем «%s» → «%s»…"), m.p.Name, p.Name))
				m.u.linkPair(m.w, m.p, p, func(err error) {
					if err != nil {
						showError(err, m.w)
					}
					m.refresh()
				})
			}
		}
	}, m.w)
}

func (u *ui) linkPair(parent fyne.Window, from, to *config.Profile, done func(error)) {
	u.withSession(parent, to, func(next *deploy.Session, err error) {
		if err != nil {
			done(fmt.Errorf("%s: %w", to.Name, err))
			return
		}
		u.withSession(parent, from, func(prev *deploy.Session, err error) {
			if err != nil {
				done(fmt.Errorf("%s: %w", from.Name, err))
				return
			}
			go func() {
				err := linkSessions(prev, next, nodeName(from), nodeName(to))
				fyne.Do(func() { done(err) })
			}()
		})
	})
}

func linkSessions(prev, next *deploy.Session, fromName, toName string) error {
	var st server.State
	if err := prev.Call("GET", "/state", nil, &st); err != nil {
		return fmt.Errorf("%s: %w", fromName, err)
	}
	for _, u := range st.Upstreams {
		if u.Name == toName {
			return nil
		}
	}
	var issued server.LinkResponse
	if err := next.Call("POST", "/relays", map[string]string{"name": fromName}, &issued); err != nil {
		if !strings.Contains(err.Error(), "уже выдан") {
			return fmt.Errorf("%s: %w", toName, err)
		}
		if err := next.Call("POST", "/relays/delete", map[string]string{"name": fromName}, nil); err != nil {
			return fmt.Errorf("%s: %w", toName, err)
		}
		if err := next.Call("POST", "/relays", map[string]string{"name": fromName}, &issued); err != nil {
			return fmt.Errorf("%s: %w", toName, err)
		}
	}
	if err := prev.Call("POST", "/upstreams", map[string]string{"name": toName, "link": issued.Link}, nil); err != nil {
		return fmt.Errorf("%s: %w", fromName, err)
	}
	return nil
}

func (m *manager) addUpstreamByKey() {
	name := widget.NewEntry()
	key := widget.NewMultiLineEntry()
	key.SetPlaceHolder("veil-relay://…")
	key.Wrapping = fyne.TextWrapBreak
	key.SetMinRowsVisible(4)
	showForm(L("Node by cascade key", "Узел по ключу каскада"), L("Connect", "Подключить"), L("Cancel", "Отмена"), []*widget.FormItem{
		widget.NewFormItem(L("Name", "Название"), name), widget.NewFormItem(L("Key", "Ключ"), key),
	}, func(ok bool) {
		if !ok {
			return
		}
		m.call("POST", "/upstreams", map[string]string{"name": name.Text, "link": key.Text}, nil, func(err error) {
			if err != nil {
				showError(err, m.w)
				return
			}
			m.refresh()
		})
	}, m.w)
}

func (m *manager) issueRelay() {
	name := widget.NewEntry()
	name.SetPlaceHolder(L("Name of the previous node", "Название предыдущего узла"))
	showForm(L("Cascade key", "Ключ каскада"), L("Issue", "Выдать"), L("Cancel", "Отмена"), []*widget.FormItem{widget.NewFormItem(L("For node", "Для узла"), name)}, func(ok bool) {
		if !ok {
			return
		}
		var r server.LinkResponse
		m.call("POST", "/relays", map[string]string{"name": name.Text}, &r, func(err error) {
			if err != nil {
				showError(err, m.w)
				return
			}
			m.u.showShare(m.w, fmt.Sprintf(L("Cascade key for “%s”", "Ключ каскада для «%s»"), r.Name), r, fmt.Sprintf(L("Paste this key on node “%s”: Cascade → Add node by cascade key.", "Вставьте этот ключ на узле «%s»: Каскад → Добавить узел по ключу каскада."), r.Name), false)
			m.refresh()
		})
	}, m.w)
}

func (m *manager) renderServer() {
	st := m.state.Server
	info := container.New(tightVBox{gap: -4},
		text(L("Server", "Сервер"), 15, colText, true),
		richText(L("Version: ", "Версия: ")+st.Version, true, false),
		richText(L("Cover site: ", "Сайт-прикрытие: ")+st.Company, true, false),
	)
	if st.AdminURL != "" {
		info.Add(richText(L("Admin panel (via VPN): ", "Админ-панель (через VPN): ")+st.AdminURL, true, false))
	}
	m.body.Add(card(info))

	domain := widget.NewEntry()
	domain.SetPlaceHolder("example.com")
	domain.SetText(st.Domain)
	certText := map[string]string{
		"none":    L("No domain set: clients connect by IP with a pinned certificate.", "Домен не задан: клиенты подключаются по IP с закреплённым сертификатом."),
		"pending": L("Obtaining a Let's Encrypt certificate…", "Получаем сертификат Let's Encrypt…"),
		"ready":   L("Certificate valid until ", "Сертификат действителен до ") + st.Cert.Expires.Format("02.01.2006") + L(". New clients connect by domain.", ". Новые клиенты подключаются по домену."),
		"error":   st.Cert.Message,
	}[st.Cert.State]
	certLbl := widget.NewLabel(certText)
	certLbl.Wrapping = fyne.TextWrapWord
	saveDomain := widget.NewButton(L("Save domain", "Сохранить домен"), func() {
		m.call("POST", "/settings/domain", map[string]string{"domain": domain.Text}, nil, func(err error) {
			if err == nil {
				time.AfterFunc(3*time.Second, func() { fyne.Do(m.refresh) })
				m.refresh()
			}
		})
	})
	hint := widget.NewLabel(L("Enter a domain whose A/AAAA records point to this server. The cover site will open on the domain, and the certificate will be issued automatically.", "Укажите домен, A/AAAA-записи которого ведут на этот сервер. Сайт-прикрытие откроется по домену, сертификат выпустится автоматически."))
	hint.Wrapping = fyne.TextWrapWord
	m.body.Add(card(container.NewVBox(lead(text(L("Domain", "Домен"), 15, colText, true)), hint, domain, saveDomain, certLbl)))

	pw := widget.NewPasswordEntry()
	rep := widget.NewPasswordEntry()
	gen := widget.NewButton(L("Generate", "Сгенерировать"), func() {
		v := randomPassword()
		pw.SetText(v)
		rep.SetText(v)
		pw.Password = false
		pw.Refresh()
	})
	savePw := widget.NewButton(L("Change admin password", "Сменить пароль админки"), func() {
		if pw.Text != rep.Text {
			showError(errors.New(L("passwords do not match", "пароли не совпадают")), m.w)
			return
		}
		newPw := pw.Text
		m.call("POST", "/settings/password", map[string]string{"new": newPw}, nil, func(err error) {
			if err != nil {
				showError(err, m.w)
				return
			}
			m.p.AdminPassword = newPw
			if m.p.AdminUser == "" {
				m.p.AdminUser = "admin"
			}
			if err := saveProfile(m.p); err != nil {
				showError(err, m.w)
				return
			}
			showInfo(L("Done", "Готово"), L("The admin password has been changed and saved in the server card.", "Пароль админки изменён и сохранён в карточке сервера."), m.w)
		})
	})
	savePw.Importance = widget.HighImportance
	m.body.Add(card(container.NewVBox(lead(text(L("Admin panel password", "Пароль админ-панели"), 15, colText, true)),
		widget.NewForm(widget.NewFormItem(L("New", "Новый"), pw), widget.NewFormItem(L("Repeat", "Ещё раз"), rep)),
		container.NewVBox(gen, savePw))))
	m.body.Add(m.updateButton(st.Version))
	m.body.Add(layout.NewSpacer())
}

func (m *manager) updateButton(serverVersion string) fyne.CanvasObject {
	label := L("Update server to ", "Обновить сервер до ") + appVersion
	if serverVersion == appVersion {
		label = L("Reinstall the server part (", "Переустановить серверную часть (") + appVersion + ")"
	}
	return widget.NewButtonWithIcon(label, theme.UploadIcon(), func() {
		if m.sess == nil {
			return
		}
		m.status.SetText(L("Updating server…", "Обновление сервера…"))
		go func() {
			err := m.sess.Update(context.Background(), "", func(f string, a ...any) {
				line := fmt.Sprintf(f, a...)
				fyne.Do(func() { m.status.SetText(line) })
			})
			fyne.Do(func() {
				if err != nil {
					showError(err, m.w)
					return
				}
				m.refresh()
			})
		}()
	})
}
