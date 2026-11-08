package gui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/client/dialer"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/provision"
	"veil/internal/server"
	"veil/internal/vpnconf/wgkey"
)

func routeDirect() string { return L("Directly to the exit", "Напрямую к выходу") }
func routeAuto() string {
	return L("Via nodes, automatically by latency", "Через узлы, автоматически по задержкам")
}
func hopsAll() string { return L("all available", "все доступные") }

func (u *ui) managedProfiles() []*config.Profile {
	var out []*config.Profile
	for _, p := range u.profiles {
		if sshManaged(p) {
			out = append(out, p)
		}
	}
	return out
}

func (u *ui) openMultiEntryBuilder() { u.openCascadeBuilder(false) }

// openCascadeBuilder — построитель подключения с несколькими входами. В режиме shareMode (вкладка «Доступ»)
// результат предназначен другому человеку: он получает ОДНО подключение (ссылка, QR или файл) со всеми входами
// внутри и не заводит серверы по отдельности; пароли, SSH и админ-доступ в него не попадают.
func (u *ui) openCascadeBuilder(shareMode bool) {
	servers := u.managedProfiles()
	if len(servers) == 0 {
		kit.ShowInfo(L("No servers of your own", "Нет своих серверов"), L("First add at least one server over SSH.", "Сначала добавьте хотя бы один сервер по SSH."), u.win)
		return
	}
	title := L("Via the nearest entry", "Через ближайший вход")
	if shareMode {
		title = L("Share a cascade", "Поделиться каскадом")
	}
	w := u.newSubPage(title)
	names := make([]string, len(servers))
	for i, p := range servers {
		names[i] = p.Name
	}
	exitName := names[len(names)-1]
	picked := map[string]bool{}
	exitAsEntry := true
	auto := false
	hopsChoice := hopsAll()
	building := false
	cascadeRole := "user"
	clientName := widget.NewEntry()
	clientName.SetText(L("This device", "Это устройство"))
	if shareMode {
		clientName.SetText(L("New client", "Новый клиент"))
	}
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = fyne.TextWrapWord
	say := func(s string) { logLbl.SetText(strings.TrimLeft(logLbl.Text+"\n"+s, "\n")) }

	entriesBox := kit.NewVBoxFit()
	renderEntries := func() {
		entriesBox.RemoveAll()
		for _, p := range servers {
			p := p
			if p.Name == exitName {
				continue
			}
			row, _ := kit.SwitchRowFit(p.Name, addresses(p), picked[p.Name], func(on bool) { picked[p.Name] = on })
			entriesBox.Add(row)
		}
		entriesBox.Refresh()
	}
	resetPicked := func() {
		for _, n := range names {
			picked[n] = n != exitName
		}
		renderEntries()
	}
	resetPicked()

	hopOptions := []string{hopsAll()}
	for i := 0; i < len(names)-1; i++ {
		hopOptions = append(hopOptions, strconv.Itoa(i))
	}
	hopsField := u.fieldSelect(L("Intermediate nodes", "Промежуточных узлов"), hopsAll(), hopOptions, func(s string) { hopsChoice = s })
	hopsField.Hide()
	exitField := u.fieldSelect(L("Exit", "Выход"), exitName, names, func(s string) {
		exitName = s
		resetPicked()
	})
	modeField := u.fieldSelect(L("Path from entry to exit", "Путь от входа до выхода"), routeDirect(), []string{routeDirect(), routeAuto()}, func(s string) {
		auto = s == routeAuto()
		if auto {
			hopsField.Show()
		} else {
			hopsField.Hide()
		}
	})
	exitRow, _ := kit.SwitchRowFit(L("The exit can also be an entry", "Выход тоже может быть входом"), "", true, func(on bool) { exitAsEntry = on })

	btnLabel, btnIcon := L("Create connection", "Создать подключение"), "check"
	if shareMode {
		btnLabel, btnIcon = L("Share the cascade", "Поделиться каскадом"), "share"
	}
	buildBtn := bigButton(btnLabel, btnIcon, true, func() {
		if building {
			return
		}
		exit := findProfile(servers, exitName)
		if exit == nil {
			kit.ShowError(errors.New(L("choose an exit server", "выберите выходной сервер")), w)
			return
		}
		var entries []*config.Profile
		for _, p := range servers {
			if p.Name != exitName && picked[p.Name] {
				entries = append(entries, p)
			}
		}
		if exitAsEntry {
			entries = append(entries, exit)
		}
		if len(entries) == 0 {
			kit.ShowError(errors.New(L("mark at least one entry", "отметьте хотя бы один вход")), w)
			return
		}
		maxHops := server.AllHops
		if hopsChoice != hopsAll() {
			maxHops, _ = strconv.Atoi(hopsChoice)
		}
		job := multiJob{u: u, w: w, say: say, exit: exit, entries: entries, auto: auto, maxHops: maxHops, name: strings.TrimSpace(clientName.Text), role: cascadeRole}
		building = true
		logLbl.SetText("")
		job.run(func(p *config.Profile, err error) {
			building = false
			if err != nil {
				say(L("Error: ", "Ошибка: ") + err.Error())
				return
			}
			say(fmt.Sprintf(L("Done: %d entries, exit “%s”", "Готово: входов %d, выход «%s»"), len(p.Entries), exit.Name))
			link, err := config.EncodeLink(p)
			if err != nil {
				say(L("Error: ", "Ошибка: ") + err.Error())
				return
			}
			noteTxt := fmt.Sprintf(L("On every connection the client will pick the nearest entry itself; the exit is always “%s”.", "При каждом подключении клиент сам выберет ближайший вход; выход всегда «%s»."), exit.Name)
			resp := server.LinkResponse{Name: p.Name, Link: link, File: config.SafeFileName(p.Name) + config.FileExt}
			if shareMode {
				u.showSharedCascade(w, resp, job)
				return
			}
			u.showShare(w, L("Connection ready", "Подключение готово"), resp, noteTxt, true)
		})
	})
	content := kit.NewVBoxFit()
	content.Add(note(L("The exit is the server through which traffic goes to the internet; it is set explicitly. Entries are the servers a client may connect to: the one with the lowest latency is chosen on every connection.", "Выход — сервер, через который трафик уходит в интернет, задаётся явно. Входы — серверы, к которым может подключаться клиент: при каждом подключении выбирается ближайший по задержке.")))
	content.Add(fieldEntry(L("Client name", "Имя клиента"), clientName))
	if shareMode {
		content.Add(roleBlock(cascadeRole, func(v string) { cascadeRole = v }))
	}
	content.Add(exitField)
	content.Add(kit.SectionLabel(L("Entries", "Входы")))
	content.Add(entriesBox)
	content.Add(exitRow)
	content.Add(kit.SectionLabel(L("Path from entry to exit", "Путь от входа до выхода")))
	content.Add(modeField)
	content.Add(hopsField)
	content.Add(buildBtn)
	content.Add(logLbl)
	w.SetContent(container.NewPadded(container.NewVScroll(content)))
	w.Resize(fyne.NewSize(winWidth+80, 720))
	w.Show()
}

