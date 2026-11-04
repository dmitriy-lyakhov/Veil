package gui

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"veil/internal/client"
	"veil/internal/helper"
	"veil/internal/settings"
)

const (
	repoURL      = "https://github.com/dmitriy-lyakhov/Veil"
	releasesAPI  = "https://api.github.com/repos/dmitriy-lyakhov/Veil/releases/latest"
	backupExt    = ".veilbackup"
	maxLogView   = 256 << 10
	pageMaxWidth = 0
)

// ---------- настройки: загрузка, сохранение, журнал приложения ----------

func (u *ui) loadSettings() {
	u.st = &settings.Settings{}
	path, err := settings.Path()
	if err != nil {
		return
	}
	st, err := settings.Load(path)
	if err != nil {
		return
	}
	u.st = st
	if st.LogExpired(time.Now()) {
		u.st.LogEnabled = false
		u.clearLogs()
		_ = u.st.Save(path)
	}
}

func (u *ui) saveSettings() {
	path, err := settings.Path()
	if err == nil {
		err = u.st.Save(path)
	}
	if err != nil {
		showError(err, u.win)
	}
}

func appLogPath() (string, error) {
	dir, err := helper.RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "app.log"), nil
}

// logf пишет в журнал приложения, если запись включена.
func (u *ui) logf(format string, args ...any) {
	if u.st == nil || !u.st.LogEnabled {
		return
	}
	path, err := appLogPath()
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

func (u *ui) logFiles() (tunnel, app string) {
	if dir, err := helper.RunDir(); err == nil {
		tunnel = filepath.Join(dir, "tunnel.log")
		app = filepath.Join(dir, "app.log")
	}
	return
}

func (u *ui) clearLogs() {
	t, a := u.logFiles()
	for _, p := range []string{t, a} {
		if p != "" {
			_ = os.Truncate(p, 0)
		}
	}
}

func fileSize(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

// ---------- общий каркас страниц ----------

func (u *ui) openPage(title string, content fyne.CanvasObject) *subPage {
	w := u.newSubPage(title)
	w.SetContent(container.NewPadded(content))
	w.Show()
	return w
}

func scrolled(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVScroll(content)
}

func note(s string) fyne.CanvasObject { return lead(prose(s)) }

// ---------- вкладка «Настройки» ----------

func (u *ui) settingsPage() fyne.CanvasObject {
	title := text("Настройки", 20, colText, false)
	title.Alignment = fyne.TextAlignCenter
	rows := container.NewVBox(
		navRow("Серверы", "", u.openServersPage),
		navRow("Подключение", "", u.openConnection),
		navRow("Приложение", "", u.openApplication),
		navRow("Резервная копия", "", u.openBackup),
		navRow("О Veil", "", u.openAbout),
		navRow("Закрыть приложение", "", u.quit),
	)
	return container.NewPadded(container.NewBorder(container.NewVBox(container.NewPadded(title)), nil, nil, nil, scrolled(rows)))
}

func (u *ui) openServersPage() {
	u.openPage("Серверы", u.serversView)
}

func (u *ui) cleanup() {
	for name := range u.sessions {
		u.dropSession(name)
	}
	u.disconnect()
}

func (u *ui) quit() {
	u.cleanup()
	u.win.Close()
}

// ---------- подключение ----------

func (u *ui) openConnection() {
	rows := container.NewVBox(
		navRow("DNS-серверы", "Заменяют DNS-серверы из профиля сервера", u.openDNS),
		navRow("Раздельное туннелирование по сайтам", "Позволяет выбрать, какие сайты открывать через VPN, а какие напрямую", u.openSplit),
		navRow("KillSwitch", "Блокирует сетевые соединения, если VPN оборвался", u.openKillSwitch),
	)
	u.openPage("Подключение", scrolled(rows))
}

func (u *ui) defaultDNSHint() (string, string) {
	if p := u.current(); p != nil && len(p.DNS) > 0 {
		a, b := p.DNS[0], ""
		if len(p.DNS) > 1 {
			b = p.DNS[1]
		}
		return a, b
	}
	return "1.1.1.1", "1.0.0.1"
}

func (u *ui) openDNS() {
	d1, d2 := u.defaultDNSHint()
	primary := widget.NewEntry()
	primary.SetPlaceHolder(d1)
	secondary := widget.NewEntry()
	secondary.SetPlaceHolder(d2)
	if len(u.st.DNS) > 0 {
		primary.SetText(u.st.DNS[0])
	}
	if len(u.st.DNS) > 1 {
		secondary.SetText(u.st.DNS[1])
	}
	w := u.newSubPage("DNS-серверы")
	save := widget.NewButton("Сохранить", func() {
		var list []string
		for _, e := range []*widget.Entry{primary, secondary} {
			v := strings.TrimSpace(e.Text)
			if v == "" {
				continue
			}
			if _, err := netip.ParseAddr(v); err != nil {
				showError(fmt.Errorf("«%s» — не IP-адрес", v), w)
				return
			}
			list = append(list, v)
		}
		u.st.DNS = list
		u.saveSettings()
		u.logf("DNS: %v", list)
		w.Close()
	})
	save.Importance = widget.HighImportance
	restore := widget.NewButton("Вернуть по умолчанию", func() {
		primary.SetText("")
		secondary.SetText("")
		u.st.DNS = nil
		u.saveSettings()
	})
	body := container.NewVBox(
		note("Используются, когда сервер не предоставляет свои DNS. Пустые поля — значения из профиля. Применяется при следующем подключении."),
		lead(text("Основной DNS", 12, colMuted, false)), primary,
		lead(text("Запасной DNS", 12, colMuted, false)), secondary,
	)
	w.SetContent(container.NewPadded(container.NewBorder(nil, container.NewVBox(restore, save), nil, nil, scrolled(body))))
	w.Show()
}

func (u *ui) openKillSwitch() {
	w := u.newSubPage("KillSwitch")
	status := prose("")
	unblock := widget.NewButtonWithIcon("Снять блокировку интернета", theme.ConfirmIcon(), nil)
	unblock.Importance = widget.DangerImportance
	refresh := func() {
		if client.Blocked() {
			unblock.Show()
			status.Segments[0].(*widget.TextSegment).Text = "Сейчас действует блокировка: прошлый сеанс завершился аварийно. Подключитесь снова или снимите блокировку."
		} else {
			unblock.Hide()
			status.Segments[0].(*widget.TextSegment).Text = ""
		}
		status.Refresh()
	}
	unblock.OnTapped = func() {
		go func() {
			err := helper.Unblock()
			time.Sleep(2 * time.Second)
			fyne.Do(func() {
				if err != nil {
					showError(err, w)
				}
				refresh()
			})
		}()
	}
	row, _ := switchRow("KillSwitch", "Включите, чтобы трафик не уходил в открытую сеть, если VPN оборвётся", u.st.KillSwitch, func(on bool) {
		u.st.KillSwitch = on
		u.saveSettings()
		u.logf("KillSwitch: %v", on)
	})
	mode := listRow("Мягкий KillSwitch", "Интернет блокируется, если VPN отключился неожиданно. При обычном отключении блокировки нет. Действует со следующего подключения.", widget.NewIcon(theme.RadioButtonCheckedIcon()), nil)
	body := container.NewVBox(row, mode, status, unblock)
	refresh()
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}

// ---------- приложение ----------

func (u *ui) openApplication() {
	var minimized *switchWidget
	autoRow, _ := switchRow("Автозапуск", "Запускать приложение при включении устройства", u.st.AutoStart, func(on bool) {
		if err := setAutostart(on); err != nil {
			showError(err, u.win)
			return
		}
		u.st.AutoStart = on
		if !on {
			u.st.StartMinimized = false
			minimized.SetOn(false)
		}
		minimized.SetDisabled(!on)
		u.saveSettings()
	})
	minRow, mw := switchRow("Запускать свёрнутым", "Запускать приложение в трее (работает при включённом автозапуске)", u.st.StartMinimized, func(on bool) {
		u.st.StartMinimized = on
		u.saveSettings()
	})
	minimized = mw
	minimized.SetDisabled(!u.st.AutoStart)
	connRow, _ := switchRow("Автоподключение", "Подключаться к VPN при запуске приложения", u.st.AutoConnect, func(on bool) {
		u.st.AutoConnect = on
		u.saveSettings()
	})
	updRow, _ := switchRow("Проверять обновления", "Проверять наличие новой версии при запуске", !u.st.NoUpdateCheck, func(on bool) {
		u.st.NoUpdateCheck = !on
		u.saveSettings()
	})
	logState := "Отключено"
	if u.st.LogEnabled {
		logState = "Включено"
	}
	rows := container.NewVBox(
		autoRow, connRow, minRow, updRow,
		navRow("Журналы", logState, u.openLogging),
		listRowColored("Сбросить настройки и удалить все данные приложения", "", theme.ColorNameError, widget.NewIcon(theme.NavigateNextIcon()), u.confirmReset),
	)
	u.openPage("Приложение", scrolled(rows))
}

func (u *ui) confirmReset() {
	showConfirm("Сброс", "Будут удалены все профили серверов, списки адресов, каталог, настройки и журналы с этого устройства. Серверы не изменятся. Продолжить?", func(ok bool) {
		if !ok {
			return
		}
		u.cleanup()
		dir, err := settings.Dir()
		if err == nil {
			err = os.RemoveAll(dir)
		}
		if err != nil {
			showError(err, u.win)
			return
		}
		_ = setAutostart(false)
		u.st = &settings.Settings{}
		u.selected = ""
		u.reload()
		u.refreshSplitLink()
		u.showPage(0)
		showInfo("Готово", "Данные приложения удалены.", u.win)
	}, u.win)
}

// ---------- журналы ----------

func (u *ui) openLogging() {
	w := u.newSubPage("Журналы")
	tunnelPath, appPath := u.logFiles()
	body := container.NewVBox()
	var rebuild func()
	rebuild = func() {
		body.RemoveAll()
		body.Add(note("Журналы помогают разобраться, что пошло не так."))
		row, _ := switchRow("Записывать журналы", "Запись отключится через 14 дней, журналы будут удалены", u.st.LogEnabled, func(on bool) {
			u.st.LogEnabled = on
			if on {
				u.st.LogSince = time.Now()
			}
			u.saveSettings()
		})
		body.Add(row)
		body.Add(sectionLabel("Журналы"))
		body.Add(navRow("Туннель", fmt.Sprintf("События подключения · %s", humanBytes(uint64(fileSize(tunnelPath)))), func() { u.openLogView("Журнал туннеля", tunnelPath) }))
		body.Add(navRow("Приложение", fmt.Sprintf("События приложения · %s", humanBytes(uint64(fileSize(appPath)))), func() { u.openLogView("Журнал приложения", appPath) }))
		body.Add(listRow("Сохранить все журналы", "Архив .zip с обоими журналами", widget.NewIcon(theme.DocumentSaveIcon()), func() { u.saveLogs(w) }))
		body.Add(listRowColored("Очистить все журналы", "", theme.ColorNameError, widget.NewIcon(theme.DeleteIcon()), func() {
			u.clearLogs()
			rebuild()
		}))
		body.Refresh()
	}
	rebuild()
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}

func (u *ui) saveLogs(parent fyne.Window) {
	fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
		if err != nil || wr == nil {
			return
		}
		defer wr.Close()
		zw := zip.NewWriter(wr)
		t, a := u.logFiles()
		for name, path := range map[string]string{"tunnel.log": t, "app.log": a} {
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			f, err := zw.Create(name)
			if err == nil {
				_, _ = f.Write(b)
			}
		}
		if err := zw.Close(); err != nil {
			showError(err, parent)
		}
	}, parent)
	fd.SetFileName("veil-logs.zip")
	fd.Show()
}

