package gui

// Связывание сетей через сервер: client-to-client, client-to-site, site-to-site.
// Сайт — локальная сеть за клиентом-шлюзом (его «Сети за клиентом»); на шлюзе туннель
// поднимается командой «veil connect -gateway», чтобы он пересылал трафик своей сети.
//
// Вкладка «Связи» страницы сервера (дизайн ServerLinks): слева схема, справа переключатели,
// сети за шлюзами и «Сохранить». Редактор сетей клиента (дизайн ClientSubnets) — openClientSubnets.

import (
	"fmt"
	"image/color"
	"net/netip"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"veil/internal/app/gui/kit"
	"veil/internal/config"
	"veil/internal/server"
)

// ---------- схема связей ----------

// Схема нарисована в координатах дизайна 380×420 и масштабируется по ширине колонки.
const (
	diagramW = 380
	diagramH = 420
)

type diagNode struct {
	x, y, w, h float32
	obj        fyne.CanvasObject
}

type diagArc struct {
	p     [4]fyne.Position // кубическая кривая Безье
	lines []*canvas.Line
	kind  int // 0 — клиент↔клиент, 1 — клиент↔сайт, 2 — сайт↔сайт
}

type linksDiagram struct {
	bg     *canvas.Rectangle
	spokes []*canvas.Line
	arcs   []*diagArc
	nodes  []diagNode
	labels [3]*canvas.Text
	labelB [3]fyne.CanvasObject // подписи на подложке: дуги проходят под ними
	labelP [3]fyne.Position
	legend fyne.CanvasObject
	on     [3]bool
	w      float32
	obj    *fyne.Container
}

const arcSegments = 22

func diagramNode(title, sub string, hub bool) fyne.CanvasObject {
	bg := canvas.NewRectangle(kit.ColCard2)
	bg.CornerRadius = 12
	tc, sc := kit.ColText, kit.ColMuted
	if hub {
		bg.FillColor = kit.ColAccent
		bg.CornerRadius = 14
		tc, sc = kit.ColAccentDark, kit.ColAccentDark
	}
	t := kit.NewFitText(title, 12, tc, true)
	s := kit.NewFitText(sub, 11, sc, false)
	if hub {
		t = kit.NewFitText(title, 13, tc, true)
		t.Alignment, s.Alignment = fyne.TextAlignCenter, fyne.TextAlignCenter
	}
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(0, 0, 10, 10), kit.Vcenter(container.New(kit.TightVBox{Gap: 0}, t, s))))
}

func legendItem(sample fyne.CanvasObject, label string) fyne.CanvasObject {
	return container.New(kit.Hrow{Gap: 6}, container.NewCenter(sample), kit.Text(label, 11, kit.ColMuted, false))
}

func legendLine(c color.Color, h float32, dashed bool) fyne.CanvasObject {
	if !dashed {
		r := canvas.NewRectangle(c)
		r.SetMinSize(fyne.NewSize(16, h))
		return r
	}
	a, b := canvas.NewRectangle(c), canvas.NewRectangle(c)
	a.SetMinSize(fyne.NewSize(6, h))
	b.SetMinSize(fyne.NewSize(6, h))
	return container.New(kit.Hrow{Gap: 4}, a, b)
}

func linkAllowedCol() color.Color { return kit.ColAccent }
func linkBlockedCol() color.Color { return kit.Mix(kit.ColCard2, kit.ColMuted, 0.35) }
func linkTunnelCol() color.Color  { return kit.Mix(kit.ColCard2, kit.ColMuted, 0.2) }

func trimHost(a string) string {
	a = strings.TrimSuffix(a, "/32")
	return strings.TrimSuffix(a, "/128")
}

