package routing

import (
	"os"
	"strconv"
	"sync"
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func fakeLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	switch host {
	case "a.example":
		return []netip.Addr{netip.MustParseAddr("203.0.113.5"), netip.MustParseAddr("2001:db8::5")}, nil
	case "b.example":
		return []netip.Addr{netip.MustParseAddr("203.0.113.6")}, nil
	}
	return nil, errors.New("nxdomain")
}

func TestBuildFull(t *testing.T) {
	c := &Config{Direct: List{Manual: "a.example\n10.0.0.0/8\nhttps://b.example/x\nnope.example"}}
	p, err := Build(context.Background(), c, fakeLookup)
	if err != nil {
		t.Fatal(err)
	}
	if p.Selective || len(p.Via) != 0 {
		t.Fatalf("ожидался полный туннель: %+v", p)
	}
	got := strings.Join(Strings(p.Direct), ",")
	if got != "10.0.0.0/8,203.0.113.5/32,203.0.113.6/31"[:0]+"10.0.0.0/8,203.0.113.5/32,203.0.113.6/32,2001:db8::5/128" {
		t.Fatalf("direct = %s", got)
	}
	if len(p.Warnings) != 1 {
		t.Fatalf("warnings %v", p.Warnings)
	}
}

func TestBuildSelective(t *testing.T) {
	c := &Config{
		Via:    List{Manual: "a.example\n198.51.100.0/24", Imports: []Import{{Prefixes: []string{"198.51.101.0/24"}}}},
		Direct: List{Manual: "198.51.100.77"},
	}
	p, err := Build(context.Background(), c, fakeLookup)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Selective {
		t.Fatal("ожидался выборочный режим")
	}
	if got := strings.Join(Strings(p.Via), ","); got != "198.51.100.0/23,203.0.113.5/32,2001:db8::5/128" {
		t.Fatalf("via = %s", got)
	}
	if got := strings.Join(Strings(p.Direct), ","); got != "198.51.100.77/32" {
		t.Fatalf("direct = %s", got)
	}
}

func TestEmpty(t *testing.T) {
	if !(&Config{}).Empty() {
		t.Fatal("пустая конфигурация")
	}
	if (&Config{Via: List{Manual: "1.1.1.1"}}).Empty() {
		t.Fatal("не пустая")
	}
}

func TestParseCatalog(t *testing.T) {
	js := `{"x.com":{"name":"x.com","group":"g","domains":["x.com"],"ip4":["1.1.1.1","1.1.1.0"],"cidr4":["1.1.2.0/24"],"ip6":["2001:db8::1"],"cidr6":[],"dns":["8.8.8.8:53"],"replace":{"a":1}},
	"y.com":{"name":"y.com","group":"h","domains":[],"ip4":[],"cidr4":["9.9.9.0/24"]}}`
	cat, err := parseCatalog(strings.NewReader(js))
	if err != nil || len(cat.Services) != 2 {
		t.Fatalf("%v %+v", err, cat)
	}
	if got := strings.Join(cat.Services[0].Prefixes, ","); got != "1.1.1.0/31,1.1.2.0/24,2001:db8::1/128" {
		t.Fatalf("prefixes %s", got)
	}
	names, groups := cat.Groups()
	if len(names) != 2 || len(groups["g"]) != 1 {
		t.Fatalf("%v %v", names, groups)
	}
	imp := ImportFrom("opencck", cat.Services)
	if imp.Title != "x.com, y.com" || len(imp.Prefixes) != 4 {
		t.Fatalf("%+v", imp)
	}
}

func TestResolveCap(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < MaxResolve+50; i++ {
		sb.WriteString("h")
		sb.WriteString(strings.Repeat("a", i%5))
		sb.WriteString(string(rune('a'+i%26)))
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(".example\n")
	}
	calls := 0
	var mu sync.Mutex
	lookup := func(ctx context.Context, host string) ([]netip.Addr, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return []netip.Addr{netip.MustParseAddr("192.0.2.1")}, nil
	}
	p, err := Build(context.Background(), &Config{Direct: List{Manual: sb.String()}}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if calls != MaxResolve || len(p.Warnings) != 1 {
		t.Fatalf("calls=%d warnings=%v", calls, p.Warnings)
	}
}

func TestEnabledMigration(t *testing.T) {
	dir := t.TempDir()
	old := dir + "/old.json"
	if err := os.WriteFile(old, []byte(`{"via":{"manual":"1.1.1.1"},"direct":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(old)
	if err != nil || !c.Enabled {
		t.Fatalf("старый файл с записями должен считаться включённым: %v %+v", err, c)
	}
	c.Enabled = false
	nw := dir + "/new.json"
	if err := c.Save(nw); err != nil {
		t.Fatal(err)
	}
	c2, _ := Load(nw)
	if c2.Enabled {
		t.Fatal("явное выключение должно сохраняться")
	}
}
