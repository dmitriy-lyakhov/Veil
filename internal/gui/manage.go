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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

const (
	sshOpenTimeout = 40 * time.Second
	exitHere       = "Выход на этом сервере"
	exitLast       = "Выход на последнем узле"
)

func nodeName(p *config.Profile) string {
	return strings.ReplaceAll(strings.TrimSpace(p.Name), "/", "-")
}

func managed(p *config.Profile) bool { return p.SSH != nil }

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
	pw := widget.NewPasswordEntry()
	items := []*widget.FormItem{widget.NewFormItem("Пароль SSH", pw)}
	dialog.ShowForm("Вход на «"+p.Name+"»", "Войти", "Отмена", items, func(ok bool) {
		if !ok {
			done(nil, errors.New("отменено"))
			return
		}
		open(pw.Text)
	}, parent)
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
	status  *widget.Label
	tabs    []*widget.Button
	section int
}

func (u *ui) openManager(p *config.Profile) {
	m := &manager{u: u, p: p, w: u.app.NewWindow("Управление · " + p.Name)}
	m.status = widget.NewLabel("Подключение по SSH…")
	m.status.Wrapping = fyne.TextWrapWord
	m.body = container.NewVBox()
	seg := container.NewGridWithColumns(3)
	for i, title := range []string{"Клиенты", "Каскад", "Сервер"} {
		b := widget.NewButton(title, func() { m.show(i) })
		m.tabs = append(m.tabs, b)
		seg.Add(b)
	}
	header := container.NewVBox(text(p.Name, 20, colText, true), text(addresses(p), 12, colMuted, false), seg, m.status)
	m.w.SetContent(container.NewPadded(container.NewBorder(header, nil, nil, nil, container.NewVScroll(m.body))))
	m.w.Resize(fyne.NewSize(winWidth+80, 720))
	m.w.Show()
	m.highlight()
	u.withSession(m.w, p, func(s *deploy.Session, err error) {
		if err != nil {
			m.status.SetText("Ошибка: " + err.Error())
			return
		}
		m.sess = s
		m.refresh()
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
		done(errors.New("нет SSH-подключения"))
		return
	}
	m.status.SetText("Выполняется…")
	go func() {
		err := m.sess.Call(method, path, in, out)
		fyne.Do(func() {
			if err != nil && !errors.Is(err, deploy.ErrOutdated) && strings.Contains(err.Error(), "ssh") {
				m.u.dropSession(m.p.Name)
			}
			if err != nil {
				m.status.SetText("Ошибка: " + err.Error())
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
			m.renderError(err)
			return
		}
		m.state = st
		m.status.SetText(fmt.Sprintf("Сайт-прикрытие: %s · клиентов %d, онлайн %d", st.Server.Company, st.Server.Clients, st.Server.Online))
		m.render()
	})
}

func (m *manager) renderError(err error) {
	m.body.RemoveAll()
	msg := widget.NewLabel(err.Error())
	msg.Wrapping = fyne.TextWrapWord
	m.body.Add(card(msg))
	if errors.Is(err, deploy.ErrOutdated) {
		m.body.Add(m.updateButton())
	}
	m.body.Refresh()
}

func (m *manager) render() {
	m.body.RemoveAll()
	switch m.section {
	case 0:
		m.renderClients()
	case 1:
		m.renderCascade()
	default:
		m.renderServer()
	}
	m.body.Refresh()
}

func iconButton(icon fyne.Resource, tap func()) *widget.Button {
	b := widget.NewButtonWithIcon("", icon, tap)
	b.Importance = widget.LowImportance
	return b
}

func (m *manager) confirm(title, msg string, yes func()) {
	dialog.ShowConfirm(title, msg, func(ok bool) {
		if ok {
			yes()
		}
	}, m.w)
}

func (m *manager) renderClients() {
	add := widget.NewButtonWithIcon("Новый клиент", theme.ContentAddIcon(), m.addClient)
	add.Importance = widget.HighImportance
	m.body.Add(container.NewGridWithColumns(2, add, widget.NewButtonWithIcon("Обновить", theme.ViewRefreshIcon(), m.refresh)))
	transit := 0
	for _, c := range m.state.Clients {
		if c.RelayOf != "" {
			transit++
			continue
		}
		m.body.Add(m.clientCard(c))
	}
	if len(m.state.Clients)-transit == 0 {
		m.body.Add(card(widget.NewLabel("Клиентов пока нет.")))
	}
	if transit > 0 {
		m.body.Add(text(fmt.Sprintf("Транзитных клиентов от других узлов каскада: %d", transit), 12, colMuted, false))
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
	lines := container.NewVBox(text(c.Name, 15, colText, true))
	if c.Note != "" {
		lines.Add(text(c.Note, 12, colMuted, false))
	}
	route := "выход на этом сервере"
	if len(c.Route) > 0 {
		route = "→ " + strings.Join(c.Route, " → ")
	}
	if c.Auto != nil {
		route = "авто " + route
		if c.Auto.CostMs > 0 {
			route += fmt.Sprintf(" · %.0f мс", c.Auto.CostMs)
		}
	}
	if c.Disabled {
		route = "заблокирован · " + route
	}
	lines.Add(text(route, 12, colMuted, false))
	lines.Add(text(fmt.Sprintf("↓ %s  ↑ %s", humanBytes(c.TxBytes), humanBytes(c.RxBytes)), 12, colMuted, false))

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
		m.confirm("Новый конфиг", "Выпустить новый конфиг для «"+c.Name+"»? Старый перестанет работать.", func() {
			var r server.LinkResponse
			m.call("POST", "/clients/reissue", map[string]string{"name": c.Name}, &r, func(err error) {
				if err == nil {
					m.u.showShare(m.w, "Новый конфиг «"+c.Name+"»", r, "Старый конфиг больше не работает. Ссылка показывается один раз.", true)
					m.refresh()
				}
			})
		})
	})
	del := iconButton(theme.DeleteIcon(), func() {
		m.confirm("Удалить клиента", "Удалить «"+c.Name+"»? Его подключения будут разорваны.", func() {
			m.call("POST", "/clients/delete", map[string]string{"name": c.Name}, nil, func(err error) {
				if err == nil {
					m.refresh()
				}
			})
		})
	})
	actions := container.NewHBox(edit, lock, reissue, del)
	return card(container.NewBorder(nil, nil, container.NewCenter(container.NewGridWrap(fyne.NewSize(10, 10), dot)), container.NewCenter(actions), lines))
}

func (m *manager) editClient(c server.ClientInfo) {
	name := widget.NewEntry()
	name.SetText(c.Name)
	note := widget.NewEntry()
	note.SetText(c.Note)
	dialog.ShowForm("Клиент", "Сохранить", "Отмена", []*widget.FormItem{
		widget.NewFormItem("Имя", name), widget.NewFormItem("Заметка", note),
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
	name.SetPlaceHolder("Например: телефон Ани")
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
			routeBox.Add(widget.NewButton("Убрать последний узел", func() {
				route = route[:len(route)-1]
				rebuild()
			}))
		}
		loading := text("Загрузка вариантов…", 12, colMuted, false)
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
			first := exitHere
			if len(route) > 0 {
				first = exitLast
			}
			options := []string{first}
			for _, o := range resp.Upstreams {
				if !containsString(route, o) {
					options = append(options, "дальше через "+o)
				}
			}
			sel := widget.NewSelect(options, nil)
			sel.SetSelected(first)
			sel.OnChanged = func(v string) {
				if hop, ok := strings.CutPrefix(v, "дальше через "); ok {
					route = append(route, hop)
					rebuild()
				}
			}
			routeBox.Add(sel)
		})
	}
	rebuild()
	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Имя", name), widget.NewFormItem("Заметка", note)),
		text("Маршрут", 14, colText, true), routeBox,
	)
	d := dialog.NewCustomConfirm("Новый клиент", "Создать", "Отмена", container.NewGridWrap(fyne.NewSize(380, 360), container.NewVScroll(content)), func(ok bool) {
		if !ok {
			return
		}
		var r server.LinkResponse
		m.call("POST", "/clients", map[string]any{"name": name.Text, "note": note.Text, "route": route}, &r, func(err error) {
			if err != nil {
				dialog.ShowError(err, m.w)
				return
			}
			m.u.showShare(m.w, "Клиент «"+r.Name+"»", r, "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте файл или ссылку. Показывается один раз: в конфиге приватный ключ.", true)
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
	m.body.Add(text("Следующие узлы", 15, colText, true))
	if len(m.state.Upstreams) == 0 {
		m.body.Add(text("Нет — этот сервер выпускает трафик в интернет сам.", 12, colMuted, false))
	}
	for _, up := range m.state.Upstreams {
		del := iconButton(theme.DeleteIcon(), func() {
			m.confirm("Отключить узел", "Отключить «"+up.Name+"»?", func() {
				m.call("POST", "/upstreams/delete", map[string]string{"name": up.Name}, nil, func(err error) {
					if err == nil {
						m.refresh()
					}
				})
			})
		})
		info := container.NewVBox(text(up.Name, 14, colText, true), text(fmt.Sprintf("%s · клиентов %d", up.Company, up.Clients), 12, colMuted, false))
		m.body.Add(card(container.NewBorder(nil, nil, nil, del, info)))
	}
	link := widget.NewButtonWithIcon("Связать со своим сервером…", theme.MailForwardIcon(), m.linkOwnServer)
	link.Importance = widget.HighImportance
	m.body.Add(link)
	m.body.Add(widget.NewButtonWithIcon("Построить каскад из нескольких серверов…", theme.ListIcon(), func() { m.u.openChainBuilder(m.p) }))
	m.body.Add(widget.NewButtonWithIcon("Подключение через ближайший вход…", theme.MailForwardIcon(), m.u.openMultiEntryBuilder))
	if len(m.state.Latency) > 0 {
		m.body.Add(text("Задержки до соседей", 15, colText, true))
		for _, l := range m.state.Latency {
			line := fmt.Sprintf("%s — %.1f мс", l.Name, l.RTTms)
			col := colText
			if !l.OK {
				line, col = l.Name+" — нет связи", colBad
			}
			m.body.Add(text(line, 13, col, false))
		}
		opt := widget.NewButtonWithIcon("Перестроить автомаршруты сейчас", theme.ViewRefreshIcon(), func() {
			var resp struct {
				Events []server.RerouteEvent `json:"events"`
			}
			m.call("POST", "/cascade/optimize", map[string]any{}, &resp, func(err error) {
				if err != nil {
					return
				}
				msg := "Текущие маршруты оптимальны (или новый путь ещё не подтвердился — нужно два замера подряд)."
				if len(resp.Events) > 0 {
					var lines []string
					for _, e := range resp.Events {
						if e.Error != "" {
							lines = append(lines, e.Client+": "+e.Error)
						} else {
							lines = append(lines, fmt.Sprintf("%s: %s → %s (%.0f → %.0f мс)", e.Client, e.From, e.To, e.OldMs, e.NewMs))
						}
					}
					msg = strings.Join(lines, "\n")
				}
				dialog.ShowInformation("Автомаршруты", msg, m.w)
				m.refresh()
			})
		})
		m.body.Add(opt)
	}
	m.body.Add(widget.NewButtonWithIcon("Добавить узел по ключу каскада…", theme.ContentPasteIcon(), m.addUpstreamByKey))

	m.body.Add(widget.NewSeparator())
	m.body.Add(text("Выданные ключи каскада", 15, colText, true))
	if len(m.state.Relays) == 0 {
		m.body.Add(text("Ключи не выдавались.", 12, colMuted, false))
	}
	for _, rl := range m.state.Relays {
		del := iconButton(theme.DeleteIcon(), func() {
			m.confirm("Отозвать ключ", "Отозвать ключ «"+rl.Name+"»? Клиенты, идущие через этот узел, будут удалены.", func() {
				m.call("POST", "/relays/delete", map[string]string{"name": rl.Name}, nil, func(err error) {
					if err == nil {
						m.refresh()
					}
				})
			})
		})
		state := "не на связи"
		if rl.Online {
			state = "на связи"
		}
		info := container.NewVBox(text(rl.Name, 14, colText, true), text(fmt.Sprintf("%s · клиентов %d", state, rl.Clients), 12, colMuted, false))
		m.body.Add(card(container.NewBorder(nil, nil, nil, del, info)))
	}
	m.body.Add(widget.NewButtonWithIcon("Выдать ключ каскада…", theme.ContentAddIcon(), m.issueRelay))
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
		dialog.ShowInformation("Каскад", "Других своих серверов в приложении нет. Добавьте ещё один сервер по SSH.", m.w)
		return
	}
	names := make([]string, len(others))
	for i, p := range others {
		names[i] = p.Name
	}
	sel := widget.NewSelect(names, nil)
	sel.SetSelected(names[0])
	dialog.ShowForm("Следующий узел", "Связать", "Отмена", []*widget.FormItem{widget.NewFormItem("Сервер", sel)}, func(ok bool) {
		if !ok {
			return
		}
		for _, p := range others {
			if p.Name == sel.Selected {
				m.status.SetText("Связываем «" + m.p.Name + "» → «" + p.Name + "»…")
				m.u.linkPair(m.w, m.p, p, func(err error) {
					if err != nil {
						dialog.ShowError(err, m.w)
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
	dialog.ShowForm("Узел по ключу каскада", "Подключить", "Отмена", []*widget.FormItem{
		widget.NewFormItem("Название", name), widget.NewFormItem("Ключ", key),
	}, func(ok bool) {
		if !ok {
			return
		}
		m.call("POST", "/upstreams", map[string]string{"name": name.Text, "link": key.Text}, nil, func(err error) {
			if err != nil {
				dialog.ShowError(err, m.w)
				return
			}
			m.refresh()
		})
	}, m.w)
}

func (m *manager) issueRelay() {
	name := widget.NewEntry()
	name.SetPlaceHolder("Название предыдущего узла")
	dialog.ShowForm("Ключ каскада", "Выдать", "Отмена", []*widget.FormItem{widget.NewFormItem("Для узла", name)}, func(ok bool) {
		if !ok {
			return
		}
		var r server.LinkResponse
		m.call("POST", "/relays", map[string]string{"name": name.Text}, &r, func(err error) {
			if err != nil {
				dialog.ShowError(err, m.w)
				return
			}
			m.u.showShare(m.w, "Ключ каскада для «"+r.Name+"»", r, "Вставьте этот ключ на узле «"+r.Name+"»: Каскад → Добавить узел по ключу каскада.", false)
			m.refresh()
		})
	}, m.w)
}

func (m *manager) renderServer() {
	st := m.state.Server
	info := container.NewVBox(
		text("Сервер", 15, colText, true),
		text("Версия: "+st.Version, 12, colMuted, false),
		text("Сайт-прикрытие: "+st.Company, 12, colMuted, false),
	)
	if st.AdminURL != "" {
		info.Add(text("Админ-панель (через VPN): "+st.AdminURL, 12, colMuted, false))
	}
	m.body.Add(card(info))

	domain := widget.NewEntry()
	domain.SetPlaceHolder("example.com")
	domain.SetText(st.Domain)
	certText := map[string]string{
		"none":    "Домен не задан: клиенты подключаются по IP с закреплённым сертификатом.",
		"pending": "Получаем сертификат Let's Encrypt…",
		"ready":   "Сертификат действителен до " + st.Cert.Expires.Format("02.01.2006") + ". Новые клиенты подключаются по домену.",
		"error":   st.Cert.Message,
	}[st.Cert.State]
	certLbl := widget.NewLabel(certText)
	certLbl.Wrapping = fyne.TextWrapWord
	saveDomain := widget.NewButton("Сохранить домен", func() {
		m.call("POST", "/settings/domain", map[string]string{"domain": domain.Text}, nil, func(err error) {
			if err == nil {
				time.AfterFunc(3*time.Second, func() { fyne.Do(m.refresh) })
				m.refresh()
			}
		})
	})
	hint := widget.NewLabel("Укажите домен, A/AAAA-записи которого ведут на этот сервер. Сайт-прикрытие откроется по домену, сертификат выпустится автоматически.")
	hint.Wrapping = fyne.TextWrapWord
	m.body.Add(card(container.NewVBox(text("Домен", 15, colText, true), hint, domain, saveDomain, certLbl)))

	pw := widget.NewPasswordEntry()
	rep := widget.NewPasswordEntry()
	gen := widget.NewButton("Сгенерировать", func() {
		v := randomPassword()
		pw.SetText(v)
		rep.SetText(v)
		pw.Password = false
		pw.Refresh()
	})
	savePw := widget.NewButton("Сменить пароль админки", func() {
		if pw.Text != rep.Text {
			dialog.ShowError(errors.New("пароли не совпадают"), m.w)
			return
		}
		newPw := pw.Text
		m.call("POST", "/settings/password", map[string]string{"new": newPw}, nil, func(err error) {
			if err != nil {
				dialog.ShowError(err, m.w)
				return
			}
			m.p.AdminPassword = newPw
			if m.p.AdminUser == "" {
				m.p.AdminUser = "admin"
			}
			if err := saveProfile(m.p); err != nil {
				dialog.ShowError(err, m.w)
				return
			}
			dialog.ShowInformation("Готово", "Пароль админки изменён и сохранён в карточке сервера.", m.w)
		})
	})
	savePw.Importance = widget.HighImportance
	m.body.Add(card(container.NewVBox(text("Пароль админ-панели", 15, colText, true),
		widget.NewForm(widget.NewFormItem("Новый", pw), widget.NewFormItem("Ещё раз", rep)),
		container.NewGridWithColumns(2, gen, savePw))))
	m.body.Add(m.updateButton())
	m.body.Add(layout.NewSpacer())
}

func (m *manager) updateButton() fyne.CanvasObject {
	return widget.NewButtonWithIcon("Обновить серверную часть до версии "+appVersion, theme.UploadIcon(), func() {
		if m.sess == nil {
			return
		}
		m.status.SetText("Обновление сервера…")
		go func() {
			err := m.sess.Update(context.Background(), "", func(f string, a ...any) {
				line := fmt.Sprintf(f, a...)
				fyne.Do(func() { m.status.SetText(line) })
			})
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, m.w)
					return
				}
				m.refresh()
			})
		}()
	})
}