func newLinksDiagram(clients []server.ClientInfo) *linksDiagram {
	d := &linksDiagram{bg: canvas.NewRectangle(kit.ColCard)}
	d.bg.CornerRadius = 16
	d.bg.StrokeColor, d.bg.StrokeWidth = kit.ColBorder, 1

	// до двух обычных клиентов слева и двух шлюзов (сайтов) справа
	var devs, sites []server.ClientInfo
	for _, c := range clients {
		if c.RelayOf != "" {
			continue
		}
		if len(c.Subnets) > 0 {
			sites = append(sites, c)
		} else {
			devs = append(devs, c)
		}
	}
	devNode := func(i int) fyne.CanvasObject {
		if i < len(devs) {
			return diagramNode(devs[i].Name, trimHost(devs[i].Address4), false)
		}
		return diagramNode(L("Device", "Устройство"), "—", false)
	}
	siteNode := func(i int) fyne.CanvasObject {
		if i < len(sites) {
			return diagramNode(sites[i].Name+L(" · site", " · сайт"), sites[i].Subnets[0], false)
		}
		return diagramNode(L("Site", "Сайт"), "—", false)
	}
	d.nodes = []diagNode{
		{130, 182, 120, 56, diagramNode(L("Veil server", "Сервер Veil"), L("hub", "хаб"), true)},
		{14, 96, 132, 48, devNode(0)},
		{14, 276, 132, 48, devNode(1)},
		{234, 96, 132, 48, siteNode(0)},
		{234, 276, 132, 48, siteNode(1)},
	}
	for range 4 {
		l := canvas.NewLine(linkTunnelCol())
		l.StrokeWidth = 2
		d.spokes = append(d.spokes, l)
	}
	pt := func(x, y float32) fyne.Position { return fyne.NewPos(x, y) }
	d.arcs = []*diagArc{
		{p: [4]fyne.Position{pt(44, 140), pt(10, 210), pt(10, 210), pt(44, 280)}, kind: 0},
		{p: [4]fyne.Position{pt(336, 140), pt(370, 210), pt(370, 210), pt(336, 280)}, kind: 2},
		{p: [4]fyne.Position{pt(110, 92), pt(190, 40), pt(190, 40), pt(270, 92)}, kind: 1},
		{p: [4]fyne.Position{pt(110, 328), pt(190, 380), pt(190, 380), pt(270, 328)}, kind: 1},
	}
	for _, a := range d.arcs {
		for range arcSegments {
			l := canvas.NewLine(linkAllowedCol())
			l.StrokeWidth = 3
			a.lines = append(a.lines, l)
		}
	}
	names := [3]string{L("c↔c", "к↔к"), L("c↔s", "к↔с"), L("s↔s", "с↔с")}
	d.labelP = [3]fyne.Position{pt(18, 210), pt(190, 53), pt(362, 210)}
	for i := range d.labels {
		d.labels[i] = kit.Text(names[i], 11, linkAllowedCol(), true)
		pill := canvas.NewRectangle(kit.ColCard)
		pill.CornerRadius = 6
		d.labelB[i] = container.NewStack(pill, container.New(layout.NewCustomPaddedLayout(1, 1, 4, 4), d.labels[i]))
	}
	d.legend = container.New(kit.Hrow{Gap: 14},
		legendItem(legendLine(linkAllowedCol(), 3, false), L("allowed", "разрешено")),
		legendItem(legendLine(linkBlockedCol(), 3, true), L("blocked", "запрещено")),
		legendItem(legendLine(linkTunnelCol(), 2, false), L("tunnel", "туннель")),
	)

	objs := []fyne.CanvasObject{d.bg}
	for _, l := range d.spokes {
		objs = append(objs, l)
	}
	for _, a := range d.arcs {
		for _, l := range a.lines {
			objs = append(objs, l)
		}
	}
	for _, n := range d.nodes {
		objs = append(objs, n.obj)
	}
	for _, b := range d.labelB {
		objs = append(objs, b)
	}
	objs = append(objs, d.legend)
	d.obj = container.New(d, objs...)
	return d
}

func (d *linksDiagram) scale(w float32) float32 {
	return max(0.6, min(w/diagramW, 1.25))
}

