package gui

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/settings"
)

const layoutTolerance = 1.5

type rect struct{ x0, y0, x1, y1 float32 }

func negativeSized(root fyne.CanvasObject, out *[]finding) {
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		if sz := o.Size(); sz.Width < -0.5 || sz.Height < -0.5 {
			*out = append(*out, finding{"negative-size", fmt.Sprintf("%T", o), rectOf(o.Position(), sz), rect{}})
		}
		for _, ch := range childrenOf(o) {
			walk(ch)
		}
	}
	walk(root)
}

func containsObj(path []fyne.CanvasObject, o fyne.CanvasObject) bool {
	for _, p := range path {
		if p == o {
			return true
		}
	}
	return false
}

func rectOf(p fyne.Position, s fyne.Size) rect { return rect{p.X, p.Y, p.X + s.Width, p.Y + s.Height} }

func (r rect) w() float32 { return r.x1 - r.x0 }
func (r rect) h() float32 { return r.y1 - r.y0 }

func (r rect) intersect(o rect) rect {
	return rect{max(r.x0, o.x0), max(r.y0, o.y0), min(r.x1, o.x1), min(r.y1, o.y1)}
}

func (r rect) overlapArea(o rect) float32 {
	i := r.intersect(o)
	if i.x1 <= i.x0 || i.y1 <= i.y0 {
		return 0
	}
	return (i.x1 - i.x0) * (i.y1 - i.y0)
}

func (r rect) String() string { return fmt.Sprintf("[%.0f,%.0f %.0fx%.0f]", r.x0, r.y0, r.w(), r.h()) }

type finding struct {
	kind  string
	text  string
	drawn rect
	box   rect
}

func (f finding) String() string {
	return fmt.Sprintf("%s: %q drawn %s, limit %s", f.kind, f.text, f.drawn, f.box)
}

type textNode struct {
	t     *canvas.Text
	drawn rect
	block *rect
	id    int
	order int
	path  []fyne.CanvasObject
	clip  rect
}

type occluder struct {
	order int
	r     rect
	par   fyne.CanvasObject
}

func childrenOf(o fyne.CanvasObject) []fyne.CanvasObject {
	switch v := o.(type) {
	case *fyne.Container:
		return v.Objects
	case fyne.Widget:
		return test.WidgetRenderer(v).Objects()
	}
	return nil
}

func hasBackground(c *fyne.Container) bool {
	for _, ch := range c.Objects {
		r, ok := ch.(*canvas.Rectangle)
		if !ok || !r.Visible() {
			continue
		}
		visibleFill := false
		if _, _, _, a := fillRGBA(r.FillColor); a > 0 {
			visibleFill = true
		}
		if (visibleFill || r.StrokeWidth > 0) && r.Size().Width >= c.Size().Width-1 && r.Size().Height >= c.Size().Height-1 && len(c.Objects) > 1 {
			return true
		}
	}
	return false
}

func fillRGBA(c color.Color) (r, g, b, a uint32) {
	if c == nil {
		return
	}
	return c.RGBA()
}

func drawnRect(t *canvas.Text, abs fyne.Position) rect {
	n := fyne.MeasureText(t.Text, t.TextSize, t.TextStyle)
	x := abs.X
	switch t.Alignment {
	case fyne.TextAlignCenter:
		x += (t.Size().Width - n.Width) / 2
	case fyne.TextAlignTrailing:
		x += t.Size().Width - n.Width
	}
	return rect{x, abs.Y, x + n.Width, abs.Y + n.Height}
}

