package site

import (
	htmlpkg "html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndRender(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		lang := []string{"ru", "en"}[i%2]
		s := Generate(lang)
		if !s.Valid() {
			t.Fatalf("invalid site %+v", s)
		}
		seen[s.Company] = true
		html := string(s.Index())
		for _, want := range []string{s.Company, s.Tagline, s.Services[0].Title, `action="/contact"`, `lang="` + lang + `"`} {
			if !strings.Contains(html, want) && !strings.Contains(html, htmlpkg.EscapeString(want)) {
				t.Fatalf("index missing %q", want)
			}
		}
		for _, bad := range []string{"VPN", "vpn", "WireGuard", "veil", "Veil", "ZgotmplZ"} {
			if strings.Contains(html+string(s.Login(""))+string(s.Stylesheet()), bad) {
				t.Fatalf("site leaks %q", bad)
			}
		}
	}
	if len(seen) < 15 {
		t.Fatalf("too little variety: %d distinct names", len(seen))
	}
	if dir := os.Getenv("VEIL_SITE_PREVIEW"); dir != "" {
		for i, lang := range []string{"ru", "en", "ru"} {
			s := Generate(lang)
			s.Layout = i
			sub := filepath.Join(dir, lang+string(rune('0'+i)))
			_ = os.MkdirAll(filepath.Join(sub, "assets"), 0o755)
			_ = os.WriteFile(filepath.Join(sub, "index.html"), s.Index(), 0o644)
			_ = os.WriteFile(filepath.Join(sub, "login.html"), s.Login(""), 0o644)
			_ = os.WriteFile(filepath.Join(sub, "assets", "site.css"), s.Stylesheet(), 0o644)
			_ = os.WriteFile(filepath.Join(sub, "favicon.svg"), s.Favicon(), 0o644)
		}
	}
}