func (d *linksDiagram) Layout(_ []fyne.CanvasObject, s fyne.Size) {
	d.w = s.Width
	k := d.scale(s.Width)
	ox := (s.Width - diagramW*k) / 2
	at := func(p fyne.Position) fyne.Position { return fyne.NewPos(ox+p.X*k, p.Y*k) }
	d.bg.Move(fyne.NewPos(0, 0))
	d.bg.Resize(s)
	hub := fyne.NewPos(190, 210)
	ends := []fyne.Position{{X: 80, Y: 120}, {X: 80, Y: 300}, {X: 300, Y: 120}, {X: 300, Y: 300}}
	for i, l := range d.spokes {
		l.Position1, l.Position2 = at(ends[i]), at(hub)
		l.Refresh()
	}
	for _, a := range d.arcs {
		on := d.on[a.kind]
		prev := a.p[0]
		for i, l := range a.lines {
			t := float32(i+1) / arcSegments
			u := 1 - t
			x := u*u*u*a.p[0].X + 3*u*u*t*a.p[1].X + 3*u*t*t*a.p[2].X + t*t*t*a.p[3].X
			y := u*u*u*a.p[0].Y + 3*u*u*t*a.p[1].Y + 3*u*t*t*a.p[2].Y + t*t*t*a.p[3].Y
			cur := fyne.NewPos(x, y)
			l.Position1, l.Position2 = at(prev), at(cur)
			prev = cur
			if on {
				l.StrokeColor = linkAllowedCol()
				l.Show()
			} else {
				l.StrokeColor = linkBlockedCol()
				// пунктир: каждый второй отрезок не рисуется
				if i%2 == 1 {
					l.Hide()
				} else {
					l.Show()
				}
			}
			l.Refresh()
		}
	}
	for _, n := range d.nodes {
		n.obj.Move(at(fyne.NewPos(n.x, n.y)))
		n.obj.Resize(fyne.NewSize(n.w*k, n.h*k))
	}
	for i, l := range d.labels {
		b := d.labelB[i]
		bm := b.MinSize()
		p := at(d.labelP[i])
		switch i {
		case 0: // слева: по центру дуги
			p.X = at(fyne.NewPos(18, 0)).X - bm.Width/2
		case 1:
			p.X = at(fyne.NewPos(190, 0)).X - bm.Width/2
		case 2:
			p.X = at(fyne.NewPos(362, 0)).X - bm.Width/2
		}
		p.Y -= bm.Height / 2
		b.Move(p)
		b.Resize(bm)
		if d.on[i] {
			l.Color = linkAllowedCol()
		} else {
			l.Color = kit.ColIdle
		}
		l.Refresh()
	}
	lm := d.legend.MinSize()
	d.legend.Resize(lm)
	d.legend.Move(fyne.NewPos(max(14, (s.Width-lm.Width)/2), s.Height-12-lm.Height))
}

func (d *linksDiagram) MinSize([]fyne.CanvasObject) fyne.Size {
	w := d.w
	if w <= 0 {
		w = diagramW
	}
	return fyne.NewSize(0, diagramH*d.scale(w))
}

func (d *linksDiagram) set(l config.Links) {
	d.on = [3]bool{l.ClientToClient, l.ClientToSite, l.SiteToSite}
	d.obj.Refresh()
}

// ---------- вкладка «Связи» ----------

func linksHint(dirty, flash bool) (string, color.Color) {
	switch {
	case flash:
		return L("Saved. Connections that are already open are not dropped.", "Сохранено. Уже открытые соединения не разрываются."), kit.ColOK
	case dirty:
		return L("There are unsaved changes", "Есть несохранённые изменения"), kit.ColMuted
	}
	return L("The rules take effect on the server right after saving", "Правила действуют на сервере сразу после сохранения"), kit.ColMuted
}

func setFlow(t textSetter, s string, c color.Color) {
	t.SetText(s)
	if cs, ok := t.(interface{ SetColor(color.Color) }); ok {
		cs.SetColor(c)
	}
}

