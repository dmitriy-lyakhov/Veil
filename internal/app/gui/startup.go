package gui

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"veil/internal/app/gui/kit"
	"veil/internal/app/gui/platform"
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

//go:embed tray_on.png
var trayOnPNG []byte

//go:embed tray_off.png
var trayOffPNG []byte

// setupTray создаёт значок в области уведомлений с меню (если платформа это поддерживает).
// Меню по дизайну: строка состояния и сервера, «Открыть Veil», «Подключить» или «Отключить», «Выход».
func (u *ui) setupTray() {
	if _, ok := u.app.(desktop.App); !ok {
		return
	}
	u.trayReady = true
	u.updateTray()
}

// updateTray обновляет меню и значок в трее под текущее состояние подключения.
func (u *ui) updateTray() {
	if !u.trayReady {
		return
	}
	desk, ok := u.app.(desktop.App)
	if !ok {
		return
	}
	on := u.power != nil && u.power.State == kit.PowerOn
	busy := u.power != nil && u.power.State == kit.PowerBusy
	status := L("Disconnected", "Отключено")
	sub := L("Not protected", "Не защищено")
	switch {
	case on:
		status = L("Connected", "Подключено")
		sub = u.srvName.Text()
		if u.tr.latency > 0 {
			sub += fmt.Sprintf(" · %.0f %s", u.tr.latency, L("ms", "мс"))
		}
	case busy:
		status = L("Connecting…", "Подключение…")
		sub = u.srvName.Text()
	}
	head := fyne.NewMenuItem("● "+status+" · "+sub, nil)
	head.Disabled = true
	toggle := L("Connect", "Подключить")
	if on || busy {
		toggle = L("Disconnect", "Отключить")
	}
	menu := fyne.NewMenu("Veil",
		head,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(L("Open Veil", "Открыть Veil"), func() { fyne.Do(u.showWindow) }),
		fyne.NewMenuItem(toggle, func() { fyne.Do(u.toggle) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(L("Quit", "Выход"), func() { fyne.Do(u.quit) }),
	)
	platform.SetTrayMenuDark(u.st.Dark())
	desk.SetSystemTrayMenu(menu)
	icon := fyne.NewStaticResource("tray_off.png", trayOffPNG)
	if on {
		icon = fyne.NewStaticResource("tray_on.png", trayOnPNG)
	}
	desk.SetSystemTrayIcon(icon)
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

// closeWindow — крестик окна оставляет Veil в трее (если трей есть); приложение и туннель останавливает только «Выход».
func (u *ui) closeWindow() {
	if u.trayReady {
		u.win.Hide()
		return
	}
	u.quit()
}
