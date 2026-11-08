package gui

import (
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"veil/internal/config"
	"veil/internal/deploy"
	"veil/internal/server"
)

const layoutPasses = 3

type uiPage struct {
	name string
	open func(u *ui)
}

type uiSize struct{ w, h float32 }

func (s uiSize) String() string { return fmt.Sprintf("%.0fx%.0f", s.w, s.h) }

var matrixSizes = []uiSize{
	{360, 560}, {360, 800}, {393, 760}, {420, 640}, {480, 640}, {520, 700}, {520, 560}, {600, 700}, {719, 700}, {720, 700}, {820, 700}, {1000, 700}, {1280, 800},
}

func demoState(name string) server.State {
	long := name + " — основной клиент с очень длинным именем для проверки"
	return server.State{
		Server: server.ServerInfo{
			Version: "1.0.0", Company: "Горизонт Инжиниринг Партнёры Северо-Запад", PublicIPv4: "72.56.74.28", PublicIPv6: longIPv6,
			Port: 443, Domain: "very-long-subdomain.example-company-name.international", AdminURL: "https://10.66.0.1/admin",
			Started: time.Now().Add(-49 * time.Hour), Clients: 3, Online: 2, RxBytes: 12 << 30, TxBytes: 3 << 30,
			Front: server.FrontInfo{Enabled: true, SNI: "ya.ru", Dest: "ya.ru:443"},
			Links: config.Links{ClientToClient: true, ClientToSite: true},
		},
		Clients: []server.ClientInfo{
			{Name: long, Note: "Ноутбук бухгалтерии, подключается из офиса и из дома, заметка тоже длинная", Address4: "10.66.0.2/32", Address6: "fd00:7e11:1234:5678::2/128",
				Online: true, Sessions: []server.SessionInfo{{Transport: "HTTP/3", Remote: "[2a03:6f02:0000:0000:0000:0000:0001:039b]:54493", Since: time.Now()}}, RxBytes: 5 << 30, TxBytes: 1 << 30,
				Route: []string{"Амстердам-основной-шлюз", "Франкфурт-резервный-узел"}},
			{Name: "Телефон", Address4: "10.66.0.3/32", Address6: "fd00:7e11:1234:5678::3/128", Disabled: true, Sessions: []server.SessionInfo{}, Subnets: []string{"192.168.1.0/24", "10.20.0.0/16"}},
			{Name: "Транзит от соседнего узла каскада с длинным названием", RelayOf: "соседний-узел", Address4: "10.66.0.4/32", Online: true,
				Sessions: []server.SessionInfo{}, Auto: &server.AutoInfo{Exit: "Франкфурт-резервный-узел", CostMs: 42}},
		},
		Relays:    []server.RelayInfo{{Name: "Узел каскада с очень длинным названием для проверки", Clients: 2, Online: true}},
		Upstreams: []server.UpstreamInfo{{Name: "Франкфурт-резервный-узел-с-очень-длинным-названием", Company: "Горизонт Инжиниринг Партнёры", IPv6: longIPv6, Domain: "frankfurt.example-company-name.international", Clients: 2}},
		Latency:   []server.LinkLatency{{Name: "Франкфурт-резервный-узел-с-очень-длинным-названием", Company: "Горизонт", RTTms: 38.5, OK: true}},
	}
}

func managerPage(i int) uiPage {
	return uiPage{fmt.Sprintf("manager-%d", i), func(u *ui) {
		p := u.profiles[0]
		for _, c := range u.profiles {
			if managed(c) {
				p = c
				break
			}
		}
		m := u.buildManager(p)
		m.state = demoState(p.Name)
		m.loaded = true
		m.show(i)
	}}
}