func (m *manager) renderLinks() {
	saved := m.state.Server.Links
	cur := saved
	d := newLinksDiagram(m.state.Clients)
	d.set(cur)
	hint := kit.NewFlowText("", 12, kit.ColMuted, false, 17)
	var save *saveButton
	update := func(flash bool) {
		dirty := cur != saved
		save.SetEnabled(dirty)
		s, c := linksHint(dirty, flash)
		setFlow(hint, s, c)
		d.set(cur)
	}
	save = newSaveButton(L("Save", "Сохранить"), func() {
		next := cur
		m.call("POST", "/settings/links", next, nil, func(err error) {
			if err != nil {
				kit.ShowError(err, m.w)
				return
			}
			saved = next
			m.state.Server.Links = next
			update(true)
		})
	})

	right := kit.NewVBoxFit()
	type sw struct {
		title, sub string
		val        *bool
	}
	for _, r := range []sw{
		{L("Client ↔ client", "Клиент ↔ клиент"), L("VPN devices see each other by tunnel addresses", "Устройства VPN видят друг друга по адресам туннеля"), &cur.ClientToClient},
		{L("Client ↔ site", "Клиент ↔ сайт"), L("VPN devices reach networks behind gateways, and back", "Устройства VPN попадают в сети за шлюзами, и обратно"), &cur.ClientToSite},
		{L("Site ↔ site", "Сайт ↔ сайт"), L("Networks behind different gateways are linked to each other", "Сети за разными шлюзами связаны между собой"), &cur.SiteToSite},
	} {
		r := r
		row, _ := kit.SwitchRow(r.title, r.sub, *r.val, func(on bool) {
			*r.val = on
			update(false)
		})
		right.Add(row)
	}
	right.Add(kit.SectionLabel(L("Networks behind gateways", "Сети за шлюзами")))
	hasNets := false
	for _, c := range m.state.Clients {
		c := c
		for _, n := range c.Subnets {
			hasNets = true
			right.Add(kit.NavRow(n, c.Name, func() { m.openClientSubnets(c, m.refresh) }))
		}
	}
	if !hasNets {
		right.Add(infoNote(L("No networks behind gateways yet: set them on a client page (“Networks behind client”). A site is a local network behind a gateway client.",
			"Сетей за шлюзами пока нет: задайте их на странице клиента («Сети за клиентом»). Сайт — локальная сеть за клиентом-шлюзом.")))
	}
	right.Add(hintAndButton(hint, save.Object()))
	update(false)
	m.body.Add(newSplit(diagramW, 0, 700, d.obj, right))
}

// ---------- «Сети за клиентом (site)» ----------

// defaultTunnel — подсеть туннеля Veil по умолчанию.
var defaultTunnel = netip.MustParsePrefix("10.66.0.0/24")

