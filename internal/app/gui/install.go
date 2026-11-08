package gui

// Экран установки (дизайн Install): шаги deploy.Step по мере выполнения, полоса прогресса, журнал
// по кнопке «Подробнее», в конце — «Готово — подключиться». Если протокол на сервере уже был,
// вместо него открывается «Сервер уже настроен» (existing.go).

import (
	"context"
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"veil/internal/app/gui/kit"
	"veil/internal/config"
	"veil/internal/deploy"
)

// installJob — что ставить: запрос deploy, протокол (ключ deploy) и включена ли админ-панель (Veil).
type installJob struct {
	req   deploy.Request
	proto string
	admin bool
}

func (j installJob) host() string {
	h := j.req.IPv4
	if h == "" {
		h = j.req.IPv6
	}
	return j.req.User + "@" + h
}

// stepTitle — подпись шага по его ID, пока сервер не прислал свою (и для английского интерфейса:
// заголовки шагов deploy пишет по-русски).
func stepTitle(id string) string {
	switch id {
	case "ssh":
		return L("SSH connection", "Подключение по SSH")
	case "system":
		return L("System check", "Проверка системы")
	case "docker":
		return "Docker"
	case "files":
		return L("Uploading files", "Загрузка файлов")
	case "build":
		return L("Building the image", "Сборка образа")
	case "start":
		return L("Starting", "Запуск")
	case "client":
		return L("Client", "Клиент")
	case "profile":
		return L("Profile saved", "Профиль сохранён")
	}
	return id
}

// stepPlan — ожидаемые шаги: показываются серыми, пока до них не дошло.
func stepPlan(proto string) []string {
	last := "client"
	if proto == "veil" {
		last = "profile"
	}
	return []string{"ssh", "system", "docker", "files", "build", "start", last}
}

type stepRow struct {
	id, state     string
	title, detail textSetter
	icon          *fyne.Container
	bg            *canvas.Rectangle
	obj           fyne.CanvasObject
	stop          func()
}

func newStepRow(id string) *stepRow {
	r := &stepRow{id: id, state: "wait", icon: container.NewStack()}
	r.title = kit.NewFlowText(stepTitle(id), 14, kit.ColMuted, true, 18)
	r.detail = kit.NewFlowText("", 12, kit.ColMuted, false, 16)
	r.detail.Hide()
	text := container.New(kit.TightVBox{Gap: 1}, r.title, r.detail)
	body := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), kit.Vcenter(container.NewGridWrap(fyne.NewSize(24, 24), r.icon))), nil, kit.Vcenter(text))
	r.obj, r.bg = padCard(body, 6, 14, 48)
	r.paint()
	return r
}

func (r *stepRow) paint() {
	if r.stop != nil {
		r.stop()
		r.stop = nil
	}
	var ic fyne.CanvasObject
	switch r.state {
	case "done":
		ic = container.NewCenter(kit.IconImg("check", kit.ColOK, 22))
	case "skip":
		ic = container.NewCenter(kit.IconImg("minus", kit.ColMuted, 20))
	case "fail":
		ic = container.NewCenter(kit.IconImg("close", kit.ColBad, 20))
	case "active":
		ic, r.stop = pulse(18)
	default:
		d := canvas.NewCircle(kit.ColCard2)
		ic = container.NewCenter(container.NewGridWrap(fyne.NewSize(10, 10), d))
	}
	r.icon.Objects = []fyne.CanvasObject{ic}
	r.icon.Refresh()
	r.bg.FillColor = kit.ColCard
	if r.state == "active" {
		r.bg.FillColor = kit.ColCard2
	}
	r.bg.Refresh()
}

func (r *stepRow) set(st deploy.Step) {
	r.state = st.State
	r.title.SetText(r.titleText(st))
	if c, ok := r.title.(interface{ SetColor(color.Color) }); ok {
		c.SetColor(kit.ColText)
	}
	r.detail.SetText(st.Detail)
	if st.Detail != "" {
		r.detail.Show()
	} else {
		r.detail.Hide()
	}
	r.paint()
}

