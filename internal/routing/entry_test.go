package routing

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestParseEntry(t *testing.T) {
	cases := []struct {
		in     string
		kind   Kind
		prefix string
		domain string
	}{
		{"1.2.3.4", KindPrefix, "1.2.3.4/32", ""},
		{"2001:db8::1", KindPrefix, "2001:db8::1/128", ""},
		{"10.1.2.3/8", KindPrefix, "10.0.0.0/8", ""},
		{"2a00:1450:4000::/37", KindPrefix, "2a00:1450:4000::/37", ""},
		{"https://Example.com/path?q=1", KindDomain, "", "example.com"},
		{"http://1.2.3.4:8080/x", KindPrefix, "1.2.3.4/32", ""},
		{"https://[2001:db8::5]:443/", KindPrefix, "2001:db8::5/128", ""},
		{"example.com/path", KindDomain, "", "example.com"},
		{"example.com:8443", KindDomain, "", "example.com"},
		{"*.Example.org.", KindDomain, "", "example.org"},
		{"[2001:db8::7]:443", KindPrefix, "2001:db8::7/128", ""},
		{"1.2.3.4:80", KindPrefix, "1.2.3.4/32", ""},
		{"::ffff:1.2.3.4", KindPrefix, "1.2.3.4/32", ""},
		{"a.example.com # комментарий", KindDomain, "", "a.example.com"},
	}
	for _, c := range cases {
		e, err := ParseEntry(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if e.Kind != c.kind || (c.prefix != "" && e.Prefix.String() != c.prefix) || e.Domain != c.domain {
			t.Fatalf("%q: got %+v", c.in, e)
		}
	}
	for _, bad := range []string{"not a host!", "http://", "999.1.1.1/8", "-bad.com"} {
		if _, err := ParseEntry(bad); err == nil || err == ErrEmpty {
			t.Fatalf("%q: ожидалась ошибка, got %v", bad, err)
		}
	}
	for _, skip := range []string{"", "  ", "# c", "// c", "; c"} {
		if _, err := ParseEntry(skip); err != ErrEmpty {
			t.Fatalf("%q: want ErrEmpty, got %v", skip, err)
		}
	}
}

func TestParseLines(t *testing.T) {
	p, d, errs := ParseLines("1.1.1.1, 8.8.8.0/24\nexample.com https://example.com/x\n# c\nbad!!\n2001:db8::/32 # tail")
	if len(p) != 3 || !reflect.DeepEqual(d, []string{"example.com"}) || len(errs) != 1 {
		t.Fatalf("p=%v d=%v errs=%v", p, d, errs)
	}
}

func pfx(ss ...string) []netip.Prefix {
	var out []netip.Prefix
	for _, s := range ss {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}

func TestAggregate(t *testing.T) {
	got := Strings(Aggregate(pfx("10.0.0.0/25", "10.0.0.128/25", "10.0.0.5/32", "10.0.1.0/24", "192.168.0.1/32", "192.168.0.0/32")))
	want := []string{"10.0.0.0/23", "192.168.0.0/31"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = Strings(Aggregate(pfx("2001:db8::/33", "2001:db8:8000::/33", "2001:db8::1/128", "1.1.1.1/32")))
	want = []string{"1.1.1.1/32", "2001:db8::/32"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("v6 got %v want %v", got, want)
	}
	if Aggregate(nil) != nil {
		t.Fatal("nil")
	}
}