func (u *ui) openLogView(title, path string) {
	w := u.newSubPage(title)
	text := "Журнал пуст."
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		if st, err := f.Stat(); err == nil && st.Size() > maxLogView {
			_, _ = f.Seek(st.Size()-maxLogView, io.SeekStart)
		}
		if b, err := io.ReadAll(f); err == nil && len(b) > 0 {
			text = string(b)
		}
	}
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapBreak
	entry.SetText(text)
	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(text) })
	copyBtn.Importance = widget.LowImportance
	w.SetRight(copyBtn)
	w.SetContent(container.NewPadded(entry))
	w.Show()
}

// ---------- резервная копия ----------

func (u *ui) openBackup() {
	w := u.newSubPage("Резервная копия")
	info := card(container.NewBorder(nil, nil, widget.NewIcon(theme.InfoIcon()), nil,
		prose("Копия содержит пароли и приватные ключи всех добавленных серверов. Храните её в надёжном месте.")))
	make := widget.NewButton("Сделать резервную копию", func() { u.makeBackup(w) })
	make.Importance = widget.HighImportance
	restore := widget.NewButton("Восстановить из копии", func() { u.restoreBackup(w) })
	body := container.NewBorder(
		container.NewVBox(note("Сохраните настройки в файл, чтобы восстановить их при следующей установке приложения."), info),
		container.NewVBox(make, restore), nil, nil)
	w.SetContent(container.NewPadded(body))
	w.Show()
}

