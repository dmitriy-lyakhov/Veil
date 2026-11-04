// Package routing — списки адресов «через VPN» и «мимо VPN»: разбор записей,
// сворачивание в подсети, разрешение доменов и каталог iplist.opencck.org.
package routing

import (
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Kind int

const (
	KindPrefix Kind = iota // IPv4/IPv6 или подсеть
	KindDomain             // имя хоста (из URI или как есть)
)

type Entry struct {
	Kind   Kind
	Prefix netip.Prefix
	Domain string
}

var hostRe = regexp.MustCompile(`^[a-z0-9_]([a-z0-9_-]{0,62}[a-z0-9_])?(\.[a-z0-9_]([a-z0-9_-]{0,62}[a-z0-9_])?)*$`)

var ErrEmpty = errors.New("пустая запись")

// ParseEntry понимает: URI (https://example.com/path), домен, IPv4, IPv6,
// подсеть IPv4/IPv6, адрес с портом ([::1]:443, 1.2.3.4:80), «*.example.com».
// Комментарии (#, //, ;) и пустые строки возвращают ErrEmpty.
func ParseEntry(s string) (Entry, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") || strings.HasPrefix(s, ";") {
		return Entry{}, ErrEmpty
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil || u.Hostname() == "" {
			return Entry{}, errors.New("не удалось разобрать URI: " + s)
		}
		return parseHost(u.Hostname())
	}
	if p, err := netip.ParsePrefix(s); err == nil {
		return Entry{Kind: KindPrefix, Prefix: p.Masked()}, nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		a = a.Unmap()
		return Entry{Kind: KindPrefix, Prefix: netip.PrefixFrom(a, a.BitLen())}, nil
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		a := ap.Addr().Unmap()
		return Entry{Kind: KindPrefix, Prefix: netip.PrefixFrom(a, a.BitLen())}, nil
	}
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end > 0 {
			return parseHost(s[1:end])
		}
	}
	// «example.com/path» и «example.com:8443» — берём только имя хоста
	host := s
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[:i], ":") {
		host = host[:i]
	}
	return parseHost(host)
}

func parseHost(h string) (Entry, error) {
	h = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
	h = strings.TrimPrefix(h, "*.")
	if a, err := netip.ParseAddr(h); err == nil {
		a = a.Unmap()
		return Entry{Kind: KindPrefix, Prefix: netip.PrefixFrom(a, a.BitLen())}, nil
	}
	if i := strings.LastIndex(h, "."); i >= 0 && isDigits(h[i+1:]) {
		return Entry{}, errors.New("некорректный IP-адрес: " + h)
	}
	if h == "" || len(h) > 253 || !hostRe.MatchString(h) {
		return Entry{}, errors.New("некорректный адрес или имя: " + h)
	}
	return Entry{Kind: KindDomain, Domain: h}, nil
}

// ParseLines разбирает многострочный текст; ошибки собираются с номерами строк.
// В одной строке можно перечислить несколько записей через пробел или запятую.
func ParseLines(text string) (prefixes []netip.Prefix, domains []string, errs []string) {
	seenDom := map[string]bool{}
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") || strings.HasPrefix(t, ";") {
			continue
		}
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i]
		}
		for _, tok := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			e, err := ParseEntry(tok)
			if errors.Is(err, ErrEmpty) {
				continue
			}
			if err != nil {
				errs = append(errs, "строка "+strconv.Itoa(i+1)+": "+err.Error())
				continue
			}
			if e.Kind == KindDomain {
				if !seenDom[e.Domain] {
					seenDom[e.Domain] = true
					domains = append(domains, e.Domain)
				}
			} else {
				prefixes = append(prefixes, e.Prefix)
			}
		}
	}
	return prefixes, domains, errs
}

func cmpPrefix(a, b netip.Prefix) int {
	if c := a.Addr().Compare(b.Addr()); c != 0 {
		return c
	}
	return a.Bits() - b.Bits()
}

// Aggregate убирает вложенные подсети и склеивает соседние (10.0.0.0/25 + 10.0.0.128/25 → /24).
func Aggregate(in []netip.Prefix) []netip.Prefix {
	if len(in) == 0 {
		return nil
	}
	ps := make([]netip.Prefix, 0, len(in))
	for _, p := range in {
		ps = append(ps, p.Masked())
	}
	sort.Slice(ps, func(i, j int) bool { return cmpPrefix(ps[i], ps[j]) < 0 })
	// 1) вложенные
	out := make([]netip.Prefix, 0, len(ps))
	for _, p := range ps {
		if n := len(out); n > 0 && out[n-1].Addr().Is4() == p.Addr().Is4() && out[n-1].Contains(p.Addr()) && out[n-1].Bits() <= p.Bits() {
			continue
		}
		out = append(out, p)
	}
	// 2) соседние
	stack := make([]netip.Prefix, 0, len(out))
	for _, p := range out {
		stack = append(stack, p)
		for len(stack) >= 2 {
			a, b := stack[len(stack)-2], stack[len(stack)-1]
			par, ok := mergePair(a, b)
			if !ok {
				break
			}
			stack = append(stack[:len(stack)-2], par)
		}
	}
	return stack
}

func mergePair(a, b netip.Prefix) (netip.Prefix, bool) {
	if a.Bits() != b.Bits() || a.Bits() == 0 || a.Addr().Is4() != b.Addr().Is4() || a.Addr() == b.Addr() {
		return netip.Prefix{}, false
	}
	pa, err := a.Addr().Prefix(a.Bits() - 1)
	if err != nil {
		return netip.Prefix{}, false
	}
	pb, err := b.Addr().Prefix(b.Bits() - 1)
	if err != nil || pa != pb {
		return netip.Prefix{}, false
	}
	return pa, true
}

// Strings — подсети строками (для хранения).
func Strings(ps []netip.Prefix) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.String()
	}
	return out
}

// ParsePrefixes — обратное к Strings; некорректные строки пропускаются.
func ParsePrefixes(ss []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(ss))
	for _, s := range ss {
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
