package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/server"
)

// agoText — «14 минут назад» для статуса обновления сайта.
func agoText(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return L("just now", "только что")
	case d < time.Hour:
		return fmt.Sprintf(L("%d min ago", "%d мин назад"), int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf(L("%d h ago", "%d ч назад"), int(d.Hours()))
	}
	return t.Format("2006-01-02")
}

// openCoverSite — экран «Сайт-прикрытие»: сайт можно брать из Git-репозитория вместо сгенерированного.
// Настройка хранится на сервере (POST settings/siterepo), токен обратно не показывается.
func (m *manager) openCoverSite() {
	w := m.u.newSubPage(L("Cover site", "Сайт-прикрытие"))
	info := m.state.SiteRepo
	repo, ref, dir, token := widget.NewEntry(), widget.NewEntry(), widget.NewEntry(), widget.NewPasswordEntry()
	repo.SetText(info.URL)
	repo.SetPlaceHolder("https://git.example.com/sites/cover.git")
	ref.SetText(info.Ref)
	ref.SetPlaceHolder("main")
	dir.SetText(info.Dir)
	dir.SetPlaceHolder("public")
	if info.HasToken {
		token.SetPlaceHolder("••••••••••••")
	}
	status := container.NewVBox()
	var render func(server.SiteRepoInfo)
	var poll func()
	var reset fyne.CanvasObject
	polling := false
	render = func(in server.SiteRepoInfo) {
		info = in
		status.RemoveAll()
		switch {
		case in.Syncing:
			status.Add(container.NewVBox(kit.Text(L("Loading the repository…", "Загрузка репозитория…"), 14, kit.ColText, true), kit.NewThinBar()))
		case !in.Enabled:
			status.Add(infoRow(L("Status", "Статус"), L("Generated site", "Сгенерированный сайт"), nil))
		case in.Error != "":
			status.Add(infoRow(L("Status", "Статус"), L("Error", "Ошибка"), nil))
		case in.Updated != nil:
			status.Add(infoRow(L("Status", "Статус"), L("Updated ", "Обновлён ")+agoText(*in.Updated), nil))
		default:
			status.Add(infoRow(L("Status", "Статус"), L("Not loaded yet", "Ещё не загружен"), nil))
		}
		if in.Error != "" && !in.Syncing {
			status.Add(noteIcon("warn", L("Could not load the repository: ", "Не удалось загрузить репозиторий: ")+in.Error, kit.ColAccentInk))
		}
		if in.Enabled && !in.Syncing && in.Error == "" && in.Files > 0 {
			status.Add(note(fmt.Sprintf(L("Files: %d, %s", "Файлов: %d, %s"), in.Files, humanBytes(uint64(in.Bytes)))))
		}
		status.Refresh()
		if reset != nil {
			if in.Enabled {
				reset.Show()
			} else {
				reset.Hide()
			}
		}
	}
	// пока сервер грузит репозиторий, раз в 2 секунды спрашиваем состояние (не дольше двух минут)
	poll = func() {
		call := m.transport()
		if polling || call == nil {
			return
		}
		polling = true
		go func() {
			defer func() { polling = false }()
			siteRepoPoller{
				fetch: func() (server.SiteRepoInfo, error) {
					var st stateRoles
					err := call("GET", "/state", nil, &st)
					if err == nil {
						fyne.Do(func() { m.state = st.State })
					}
					return st.SiteRepo, err
				},
				sleep:    time.Sleep,
				interval: 2 * time.Second,
				limit:    2 * time.Minute,
				onState:  func(s server.SiteRepoInfo) { fyne.Do(func() { render(s) }) },
			}.run()
		}()
	}
	apply := func(req map[string]any, refresh bool) {
		path := "/settings/siterepo"
		if refresh {
			path, req = "/settings/siterepo/refresh", nil
		}
		var out server.SiteRepoInfo
		m.call("POST", path, req, &out, func(err error) {
			if err != nil {
				kit.ShowError(err, m.w)
				return
			}
			render(out)
			if out.Syncing {
				poll()
			}
		})
	}
	update := bigButton(L("Update now", "Обновить"), "refresh", true, func() {
		url := repo.Text
		if url == "" {
			kit.ShowError(fmt.Errorf("%s", L("enter the repository address", "укажите адрес репозитория")), w)
			return
		}
		unchanged := info.Enabled && url == info.URL && ref.Text == info.Ref && dir.Text == info.Dir && token.Text == ""
		if unchanged {
			apply(nil, true)
			return
		}
		apply(map[string]any{"url": url, "ref": ref.Text, "dir": dir.Text, "token": token.Text}, false)
		token.SetText("")
	})
	reset = dangerButton(L("Reset to the generated site", "Сбросить на сгенерированный сайт"), func() {
		kit.ShowConfirm(L("Reset the site", "Сбросить сайт"), L("The generated cover site will be served again; the repository settings and token will be deleted from the server. Continue?", "Снова будет раздаваться сгенерированный сайт-прикрытие, настройки репозитория и токен удалятся с сервера. Продолжить?"), func(ok bool) {
			if !ok {
				return
			}
			apply(map[string]any{"url": "", "clear_token": true}, false)
			repo.SetText("")
			ref.SetText("")
			dir.SetText("")
			token.SetText("")
			token.SetPlaceHolder("")
		}, w)
	})
	body := kit.NewVBoxFit()
	body.Add(kit.Prose(L("The site that outsiders see on this server. It can be taken from a Git repository instead of the generated one.", "Сайт, который видят посторонние на этом сервере. Его можно брать из Git-репозитория вместо сгенерированного.")))
	body.Add(status)
	body.Add(fieldEntry(L("Repository", "Адрес репозитория"), repo))
	body.Add(fieldEntry(L("Branch", "Ветка"), ref))
	body.Add(fieldEntry(L("Subdirectory", "Подкаталог"), dir))
	body.Add(fieldEntry(L("Access token", "Токен доступа"), token))
	body.Add(update)
	body.Add(reset)
	body.Add(noteIcon("lock", L("The site is rebuilt on the server. The token is stored on the server and is not shown again.", "Сайт собирается на сервере. Токен хранится на сервере и повторно не показывается."), kit.ColMuted))
	render(info)
	if info.Syncing {
		poll()
	}
	w.SetContent(container.NewPadded(container.NewVScroll(body)))
	w.Show()
}

// loadCoverSite подгружает свежее состояние сервера и открывает экран «Сайт-прикрытие».
func (m *manager) loadCoverSite() {
	var st stateRoles
	m.call("GET", "/state", nil, &st, func(err error) {
		if err != nil {
			kit.ShowError(err, m.w)
			return
		}
		m.state, m.roles, m.loaded = st.State, st.roles, true
		m.openCoverSite()
	})
}
