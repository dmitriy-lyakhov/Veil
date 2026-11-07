//go:build uidemo

package gui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"

	"veil/internal/app/gui/kit"
	"veil/internal/app/helper"
	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/routing"
	"veil/internal/server"
)

// Сборка с -tags uidemo: после запуска открывает страницу из VEIL_DEMO (для снимков экрана).
func init() {
	demoHook = func(u *ui) {
		page := os.Getenv("VEIL_DEMO")
		time.AfterFunc(1500*time.Millisecond, func() {
			fyne.Do(func() {
				demoAppearance(u)
				switch page {
				case "connected":
					u.power.SetState(kit.PowerOn)
					u.power.SetTimer("00:00:12")
					u.setStatus("Подключено", "HTTP/3 · QUIC · IPv6 · 00:00:12", "")
					u.traffic.Text = "↓ 11,2 МБ     ↑ 1,5 МБ"
					u.traffic.Refresh()
					u.stats[0].set("11.2", "▼ входящий")
					u.stats[1].set("1.5", "▲ исходящий")
					u.stats[2].set("38", "▲ стабильно")
					u.stats[3].set("00:00:12", "")
				case "traffic":
					u.connected = true
					u.power.SetState(kit.PowerOn)
					u.tr.store = loadTrafficStore()
					u.tr.store.Day, u.tr.store.DayRx, u.tr.store.DayTx = time.Now().Format("2006-01-02"), 1100<<20, 130<<20
					u.tr.store.Month, u.tr.store.MonthRx, u.tr.store.MonthTx = time.Now().Format("2006-01"), 34<<30, 4<<30
					for i := 0; i < 60; i++ {
						u.tr.push(float64(500000 + (i*37%11)*400000))
					}
					u.tr.rxRate, u.tr.txRate, u.tr.latency, u.tr.rx, u.tr.tx, u.tr.session = 4.2*(1<<20), 0.6*(1<<20), 41, 186<<20, 24<<20, "00:14:32"
					u.openTraffic()
				case "share":
					u.showPage(1)
				case "users":
					u.showPage(1)
					u.share.tab = 1
					u.share.render()
				case "server", "server2", "server3", "server4":
					for _, p := range u.profiles {
						if managed(p) {
							u.openManager(p)
							break
						}
					}
				case "longtitle":
					u.openMultiEntryBuilder()
				case "bench":
					steps := []func(){
						func() { u.showPage(1) }, func() { u.showPage(0) }, func() { u.showPage(2) }, u.openConnection,
						func() { u.openDNS() }, u.openSplitSheet, u.openServerSheet, func() { u.showPage(0) },
					}
					for i, f := range steps {
						f := f
						time.AfterFunc(time.Duration(i+1)*1300*time.Millisecond, func() { fyne.Do(f) })
					}
				case "wginstall":
					for _, p := range u.profiles {
						if managed(p) {
							u.openWGInstall(&manager{u: u, p: p, w: u.win, status: kit.NewStatusLine(), body: kit.NewVBoxFit()})
							break
						}
					}
				case "catalog":
					u.openCatalog(&routing.List{}, func() {})
				case "settings":
					u.showPage(2)
				case "add":
					u.showPage(3)
				case "servers":
					u.openServersPage()
				case "connection":
					u.openConnection()
				case "dns":
					u.openDNS()
				case "split":
					u.openSplit()
				case "splitapps":
					u.openSplitApps()
				case "splitsheet":
					u.openSplitSheet()
				case "details":
					if p := u.current(); p != nil {
						u.showDetails(p)
					}
				case "serversheet":
					u.openServerSheet()
				case "killswitch":
					u.openKillSwitch()
				case "application":
					u.openApplication()
				case "logging":
					u.openLogging()
				case "backup":
					u.openBackup()
				case "about":
					u.openAbout()
				case "iface", "ifacebad":
					if page == "ifacebad" {
						u.st.Interface = "Ethernet 2"
					}
					u.openInterface()
				case "coversite", "coversitebusy", "coversiteerr":
					p := demoServer()
					m := &manager{u: u, p: p, w: u.win, status: kit.NewStatusLine(), body: kit.NewVBoxFit()}
					now := time.Now().Add(-14 * time.Minute)
					m.state.SiteRepo = server.SiteRepoInfo{Enabled: true, URL: "https://gitlab.admortgage.com/sites/cover.git", Ref: "main", Dir: "public", HasToken: true, Active: true, Updated: &now, Files: 42, Bytes: 3 << 20}
					if page == "coversitebusy" {
						m.state.SiteRepo.Syncing = true
					}
					if page == "coversiteerr" {
						m.state.SiteRepo.Error = "ошибка аутентификации. Проверьте токен."
					}
					m.openCoverSite()
				case "access":
					u.profiles = append(u.profiles, demoServer())
					u.showPage(1)
					u.share.refreshServers()
				case "clientnew":
					m := demoManagerUI(u, demoServer())
					m.addClient()
				case "clientdetail":
					m := demoManagerUI(u, demoServer())
					m.openClient(m.state.Clients[0].Name)
				case "shareresult":
					u.showShare(u.win, "Клиент «Телефон Ани»", server.LinkResponse{Name: "Телефон Ани", Link: "veil://eyJob3N0IjoiMjAzLjAuMTEzLjEwIn0"}, L("Scan the QR in the Veil app on the client device, or send the link or file.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте ссылку или файл."), true, infoRow(L("Role", "Роль"), roleName("user"), nil))
				case "admindetails":
					p := demoServer()
					p.Role = "admin"
					p.AdminURL = "https://10.66.0.1/admin"
					u.profiles = append(u.profiles, p)
					u.showDetails(p)
				case "admindetailsfail":
					p := demoServer()
					p.Role = "admin"
					p.AdminURL = "https://10.66.0.1/admin"
					u.profiles = append(u.profiles, p)
					u.active, u.connected, u.adminHosts = p.Name, true, client.AdminHostsFailed
					u.showDetails(p)
				case "sharedcascade":
					a, b := demoServer(), demoServer()
					a.Name, b.Name = "Амстердам", "Франкфурт"
					job := multiJob{u: u, exit: b, entries: []*config.Profile{a, b}}
					u.showSharedCascade(u.win, server.LinkResponse{Name: "Каскад команды", Link: "veil://eyJjYXNj"}, job)
				case "switching":
					p := demoServer()
					p.Entries = []config.Entry{{Name: "Амстердам"}, {Name: "Токио"}}
					u.profiles = append(u.profiles, p)
					u.active = p.Name
					u.power.SetState(kit.PowerBusy)
					u.showReport(&helper.Report{Status: client.Status{State: client.StateReconnecting, Entry: "Амстердам"}})
				}
			})
		})
	}
}