func collectTexts(root fyne.CanvasObject, origin fyne.Position, clip rect, out *[]textNode, occ *[]occluder, ids *int) {
	order := 0
	var walk func(o fyne.CanvasObject, parent fyne.Position, clip rect, blk *rect, path []fyne.CanvasObject)
	walk = func(o fyne.CanvasObject, parent fyne.Position, clip rect, blk *rect, path []fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		abs := parent.Add(o.Position())
		order++
		inEntry := false
		for _, p := range path {
			if _, ok := p.(*widget.Entry); ok {
				inEntry = true
				break
			}
		}
		if t, ok := o.(*canvas.Text); ok && !inEntry && strings.TrimSpace(t.Text) != "" && t.Color != nil {
			if _, _, _, a := t.Color.RGBA(); a > 0 {
				*ids++
				*out = append(*out, textNode{t: t, drawn: drawnRect(t, abs), block: blk, id: *ids, order: order, clip: clip, path: append([]fyne.CanvasObject(nil), path...)})
			}
		}
		if r, ok := o.(*canvas.Rectangle); ok && len(path) > 0 {
			if _, _, _, a := fillRGBA(r.FillColor); a == 0xffff && r.Size().Width > 8 && r.Size().Height > 8 {
				*occ = append(*occ, occluder{order: order, r: rectOf(abs, r.Size()).intersect(clip), par: path[len(path)-1]})
			}
		}
		nclip, nblk := clip, blk
		if _, ok := o.(*container.Scroll); ok {
			nclip = clip.intersect(rectOf(abs, o.Size()))
		}
		if c, ok := o.(*fyne.Container); ok && hasBackground(c) {
			r := rectOf(abs, c.Size())
			nblk = &r
		}
		npath := append(path, o)
		for _, ch := range childrenOf(o) {
			walk(ch, abs, nclip, nblk, npath)
		}
	}
	walk(root, origin, clip, nil, nil)
}

func analyze(c fyne.Canvas) []finding {
	size := c.Size()
	full := rect{0, 0, size.Width, size.Height}
	var texts []textNode
	var occs []occluder
	var out []finding
	ids := 0
	roots := []fyne.CanvasObject{c.Content()}
	if ov := c.Overlays().List(); len(ov) > 0 {
		if os.Getenv("VEIL_UI_ALLROOTS") != "" {
			roots = append(roots, ov...)
		} else {
			roots = []fyne.CanvasObject{ov[len(ov)-1]}
		}
	}
	for _, r := range roots {
		collectTexts(r, fyne.NewPos(0, 0), full, &texts, &occs, &ids)
		negativeSized(r, &out)
	}
	for _, n := range texts {
		t := n.t
		natural := fyne.MeasureText(t.Text, t.TextSize, t.TextStyle)
		if t.Size().Width < natural.Width-layoutTolerance && t.Size().Width > 0 {
			out = append(out, finding{"squeezed", t.Text, n.drawn, rectOf(fyne.NewPos(0, 0), t.Size())})
		}
		if n.drawn.x0 < full.x0-layoutTolerance || n.drawn.x1 > full.x1+layoutTolerance {
			out = append(out, finding{"off-window", t.Text, n.drawn, full})
			continue
		}
		if n.block != nil {
			b := *n.block
			if n.drawn.x0 < b.x0-layoutTolerance || n.drawn.x1 > b.x1+layoutTolerance {
				out = append(out, finding{"outside-block", t.Text, n.drawn, b})
			}
		}
	}
	for _, n := range texts {
		vis := n.drawn.intersect(n.clip)
		area := vis.w() * vis.h()
		if vis.x1 <= vis.x0 || vis.y1 <= vis.y0 || area <= 0 {
			continue
		}
		for _, o := range occs {
			if o.order <= n.order || containsObj(n.path, o.par) {
				continue
			}
			if o.r.overlapArea(vis) > 0.35*area {
				out = append(out, finding{"covered", n.t.Text, vis, o.r})
				break
			}
		}
	}
	for i := range texts {
		for j := i + 1; j < len(texts); j++ {
			a, b := texts[i], texts[j]
			if a.block == nil || b.block == nil || *a.block != *b.block {
				continue
			}
			av, bv := a.drawn.intersect(a.clip), b.drawn.intersect(b.clip)
			if ov := av.overlapArea(bv); ov > 6 && ov > 0.12*min(av.w()*av.h(), bv.w()*bv.h()) {
				out = append(out, finding{"overlap", a.t.Text + "  ×  " + b.t.Text, av.intersect(bv), *a.block})
			}
		}
	}
	return out
}

type scenario struct {
	name     string
	sub      string
	long     string
	noSpaces string
}

var nameScenarios = []scenario{
	{name: "short", long: "Home"},
	{name: "medium", long: "Сервер в Амстердаме"},
	{name: "long", long: "TimeWeb VDS | Netherlands | Amsterdam | production gateway 01 (резервный канал)"},
	{name: "nospaces", long: "Veil-сервер-Нидерланды-Амстердам-основной-шлюз-Veil-сервер-Нидерланды-Амстердам-основной"},
}