type installView struct {
	u    *ui
	w    *subPage
	form *subPage
	job  installJob

	body    *fyne.Container
	list    *fyne.Container
	rows    []*stepRow
	bar     *fracBar
	logText textSetter
	logCard fyne.CanvasObject
	logs    []string
	logOpen bool
	footer  *fyne.Container
	status  textSetter

	finished, failed bool
	done             func() // «Готово — подключиться»
}

// openInstall показывает экран установки поверх формы и запускает установку.
func (u *ui) openInstall(form *subPage, job installJob) *installView {
	v := u.newInstallView(form, job)
	v.w.Show()
	go v.run()
	return v
}

func (u *ui) newInstallView(form *subPage, job installJob) *installView {
	v := &installView{u: u, form: form, job: job}
	v.w = u.newSubPage(fmt.Sprintf(L("Installation · %s", "Установка · %s"), job.req.Name))
	panel := L("no", "нет")
	if job.admin {
		panel = L("yes", "да")
	}
	sub := protoLabel(job.proto) + " · " + job.host()
	if job.proto == "veil" {
		sub += " · " + L("admin panel: ", "админ-панель: ") + panel
	}
	v.bar = newFracBar()
	v.list = container.New(kit.VboxFit{Gap: 6})
	for _, id := range stepPlan(job.proto) {
		r := newStepRow(id)
		v.rows = append(v.rows, r)
		v.list.Add(r.obj)
	}
	v.status = kit.NewFlowText("", 13, kit.ColBad, true, 18)
	v.status.Hide()
	v.logText = kit.NewFlowText("", 12, kit.ColMuted, false, 18)
	v.logCard, _ = padCard(v.logText, 14, 14, 40)
	v.logCard.Hide()
	v.body = kit.NewVBoxFit()
	v.body.Add(kit.NewFlowText(sub, 13, kit.ColMuted, false, 18))
	v.body.Add(v.bar.obj)
	v.body.Add(v.list)
	v.body.Add(v.status)
	v.body.Add(v.logCard)
	v.footer = container.NewStack()
	v.renderFooter()
	pageWithFooter(v.w, v.body, v.footer)
	v.w.OnClosed = func() {
		for _, r := range v.rows {
			if r.stop != nil {
				r.stop()
				r.stop = nil
			}
		}
	}
	return v
}

func (v *installView) renderFooter() {
	logLabel := L("Details", "Подробнее")
	if v.logOpen {
		logLabel = L("Hide log", "Скрыть журнал")
	}
	logBtn := bigButton(logLabel, "", false, v.toggleLog)
	var objs []fyne.CanvasObject
	switch {
	case v.finished:
		label := L("Done — connect", "Готово — подключиться")
		if v.job.proto == "openvpn" || v.job.proto == "ipsec" {
			label = L("Done — show the configuration", "Готово — показать конфигурацию")
		}
		objs = []fyne.CanvasObject{logBtn, bigButton(label, "check", true, func() {
			if v.done != nil {
				v.done()
			}
		})}
	case v.failed:
		objs = []fyne.CanvasObject{logBtn, bigButton(L("Back to the form", "Вернуться к форме"), "back", false, v.w.Close)}
	default:
		objs = []fyne.CanvasObject{logBtn}
	}
	v.footer.Objects = []fyne.CanvasObject{newAutoGrid(220, objs...)}
	v.footer.Refresh()
}

func (v *installView) toggleLog() {
	v.logOpen = !v.logOpen
	if v.logOpen {
		v.logCard.Show()
	} else {
		v.logCard.Hide()
	}
	v.renderFooter()
	v.body.Refresh()
}

func (v *installView) addLog(line string) {
	v.logs = append(v.logs, line)
	if len(v.logs) > 400 {
		v.logs = v.logs[len(v.logs)-400:]
	}
	v.logText.SetText(strings.Join(v.logs, "\n"))
	if v.logOpen {
		v.body.Refresh()
	}
}

