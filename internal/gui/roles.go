package gui

import (
	"slices"
	"net/netip"
	"net/url"
	"encoding/json"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"veil/internal/config"
	"veil/internal/server"
)

// stateRoles — ответ /state вместе с ролями клиентов (имя → "user"/"admin"/""): роль нужна только интерфейсу.
type stateRoles struct {
	server.State
	roles map[string]string
}

func (s *stateRoles) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &s.State); err != nil {
		return err
	}
	var ext struct {
		Clients []struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(b, &ext); err != nil {
		return err
	}
	s.roles = make(map[string]string, len(ext.Clients))
	for _, c := range ext.Clients {
		s.roles[c.Name] = c.Role
	}
	return nil
}

func roleName(role string) string {
	if role == "admin" {
		return L("Administrator", "Администратор")
	}
	return L("User", "Пользователь")
}

// roleNote — пояснение под выбором роли (для администратора — предупреждение).
func roleNote(role string) fyne.CanvasObject {
	if role == "admin" {
		return noteIcon("warn", L("An administrator gets full access to the admin panel of this server: clients, cascade and password.", "Администратор получает полный доступ к админ-панели этого сервера: клиенты, каскад и пароль."), colAccentInk)
	}
	return noteIcon("info", L("A user only connects to the VPN and cannot open the admin panel.", "Пользователь только подключается к VPN и не может открыть админ-панель."), colMuted)
}

// noteIcon — пояснение со значком слева (в дизайне — note с иконкой info/warn/lock).
func noteIcon(icon, s string, c color.Color) fyne.CanvasObject {
	row := container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 4), container.NewCenter(container.NewGridWrap(fyne.NewSize(22, 22), iconImg(icon, c, 22)))), nil, vcenter(prose(s)))
	return container.NewStack(infoBG(), container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), row))
}

// roleBadge — значок «Админ» рядом с именем клиента в списке.
func roleBadge() fyne.CanvasObject {
	t := text(L("Admin", "Админ"), 11, colAccentInk, true)
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 6
	bg.StrokeColor, bg.StrokeWidth = colAccentInk, 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(1, 1, 6, 6), t))
}

// roleBlock — подпись «Роль», переключатель «Пользователь / Администратор» и пояснение, которое меняется с выбором.
func roleBlock(current string, changed func(string)) fyne.CanvasObject {
	sel := 0
	if current == "admin" {
		sel = 1
	}
	hint := container.NewStack(roleNote(current))
	seg := newSegmented([]string{roleName("user"), roleName("admin")}, sel, func(i int) {
		v := "user"
		if i == 1 {
			v = "admin"
		}
		changed(v)
		hint.RemoveAll()
		hint.Add(roleNote(v))
		hint.Refresh()
	})
	return container.NewVBox(sectionLabel(L("Role", "Роль")), seg, hint)
}

// adminAliasURL — короткий адрес админки: клиент прописывает «admin» в hosts при подключении.
// Пусто, если админка профиля открывается не по IP-адресу (имя прописывать не на что).
func adminAliasURL(p *config.Profile) string {
	if p == nil || p.AdminURL == "" {
		return ""
	}
	u, err := url.Parse(p.AdminURL)
	if err != nil {
		return ""
	}
	if _, err := netip.ParseAddr(u.Hostname()); err != nil {
		return ""
	}
	return "https://admin"
}

// nextEntry — вход, на который перейдёт подключение вместо отказавшего: первый по списку профиля,
// кроме него самого и входов, которые уже не удалось использовать.
func nextEntry(p *config.Profile, failed string, skipped []string) string {
	if p == nil {
		return ""
	}
	for _, e := range p.Entries {
		if e.Name == failed || slices.Contains(skipped, e.Name) {
			continue
		}
		return e.Name
	}
	return ""
}
