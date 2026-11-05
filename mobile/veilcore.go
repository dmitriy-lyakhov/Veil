//go:build linux

// Package veilcore — Go-ядро Android-клиента Veil (WireGuard внутри HTTPS) для gomobile bind.
//
// Жизненный цикл: Parse(link) → настройка VpnService.Builder → establish() → Start(link, fd, protector).
// Сокеты клиента защищаются от петли через Protector (VpnService.protect).
package veilcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/netpin"
)

// Protector исключает сокет из VPN (на Android — VpnService.protect(fd)).
type Protector interface {
	Protect(fd int32) bool
}

var (
	mu      sync.Mutex
	tunnel  *client.Tunnel
	profile *config.Profile
	lastErr string
)

type addr struct {
	Addr   string `json:"addr"`
	Prefix int    `json:"prefix"`
	V6     bool   `json:"v6"`
}

type parsed struct {
	Name      string   `json:"name"`
	Addresses []addr   `json:"addresses"`
	DNS       []string `json:"dns"`
	MTU       int      `json:"mtu"`
	Entries   []string `json:"entries"`
	Role      string   `json:"role,omitempty"`
}

func decode(link string) (*config.Profile, error) {
	p, err := config.DecodeLink(link)
	if err != nil {
		return nil, err
	}
	if !p.IsVeil() {
		return nil, errors.New("Android-клиент поддерживает только протокол Veil")
	}
	return p, nil
}

func prefixOf(s string, v6 bool) (addr, error) {
	pf, err := netip.ParsePrefix(s)
	if err != nil {
		a, err2 := netip.ParseAddr(s)
		if err2 != nil {
			return addr{}, fmt.Errorf("некорректный адрес туннеля %q", s)
		}
		bits := 32
		if a.Is6() {
			bits = 128
		}
		pf = netip.PrefixFrom(a, bits)
	}
	return addr{Addr: pf.Addr().String(), Prefix: pf.Bits(), V6: pf.Addr().Is6()}, nil
}

// Parse проверяет ссылку и возвращает JSON для настройки VpnService.Builder:
// {name, addresses:[{addr,prefix,v6}], dns:[...], mtu, entries:[...], role}. Ключи в ответ не попадают.
func Parse(link string) (string, error) {
	p, err := decode(link)
	if err != nil {
		return "", err
	}
	out := parsed{Name: p.Name, MTU: p.EffectiveMTU(), Role: p.Role, DNS: []string{}, Entries: []string{}, Addresses: []addr{}}
	for _, s := range []string{p.Address4, p.Address6} {
		if s == "" {
			continue
		}
		a, err := prefixOf(s, false)
		if err != nil {
			return "", err
		}
		out.Addresses = append(out.Addresses, a)
	}
	if len(out.Addresses) == 0 {
		return "", errors.New("в конфигурации нет адреса туннеля")
	}
	for _, d := range p.DNS {
		if _, err := netip.ParseAddr(d); err == nil {
			out.DNS = append(out.DNS, d)
		}
	}
	if len(out.DNS) == 0 {
		out.DNS = []string{"1.1.1.1", "8.8.8.8"}
	}
	for _, e := range p.Entries {
		out.Entries = append(out.Entries, e.Name)
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// Start поднимает туннель на дескрипторе TUN, полученном из VpnService.Builder.establish() (detachFd:
// дескриптором владеет Go). Возвращается сразу; ход подключения — в Status().
func Start(link string, tunFd int32, protector Protector) error {
	mu.Lock()
	defer mu.Unlock()
	if tunnel != nil {
		return errors.New("туннель уже запущен")
	}
	p, err := decode(link)
	if err != nil {
		return err
	}
	if protector != nil {
		netpin.SetProtect(func(fd int) bool { return protector.Protect(int32(fd)) })
	}
	dev, _, err := tun.CreateUnmonitoredTUNFromFD(int(tunFd))
	if err != nil {
		netpin.SetProtect(nil)
		return fmt.Errorf("TUN: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	t, err := client.Start(p, dev, nil, log)
	if err != nil {
		_ = dev.Close()
		netpin.SetProtect(nil)
		return err
	}
	tunnel, profile, lastErr = t, p, ""
	return nil
}

// Stop закрывает туннель (и TUN). Безопасно вызывать повторно.
func Stop() {
	mu.Lock()
	t := tunnel
	tunnel, profile = nil, nil
	mu.Unlock()
	if t != nil {
		t.Close()
	}
	netpin.SetProtect(nil)
}

type statusJSON struct {
	State     string `json:"state"` // stopped, connecting, connected (транспорт есть), reconnecting
	Up        bool   `json:"up"`    // рукопожатие WireGuard выполнено
	Transport string `json:"transport,omitempty"`
	Entry     string `json:"entry,omitempty"`
	Name      string `json:"name,omitempty"`
	Exit      string `json:"exit,omitempty"`
	Rx        uint64 `json:"rx"`
	Tx        uint64 `json:"tx"`
	Handshake int64  `json:"handshake,omitempty"` // unix-секунды
	LastError string `json:"last_error,omitempty"`
}

// Status возвращает JSON {state, up, transport, entry, name, exit, rx, tx, handshake, last_error}.
func Status() string {
	mu.Lock()
	t, p := tunnel, profile
	mu.Unlock()
	if t == nil {
		b, _ := json.Marshal(statusJSON{State: "stopped"})
		return string(b)
	}
	s := t.Status()
	out := statusJSON{
		State: string(s.State), Transport: s.Transport, Entry: s.Entry, Exit: s.Exit,
		Rx: s.RxBytes, Tx: s.TxBytes, LastError: s.LastError,
	}
	if p != nil {
		out.Name = p.Name
	}
	if !s.Handshake.IsZero() {
		out.Up = true
		out.Handshake = s.Handshake.Unix()
	}
	out.LastError = strings.TrimSpace(out.LastError)
	b, _ := json.Marshal(out)
	return string(b)
}
