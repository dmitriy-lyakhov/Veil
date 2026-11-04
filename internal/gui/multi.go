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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/provision"
	"veil/internal/server"
	"veil/internal/wgkey"
)

func routeDirect() string { return L("Directly to the exit", "Напрямую к выходу") }
func routeAuto() string {
	return L("Via nodes, automatically by latency", "Через узлы, автоматически по задержкам")
}
func hopsAll() string { return L("all available", "все доступные") }

func (u *ui) managedProfiles() []*config.Profile {
	var out []*config.Profile
	for _, p := range u.profiles {
		if managed(p) {
			out = append(out, p)
		}
	}
	return out
}

func (u *ui) openMultiEntryBuilder() {
	servers := u.managedProfiles()
	if len(servers) == 0 {
		showInfo(L("No servers of your own", "Нет своих серверов"), L("First add at least one server over SSH.", "Сначала добавьте хотя бы один сервер по SSH."), u.win)
		return
	}
	w := u.newSubPage(L("Via the nearest entry", "Через ближайший вход"))
	names := make([]string, len(servers))
	for i, p := range servers {
		names[i] = p.Name
	}
	exitSel := widget.NewSelect(names, nil)
	entryChecks := widget.NewCheckGroup(nil, nil)
	exitAsEntry := widget.NewCheck(L("The exit can also be an entry", "Выход тоже может быть входом"), nil)
	exitAsEntry.SetChecked(true)
	mode := widget.NewRadioGroup([]string{routeDirect(), routeAuto()}, nil)
	mode.SetSelected(routeDirect())
	hops := widget.NewSelect([]string{hopsAll()}, nil)
	hops.SetSelected(hopsAll())
	hops.Disable()
	mode.OnChanged = func(v string) {
		if v == routeAuto() {
			hops.Enable()
		} else {
			hops.Disable()
		}
	}
	exitSel.OnChanged = func(exit string) {
		var opts []string
		for _, n := range names {
			if n != exit {
				opts = append(opts, n)
			}
		}
		entryChecks.Options = opts
		entryChecks.SetSelected(opts)
		hopOpts := []string{hopsAll()}
		for i := 0; i < len(names)-1; i++ {
			hopOpts = append(hopOpts, strconv.Itoa(i))
		}
		hops.Options = hopOpts
		hops.SetSelected(hopsAll())
		entryChecks.Refresh()
	}
	exitSel.SetSelected(names[len(names)-1])
	clientName := widget.NewEntry()
	clientName.SetText(L("This device", "Это устройство"))
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = fyne.TextWrapWord
	say := func(s string) { logLbl.SetText(strings.TrimLeft(logLbl.Text+"\n"+s, "\n")) }

	var build *widget.Button
	build = widget.NewButtonWithIcon(L("Create connection", "Создать подключение"), theme.ConfirmIcon(), func() {
		exit := findProfile(servers, exitSel.Selected)
		if exit == nil {
			showError(errors.New(L("choose an exit server", "выберите выходной сервер")), w)
			return
		}
		var entries []*config.Profile
		for _, n := range entryChecks.Selected {
			if p := findProfile(servers, n); p != nil {
				entries = append(entries, p)
			}
		}
		if exitAsEntry.Checked {
			entries = append(entries, exit)
		}
		if len(entries) == 0 {
			showError(errors.New(L("mark at least one entry", "отметьте хотя бы один вход")), w)
			return
		}
		maxHops := server.AllHops
		if hops.Selected != hopsAll() {
			maxHops, _ = strconv.Atoi(hops.Selected)
		}
		job := multiJob{u: u, w: w, say: say, exit: exit, entries: entries, auto: mode.Selected == routeAuto(), maxHops: maxHops, name: strings.TrimSpace(clientName.Text)}
		build.Disable()
		logLbl.SetText("")
		job.run(func(p *config.Profile, err error) {
			build.Enable()
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
			u.showShare(w, L("Connection ready", "Подключение готово"), server.LinkResponse{Name: p.Name, Link: link, File: config.SafeFileName(p.Name) + config.FileExt},
				fmt.Sprintf(L("On every connection the client will pick the nearest entry itself; the exit is always “%s”.", "При каждом подключении клиент сам выберет ближайший вход; выход всегда «%s»."), exit.Name), true)
		})
	})
	build.Importance = widget.HighImportance
	hint := widget.NewLabel(L("The exit is the server through which traffic goes to the internet; it is set explicitly. Entries are the servers a client may connect to: the one with the lowest latency is chosen on every connection.", "Выход — сервер, через который трафик уходит в интернет, задаётся явно. Входы — серверы, к которым может подключаться клиент: при каждом подключении выбирается ближайший по задержке."))
	hint.Wrapping = fyne.TextWrapWord
	form := widget.NewForm(
		widget.NewFormItem(L("Client name", "Имя клиента"), clientName),
		widget.NewFormItem(L("Exit", "Выход"), exitSel),
	)
	content := container.NewVBox(hint, form,
		text(L("Entries", "Входы"), 14, colText, true), entryChecks, exitAsEntry,
		text(L("Path from entry to exit", "Путь от входа до выхода"), 14, colText, true), mode,
		widget.NewForm(widget.NewFormItem(L("Intermediate nodes", "Промежуточных узлов"), hops)),
		build, logLbl)
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
	body := map[string]any{"name": j.name, "public_key": pub}
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
	prog := newCustomNoButtons(L("Checking entries…", "Проверяем входы…"), newThinBar(), u.win)
	prog.Show()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ranked := client.ProbeEntries(ctx, p, nil)
		fyne.Do(func() {
			prog.Hide()
			rows := container.NewVBox()
			for i, r := range ranked {
				line := fmt.Sprintf(L("%s — %.1f ms", "%s — %.1f мс"), r.Name, float64(r.RTT.Microseconds())/1000)
				col := colText
				if !r.OK() {
					line, col = r.Name+" — "+r.Err, colBad
				} else if i == 0 {
					line += L("  · nearest", "  · ближайший")
					col = colAccent
				}
				rows.Add(text(line, 14, col, i == 0 && r.OK()))
				if r.Via != "" {
					rows.Add(text("   "+r.Via, 11, colMuted, false))
				}
			}
			if len(ranked) == 0 || !ranked[0].OK() {
				showCustom(L("Entries", "Входы"), L("Close", "Закрыть"), rows, u.win)
				return
			}
			p.PreferredEntry = ranked[0].Name
			if err := saveProfile(p); err != nil {
				showError(err, u.win)
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
			showCustom(L("Entries", "Входы"), L("Done", "Готово"), container.NewGridWrap(fyne.NewSize(340, 300), container.NewVScroll(rows)), u.win)
			u.refreshHome()
		})
	}()
}
