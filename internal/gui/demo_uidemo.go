//go:build uidemo

package gui

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"

	"veil/internal/routing"
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
					u.power.SetState(powerOn)
					u.power.SetTimer("00:00:12")
					u.setStatus("Подключено", "HTTP/3 · QUIC · IPv6 · 00:00:12", "")
					u.traffic.Text = "↓ 11,2 МБ     ↑ 1,5 МБ"
					u.traffic.Refresh()
					u.stats[0].set("11.2", "▼ входящий")
					u.stats[1].set("1.5", "▲ исходящий")
					u.stats[2].set("38", "▲ стабильно")
					u.stats[3].set("00:00:12", "")
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
							u.openWGInstall(&manager{u: u, p: p, w: u.win, status: newStatusLine(), body: newVBoxFit()})
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
