//go:build uidemo

package gui

import (
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
				switch page {
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
