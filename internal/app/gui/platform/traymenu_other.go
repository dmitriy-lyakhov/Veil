//go:build !windows

package platform

// setTrayMenuDark нужен только на Windows: там меню трея рисует ОС. На других системах меню оформляет сама платформа.
func SetTrayMenuDark(bool) {}
