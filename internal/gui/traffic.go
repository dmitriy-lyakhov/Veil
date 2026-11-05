package gui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"veil/internal/settings"
)

const (
	histSamples = 60 // секунд истории для графика
	chartBars   = 30 // столбцов: каждый — две секунды
	chartHeight = 120
)

// trafficStore — счётчики за день и месяц; хранятся только на этом устройстве.
type trafficStore struct {
	mu      sync.Mutex
	Day     string `json:"day"`
	DayRx   uint64 `json:"day_rx"`
	DayTx   uint64 `json:"day_tx"`
	Month   string `json:"month"`
	MonthRx uint64 `json:"month_rx"`
	MonthTx uint64 `json:"month_tx"`
	path    string
	dirty   bool
	saved   time.Time
}

func loadTrafficStore() *trafficStore {
	s := &trafficStore{}
	if p, err := settings.Path(); err == nil {
		s.path = filepath.Join(filepath.Dir(p), "traffic.json")
		if b, err := os.ReadFile(s.path); err == nil {
			_ = json.Unmarshal(b, s)
		}
	}
	return s
}

func (s *trafficStore) roll(now time.Time) {
	if d := now.Format("2006-01-02"); s.Day != d {
		s.Day, s.DayRx, s.DayTx = d, 0, 0
	}
	if m := now.Format("2006-01"); s.Month != m {
		s.Month, s.MonthRx, s.MonthTx = m, 0, 0
	}
}

func (s *trafficStore) add(rx, tx uint64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roll(now)
	s.DayRx, s.DayTx, s.MonthRx, s.MonthTx = s.DayRx+rx, s.DayTx+tx, s.MonthRx+rx, s.MonthTx+tx
	s.dirty = true
	if now.Sub(s.saved) > 30*time.Second {
		s.saveLocked(now)
	}
}

func (s *trafficStore) flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLocked(time.Now())
}

func (s *trafficStore) saveLocked(now time.Time) {
	if !s.dirty || s.path == "" {
		return
	}
	if b, err := json.Marshal(s); err == nil {
		_ = os.MkdirAll(filepath.Dir(s.path), 0o700)
		_ = os.WriteFile(s.path, b, 0o600)
	}
	s.dirty, s.saved = false, now
}

func (s *trafficStore) totals(now time.Time) (day, month uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roll(now)
	return s.DayRx + s.DayTx, s.MonthRx + s.MonthTx
}

// trafficState — живые данные подключения для плиток и страницы «Трафик».
type trafficState struct {
	store      *trafficStore
	rxHist     []float64 // байт/с, последние histSamples секунд
	rxRate     float64
	txRate     float64
	latency    float64
	rx, tx     uint64
	session    string
	refresh    func() // обновляет открытую страницу «Трафик»
	sessionRxB uint64
	trayLat    int // задержка, с которой последний раз обновлялось меню в трее
}

func (t *trafficState) push(rate float64) {
	t.rxHist = append(t.rxHist, rate)
	if len(t.rxHist) > histSamples {
		t.rxHist = t.rxHist[len(t.rxHist)-histSamples:]
	}
}

func (t *trafficState) reset() {
	t.rxHist, t.rxRate, t.txRate, t.latency, t.rx, t.tx, t.session = nil, 0, 0, 0, 0, 0, ""
	if t.store != nil {
		t.store.flush()
	}
	if t.refresh != nil {
		t.refresh()
	}
}

// fmtSize форматирует байты: «1,2 ГБ» по-русски, «1.2 GB» по-английски.
func fmtSize(n float64) string {
	units := []string{L("B", "Б"), L("KB", "КБ"), L("MB", "МБ"), L("GB", "ГБ"), L("TB", "ТБ")}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	s := fmt.Sprintf("%.0f", n)
	if i >= 3 {
		s = fmt.Sprintf("%.1f", n)
	}
	if curLang != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + " " + units[i]
}

func fmtRate(bps float64) string {
	s := fmt.Sprintf("%.1f", bps/(1<<20))
	if curLang != "en" {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s + " " + L("MB/s", "МБ/с")
}

// barChart — столбчатый график скорости: слева «60 с назад», справа «сейчас».
type barChart struct {
	bars []*canvas.Rectangle
	obj  fyne.CanvasObject
}

type barsLayout struct{}

func (barsLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(100, chartHeight) }
func (barsLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	const gap = 4
	n := float32(len(objs))
	w := (s.Width - gap*(n-1)) / n
	for i, o := range objs {
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(w, h))
		o.Move(fyne.NewPos(float32(i)*(w+gap), s.Height-h))
	}
}

