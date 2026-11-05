//go:build !windows

package gui

// setTrayMenuDark нужен только на Windows: там меню трея рисует ОС. На других системах меню оформляет сама платформа.
func setTrayMenuDark(bool) {}
