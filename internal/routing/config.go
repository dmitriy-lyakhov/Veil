package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Import — набор адресов, добавленный целиком (например, из каталога opencck).
type Import struct {
	Source   string    `json:"source"`
	Title    string    `json:"title"`
	Added    time.Time `json:"added"`
	Domains  []string  `json:"domains,omitempty"`
	Prefixes []string  `json:"prefixes,omitempty"`
}

// List — один список: записи, введённые вручную (по одной в строке), и импортированные наборы.
type List struct {
	Manual  string   `json:"manual,omitempty"`
	Imports []Import `json:"imports,omitempty"`
}

// Config — настройки маршрутизации приложения.
//
//	Via    — «через VPN»: если список не пуст, в туннель идёт только он;
//	Direct — «мимо VPN»: эти адреса всегда идут напрямую (даже если попали в Via).
type Config struct {
	Enabled bool `json:"enabled"` // раздельное туннелирование включено
	Via     List `json:"via"`
	Direct  List `json:"direct"`

	AppsEnabled bool     `json:"apps_enabled,omitempty"`
	Apps        []string `json:"apps,omitempty"`
}

func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Veil", "routing.json"), nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	c := &Config{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// файлы прежней версии не знали о переключателе: непустые списки считаем включёнными
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) == nil {
		if _, ok := raw["enabled"]; !ok {
			c.Enabled = !c.Empty()
		}
	}
	return c, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Count — сколько записей в списке (домены и подсети раздельно), без сворачивания.
func (l List) Count() (domains, prefixes int) {
	p, d, _ := ParseLines(l.Manual)
	domains, prefixes = len(d), len(p)
	for _, im := range l.Imports {
		domains += len(im.Domains)
		prefixes += len(im.Prefixes)
	}
	return
}

func (c *Config) AddApp(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, a := range c.Apps {
		if strings.EqualFold(a, name) {
			return false
		}
	}
	c.Apps = append(c.Apps, name)
	return true
}

func (c *Config) RemoveApp(i int) {
	if i >= 0 && i < len(c.Apps) {
		c.Apps = append(c.Apps[:i], c.Apps[i+1:]...)
	}
}

func (c *Config) Empty() bool {
	d1, p1 := c.Via.Count()
	d2, p2 := c.Direct.Count()
	return d1+p1+d2+p2 == 0
}

// Plan — готовый к применению набор маршрутов.
type Plan struct {
	Via        []netip.Prefix // в туннель
	Direct     []netip.Prefix // мимо туннеля
	Selective  bool           // true: в туннель идёт только Via, остальное напрямую
	ViaDomains []string       // домены, превращённые в Via (для периодического обновления)
	DirDomains []string
	Warnings   []string
}

const (
	MaxRoutes  = 300000
	MaxResolve = 4000
)

func (l List) collect() (prefixes []netip.Prefix, domains []string, warns []string) {
	p, d, errs := ParseLines(l.Manual)
	prefixes, domains, warns = p, d, errs
	seen := map[string]bool{}
	for _, x := range d {
		seen[x] = true
	}
	for _, im := range l.Imports {
		prefixes = append(prefixes, ParsePrefixes(im.Prefixes)...)
		for _, x := range im.Domains {
			if !seen[x] {
				seen[x] = true
				domains = append(domains, x)
			}
		}
	}
	return
}

// LookupFunc разрешает имя в адреса (по умолчанию — системный резолвер).
type LookupFunc func(ctx context.Context, host string) ([]netip.Addr, error)

func SystemLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// Resolve разрешает домены параллельно; нерезолвящиеся пропускает и возвращает их число.
func Resolve(ctx context.Context, domains []string, lookup LookupFunc) (map[string][]netip.Addr, int) {
	if lookup == nil {
		lookup = SystemLookup
	}
	out := make(map[string][]netip.Addr, len(domains))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 64)
	failed := 0
	for _, d := range domains {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			addrs, err := lookup(cctx, d)
			mu.Lock()
			defer mu.Unlock()
			if err != nil || len(addrs) == 0 {
				failed++
				return
			}
			out[d] = addrs
		}()
	}
	wg.Wait()
	return out, failed
}

func hostPrefixes(m map[string][]netip.Addr, domains []string) []netip.Prefix {
	var out []netip.Prefix
	for _, d := range domains {
		for _, a := range m[d] {
			a = a.Unmap().WithZone("")
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return out
}

// Build собирает план: разбирает записи, разрешает домены, сворачивает подсети.
func Build(ctx context.Context, c *Config, lookup LookupFunc) (*Plan, error) {
	vp, vd, w1 := c.Via.collect()
	dp, dd, w2 := c.Direct.collect()
	pl := &Plan{Selective: len(vp)+len(vd) > 0, ViaDomains: vd, DirDomains: dd, Warnings: append(w1, w2...)}
	all := append(append([]string(nil), vd...), dd...)
	sort.Strings(all)
	if len(all) > MaxResolve {
		pl.Warnings = append(pl.Warnings, fmt.Sprintf("доменов %d: разрешены только первые %d (остальные покрываются подсетями из каталога)", len(all), MaxResolve))
		all = all[:MaxResolve]
	}
	if len(all) > 0 {
		resolved, failed := Resolve(ctx, all, lookup)
		if failed > 0 {
			pl.Warnings = append(pl.Warnings, fmt.Sprintf("не удалось разрешить доменов: %d из %d", failed, len(all)))
		}
		vp = append(vp, hostPrefixes(resolved, vd)...)
		dp = append(dp, hostPrefixes(resolved, dd)...)
	}
	pl.Via, pl.Direct = Aggregate(vp), Aggregate(dp)
	if n := len(pl.Via) + len(pl.Direct); n > MaxRoutes {
		return nil, fmt.Errorf("слишком много маршрутов после объединения: %d (максимум %d) — уберите лишнее из списков", n, MaxRoutes)
	}
	return pl, nil
}

// Summary — короткое описание для интерфейса и журнала.
func (p *Plan) Summary() string {
	mode := "весь трафик в туннель"
	if p.Selective {
		mode = "в туннель только выбранное"
	}
	return fmt.Sprintf("%s; через VPN: %d подсетей, мимо VPN: %d подсетей", mode, len(p.Via), len(p.Direct))
}

func (p *Plan) String() string { return strings.TrimSpace(p.Summary()) }