const longIPv6 = "2a03:6f02:0000:0000:0000:0000:0001:039b"

func testProfiles(name string) []*config.Profile {
	return []*config.Profile{
		{Name: name, IPv4: "72.56.74.28", IPv6: longIPv6, Port: 443, Path: "/api/v2/abc", CertPin: "pin", AuthKey: "k",
			SSH: &config.SSHInfo{Port: 22, User: "root"}, FrontSNI: "ya.ru", AdminURL: "https://10.66.0.1/admin"},
		{Name: name + " (копия)", IPv4: "203.0.113.5", Port: 443, Path: "/x", CertPin: "pin", AuthKey: "k"},
		{Name: "Второй сервер", IPv6: longIPv6, Port: 443, Path: "/x", CertPin: "pin", AuthKey: "k", SSH: &config.SSHInfo{Port: 22, User: "deployer"}},
	}
}

func headlessUI(t *testing.T, lang string, dark bool, profiles []*config.Profile) *ui {
	t.Helper()
	dir := t.TempDir()
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "LOCALAPPDATA", "USERPROFILE"} {
		t.Setenv(k, dir)
	}
	for _, p := range profiles {
		path, err := config.ProfilePath(p.Name)
		if err != nil {
			t.Fatal(err)
		}
		if err := config.Save(path, p); err != nil {
			t.Fatal(err)
		}
	}
	a := test.NewApp()
	t.Cleanup(func() {
		time.Sleep(600 * time.Millisecond)
		test.NewApp()
	})
	a.Settings().SetTheme(veilTheme{})
	u := &ui{app: a, win: a.NewWindow("Veil"), selected: "", sessions: map[string]*deploy.Session{}}
	u.st = &settings.Settings{Lang: lang}
	if !dark {
		u.st.Theme = "light"
	} else {
		u.st.Theme = "dark"
	}
	applyPalette(u.st.Dark())
	curLang = u.st.Language()
	u.buildPages()
	u.win.SetPadded(false)
	u.win.SetContent(u.buildShell())
	u.showPage(0)
	u.reload()
	return u
}

func (u *ui) resizeTo(w, h float32) {
	u.win.Resize(fyne.NewSize(w, h))
	if c, ok := u.win.Canvas().(test.WindowlessCanvas); ok {
		c.Resize(fyne.NewSize(w, h))
	}
}

func (u *ui) relayoutPasses(n int) {
	size := u.win.Canvas().Size()
	for i := 0; i < n; i++ {
		u.win.Content().Refresh()
		for _, o := range u.win.Canvas().Overlays().List() {
			o.Refresh()
			o.Resize(size)
		}
		u.win.Content().Resize(size)
	}
}

func (u *ui) settle() {
	c := u.win.Canvas()
	for _, p := range u.subpages {
		if p.anim != nil {
			p.anim.Stop()
		}
		p.lay.off = 0
		p.root.Resize(c.Size())
		p.lay.Layout(p.root.Objects, p.root.Size())
	}
}

func (u *ui) dropOverlays() {
	c := u.win.Canvas()
	for _, p := range u.subpages {
		if p.anim != nil {
			p.anim.Stop()
		}
		p.shown = false
	}
	u.subpages = nil
	for _, o := range c.Overlays().List() {
		c.Overlays().Remove(o)
	}
}

func savePNG(c fyne.Canvas, dir, name string) {
	defer func() { _ = recover() }()
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, c.Capture())
}

func groupFindings(all map[string][]finding) []string {
	var lines []string
	for key, fs := range all {
		seen := map[string]bool{}
		for _, f := range fs {
			k := f.kind + "|" + f.text
			if seen[k] {
				continue
			}
			seen[k] = true
			lines = append(lines, key+"  "+f.String())
		}
	}
	sort.Strings(lines)
	return lines
}

// waitFrames ждёт, пока отработают отложенные первые кадры анимаций (см. afterFrame).
func waitFrames() {
	for i := 0; i < 300 && framePending.Load() > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
}

func init() { afterFrameHook = func(f func()) { f() } }