// backupFiles — что входит в копию: профили, списки адресов и настройки.
func backupFiles() (map[string]string, error) {
	dir, err := settings.Dir()
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	profiles, _ := filepath.Glob(filepath.Join(dir, "profiles", "*.json"))
	for _, p := range profiles {
		files["profiles/"+filepath.Base(p)] = p
	}
	for _, name := range []string{"routing.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			files[name] = filepath.Join(dir, name)
		}
	}
	return files, nil
}

func (u *ui) makeBackup(parent fyne.Window) {
	fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
		if err != nil || wr == nil {
			return
		}
		defer wr.Close()
		files, err := backupFiles()
		if err != nil {
			showError(err, parent)
			return
		}
		zw := zip.NewWriter(wr)
		for name, path := range files {
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			f, err := zw.Create(name)
			if err != nil {
				continue
			}
			_, _ = f.Write(b)
		}
		if err := zw.Close(); err != nil {
			showError(err, parent)
			return
		}
		showInfo("Готово", fmt.Sprintf("Копия сохранена (файлов: %d).", len(files)), parent)
	}, parent)
	fd.SetFileName("veil-" + time.Now().Format("20060102") + backupExt)
	fd.Show()
}

func (u *ui) restoreBackup(parent fyne.Window) {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		b, err := io.ReadAll(io.LimitReader(r, 64<<20))
		if err != nil {
			showError(err, parent)
			return
		}
		showConfirm("Восстановление", "Профили и настройки из копии будут добавлены к текущим; совпадающие заменятся. Продолжить?", func(ok bool) {
			if !ok {
				return
			}
			n, err := restoreFromZip(b)
			if err != nil {
				showError(err, parent)
				return
			}
			u.loadSettings()
			u.reload()
			u.refreshSplitLink()
			showInfo("Готово", fmt.Sprintf("Восстановлено файлов: %d.", n), parent)
		}, parent)
	}, parent)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{backupExt, ".zip"}))
	fd.Show()
}

