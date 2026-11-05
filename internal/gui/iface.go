package gui

import (
	"image/color"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"net"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"veil/internal/client"
)

type ifaceChoice struct {
	name   string // пусто — автоматически
	addrs  []string
	active bool // интерфейс поднят и у него есть адрес
	gw     string
}

// subtitle — вторая строка радио-строки: «адрес · шлюз …» или причина, почему интерфейс не годится.
func (c ifaceChoice) subtitle() string {
	if c.name == "" {
		return L("Recommended", "Рекомендуется")
	}
	if !c.active {
		if len(c.addrs) == 0 {
			return L("inactive · no address", "не активен · нет адреса")
		}
		return L("inactive", "не активен")
	}
	s := strings.Join(c.addrs, ", ")
	if c.gw != "" {
		s += " · " + L("gateway", "шлюз") + " " + c.gw
	}
	return s
}

// networkInterfaces — интерфейсы, на которых может работать приложение: все не петлевые, кроме собственных
// туннельных адаптеров Veil; подходящие (активные, с адресом) идут первыми.
func networkInterfaces() []ifaceChoice {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []ifaceChoice
	for _, ifc := range ifs {
		low := strings.ToLower(ifc.Name)
		if ifc.Flags&net.FlagLoopback != 0 || strings.Contains(low, "veil") || strings.Contains(low, "wintun") {
			continue
		}
		c := ifaceChoice{name: ifc.Name}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLinkLocalUnicast() {
				c.addrs = append(c.addrs, ipn.IP.String())
			}
		}
		c.active = ifc.Flags&net.FlagUp != 0 && len(c.addrs) > 0
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].active != out[j].active {
			return out[i].active
		}
		return out[i].name < out[j].name
	})
	return out
}

// ifaceAvailable — выбранный интерфейс сейчас существует и активен (пустое имя — «автоматически» — всегда годится).
func ifaceAvailable(name string, list []ifaceChoice) bool {
	if name == "" {
		return true
	}
	for _, c := range list {
		if c.name == name {
			return c.active
		}
	}
	return false
}

func (u *ui) interfaceLabel() string {
	if u.st.Interface == "" {
		return L("Automatic", "Автоматически")
	}
	return u.st.Interface
}

// openInterface — страница «Сетевой интерфейс»: через какой интерфейс VPN выходит к серверу (например, только VLAN 200).
func (u *ui) openInterface() {
	list := networkInterfaces()
	known := u.st.Interface == ""
	for _, c := range list {
		if c.name == u.st.Interface {
			known = true
		}
	}
	if !known {
		list = append(list, ifaceChoice{name: u.st.Interface})
	}
	rows := newVBoxFit()
	hint := container.NewStack()
	var render func()
	render = func() {
		rows.RemoveAll()
		add := func(c ifaceChoice, title string) {
			rows.Add(radioRow(title, c.subtitle(), c.name == u.st.Interface, func() {
				u.st.Interface = c.name
				u.saveSettings()
				u.logf("сетевой интерфейс: %q", c.name)
				render()
			}))
		}
		add(ifaceChoice{}, L("Automatic", "Автоматически"))
		for _, c := range list {
			add(c, c.name)
		}
		rows.Refresh()
		hint.RemoveAll()
		if ifaceAvailable(u.st.Interface, list) {
			hint.Add(infoNote(L("Traffic to the server goes through the gateway of the chosen interface. Applied at the next connection.", "Трафик к серверу идёт через шлюз выбранного интерфейса. Применяется при следующем подключении.")))
		} else {
			hint.Add(noteIcon("warn", L("Interface not found or not active. Connection will not start: choose another interface or Automatic.", "Интерфейс не найден или не активен. Подключение не запустится: выберите другой интерфейс или «Автоматически»."), colAccentInk))
		}
		hint.Refresh()
	}
	render()
	body := pageBox(
		prose(L("The interface through which the VPN reaches the server. Automatic uses the system default route.", "Интерфейс, через который VPN выходит к серверу. «Автоматически» использует маршрут системы по умолчанию.")),
		rows, hint,
	)
	u.openPage(L("Network interface", "Сетевой интерфейс"), scrolled(body))
	// шлюзы спрашиваем у системы в фоне: запрос может занять заметное время
	go func() {
		gws := client.DefaultGateways()
		fyne.Do(func() {
			for i := range list {
				list[i].gw = gws[list[i].name]
			}
			render()
		})
	}()
}


// radioRow — строка выбора по дизайну: карточка 64 px, радиокнопка слева (круг 22 px, рамка 2 px, точка 10 px),
// заголовок 16 px и пояснение 13 px.
func radioRow(title, sub string, selected bool, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(colCard)
	bg.CornerRadius = 14
	bg.StrokeColor, bg.StrokeWidth = colBorder, 1
	bg.SetMinSize(fyne.NewSize(1, 64))
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeWidth = 2
	ring.StrokeColor = colMuted
	parts := []fyne.CanvasObject{ring}
	if selected {
		ring.StrokeColor = colAccent
		parts = append(parts, container.NewCenter(container.NewGridWrap(fyne.NewSize(10, 10), canvas.NewCircle(colAccent))))
	}
	radio := container.NewCenter(container.NewGridWrap(fyne.NewSize(22, 22), container.NewStack(parts...)))
	t := newFitText(title, fitRowTitleSz, colText, true)
	var text fyne.CanvasObject = t
	if sub != "" {
		text = container.New(tightVBox{gap: fitRowSubShift}, t, newFitText(sub, 13, colMuted, false))
	}
	row := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 6), radio), nil, vcenter(text))
	return newRowTap(container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16), row)), onTap)
}

