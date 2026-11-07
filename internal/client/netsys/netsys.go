// Package netsys — системная часть туннеля: TUN-адаптер, адреса, маршруты, DNS, KillSwitch,
// запись «admin» в hosts. Реализации под Windows, macOS и Linux; движки туннеля — в пакете client.
package netsys

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"sync"

	"veil/internal/routing"
)

// Результат записи имени «admin» в hosts (см. InstallAdminAlias).
const (
	AdminHostsOK     = "ok"
	AdminHostsFailed = "failed"
)

type NetConfig struct {
	Iface   string
	Addr4   netip.Prefix
	Addr6   netip.Prefix
	MTU     int
	Servers []netip.Addr
	DNS     []netip.Addr
	Plan    *routing.Plan // nil — весь трафик в туннель

	// BypassServers — собственные пакеты движка идут до сервера в обход туннеля отдельным маршрутом
	// (протоколы без привязки сокетов к интерфейсу: обычный WireGuard и т.п.)
	BypassServers bool
}

// applyPlan добавляет маршруты списков. Вызывается из ApplyNetwork после основных настроек.
func ApplyPlan(nc NetConfig, u *Undo, log *slog.Logger) error {
	if nc.Plan == nil {
		return nil
	}
	via := append([]netip.Prefix(nil), nc.Plan.Via...)
	if nc.Plan.Selective {
		// в выборочном режиме запросы к DNS туннеля тоже должны идти в туннель
		for _, d := range nc.DNS {
			via = append(via, netip.PrefixFrom(d, d.BitLen()))
		}
	}
	return ApplyRoutes(nc, via, nc.Plan.Direct, u, log)
}

// fullTunnel — нужен ли общий маршрут «всё в туннель» (половинки 0.0.0.0/1 и 128.0.0.0/1).
func FullTunnel(nc NetConfig) bool { return nc.Plan == nil || !nc.Plan.Selective }

// Undo — стек отмены системных настроек: Run откатывает их в обратном порядке.
type Undo struct {
	mu  sync.Mutex
	fns []func()
}

func (u *Undo) Push(f func()) {
	u.mu.Lock()
	u.fns = append(u.fns, f)
	u.mu.Unlock()
}

func (u *Undo) Run() {
	u.mu.Lock()
	fns := u.fns
	u.fns = nil
	u.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// Run выполняет системную команду без окна консоли и возвращает её вывод.
func Run(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	HideWindow(cmd)
	raw, err := cmd.CombinedOutput()
	out := []byte(DecodeOutput(raw))
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("%s: %v: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

func fields(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// stepIsV6 — шаг настройки адаптера, относящийся к IPv6.
func stepIsV6(args []string) bool {
	for _, a := range args {
		if a == "-6" || a == "inet6" {
			return true
		}
	}
	return false
}

// pinnedIface — имя сетевого интерфейса, выбранного пользователем (пусто — автоматически).
// Задаётся через SetInterface до подготовки сети.
var pinnedIface string

// SetInterface задаёт сетевой интерфейс, через который идёт трафик до сервера (пусто — автоматически).
func SetInterface(name string) { pinnedIface = name }

// pinnedInterface возвращает выбранный интерфейс или ошибку, если его нет или он не активен.
func pinnedInterface() (*net.Interface, error) {
	if pinnedIface == "" {
		return nil, nil
	}
	ifc, err := net.InterfaceByName(pinnedIface)
	if err != nil {
		return nil, fmt.Errorf("сетевой интерфейс «%s» не найден: выберите другой в Настройки → Подключение → Сетевой интерфейс", pinnedIface)
	}
	if ifc.Flags&net.FlagUp == 0 {
		return nil, fmt.Errorf("сетевой интерфейс «%s» не активен", pinnedIface)
	}
	return ifc, nil
}