// restoreFromZip разворачивает только известные имена (защита от путей вне каталога).
func restoreFromZip(data []byte) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("это не резервная копия Veil: %w", err)
	}
	dir, err := settings.Dir()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		var dst string
		switch {
		case name == "routing.json" || name == "settings.json":
			dst = filepath.Join(dir, name)
		case strings.HasPrefix(name, "profiles/") && strings.HasSuffix(name, ".json") && !strings.Contains(strings.TrimPrefix(name, "profiles/"), "/") && !strings.Contains(name, ".."):
			dst = filepath.Join(dir, filepath.FromSlash(name))
		default:
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return n, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		rc.Close()
		if err != nil {
			return n, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return n, err
		}
		if err := os.WriteFile(dst, b, 0o600); err != nil {
			return n, err
		}
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("в архиве нет данных Veil")
	}
	return n, nil
}

// ---------- о приложении ----------

func (u *ui) openAbout() {
	w := u.newSubPage("О Veil")
	logo := canvas.NewImageFromResource(bundledIcon())
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(96, 96))
	name := text("Veil", 26, colText, true)
	name.Alignment = fyne.TextAlignCenter
	desc := prose("VPN с маскировкой под HTTPS: ядро WireGuard внутри HTTP/3 и HTTP/2 на порту 443. Свободное приложение с открытым исходным кодом.")
	open := func(link string) func() {
		return func() {
			if uu, err := url.Parse(link); err == nil {
				_ = u.app.OpenURL(uu)
			}
		}
	}
	copyVer := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { u.app.Clipboard().SetContent(appVersion) })
	copyVer.Importance = widget.LowImportance
	check := widget.NewButton("Проверить обновления", func() { u.checkUpdates(w, true) })
	check.Importance = widget.LowImportance
	body := container.NewVBox(
		container.NewCenter(container.NewGridWrap(fyne.NewSize(96, 96), logo)),
		name, desc,
		sectionLabel("Контакты"),
		listRow("GitHub", "Исходный код и сообщения об ошибках", widget.NewIcon(theme.NavigateNextIcon()), open(repoURL)),
		listRow("Версия "+appVersion, "", copyVer, nil),
		check,
	)
	w.SetContent(container.NewPadded(scrolled(body)))
	w.Show()
}

