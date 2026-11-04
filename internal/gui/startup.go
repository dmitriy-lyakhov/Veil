package gui

import (
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func hasArg(name string) bool {
	for _, a := range os.Args[1:] {
		if a == name {
			return true
		}
	}
	return false
}

func (u *ui) showWindow() {
	u.win.Show()
	u.win.RequestFocus()
}

// setupTray создаёт значок в области уведомлений с меню (если платформа это поддерживает).
func (u *ui) setupTray() {
	desk, ok := u.app.(desktop.App)
	if !ok {
		return
	}
	menu := fyne.NewMenu("Veil",
		fyne.NewMenuItem(L("Open Veil", "Открыть Veil"), func() { fyne.Do(u.showWindow) }),
		fyne.NewMenuItem(L("Connect / disconnect", "Подключить / отключить"), func() { fyne.Do(u.toggle) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(L("Quit", "Выход"), func() { fyne.Do(u.quit) }),
	)
	desk.SetSystemTrayMenu(menu)
	if icon := bundledIcon(); icon != nil {
		desk.SetSystemTrayIcon(icon)
	}
}

// afterStart — действия после запуска: свёрнутый старт, автоподключение, проверка обновлений.
// demoHook задаётся только в сборке с тегом uidemo.
var demoHook func(u *ui)

func (u *ui) afterStart() {
	if demoHook != nil {
		demoHook(u)
	}
	u.setupTray()
	if hasArg("--autostart") && u.st.StartMinimized {
		u.app.Lifecycle().SetOnStarted(func() { fyne.Do(u.win.Hide) })
	}
	if u.st.AutoConnect && u.current() != nil {
		time.AfterFunc(1200*time.Millisecond, func() {
			fyne.Do(func() {
				if u.handle == nil {
					u.logf("автоподключение")
					u.toggle()
				}
			})
		})
	}
	if !u.st.NoUpdateCheck {
		time.AfterFunc(4*time.Second, func() { fyne.Do(func() { u.checkUpdates(u.win, false) }) })
	}
}
