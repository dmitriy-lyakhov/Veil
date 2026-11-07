package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/i18n"
	"veil/internal/app/gui/kit"
)

func themeName() string {
	if kit.PaletteDark {
		return L("Dark", "Тёмная")
	}
	return L("Light", "Светлая")
}

func langName() string {
	if i18n.Lang == "en" {
		return "English"
	}
	return "Русский"
}

// navRowVal — строка-переход со значением справа (как «Тема · Тёмная» в дизайне).
func navRowVal(title, value string, onTap func()) fyne.CanvasObject {
	val := kit.Text(value, 13, kit.ColMuted, false)
	return kit.ListRowH(kit.RowText(title, "", theme.ColorNameForeground), container.New(kit.Hrow{Gap: 4}, container.NewCenter(val), container.NewCenter(kit.IconImg("right", kit.ColMuted, 20))), onTap, 56)
}

// openAppearance — страница выбора темы (theme=true) или языка.
func (u *ui) openAppearance(themePage bool) {
	var pg *subPage
	var opts []string
	var sel string
	title := L("Language", "Язык")
	if themePage {
		title = L("Theme", "Тема")
		opts = []string{L("Dark", "Тёмная"), L("Light", "Светлая")}
		sel = opts[0]
		if !u.st.Dark() {
			sel = opts[1]
		}
	} else {
		opts = []string{"Русский", "English"}
		sel = opts[0]
		if u.st.Language() == "en" {
			sel = opts[1]
		}
	}
	rg := widget.NewRadioGroup(opts, nil)
	rg.SetSelected(sel)
	rg.OnChanged = func(v string) {
		if themePage {
			u.st.Theme = "dark"
			if v == opts[1] {
				u.st.Theme = "light"
			}
		} else {
			u.st.Lang = "ru"
			if v == opts[1] {
				u.st.Lang = "en"
			}
		}
		u.saveSettings()
		pg.Close()
		u.rebuildUI()
	}
	pg = u.openPage(title, scrolled(kit.PageBox(rg)))
}

// rebuildUI пересоздаёт интерфейс после смены темы или языка.
func (u *ui) rebuildUI() {
	kit.ApplyPalette(u.st.Dark())
	i18n.Lang = u.st.Language()
	u.app.Settings().SetTheme(kit.VeilTheme{})
	cur := u.curPage
	u.buildPages()
	u.win.SetContent(u.buildShell())
	u.pageShown = false
	u.showPage(cur)
	u.reload()
	u.win.Content().Refresh()
	u.updateTray() // меню в трее: тема и язык
}
