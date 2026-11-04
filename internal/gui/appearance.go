package gui

import (
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// openAppearance — страница «Внешний вид»: тема и язык.
func (u *ui) openAppearance() {
	var pg *subPage
	themeOpts := []string{L("Dark", "Тёмная"), L("Light", "Светлая")}
	th := widget.NewRadioGroup(themeOpts, nil)
	if u.st.Dark() {
		th.SetSelected(themeOpts[0])
	} else {
		th.SetSelected(themeOpts[1])
	}
	langOpts := []string{"Русский", "English"}
	lg := widget.NewRadioGroup(langOpts, nil)
	if u.st.Language() == "en" {
		lg.SetSelected(langOpts[1])
	} else {
		lg.SetSelected(langOpts[0])
	}
	apply := func() {
		u.st.Theme, u.st.Lang = "dark", "ru"
		if th.Selected == themeOpts[1] {
			u.st.Theme = "light"
		}
		if lg.Selected == langOpts[1] {
			u.st.Lang = "en"
		}
		u.saveSettings()
		pg.Close()
		u.rebuildUI()
	}
	th.OnChanged = func(string) { apply() }
	lg.OnChanged = func(string) { apply() }
	content := container.NewVBox(
		text(L("Theme", "Тема"), 16, colText, true), th,
		canvas.NewRectangle(colBackground),
		text(L("Language", "Язык"), 16, colText, true), lg,
	)
	pg = u.openPage(L("Appearance", "Внешний вид"), scrolled(content))
}

// rebuildUI пересоздаёт интерфейс после смены темы или языка.
func (u *ui) rebuildUI() {
	applyPalette(u.st.Dark())
	curLang = u.st.Language()
	u.app.Settings().SetTheme(veilTheme{})
	cur := u.curPage
	u.buildPages()
	u.win.SetContent(u.buildShell())
	u.pageShown = false
	u.showPage(cur)
	u.reload()
	u.win.Content().Refresh()
}
