//go:build windows

package gui

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW   = user32.NewProc("FindWindowW")
	procShowWindow    = user32.NewProc("ShowWindow")
	procSetForeground = user32.NewProc("SetForegroundWindow")
)

// acquireSingleInstance занимает именованный мьютекс сеанса пользователя. false — приложение
// уже запущено. Если мьютекс создать не удалось, запуск не блокируется.
func acquireSingleInstance() (release func(), ok bool) {
	if os.Getenv("VEIL_MULTI") != "" { // для тестов и снимков экрана
		return func() {}, true
	}
	name, err := windows.UTF16PtrFromString(`Local\Veil-VPN-single-instance`)
	if err != nil {
		return func() {}, true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
			return nil, false
		}
		return func() {}, true
	}
	return func() { _ = windows.CloseHandle(h) }, true
}

// activateExisting показывает окно уже запущенного экземпляра (в том числе спрятанное в трей).
func activateExisting() {
	title, err := windows.UTF16PtrFromString("Veil")
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafePointer(title)))
	if hwnd == 0 {
		return
	}
	const swRestore = 9
	_, _, _ = procShowWindow.Call(hwnd, swRestore)
	_, _, _ = procSetForeground.Call(hwnd)
}