// apply показывает шаг: обновляет уже известный или вставляет новый перед ожидающими.
// Ожидающие шаги выше пришедшего пропущены (например, сборка образа без Docker) — они убираются.
func (v *installView) apply(st deploy.Step) {
	idx := -1
	for i, r := range v.rows {
		if r.id == st.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = len(v.rows)
		for i, r := range v.rows {
			if r.state == "wait" {
				idx = i
				break
			}
		}
		r := newStepRow(st.ID)
		v.rows = append(v.rows[:idx], append([]*stepRow{r}, v.rows[idx:]...)...)
	} else {
		var keep []*stepRow
		for i, r := range v.rows {
			if i < idx && r.state == "wait" {
				continue
			}
			keep = append(keep, r)
		}
		for i, r := range keep {
			if r.id == st.ID {
				idx = i
			}
		}
		v.rows = keep
	}
	v.rows[idx].set(st)
	if st.Detail != "" {
		v.addLog("› " + v.rows[idx].titleText(st) + ": " + st.Detail)
	}
	v.relist()
}

func (r *stepRow) titleText(st deploy.Step) string {
	if st.Title != "" && L("en", "ru") == "ru" {
		return st.Title
	}
	return stepTitle(st.ID)
}

func (v *installView) relist() {
	v.list.Objects = nil
	done := 0
	for _, r := range v.rows {
		v.list.Add(r.obj)
		if r.state == "done" || r.state == "skip" {
			done++
		}
	}
	if len(v.rows) > 0 {
		v.bar.set(float32(done) / float32(len(v.rows)))
	}
	v.list.Refresh()
	v.body.Refresh()
}

// finishOK — установка прошла: ожидающие шаги убираются, полоса заполняется, появляется «Готово».
func (v *installView) finishOK(done func()) {
	var keep []*stepRow
	for _, r := range v.rows {
		if r.state == "wait" {
			continue
		}
		if r.state == "active" {
			r.state = "done"
			r.paint()
		}
		keep = append(keep, r)
	}
	v.rows = keep
	v.relist()
	v.bar.set(1)
	v.finished, v.done = true, done
	v.renderFooter()
}

func (v *installView) finishErr(err error) {
	for _, r := range v.rows {
		if r.state == "active" {
			r.state = "fail"
			r.paint()
		}
	}
	v.failed = true
	v.status.SetText(L("Error: ", "Ошибка: ") + err.Error())
	v.status.Show()
	v.addLog(L("Error: ", "Ошибка: ") + err.Error())
	v.renderFooter()
	v.body.Refresh()
}

// run выполняет установку в фоне; все изменения интерфейса — через fyne.Do.
func (v *installView) run() {
	req := v.job.req
	req.Progress = func(st deploy.Step) { fyne.Do(func() { v.apply(st) }) }
	logf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		fyne.Do(func() { v.addLog(line) })
	}
	var prof *config.Profile
	var res *deploy.ProtocolResult
	var err error
	if v.job.proto == "veil" {
		prof, err = deploy.Deploy(context.Background(), req, logf)
	} else if res, err = deploy.DeployProtocol(context.Background(), req, v.job.proto, 0, logf); err == nil {
		prof = protocolProfile(v.job.req, v.job.proto, res)
	}
	if err == nil {
		err = saveProfile(prof)
	}
	fyne.Do(func() {
		if err != nil {
			v.finishErr(err)
			return
		}
		v.u.reload()
		if res != nil && res.Existing {
			v.w.Close()
			v.u.openExisting(v.form, v.job, prof, res)
			return
		}
		v.finishOK(func() { v.u.afterInstall(v.form, v.w, v.job.proto, prof) })
	})
}

// afterInstall закрывает страницы установки, выбирает новый сервер и подключается
// (для OpenVPN и IPsec — показывает конфигурацию клиента, как раньше).
func (u *ui) afterInstall(form, page *subPage, proto string, prof *config.Profile) {
	u.reload()
	u.selectProfile(prof.Name)
	if page != nil {
		page.Close()
	}
	if form != nil {
		form.Close()
	}
	u.showPage(0)
	switch proto {
	case "openvpn":
		u.showOVShare(prof.Name, prof.OVPN)
	case "ipsec":
		u.showIPsecShare(prof.Name, prof.IPsec)
	default:
		if u.handle == nil {
			u.toggle()
		}
	}
}
