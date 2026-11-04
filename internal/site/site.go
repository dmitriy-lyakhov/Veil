package site

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"html/template"
	"math/rand/v2"
	"strings"
	"time"
	"unicode"
)

type Item struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Site struct {
	Lang     string   `json:"lang"`
	Company  string   `json:"company"`
	Tagline  string   `json:"tagline"`
	About    []string `json:"about"`
	Services []Item   `json:"services"`
	Values   []Item   `json:"values"`
	City     string   `json:"city"`
	Hours    string   `json:"hours"`
	Since    int      `json:"since"`
	Palette  int      `json:"palette"`
	Layout   int      `json:"layout"`
	Order    []string `json:"order"`
}

type palette struct {
	Primary, Accent, Ink, Soft, Hero string
}

var palettes = []palette{
	{"#1f4e79", "#2e86c1", "#1b2631", "#eef3f8", "#163a5a"},
	{"#145a32", "#27ae60", "#1c2a22", "#eef6f0", "#0f3f24"},
	{"#5b2c6f", "#8e44ad", "#241a2b", "#f4eff7", "#40204f"},
	{"#7b3f00", "#d35400", "#2b1d12", "#fbf3ec", "#5a2e00"},
	{"#17202a", "#c0392b", "#17202a", "#f4f6f7", "#1c2833"},
	{"#0e6655", "#16a085", "#14231f", "#ecf7f4", "#0b4d40"},
}

const layouts = 3

func newRand() *rand.Rand {
	var seed [16]byte
	_, _ = crand.Read(seed[:])
	return rand.New(rand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
}

func Generate(lang string) Site {
	if _, ok := packs[lang]; !ok {
		lang = "ru"
	}
	r := newRand()
	pk := packs[lang]
	cat := pk.Categories[r.IntN(len(pk.Categories))]
	pick := func(v []string) string { return v[r.IntN(len(v))] }
	services := make([]Item, len(cat.Services))
	for i, s := range cat.Services {
		services[i] = Item{s.Title, s.Text}
	}
	r.Shuffle(len(services), func(i, j int) { services[i], services[j] = services[j], services[i] })
	services = services[:3+r.IntN(len(services)-2)]
	values := make([]Item, len(cat.Values))
	for i, s := range cat.Values {
		values[i] = Item{s.Title, s.Text}
	}
	about := append([]string(nil), cat.About...)
	if r.IntN(2) == 0 {
		about = about[:1]
	}
	order := []string{"services", "about", "why"}
	r.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	return Site{
		Lang:     lang,
		Company:  pick(pk.Prefixes) + " " + pick(cat.Names),
		Tagline:  pick(cat.Taglines),
		About:    about,
		Services: services,
		Values:   values,
		City:     pick(pk.Cities),
		Hours:    pick(pk.Hours),
		Since:    time.Now().Year() - 3 - r.IntN(15),
		Palette:  r.IntN(len(palettes)),
		Layout:   r.IntN(layouts),
		Order:    order,
	}
}

func (s *Site) Valid() bool {
	_, ok := packs[s.Lang]
	return ok && s.Company != "" && len(s.Services) > 0
}

func (s *Site) Initials() string {
	var out []rune
	for _, w := range strings.Fields(s.Company) {
		for _, r := range w {
			if unicode.IsLetter(r) {
				out = append(out, unicode.ToUpper(r))
				break
			}
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

func (s *Site) T(key string) string { return packs[s.Lang].UI[key] }

func (s *Site) pal() palette { return palettes[s.Palette%len(palettes)] }

type view struct {
	*Site
	P     palette
	Year  int
	Error string
}

func (s *Site) render(name string, errMsg string) []byte {
	var b bytes.Buffer
	if err := pages.ExecuteTemplate(&b, name, view{Site: s, P: s.pal(), Year: time.Now().Year(), Error: errMsg}); err != nil {
		return []byte(err.Error())
	}
	return b.Bytes()
}

func (s *Site) Index() []byte                    { return s.render("index", "") }
func (s *Site) Thanks() []byte                   { return s.render("thanks", "") }
func (s *Site) Login(errMsg string) []byte       { return s.render("login", errMsg) }
func (s *Site) Favicon() []byte                  { return s.render("favicon", "") }
func (s *Site) Stylesheet() []byte               { return s.render("css", "") }
func (s *Site) Colors() (primary, accent string) { p := s.pal(); return p.Primary, p.Accent }

var pages = template.Must(template.New("").Parse(templates))
