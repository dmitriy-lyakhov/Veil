//go:build windows

package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Маршруты списков добавляются через IP Helper API (CreateIpForwardEntry2): netsh на
// тысячах записей работал бы минутами, а здесь это миллисекунды на запись.

var (
	modIphlpapi   = windows.NewLazySystemDLL("iphlpapi.dll")
	procInitFwd   = modIphlpapi.NewProc("InitializeIpForwardEntry")
	procCreateFwd = modIphlpapi.NewProc("CreateIpForwardEntry2")
	procDeleteFwd = modIphlpapi.NewProc("DeleteIpForwardEntry2")
)

const (
	afInet                 = 2
	afInet6                = 23
	mibIpprotoNetmgmt      = 3
	errObjectAlreadyExists = 5010
)

// sockaddrInet — SOCKADDR_INET (28 байт, выравнивание 4).
type sockaddrInet [28]byte

// mibIpforwardRow2 — MIB_IPFORWARD_ROW2 (104 байта на 64-битной Windows).
type mibIpforwardRow2 struct {
	InterfaceLuid        uint64
	InterfaceIndex       uint32
	DestPrefix           sockaddrInet
	DestPrefixLen        uint8
	_                    [3]byte
	NextHop              sockaddrInet
	SitePrefixLength     uint8
	_                    [3]byte
	ValidLifetime        uint32
	PreferredLifetime    uint32
	Metric               uint32
	Protocol             uint32
	Loopback             uint8
	AutoconfigureAddress uint8
	Publish              uint8
	Immortal             uint8
	Age                  uint32
	Origin               uint32
}

func putSockaddr(sa *sockaddrInet, a netip.Addr) {
	*sa = sockaddrInet{}
	if a.Is4() {
		sa[0] = afInet
		b := a.As4()
		copy(sa[4:8], b[:])
		return
	}
	sa[0] = afInet6
	b := a.As16()
	copy(sa[8:24], b[:])
}

func newForwardRow(ifIndex uint32, p netip.Prefix, hop netip.Addr, metric uint32) mibIpforwardRow2 {
	var r mibIpforwardRow2
	_, _, _ = procInitFwd.Call(uintptr(unsafe.Pointer(&r)))
	r.InterfaceIndex = ifIndex
	putSockaddr(&r.DestPrefix, p.Addr())
	r.DestPrefixLen = uint8(p.Bits())
	if !hop.IsValid() {
		hop = netip.IPv4Unspecified()
		if p.Addr().Is6() {
			hop = netip.IPv6Unspecified()
		}
	}
	putSockaddr(&r.NextHop, hop)
	r.Metric = metric
	r.Protocol = mibIpprotoNetmgmt
	return r
}

func createRoute(r *mibIpforwardRow2) error {
	code, _, _ := procCreateFwd.Call(uintptr(unsafe.Pointer(r)))
	if code != 0 && code != errObjectAlreadyExists {
		return windows.Errno(code)
	}
	return nil
}

func deleteRoute(r *mibIpforwardRow2) {
	_, _, _ = procDeleteFwd.Call(uintptr(unsafe.Pointer(r)))
}

// шлюз и интерфейс физической сети — определяются до создания адаптера туннеля
var (
	winGW4, winGW6 netip.Addr
	winIf4, winIf6 uint32
)

func defaultGateway(v6 bool) (netip.Addr, uint32) {
	dst, none := "0.0.0.0/0", "0.0.0.0"
	if v6 {
		dst, none = "::/0", "::"
	}
	filter := ""
	if ifc, _ := pinnedInterface(); ifc != nil {
		filter = fmt.Sprintf("-InterfaceIndex %d ", ifc.Index)
	}
	ps := fmt.Sprintf(`$r = Get-NetRoute -DestinationPrefix '%s' %s-ErrorAction SilentlyContinue | Where-Object { $_.NextHop -ne '%s' } | Sort-Object { $_.RouteMetric + $_.InterfaceMetric } | Select-Object -First 1; if ($r) { "$($r.NextHop) $($r.InterfaceIndex)" }`, dst, filter, none)
	out, err := run("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		return netip.Addr{}, 0
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return netip.Addr{}, 0
	}
	gw, err := netip.ParseAddr(strings.SplitN(f[0], "%", 2)[0])
	n, err2 := strconv.ParseUint(f[1], 10, 32)
	if err != nil || err2 != nil {
		return netip.Addr{}, 0
	}
	return gw, uint32(n)
}

