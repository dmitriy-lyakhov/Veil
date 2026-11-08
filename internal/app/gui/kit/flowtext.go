package kit

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// flowText — текст с переносом по словам и заданной высотой строки (у RichText в Fyne она фиксирована и ниже,
// чем в дизайне: 13 px → 17,1 px в строках списков и 18,85 px в пояснениях). Без внутренних полей.
// Высота зависит от ширины: после Resize MinSize возвращает высоту всех строк (как heightAtWidth ждёт от RichText).
type flowText struct {
	widget.BaseWidget
	text  string
	size  float32
	col   color.Color
	style fyne.TextStyle
	lh    float32
	w     float32
	lines []string
	rend  *flowRenderer
}

func NewFlowText(s string, size float32, c color.Color, bold bool, lineHeight float32) *flowText {
	f := &flowText{text: s, size: size, col: c, style: fyne.TextStyle{Bold: bold}, lh: lineHeight}
	f.ExtendBaseWidget(f)
	return f
}

// proseFlush — пояснение 13 px в строках списков (высота строки 17,1 px, приглушённый цвет).
func ProseFlush(s string) fyne.CanvasObject { return NewFlowText(s, 13, ColMuted, false, 17.1) }

// SetColor меняет цвет текста.
func (f *flowText) SetColor(c color.Color) {
	f.col = c
	f.Refresh()
}

func (f *flowText) SetText(s string) {
	f.text = s
	f.lines = nil
	f.w = 0
	f.Refresh()
}

// wrap разбивает текст на строки не шире w; переводы строк сохраняются, слишком длинные слова режутся по символам.
func (f *flowText) wrap(w float32) []string {
	var out []string
	for _, para := range strings.Split(f.text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		cur := ""
		for _, word := range words {
			try := word
			if cur != "" {
				try = cur + " " + word
			}
			if cur != "" && fyne.MeasureText(try, f.size, f.style).Width > w {
				out = append(out, cur)
				try = word
			}
			cur = try
			// слово длиннее строки (адрес, имя без пробелов) режется по символам
			for fyne.MeasureText(cur, f.size, f.style).Width > w && len([]rune(cur)) > 1 {
				r := []rune(cur)
				k := len(r) - 1
				for k > 1 && fyne.MeasureText(string(r[:k]), f.size, f.style).Width > w {
					k--
				}
				out = append(out, string(r[:k]))
				cur = string(r[k:])
			}
		}
		out = append(out, cur)
	}
	return out
}

func (f *flowText) relayout(w float32) {
	if w <= 0 {
		w = 10000
	}
	if f.lines != nil && f.w == w {
		return
	}
	f.w = w
	f.lines = f.wrap(w)
}

func (f *flowText) MinSize() fyne.Size {
	f.relayout(f.Size().Width)
	return fyne.NewSize(24, f.lh*float32(len(f.lines)))
}

func (f *flowText) CreateRenderer() fyne.WidgetRenderer {
	f.rend = &flowRenderer{f: f}
	return f.rend
}

type flowRenderer struct {
	f    *flowText
	objs []fyne.CanvasObject
}

func (r *flowRenderer) Layout(s fyne.Size) {
	f := r.f
	f.relayout(s.Width)
	for len(r.objs) < len(f.lines) {
		t := canvas.NewText("", f.col)
		t.TextSize, t.TextStyle = f.size, f.style
		r.objs = append(r.objs, t)
	}
	r.objs = r.objs[:len(f.lines)]
	th := fyne.MeasureText("Ag", f.size, f.style).Height
	for i, o := range r.objs {
		t := o.(*canvas.Text)
		t.Text, t.Color, t.TextSize, t.TextStyle = f.lines[i], f.col, f.size, f.style
		t.Resize(fyne.NewSize(s.Width, th))
		t.Move(fyne.NewPos(0, float32(i)*f.lh+(f.lh-th)/2))
	}
}

func (r *flowRenderer) MinSize() fyne.Size { return r.f.MinSize() }
func (r *flowRenderer) Refresh() {
	r.f.lines = nil
	r.Layout(r.f.Size())
	for _, o := range r.objs {
		o.Refresh()
	}
}
func (r *flowRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *flowRenderer) Destroy()                     {}
