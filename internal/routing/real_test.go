package routing

import (
	"os"
	"testing"
	"time"
)

// Прогон на настоящем файле каталога: VEIL_CATALOG_FILE=путь go test -run TestRealCatalog -v
func TestRealCatalog(t *testing.T) {
	path := os.Getenv("VEIL_CATALOG_FILE")
	if path == "" {
		t.Skip("VEIL_CATALOG_FILE не задан")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	start := time.Now()
	cat, err := parseCatalog(f)
	if err != nil {
		t.Fatal(err)
	}
	var p, d, big int
	var bigName string
	for _, s := range cat.Services {
		p += len(s.Prefixes)
		d += len(s.Domains)
		if len(s.Prefixes) > big {
			big, bigName = len(s.Prefixes), s.Name
		}
	}
	groups, _ := cat.Groups()
	t.Logf("сервисов %d, групп %d, подсетей после сворачивания %d, доменов %d; самый большой %s: %d; разбор %s", len(cat.Services), len(groups), p, d, bigName, big, time.Since(start))
	out := t.TempDir() + "/cat.json"
	if err := cat.Save(out); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(out)
	t.Logf("размер кэша каталога: %d КБ", st.Size()/1024)
}