func newBarChart() *barChart {
	c := &barChart{}
	var objs []fyne.CanvasObject
	for i := 0; i < chartBars; i++ {
		r := canvas.NewRectangle(colMuted)
		r.SetMinSize(fyne.NewSize(1, 4))
		c.bars = append(c.bars, r)
		objs = append(objs, r)
	}
	area := container.New(barsLayout{}, objs...)
	capL := text(L("60 s ago", "60 с назад"), 12, colMuted, false)
	capR := text(L("now", "сейчас"), 12, colMuted, false)
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	col := container.NewVBox(container.New(&fillMin{h: chartHeight}, area), container.NewHBox(capL, layout.NewSpacer(), capR))
	c.obj = container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(16, 16, 16, 16), col))
	return c
}

func (c *barChart) set(hist []float64) {
	// две секунды на столбец, самые свежие справа
	vals := make([]float64, chartBars)
	for i := 0; i < chartBars; i++ {
		idx := len(hist) - 2*(chartBars-i)
		var m float64
		for k := 0; k < 2; k++ {
			if j := idx + k; j >= 0 && j < len(hist) && hist[j] > m {
				m = hist[j]
			}
		}
		vals[i] = m
	}
	var peak float64
	for _, v := range vals {
		if v > peak {
			peak = v
		}
	}
	for i, r := range c.bars {
		h := float32(4)
		if peak > 0 {
			h = max(4, float32(vals[i]/peak)*chartHeight)
		}
		r.SetMinSize(fyne.NewSize(1, h))
		if i == len(c.bars)-1 {
			r.FillColor = colAccent
		} else {
			r.FillColor = color.Color(colMuted)
		}
		r.Refresh()
	}
}

// kvRow — строка «подпись — значение» страницы «Трафик» (52 px, значение жирным).
func kvRow(label string) (fyne.CanvasObject, *canvas.Text) {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.SetMinSize(fyne.NewSize(1, 52))
	val := text("—", 14, colText, true)
	val.Alignment = fyne.TextAlignTrailing
	row := container.NewBorder(nil, nil, container.NewCenter(text(label, 14, colMuted, false)), container.NewCenter(val))
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 16, 16), row)), val
}

// openTraffic — страница «Трафик»: график, текущие значения, итоги за сессию, за сегодня и за месяц.
func (u *ui) openTraffic() {
	chart := newBarChart()
	dl, dlV := kvRow(L("Download", "Загрузка"))
	ul, ulV := kvRow(L("Upload", "Отдача"))
	lt, ltV := kvRow(L("Latency", "Задержка"))
	rc, rcV := kvRow(L("Received", "Получено"))
	sn, snV := kvRow(L("Sent", "Отправлено"))
	du, duV := kvRow(L("Duration", "Длительность"))
	td, tdV := kvRow(L("Today", "Сегодня"))
	mo, moV := kvRow(L("This month", "За месяц"))
	hd := func(s string) fyne.CanvasObject { return text(s, 18, colText, true) }
	body := container.NewVBox(
		hd(L("Right now", "Сейчас")), chart.obj, dl, ul, lt,
		hd(L("This session", "За сессию")), rc, sn, du,
		hd(L("Total", "Всего")), td, mo,
		prose(L("Counters are kept on this device only.", "Счётчики хранятся только на этом устройстве.")),
	)
	t := &u.tr
	refresh := func() {
		on := u.connected
		set := func(v *canvas.Text, s string) {
			v.Text = s
			v.Refresh()
		}
		chart.set(t.rxHist)
		if on {
			set(dlV, fmtRate(t.rxRate))
			set(ulV, fmtRate(t.txRate))
			if t.latency > 0 {
				set(ltV, fmt.Sprintf("%.0f %s", t.latency, L("ms", "мс")))
			} else {
				set(ltV, "—")
			}
			set(rcV, fmtSize(float64(t.rx)))
			set(snV, fmtSize(float64(t.tx)))
			set(duV, t.session)
		} else {
			for _, v := range []*canvas.Text{dlV, ulV, ltV, rcV, snV, duV} {
				set(v, "—")
			}
		}
		day, month := t.store.totals(time.Now())
		set(tdV, fmtSize(float64(day)))
		set(moV, fmtSize(float64(month)))
	}
	w := u.newSubPage(L("Traffic", "Трафик"))
	w.OnClosed = func() { t.refresh = nil }
	t.refresh = refresh
	refresh()
	w.SetContent(container.NewPadded(container.NewVScroll(body)))
	w.Show()
}
