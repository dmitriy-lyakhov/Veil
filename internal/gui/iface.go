package gui

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type ifaceChoice struct {
	name  string // пусто — автоматически
	label string
}

// networkInterfaces — интерфейсы, на которых может работать приложение: активные, не петлевые, с адресом,
// кроме собственных туннельных адаптеров Veil.
func networkInterfaces() []ifaceChoice {
	out := []ifaceChoice{{"", L("Automatic (by system routes)", "Автоматически (по маршрутам системы)")}}
	ifs, err := net.Interfaces()
	if err != nil {
		return out
	}
	sort.Slice(ifs, func(i, j int) bool { return ifs[i].Name < ifs[j].Name })
	for _, ifc := range ifs {
		low := strings.ToLower(ifc.Name)
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || strings.Contains(low, "veil") || strings.Contains(low, "wintun") {
			continue
		}
		addrs, _ := ifc.Addrs()
		var ips []string
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLinkLocalUnicast() {
				ips = append(ips, ipn.IP.String())
			}
		}
		if len(ips) == 0 {
			continue
		}
		out = append(out, ifaceChoice{ifc.Name, fmt.Sprintf("%s — %s", ifc.Name, strings.Join(ips, ", "))})
	}
	return out
}

func (u *ui) interfaceLabel() string {
	if u.st.Interface == "" {
		return L("Automatic", "Автоматически")
	}
	return u.st.Interface
}

// openInterface — страница «Сетевой интерфейс»: на каком интерфейсе работает приложение (например, только на VLAN 200).
func (u *ui) openInterface() {
	choices := networkInterfaces()
	known := false
	for _, c := range choices {
		if c.name == u.st.Interface {
			known = true
		}
	}
	if !known {
		choices = append(choices, ifaceChoice{u.st.Interface, fmt.Sprintf(L("%s — not found now", "%s — сейчас не найден"), u.st.Interface)})
	}
	labels := make([]string, len(choices))
	cur := ""
	for i, c := range choices {
		labels[i] = c.label
		if c.name == u.st.Interface {
			cur = c.label
		}
	}
	rg := widget.NewRadioGroup(labels, nil)
	rg.SetSelected(cur)
	rg.OnChanged = func(v string) {
		for _, c := range choices {
			if c.label == v {
				u.st.Interface = c.name
				u.saveSettings()
				u.logf("сетевой интерфейс: %q", c.name)
				return
			}
		}
	}
	body := container.NewVBox(
		infoNote(L("All Veil traffic to the server goes only through the chosen interface: useful when the machine has several networks or VLANs. The interface needs a route to the server (a default gateway). Applies on the next connection.", "Весь трафик Veil до сервера идёт только через выбранный интерфейс: удобно, когда в компьютере несколько сетей или VLAN. У интерфейса должен быть маршрут до сервера (шлюз по умолчанию). Применяется при следующем подключении.")),
		rg,
	)
	u.openPage(L("Network interface", "Сетевой интерфейс"), scrolled(body))
}
