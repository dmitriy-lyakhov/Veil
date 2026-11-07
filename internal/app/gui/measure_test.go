package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/server"
)

// fakeCall — подмена ответов сервера для страниц с запросами (проверка вёрстки без SSH).
func fakeCall(method, path string, in, out any, done func(error)) {
	if path == "/explore" {
		_ = json.Unmarshal([]byte(`{"upstreams":["Токио","Франкфурт"]}`), out)
	}
	if strings.HasPrefix(path, "/clients") && out != nil {
		if r, ok := out.(*server.LinkResponse); ok {
			*r = server.LinkResponse{Name: "Телефон Ани", Link: "veil://eyJob3N0IjoiMjAzLjAuMTEzLjEwIn0", Via: ""}
		}
	}
	done(nil)
}

func demoManager(u *ui) *manager {
	for _, p := range u.profiles {
		if managed(p) {
			m := &manager{u: u, p: p, w: u.win, status: kit.NewStatusLine(), body: kit.NewVBoxFit(), callHook: fakeCall}
			m.state, m.loaded = demoState(p.Name), true
			m.roles = map[string]string{m.state.Clients[0].Name: "admin"}
			return m
		}
	}
	return nil
}

type dumpNode struct {
	kind string
	text string
	x, y float32
	w, h float32
	fs   float32
	rad  float32
	fill string
}

func walkDump(o fyne.CanvasObject, off fyne.Position, out *[]dumpNode) {
	if o == nil || !o.Visible() {
		return
	}
	pos := off.Add(o.Position())
	switch v := o.(type) {
	case *canvas.Text:
		if v.Text != "" {
			*out = append(*out, dumpNode{kind: "T", text: v.Text, x: pos.X, y: pos.Y, w: v.Size().Width, h: v.Size().Height, fs: v.TextSize})
		}
	case *canvas.Rectangle:
		if _, _, _, a := fillRGBA(v.FillColor); a > 0 || v.StrokeWidth > 0 {
			*out = append(*out, dumpNode{kind: "R", x: pos.X, y: pos.Y, w: v.Size().Width, h: v.Size().Height, rad: v.CornerRadius})
		}
	case *widget.RichText:
		for _, s := range v.Segments {
			if ts, ok := s.(*widget.TextSegment); ok && ts.Text != "" {
				*out = append(*out, dumpNode{kind: "RT", text: ts.Text, x: pos.X, y: pos.Y, w: v.Size().Width, h: v.Size().Height})
			}
		}
		return
	}
	for _, ch := range childrenOf(o) {
		walkDump(ch, pos, out)
	}
}

// TestMeasureDump печатает геометрию страницы (текст, карточки) для сверки с замерами DOM дизайна.
// Запуск: VEIL_MEASURE=clientnew,... VEIL_MEASURE_OUT=<папка> go test -run TestMeasureDump ./internal/app/gui
func TestMeasureDump(t *testing.T) {
	want := os.Getenv("VEIL_MEASURE")
	outDir := os.Getenv("VEIL_MEASURE_OUT")
	if want == "" || outDir == "" {
		t.Skip("VEIL_MEASURE не задан")
	}
	u := headlessUI(t, os.Getenv("VEIL_MEASURE_LANG"), os.Getenv("VEIL_MEASURE_THEME") != "light", testProfiles("Амстердам"))
	pages := matrixPages()
	for _, pg := range pages {
		if !strings.Contains(","+want+",", ","+pg.name+",") && want != "all" {
			continue
		}
		u.dropOverlays()
		u.resizeTo(1000, 668)
		pg.open(u)
		u.settle()
		u.relayoutPasses(layoutPasses)
		u.settle()
		var nodes []dumpNode
		c := u.win.Canvas()
		if ovs := c.Overlays().List(); len(ovs) > 0 {
			for _, ov := range ovs {
				walkDump(ov, fyne.NewPos(0, 0), &nodes)
			}
		} else {
			walkDump(c.Content(), fyne.NewPos(0, 0), &nodes)
		}
		var sb strings.Builder
		for _, n := range nodes {
			if n.kind == "R" {
				fmt.Fprintf(&sb, "R|%.0f|%.0f|%.1f|%.1f|r%.0f\n", n.x, n.y, n.w, n.h, n.rad)
			} else {
				fmt.Fprintf(&sb, "%s \"%.28s\"|%.0f|%.0f|%.1f|%.1f|%.0f\n", n.kind, n.text, n.x, n.y, n.w, n.h, n.fs)
			}
		}
		_ = os.MkdirAll(outDir, 0o755)
		if err := os.WriteFile(filepath.Join(outDir, pg.name+".txt"), []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func measureExtra() []uiPage {
	return []uiPage{
		{"client-new", func(u *ui) {
			if m := demoManager(u); m != nil {
				m.addClient()
			}
		}},
		{"client-detail", func(u *ui) {
			if m := demoManager(u); m != nil {
				m.openClient(m.state.Clients[0].Name)
			}
		}},
		{"share-result", func(u *ui) {
			u.showShare(u.win, "Клиент «Телефон Ани»", server.LinkResponse{Name: "Телефон Ани", Link: "veil://eyJob3N0IjoiMjAzLjAuMTEzLjEwIn0"}, L("Scan the QR in the Veil app on the client device, or send the link or file.", "Отсканируйте QR в приложении Veil на устройстве клиента, отправьте ссылку или файл."), true, infoRow(L("Role", "Роль"), roleName("user"), nil))
		}},
		{"users-list", func(u *ui) {
			if m := demoManager(u); m != nil {
				u.showPage(1)
				u.share.tab = 1
				u.share.mgrs[m.p.Name] = m
				u.share.server = m.p.Name
				m.clientsOnly = true
				u.share.render()
			}
		}},
	}
}