func findProfile(v []*config.Profile, name string) *config.Profile {
	for _, p := range v {
		if p.Name == name {
			return p
		}
	}
	return nil
}

type multiJob struct {
	u       *ui
	w       fyne.Window
	say     func(string)
	exit    *config.Profile
	entries []*config.Profile
	auto    bool
	maxHops int
	name    string
	role    string // роль выдаваемого клиента: user или admin
}

func (j multiJob) pool() []*config.Profile {
	pool := slices.Clone(j.entries)
	if !slices.Contains(pool, j.exit) {
		pool = append(pool, j.exit)
	}
	return pool
}

func (j multiJob) links() [][2]*config.Profile {
	var out [][2]*config.Profile
	if !j.auto {
		for _, e := range j.entries {
			if e != j.exit {
				out = append(out, [2]*config.Profile{e, j.exit})
			}
		}
		return out
	}
	pool := j.pool()
	for _, a := range pool {
		if a == j.exit {
			continue
		}
		for _, b := range pool {
			if a != b {
				out = append(out, [2]*config.Profile{a, b})
			}
		}
	}
	return out
}

func (j multiJob) run(done func(*config.Profile, error)) {
	if j.name == "" || strings.ContainsAny(j.name, "/#") {
		done(nil, errors.New(L("specify a client name without “/” and “#”", "укажите имя клиента без «/» и «#»")))
		return
	}
	pairs := j.links()
	var step func(i int)
	step = func(i int) {
		if i < len(pairs) {
			j.say(fmt.Sprintf(L("Link %d/%d: “%s” → “%s”…", "Связь %d/%d: «%s» → «%s»…"), i+1, len(pairs), pairs[i][0].Name, pairs[i][1].Name))
			j.u.linkPair(j.w, pairs[i][0], pairs[i][1], func(err error) {
				if err != nil {
					done(nil, err)
					return
				}
				step(i + 1)
			})
			return
		}
		j.register(done)
	}
	step(0)
}

