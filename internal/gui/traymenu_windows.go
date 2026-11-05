//go:build windows

package gui

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// setTrayMenuDark включает у нативных меню Windows (меню значка в трее) тёмное или светлое оформление по теме приложения.
// Меню рисует сама ОС и по умолчанию берёт тему Windows, поэтому при тёмной теме Veil и светлой теме системы оно было белым.
// Используются недокументированные функции uxtheme.dll по номерам (так делают Проводник и Chromium): 135 — SetPreferredAppMode
// (2 — всегда тёмное, 3 — всегда светлое), 136 — FlushMenuThemes. Если функций нет (Windows старше 10 1903), ничего не делаем.
func setTrayMenuDark(dark bool) {
	dll := windows.NewLazySystemDLL("uxtheme.dll")
	if dll.Load() != nil {
		return
	}
	h := windows.Handle(dll.Handle())
	setMode, err := windows.GetProcAddressByOrdinal(h, 135)
	if err != nil {
		return
	}
	mode := uintptr(3)
	if dark {
		mode = 2
	}
	syscall.SyscallN(setMode, mode)
	if flush, err := windows.GetProcAddressByOrdinal(h, 136); err == nil {
		syscall.SyscallN(flush)
	}
}
