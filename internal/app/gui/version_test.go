package gui

import (
	"os"
	"regexp"
	"testing"

	"veil/internal/server"
)

func firstMatch(t *testing.T, file, pattern string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(pattern).FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s: no match for %s", file, pattern)
	}
	return string(m[1])
}

func TestVersionsAgreeEverywhere(t *testing.T) {
	want := appVersion
	checks := map[string]string{
		"server.Version":                server.Version,
		"Makefile VERSION":              firstMatch(t, "../../../Makefile", `(?m)^VERSION\s*:=\s*(\S+)`),
		"packaging/windows/install.cmd": firstMatch(t, "../../../packaging/windows/install.cmd", `DisplayVersion /d "([^"]+)"`),
	}
	for name, got := range checks {
		if got != want {
			t.Errorf("%s = %s, app version is %s: bump them together", name, got, want)
		}
	}
}