func prepareRoutes(nc NetConfig) {
	winGW4, winIf4, winGW6, winIf6 = netip.Addr{}, 0, netip.Addr{}, 0
	if nc.Plan != nil && len(nc.Plan.Direct) > 0 {
		winGW4, winIf4 = defaultGateway(false)
		winGW6, winIf6 = defaultGateway(true)
	}
}

func applyRoutes(nc NetConfig, via, direct []netip.Prefix, u *undoStack, log *slog.Logger) error {
	var rows []mibIpforwardRow2
	if len(via) > 0 {
		ifc, err := net.InterfaceByName(nc.Iface)
		if err != nil {
			return fmt.Errorf("интерфейс %s: %w", nc.Iface, err)
		}
		failed := 0
		for _, p := range via {
			r := newForwardRow(uint32(ifc.Index), p, netip.Addr{}, 1)
			if err := createRoute(&r); err != nil {
				failed++
				continue
			}
			rows = append(rows, r)
		}
		if failed > 0 {
			log.Warn("routes via tunnel", "failed", failed, "total", len(via))
		}
	}
	if len(direct) > 0 {
		failed, skipped := 0, 0
		for _, p := range direct {
			gw, idx := winGW4, winIf4
			if p.Addr().Is6() {
				gw, idx = winGW6, winIf6
			}
			if !gw.IsValid() || idx == 0 {
				skipped++
				continue
			}
			r := newForwardRow(idx, p, gw, 1)
			if err := createRoute(&r); err != nil {
				failed++
				continue
			}
			rows = append(rows, r)
		}
		if failed > 0 || skipped > 0 {
			log.Warn("routes bypassing tunnel", "failed", failed, "no_gateway", skipped, "total", len(direct))
		}
	}
	if len(rows) > 0 {
		log.Info("routes added", "count", len(rows))
		u.push(func() {
			for i := range rows {
				deleteRoute(&rows[i])
			}
		})
	}
	if len(via)+len(direct) > 0 && len(rows) == 0 {
		return errors.New("ни один маршрут из списков не добавлен")
	}
	return nil
}

// bypassServer ведёт пакеты самого движка до сервера мимо туннеля: отдельным маршрутом через
// физический интерфейс. Нужен, когда сокеты движка не привязаны к интерфейсу (обычный WireGuard).
func bypassServer(srv netip.Addr, u *undoStack) error {
	ps := fmt.Sprintf(`$r = Find-NetRoute -RemoteIPAddress '%s' | Where-Object { $_.NextHop } | Select-Object -First 1; "$($r.NextHop) $($r.InterfaceIndex)"`, srv)
	out, err := run("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err != nil {
		return err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return fmt.Errorf("не удалось определить маршрут до %s: %q", srv, out)
	}
	hop, err := netip.ParseAddr(strings.SplitN(f[0], "%", 2)[0])
	idx, err2 := strconv.ParseUint(f[1], 10, 32)
	if err != nil || err2 != nil {
		return fmt.Errorf("не удалось разобрать маршрут до %s: %q", srv, out)
	}
	if hop.IsUnspecified() {
		hop = netip.Addr{} // на каналe: следующий узел не нужен
	}
	row := newForwardRow(uint32(idx), netip.PrefixFrom(srv, srv.BitLen()), hop, 1)
	if err := createRoute(&row); err != nil {
		return err
	}
	u.push(func() { deleteRoute(&row) })
	return nil
}