// ---------- проверка обновлений ----------

func newerVersion(latest, current string) bool {
	parse := func(s string) [3]int {
		var v [3]int
		fmt.Sscanf(strings.TrimPrefix(strings.TrimSpace(s), "v"), "%d.%d.%d", &v[0], &v[1], &v[2])
		return v
	}
	a, b := parse(latest), parse(current)
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// checkUpdates спрашивает у GitHub последнюю версию; verbose — показывать и «обновлений нет».
func (u *ui) checkUpdates(parent fyne.Window, verbose bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		latest, link, err := fetchLatestRelease(ctx)
		fyne.Do(func() {
			switch {
			case err != nil:
				if verbose {
					showError(err, parent)
				}
			case newerVersion(latest, appVersion):
				showConfirm("Доступно обновление", fmt.Sprintf("Вышла версия %s (у вас %s). Открыть страницу загрузки?", latest, appVersion), func(ok bool) {
					if ok {
						if uu, err := url.Parse(link); err == nil {
							_ = u.app.OpenURL(uu)
						}
					}
				}, parent)
			case verbose:
				showInfo("Обновлений нет", "Установлена последняя версия: "+appVersion, parent)
			}
		})
	}()
}

// ---------- страница «Добавить» ----------

func (u *ui) addPage() fyne.CanvasObject {
	title := text("Подключение", 20, colText, false)
	title.Alignment = fyne.TextAlignCenter
	key := widget.NewMultiLineEntry()
	key.Wrapping = fyne.TextWrapBreak
	key.SetMinRowsVisible(2)
	key.SetPlaceHolder("Вставьте ключ veil://…")
	insert := widget.NewButton("Вставить", func() {
		if strings.TrimSpace(key.Text) == "" {
			key.SetText(strings.TrimSpace(u.app.Clipboard().Content()))
		}
		p, err := decodeAny(key.Text)
		if err != nil {
			showError(err, u.win)
			return
		}
		base, n := p.Name, 2
		for u.profileExists(p.Name) {
			p.Name = fmt.Sprintf("%s %d", base, n)
			n++
		}
		if err := saveProfile(p); err != nil {
			showError(err, u.win)
			return
		}
		key.SetText("")
		u.reload()
		u.selectProfile(p.Name)
		u.showPage(0)
	})
	insert.Importance = widget.HighImportance
	rows := container.NewVBox(
		note("Вставьте ключ, добавьте файл конфигурации или отсканируйте QR-код из приложения администратора."),
		key, insert,
		sectionLabel("Другие способы подключения"),
		navRow("Свой сервер по SSH", "Veil установится на ваш сервер вместе с сайтом-прикрытием", u.showDeployForm),
		navRow("Файл с настройками подключения", "Файл .veil, выданный админ-панелью", u.showImport),
		navRow("Через ближайший вход", "Выход задаётся явно, вход выбирается ближайший", u.openMultiEntryBuilder),
	)
	return container.NewPadded(container.NewBorder(container.NewVBox(container.NewPadded(title)), nil, nil, nil, scrolled(rows)))
}

var _ = layout.NewSpacer