// tunnelNets — подсети туннеля: по умолчанию и по адресам клиентов (/24 для IPv4, /64 для IPv6).
func (m *manager) tunnelNets() []netip.Prefix {
	out := []netip.Prefix{defaultTunnel}
	add := func(a string, bits int) {
		p, err := netip.ParsePrefix(a)
		if err != nil {
			return
		}
		n, err := p.Addr().Prefix(bits)
		if err == nil && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, c := range m.state.Clients {
		add(c.Address4, 24)
		add(c.Address6, 64)
	}
	return out
}

// checkSubnet проверяет подсеть для клиента self и возвращает её в каноническом виде или текст ошибки.
func (m *manager) checkSubnet(raw string, list []string, self string) (string, string) {
	v := strings.TrimSpace(raw)
	bad := L("Not a subnet: for example, 192.168.1.0/24", "Не подсеть: нужно, например, 192.168.1.0/24")
	if v == "" {
		return "", bad
	}
	var p netip.Prefix
	if strings.Contains(v, "/") {
		var err error
		if p, err = netip.ParsePrefix(v); err != nil {
			if a, aerr := netip.ParseAddr(strings.SplitN(v, "/", 2)[0]); aerr == nil && a.Is4() {
				return "", L("IPv4 prefix length is up to /32", "Длина префикса IPv4 — до /32")
			}
			return "", bad
		}
	} else {
		a, err := netip.ParseAddr(v)
		if err != nil || a.Zone() != "" {
			return "", bad
		}
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if p.Addr().Is4In6() {
		return "", bad
	}
	p = p.Masked()
	a := p.Addr()
	if (a.Is4() && p.Bits() < 8) || (a.Is6() && p.Bits() < 16) {
		return "", L("Too wide a network — specify the gateway's local network", "Слишком широкая сеть — укажите локальную сеть шлюза")
	}
	if a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() {
		return "", L("Service addresses cannot be routed", "Служебные адреса нельзя маршрутизировать")
	}
	for _, t := range m.tunnelNets() {
		if p.Overlaps(t) {
			return "", fmt.Sprintf(L("Overlaps the tunnel subnet %s", "Пересекается с подсетью туннеля %s"), t)
		}
	}
	for _, x := range list {
		px, err := netip.ParsePrefix(x)
		if err != nil {
			continue
		}
		if px.Masked() == p {
			return "", L("Already in the list", "Уже есть в списке")
		}
		if px.Overlaps(p) {
			return "", fmt.Sprintf(L("Overlaps %s from the list", "Пересекается с %s из списка"), x)
		}
	}
	for _, c := range m.state.Clients {
		if c.Name == self {
			continue
		}
		for _, x := range c.Subnets {
			if px, err := netip.ParsePrefix(x); err == nil && px.Overlaps(p) {
				return "", fmt.Sprintf(L("Already set for client “%s”", "Уже задана у клиента «%s»"), c.Name)
			}
		}
	}
	return p.String(), ""
}

// shellArg — имя профиля для команды в оболочке: в кавычках, если в нём есть пробелы.
func shellArg(s string) string {
	if strings.ContainsAny(s, " \t'\"$`\\|&;()<>*?") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// subnetChip — подсеть-«чип» с кнопкой удаления.
func subnetChip(cidr string, remove func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(kit.ColCard2)
	bg.CornerRadius = 9
	del := kit.NewTapCardR(container.NewCenter(kit.IconImg("close", kit.ColMuted, 14)), color.Transparent, kit.ColCardHover, 6, 1, remove)
	row := container.New(kit.Hrow{Gap: 6}, container.NewCenter(kit.Text(cidr, 13, kit.ColText, false)), container.NewGridWrap(fyne.NewSize(24, 24), del))
	return container.New(&kit.FillMin{H: 34}, container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(5, 5, 12, 5), row)))
}

// openClientSubnets — страница «Сети за клиентом (site)»: подсети локальной сети, шлюзом которой служит клиент.
func (m *manager) openClientSubnets(c server.ClientInfo, saved func()) *subPage {
	w := m.u.newSubPage(c.Name)
	list := append([]string(nil), c.Subnets...)
	orig := append([]string(nil), c.Subnets...)

	addr := trimHost(c.Address4)
	if addr == "" {
		addr = trimHost(c.Address6)
	}
	head := kit.NewFlowText("", 13, kit.ColMuted, false, 18)
	setHead := func() {
		kind := L("device", "устройство")
		if len(list) > 0 {
			kind = L("site gateway", "шлюз сайта")
		}
		if addr != "" {
			kind = addr + " · " + kind
		}
		head.SetText(kind)
	}

	body := kit.NewVBoxFit()
	chips := container.New(&flowRow{gap: 8})
	errT := kit.NewFlowText("", 12, kit.ColBad, true, 16)
	errT.Hide()
	hint := kit.NewFlowText("", 12, kit.ColMuted, false, 17)
	var save *saveButton
	var renderChips func()
	update := func(flash bool) {
		dirty := !slices.Equal(list, orig)
		save.SetEnabled(dirty)
		switch {
		case flash:
			setFlow(hint, L("Saved: routes and rules on the server are updated", "Сохранено: маршруты и правила на сервере обновлены"), kit.ColOK)
		case dirty:
			setFlow(hint, L("There are unsaved changes", "Есть несохранённые изменения"), kit.ColMuted)
		default:
			setFlow(hint, L("Changes apply immediately, without a restart", "Изменения применяются сразу, без перезапуска"), kit.ColMuted)
		}
		setHead()
	}
	renderChips = func() {
		chips.RemoveAll()
		for _, n := range list {
			n := n
			chips.Add(subnetChip(n, func() {
				list = slices.DeleteFunc(list, func(x string) bool { return x == n })
				renderChips()
				update(false)
			}))
		}
		if len(list) == 0 {
			chips.Add(container.New(&kit.FillMin{H: 34}, kit.Vcenter(kit.Text(L("None — an ordinary device", "Нет — обычное устройство"), 13, kit.ColIdle, false))))
		}
		chips.Refresh()
		body.Refresh()
	}
	save = newSaveButton(L("Save", "Сохранить"), func() {
		next := append([]string(nil), list...)
		m.call("POST", "/clients/subnets", map[string]any{"name": c.Name, "subnets": next}, nil, func(err error) {
			if err != nil {
				kit.ShowError(err, w)
				return
			}
			orig = next
			update(true)
			if saved != nil {
				saved()
			}
		})
	})

	entry := widget.NewEntry()
	entry.SetPlaceHolder("192.168.1.0/24")
	showErr := func(msg string) {
		errT.SetText(msg)
		if msg == "" {
			errT.Hide()
		} else {
			errT.Show()
		}
		body.Refresh()
	}
	add := func() {
		v, msg := m.checkSubnet(entry.Text, list, c.Name)
		if msg != "" {
			showErr(msg)
			return
		}
		list = append(list, v)
		entry.SetText("")
		showErr("")
		renderChips()
		update(false)
	}
	entry.OnSubmitted = func(string) { add() }
	entry.OnChanged = func(string) {
		if errT.Visible() {
			showErr("")
		}
	}
	addBtn := container.New(&minWidth{w: 130}, bigButton(L("Add", "Добавить"), "plus", false, add))
	input := container.NewBorder(nil, nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 8, 0), addBtn), fieldEntry(L("New subnet", "Новая подсеть"), entry))

	cmd := "sudo veil connect -gateway " + shellArg(c.Name)
	gwText := container.New(kit.TightVBox{Gap: 6},
		kit.NewFlowText(L("On the gateway itself (Linux)", "На самом шлюзе (Linux)"), 12, kit.ColMuted, true, 16),
		kit.NewFlowText(cmd, 13, kit.ColText, false, 18),
		kit.NewFlowText(fmt.Sprintf(L("Hosts of the network send traffic to %s and other sites through the gateway. Tap to copy the command.", "Хосты сети отправляют трафик к %s и другим сайтам через шлюз. Нажмите, чтобы скопировать команду."), defaultTunnel), 12, kit.ColMuted, false, 17),
	)
	gwCard, _ := padCard(gwText, 12, 14, 0)
	gw := kit.NewRowTap(gwCard, func() { m.u.app.Clipboard().SetContent(cmd) })

	body.Add(head)
	body.Add(kit.SectionLabel(L("Networks behind client (site)", "Сети за клиентом (site)")))
	body.Add(kit.NewFlowText(L("If this device is the gateway of a local network (a router, a server in the office), list its subnets. The server will route them to this client; who may reach them is set on the “Links” tab.",
		"Если это устройство — шлюз локальной сети (роутер, сервер в офисе), перечислите её подсети. Сервер будет направлять их этому клиенту; кому они доступны — во вкладке «Связи»."), 13, kit.ColMuted, false, 18))
	body.Add(chips)
	body.Add(input)
	body.Add(errT)
	body.Add(gw)
	renderChips()
	update(false)
	pageWithFooter(w, body, hintAndButton(hint, save.Object()))
	w.Show()
	return w
}
