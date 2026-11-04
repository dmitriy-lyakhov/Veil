package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/provision"
	"veil/internal/server"
)

func randomPassword() string { return provision.NewAdminPassword() }

func (u *ui) openChainBuilder(entry *config.Profile) {
	w := u.newSubPage(L("Build a cascade", "Построить каскад"))
	chain := []*config.Profile{entry}
	list := newVBoxFit()
	clientName := widget.NewEntry()
	clientName.SetText(L("This device", "Это устройство"))
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = fyne.TextWrapWord
	building := false

	inChain := func(p *config.Profile) bool {
		for _, c := range chain {
			if c.Name == p.Name {
				return true
			}
		}
		return false
	}
	freeServers := func() []string {
		var opts []string
		for _, p := range u.profiles {
			if managed(p) && !inChain(p) {
				opts = append(opts, p.Name)
			}
		}
		return opts
	}
	redraw := func() {
		list.RemoveAll()
		for i, p := range chain {
			role := L("intermediate", "промежуточный")
			switch {
			case i == 0:
				role = L("entry", "вход")
			case i == len(chain)-1:
				role = L("exit", "выход")
			}
			list.Add(card(container.NewVBox(newFitText(fmt.Sprintf("%d. %s", i+1, p.Name), 15, colText, true), newFitText(role+" · "+addresses(p), 12, colMuted, false))))
		}
		list.Refresh()
	}
	var addBtn fyne.CanvasObject
	addBtn = bigButton(L("Add the next node…", "Добавить следующий узел…"), "plus", false, func() {
		opts := freeServers()
		if len(opts) == 0 {
			showInfo(L("No free servers", "Свободных серверов нет"), L("All your servers are already in the chain.", "Все ваши серверы уже в цепочке."), w)
			return
		}
		u.showMenuAt(addBtn, opts, func(name string) {
			for _, p := range u.profiles {
				if p.Name == name && !inChain(p) {
					chain = append(chain, p)
					redraw()
					return
				}
			}
		})
	})
	removeBtn := bigButton(L("Remove last", "Убрать последний"), "trash", false, func() {
		if len(chain) > 1 {
			chain = chain[:len(chain)-1]
			redraw()
		}
	})
	say := func(s string) { logLbl.SetText(strings.TrimLeft(logLbl.Text+"\n"+s, "\n")) }
	finish := func() { building = false }
	buildBtn := bigButton(L("Build and create config", "Построить и создать конфиг"), "check", true, func() {
		if building {
			return
		}
		if len(chain) < 2 {
			showError(fmt.Errorf(L("add at least one node after “%s”", "добавьте хотя бы один узел после «%s»"), entry.Name), w)
			return
		}
		building = true
		var step func(i int)
		step = func(i int) {
			if i == len(chain)-1 {
				route := make([]string, 0, len(chain)-1)
				for _, p := range chain[1:] {
					route = append(route, nodeName(p))
				}
				say(fmt.Sprintf(L("Creating a client on “%s”…", "Создаём клиента на «%s»…"), entry.Name))
				u.withSession(w, entry, func(s *deploy.Session, err error) {
					if err != nil {
						say(L("Error: ", "Ошибка: ") + err.Error())
						finish()
						return
					}
					go func() {
						var r server.LinkResponse
						err := s.Call("POST", "/clients", map[string]any{"name": clientName.Text, "route": route}, &r)
						fyne.Do(func() {
							finish()
							if err != nil {
								say(L("Error: ", "Ошибка: ") + err.Error())
								return
							}
							say(L("Done: ", "Готово: ") + r.Via)
							u.showShare(w, L("Cascade ready", "Каскад готов"), r, L("The config goes through all nodes of the chain. Add it to this device or hand it to a client.", "Конфиг ведёт через все узлы цепочки. Добавьте его на это устройство или передайте клиенту."), true)
						})
					}()
				})
				return
			}
			say(fmt.Sprintf(L("Linking “%s” → “%s”…", "Связываем «%s» → «%s»…"), chain[i].Name, chain[i+1].Name))
			u.linkPair(w, chain[i], chain[i+1], func(err error) {
				if err != nil {
					say(L("Error: ", "Ошибка: ") + err.Error())
					finish()
					return
				}
				step(i + 1)
			})
		}
		step(0)
	})
	redraw()
	content := newVBoxFit()
	content.Add(note(L("Choose servers in the order traffic passes through them: the first is the entry, the last is the exit to the internet. The number of nodes is unlimited. The app will issue cascade keys and link neighbouring nodes itself.", "Выберите серверы в порядке прохождения трафика: первый — вход, последний — выход в интернет. Число узлов не ограничено. Приложение само выдаст ключи каскада и свяжет соседние узлы.")))
	content.Add(list)
	content.Add(addBtn)
	content.Add(removeBtn)
	content.Add(fieldEntry(L("Client name", "Имя клиента"), clientName))
	content.Add(buildBtn)
	content.Add(logLbl)
	w.SetContent(container.NewPadded(container.NewVScroll(content)))
	w.Resize(fyne.NewSize(winWidth+60, 680))
	w.Show()
}
