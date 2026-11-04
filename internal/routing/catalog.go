package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const DefaultCatalogURL = "https://iplist.opencck.org/?format=json&filesave=1"

// Service — одна запись каталога: сервис с доменами и свёрнутыми подсетями.
type Service struct {
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Domains  []string `json:"domains,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
}

type Catalog struct {
	URL      string    `json:"url"`
	Fetched  time.Time `json:"fetched"`
	Services []Service `json:"services"`
}

func CatalogPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Veil", "catalog.json"), nil
}

func LoadCatalog(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Catalog{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalog) Save(path string) error {
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

// Groups — группы каталога в алфавитном порядке с сервисами внутри.
func (c *Catalog) Groups() ([]string, map[string][]Service) {
	m := map[string][]Service{}
	for _, s := range c.Services {
		g := s.Group
		if g == "" {
			g = "other"
		}
		m[g] = append(m[g], s)
	}
	var names []string
	for g, ss := range m {
		sort.Slice(ss, func(i, j int) bool { return ss[i].Name < ss[j].Name })
		names = append(names, g)
	}
	sort.Strings(names)
	return names, m
}

// ImportFrom объединяет выбранные сервисы в один набор для списка.
func ImportFrom(source string, ss []Service) Import {
	var prefixes []netip.Prefix
	var domains []string
	seen := map[string]bool{}
	var titles []string
	for _, s := range ss {
		titles = append(titles, s.Name)
		prefixes = append(prefixes, ParsePrefixes(s.Prefixes)...)
		for _, d := range s.Domains {
			if !seen[d] {
				seen[d] = true
				domains = append(domains, d)
			}
		}
	}
	title := strings.Join(titles, ", ")
	if len(titles) > 3 {
		title = strings.Join(titles[:3], ", ") + fmt.Sprintf(" и ещё %d", len(titles)-3)
	}
	return Import{Source: source, Title: title, Added: time.Now(), Domains: domains, Prefixes: Strings(Aggregate(prefixes))}
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

type rawService struct {
	Name    string   `json:"name"`
	Group   string   `json:"group"`
	Domains []string `json:"domains"`
	IP4     []string `json:"ip4"`
	IP6     []string `json:"ip6"`
	CIDR4   []string `json:"cidr4"`
	CIDR6   []string `json:"cidr6"`
}

// FetchCatalog скачивает и разбирает каталог потоком (файл бывает десятки мегабайт и
// отдаётся медленно). progress получает прочитанные байты и общий размер (0, если неизвестен).
func FetchCatalog(ctx context.Context, url string, progress func(read, total int64)) (*Catalog, error) {
	if url == "" {
		url = DefaultCatalogURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер списка ответил %s", resp.Status)
	}
	cr := &countingReader{r: resp.Body}
	if progress != nil {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			t := time.NewTicker(300 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					progress(cr.n.Load(), resp.ContentLength)
				}
			}
		}()
	}
	cat, err := parseCatalog(cr)
	if err != nil {
		return nil, err
	}
	cat.URL, cat.Fetched = url, time.Now()
	if progress != nil {
		progress(cr.n.Load(), resp.ContentLength)
	}
	return cat, nil
}

func parseCatalog(r io.Reader) (*Catalog, error) {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("неожиданный формат списка: ожидался JSON-объект")
	}
	cat := &Catalog{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var rs rawService
		if err := dec.Decode(&rs); err != nil {
			return nil, fmt.Errorf("запись %v: %w", key, err)
		}
		name := rs.Name
		if name == "" {
			name, _ = key.(string)
		}
		var ps []netip.Prefix
		for _, group := range [][]string{rs.IP4, rs.IP6, rs.CIDR4, rs.CIDR6} {
			for _, s := range group {
				if e, err := ParseEntry(s); err == nil && e.Kind == KindPrefix {
					ps = append(ps, e.Prefix)
				}
			}
		}
		cat.Services = append(cat.Services, Service{Name: name, Group: rs.Group, Domains: rs.Domains, Prefixes: Strings(Aggregate(ps))})
	}
	if len(cat.Services) == 0 {
		return nil, errors.New("в списке нет ни одного сервиса")
	}
	return cat, nil
}