func (j multiJob) register(done func(*config.Profile, error)) {
	priv, pub, err := wgkey.Generate()
	if err != nil {
		done(nil, err)
		return
	}
	result := &config.Profile{Name: j.name, PrivateKey: priv, ExitName: j.exit.Name, DNS: append([]string(nil), provision.DefaultDNS...)}
	var next func(i int)
	next = func(i int) {
		if i == len(j.entries) {
			if err := result.Validate(); err != nil {
				done(nil, err)
				return
			}
			done(result, nil)
			return
		}
		entry := j.entries[i]
		j.say(fmt.Sprintf(L("Entry “%s”: registering the client…", "Вход «%s»: регистрируем клиента…"), entry.Name))
		j.u.withSession(j.w, entry, func(s *deploy.Session, err error) {
			if err != nil {
				done(nil, fmt.Errorf("%s: %w", entry.Name, err))
				return
			}
			go func() {
				prof, err := registerOn(s, j, entry, pub)
				fyne.Do(func() {
					if err != nil {
						done(nil, fmt.Errorf("%s: %w", entry.Name, err))
						return
					}
					if result.Address4 != "" && result.Address4 != prof.Address4 {
						done(nil, fmt.Errorf(L("%s issued a different client address — the exit must be shared", "%s выдал другой адрес клиента — выход должен быть общим"), entry.Name))
						return
					}
					result.ServerPublicKey, result.Address4, result.Address6, result.MTU = prof.ServerPublicKey, prof.Address4, prof.Address6, prof.MTU
					result.Role = prof.Role
					if result.AdminURL == "" {
						result.AdminURL = prof.AdminURL
					}
					result.Entries = append(result.Entries, config.Entry{
						Name: entry.Name, IPv4: prof.IPv4, IPv6: prof.IPv6, Port: prof.Port, ServerName: prof.ServerName,
						Path: prof.Path, CertPin: prof.CertPin, AuthKey: prof.AuthKey, Via: prof.Via,
						FrontSNI: prof.FrontSNI, FrontKey: prof.FrontKey,
					})
					if prof.Via != "" {
						j.say(L("   route: ", "   маршрут: ") + prof.Via)
					}
					next(i + 1)
				})
			}()
		})
	}
	next(0)
}

func registerOn(s *deploy.Session, j multiJob, entry *config.Profile, pub string) (*config.Profile, error) {
	body := map[string]any{"name": j.name, "public_key": pub, "role": normRole(j.role)}
	switch {
	case entry == j.exit:
	case j.auto:
		body["auto"], body["exit"], body["max_hops"] = true, nodeName(j.exit), j.maxHops
	default:
		body["route"] = []string{nodeName(j.exit)}
	}
	var resp struct {
		Profile *config.Profile `json:"profile"`
	}
	if err := s.Call("POST", "/clients", body, &resp); err != nil {
		return nil, err
	}
	if resp.Profile == nil {
		return nil, errors.New(L("the server did not return a profile — update the server part", "сервер не вернул профиль — обновите серверную часть"))
	}
	return resp.Profile, nil
}

func (u *ui) recheckEntries(p *config.Profile) {
	prog := kit.NewCustomNoButtons(L("Checking entries…", "Проверяем входы…"), kit.NewThinBar(), u.win)
	prog.Show()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ranked := dialer.ProbeEntries(ctx, p, nil)
		fyne.Do(func() {
			prog.Hide()
			rows := container.NewVBox()
			for i, r := range ranked {
				line := fmt.Sprintf(L("%s — %.1f ms", "%s — %.1f мс"), r.Name, float64(r.RTT.Microseconds())/1000)
				col := kit.ColText
				if !r.OK() {
					line, col = r.Name+" — "+r.Err, kit.ColBad
				} else if i == 0 {
					line += L("  · nearest", "  · ближайший")
					col = kit.ColAccent
				}
				rows.Add(kit.Text(line, 14, col, i == 0 && r.OK()))
				if r.Via != "" {
					rows.Add(kit.Text("   "+r.Via, 11, kit.ColMuted, false))
				}
			}
			rows.Add(noteIcon("warn", L("Unavailable entries are skipped: the connection switches to the next one when an entry stops responding.", "Недоступные входы пропускаются: если вход перестал отвечать, подключение переключается на следующий."), kit.ColAccentInk))
			if len(ranked) == 0 || !ranked[0].OK() {
				kit.ShowCustom(L("Entries", "Входы"), L("Close", "Закрыть"), rows, u.win)
				return
			}
			p.PreferredEntry = ranked[0].Name
			if err := saveProfile(p); err != nil {
				kit.ShowError(err, u.win)
				return
			}
			note := fmt.Sprintf(L("The nearest entry “%s” has been saved.", "Ближайший вход «%s» сохранён."), ranked[0].Name)
			if u.handle != nil && u.active == p.Name {
				_ = u.handle.Reselect()
				note += L(" Connection is switching to it.", " Подключение переключается на него.")
			}
			rows.Add(widget.NewSeparator())
			l := widget.NewLabel(note)
			l.Wrapping = fyne.TextWrapWord
			rows.Add(l)
			kit.ShowCustom(L("Entries", "Входы"), L("Done", "Готово"), container.NewGridWrap(fyne.NewSize(340, 300), container.NewVScroll(rows)), u.win)
			u.refreshHome()
		})
	}()
}