// demoAppearance: VEIL_SIZE=393x760, VEIL_THEME=light, VEIL_LANG=en.
func demoAppearance(u *ui) {
	if s := os.Getenv("VEIL_SIZE"); s != "" {
		var w, h float32
		if _, err := fmt.Sscanf(s, "%fx%f", &w, &h); err == nil {
			u.win.Resize(fyne.NewSize(w, h))
		}
	}
	changed := false
	if t := os.Getenv("VEIL_THEME"); t != "" {
		u.st.Theme, changed = t, true
	}
	if l := os.Getenv("VEIL_LANG"); l != "" {
		u.st.Lang, changed = l, true
	}
	if changed {
		u.rebuildUI()
	}
}

func demoServer() *config.Profile {
	return &config.Profile{Name: "Амстердам", IPv4: "203.0.113.10", SSH: &config.SSHInfo{Port: 22, User: "root"}}
}

// demoManager — менеджер сервера с демо-состоянием и подменой запросов (без SSH).
func demoManagerUI(u *ui, p *config.Profile) *manager {
	m := &manager{u: u, p: p, w: u.win, status: kit.NewStatusLine(), body: kit.NewVBoxFit(), loaded: true}
	m.callHook = func(method, path string, in, out any, done func(error)) {
		if path == "/explore" {
			if r, ok := out.(*struct {
				Upstreams []string `json:"upstreams"`
			}); ok {
				r.Upstreams = []string{"Токио", "Франкфурт"}
			}
		}
		done(nil)
	}
	m.state = server.State{Clients: []server.ClientInfo{
		{Name: "Телефон Ани", Note: "Android", TxBytes: 1200 << 20, RxBytes: 340 << 20, Route: []string{"Амстердам", "Токио", "Франкфурт"}, Auto: &server.AutoInfo{CostMs: 142}},
		{Name: "Ноутбук", TxBytes: 4800 << 20, RxBytes: 900 << 20},
		{Name: "Роутер", Disabled: true},
	}}
	m.roles = map[string]string{"Ноутбук": "admin"}
	return m
}