func matrixPages() []uiPage {
	cur := func(f func(u *ui, p *config.Profile)) func(u *ui) {
		return func(u *ui) {
			if p := u.current(); p != nil {
				f(u, p)
			}
		}
	}
	managedOf := func(u *ui) *manager {
		for _, p := range u.profiles {
			if managed(p) {
				m := u.buildManager(p)
				m.state, m.loaded = demoState(p.Name), true
				return m
			}
		}
		return nil
	}
	pages := []uiPage{
		{"home", func(u *ui) { u.showPage(0) }},
		{"share", func(u *ui) { u.showPage(1) }},
		{"share-users", func(u *ui) { u.showPage(1); u.share.tab = 1; u.share.render() }},
		{"settings", func(u *ui) { u.showPage(2) }},
		{"add", func(u *ui) { u.showPage(3) }},
		{"servers", func(u *ui) { u.openServersPage() }},
		{"connection", func(u *ui) { u.openConnection() }},
		{"dns", func(u *ui) { u.openDNS() }},
		{"split", func(u *ui) { u.openSplit() }},
		{"splitapps", func(u *ui) { u.openSplitApps() }},
		{"splitsheet", func(u *ui) { u.openSplitSheet() }},
		{"serversheet", func(u *ui) { u.openServerSheet() }},
		{"details", cur(func(u *ui, p *config.Profile) { u.showDetails(p) })},
		{"killswitch", func(u *ui) { u.openKillSwitch() }},
		{"application", func(u *ui) { u.openApplication() }},
		{"logging", func(u *ui) { u.openLogging() }},
		{"backup", func(u *ui) { u.openBackup() }},
		{"about", func(u *ui) { u.openAbout() }},
		{"appearance-theme", func(u *ui) { u.openAppearance(true) }},
		{"appearance-lang", func(u *ui) { u.openAppearance(false) }},
		{"deploy-form", func(u *ui) { u.showDeployForm() }},
		{"import", func(u *ui) { u.showImport() }},
		{"add-chooser", func(u *ui) { u.showAddChooser() }},
		{"multi-entry", func(u *ui) { u.openMultiEntryBuilder() }},
		{"chain", cur(func(u *ui, p *config.Profile) { u.openChainBuilder(p) })},
		{"wg-install", func(u *ui) {
			if m := managedOf(u); m != nil {
				u.openWGInstall(m)
			}
		}},
		{"awg-install", func(u *ui) {
			if m := managedOf(u); m != nil {
				u.openAWGInstall(m)
			}
		}},
		{"ov-install", func(u *ui) {
			if m := managedOf(u); m != nil {
				u.openOVInstall(m)
			}
		}},
		{"ipsec-install", func(u *ui) {
			if m := managedOf(u); m != nil {
				u.openIPsecInstall(m)
			}
		}},
	}
	for i := 0; i < len(managerTabs()); i++ {
		pages = append(pages, managerPage(i))
	}
	pages = append(pages, measureExtra()...)
	pages = append(pages, screenPages()...)
	return pages
}

type matrixResult struct {
	findings map[string][]finding
	panics   map[string]string
	cases    int
}

func runMatrix(t *testing.T, langs []string, dark bool, scenarios []scenario, sizes []uiSize, snapDir string) matrixResult {
	res := matrixResult{findings: map[string][]finding{}, panics: map[string]string{}}
	for _, lang := range langs {
		for _, sc := range scenarios {
			u := headlessUI(t, lang, dark, testProfiles(sc.long))
			u.sessions = map[string]*deploy.Session{}
			u.selected = sc.long
			u.reload()
			for _, pg := range matrixPages() {
				if only := os.Getenv("VEIL_UI_PAGES"); only != "" && !strings.Contains(","+only+",", ","+pg.name+",") {
					continue
				}
				for _, sz := range sizes {
					key := fmt.Sprintf("%s/%s/%s/%s", lang, sc.name, pg.name, sz)
					func() {
						defer func() {
							if r := recover(); r != nil {
								res.panics[key] = fmt.Sprint(r) + "\n" + string(debug.Stack())
							}
						}()
						u.dropOverlays()
						u.resizeTo(sz.w, sz.h)
						pg.open(u)
						u.settle()
						u.relayoutPasses(layoutPasses)
						u.settle()
						res.cases++
						if fs := analyze(u.win.Canvas()); len(fs) > 0 {
							res.findings[key] = fs
							if snapDir != "" {
								savePNG(u.win.Canvas(), snapDir, strings.ReplaceAll(key, "/", "_"))
							}
						}
					}()
				}
			}
		}
	}
	return res
}

func summarize(res matrixResult) []string {
	type agg struct {
		page, kind, text string
		keys             []string
	}
	by := map[string]*agg{}
	for key, fs := range res.findings {
		parts := strings.Split(key, "/")
		for _, f := range fs {
			k := parts[2] + "|" + f.kind + "|" + f.text
			a := by[k]
			if a == nil {
				a = &agg{page: parts[2], kind: f.kind, text: f.text}
				by[k] = a
			}
			a.keys = append(a.keys, parts[0]+"/"+parts[1]+"/"+parts[3])
		}
	}
	var lines []string
	for _, a := range by {
		sizes := map[string]bool{}
		scs := map[string]bool{}
		for _, k := range a.keys {
			p := strings.Split(k, "/")
			scs[p[1]] = true
			sizes[p[2]] = true
		}
		var sl, cl []string
		for s := range sizes {
			sl = append(sl, s)
		}
		for s := range scs {
			cl = append(cl, s)
		}
		sort.Strings(sl)
		sort.Strings(cl)
		txt := a.text
		if len(txt) > 70 {
			txt = txt[:70] + "…"
		}
		lines = append(lines, fmt.Sprintf("%-18s %-13s %q  names=%s sizes=%s", a.page, a.kind, txt, strings.Join(cl, ","), strings.Join(sl, ",")))
	}
	sort.Strings(lines)
	return lines
}

func TestUIMatrixOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	snap := os.Getenv("VEIL_UI_SNAP")
	res := runMatrix(t, []string{"ru", "en"}, true, nameScenarios, matrixSizes, snap)
	t.Logf("проверено сочетаний: %d, с замечаниями: %d, аварий: %d", res.cases, len(res.findings), len(res.panics))
	for k, v := range res.panics {
		t.Errorf("авария %s: %s", k, v)
	}
	lines := summarize(res)
	for _, l := range lines {
		t.Log(l)
	}
	if len(lines) > 0 {
		t.Errorf("текст выходит за пределы блоков: %d различных замечаний", len(lines))
	}
}
