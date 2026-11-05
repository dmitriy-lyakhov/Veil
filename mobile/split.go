package veilcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"time"

	"veil/internal/routing"
)

type splitIn struct {
	Mode string `json:"mode"` // via — в туннель только это; direct — это мимо туннеля, остальное в туннель
	Text string `json:"text"` // записи по одной в строке: домены, URI, IPv4/IPv6, подсети (как в десктопе)
}

type splitOut struct {
	Mode     string   `json:"mode"`
	Routes   []string `json:"routes"`  // маршруты в туннель (для direct — дополнение до всего адресного пространства)
	Exclude  []string `json:"exclude"` // direct: исключаемые сети (для excludeRoute на Android 13+)
	Warnings []string `json:"warnings"`
	Count    int      `json:"count"`
}

// Plan строит план раздельного туннелирования по сайтам и адресам: домены разрешаются ДО подъёма туннеля.
// Вход: JSON {mode:"via"|"direct", text:"..."}. Выход: JSON {mode, routes, exclude, warnings, count}.
func Plan(specJSON string) (string, error) {
	var in splitIn
	if err := json.Unmarshal([]byte(specJSON), &in); err != nil {
		return "", err
	}
	cfg := &routing.Config{Enabled: true}
	switch in.Mode {
	case "via":
		cfg.Via.Manual = in.Text
	case "direct":
		cfg.Direct.Manual = in.Text
	default:
		return "", errors.New(ErrFormat)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	plan, err := routing.Build(ctx, cfg, nil)
	if err != nil {
		return "", err
	}
	out := splitOut{Mode: in.Mode, Warnings: plan.Warnings, Routes: []string{}, Exclude: []string{}}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	if in.Mode == "via" {
		if len(plan.Via) == 0 {
			return "", errors.New(ErrEmpty)
		}
		out.Routes = routing.Strings(plan.Via)
		out.Count = len(plan.Via)
	} else {
		if len(plan.Direct) == 0 {
			return "", errors.New(ErrEmpty)
		}
		out.Exclude = routing.Strings(plan.Direct)
		out.Routes = routing.Strings(Complement(plan.Direct))
		out.Count = len(plan.Direct)
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// Complement возвращает набор подсетей, покрывающих всё адресное пространство IPv4 и IPv6 кроме excl.
func Complement(excl []netip.Prefix) []netip.Prefix {
	var v4, v6 []netip.Prefix
	for _, p := range excl {
		p = p.Masked()
		if p.Addr().Is4() {
			v4 = append(v4, p)
		} else {
			v6 = append(v6, p)
		}
	}
	out := subtract(netip.MustParsePrefix("0.0.0.0/0"), v4)
	out = append(out, subtract(netip.MustParsePrefix("::/0"), v6)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Addr().Less(out[j].Addr()) })
	return out
}

func subtract(base netip.Prefix, excl []netip.Prefix) []netip.Prefix {
	var rel []netip.Prefix
	for _, e := range excl {
		if e.Bits() <= base.Bits() && e.Contains(base.Addr()) {
			return nil // база целиком исключена
		}
		if e.Bits() > base.Bits() && base.Contains(e.Addr()) {
			rel = append(rel, e)
		}
	}
	if len(rel) == 0 {
		return []netip.Prefix{base}
	}
	lo, hi := halves(base)
	return append(subtract(lo, rel), subtract(hi, rel)...)
}

func halves(p netip.Prefix) (netip.Prefix, netip.Prefix) {
	b := p.Bits() + 1
	lo := netip.PrefixFrom(p.Addr(), b)
	a := p.Addr().AsSlice()
	a[(b-1)/8] |= 0x80 >> ((b - 1) % 8)
	hiAddr, _ := netip.AddrFromSlice(a)
	return lo, netip.PrefixFrom(hiAddr, b)
}
