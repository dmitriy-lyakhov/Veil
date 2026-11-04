package wgconf

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// AWG — параметры обфускации AmneziaWG из раздела [Interface].
// Нулевое значение числового поля — «не задано» (в .conf не выводится).
type AWG struct {
	Jc   int `json:"jc,omitempty"`   // число мусорных пакетов перед рукопожатием
	Jmin int `json:"jmin,omitempty"` // минимальный размер мусорного пакета
	Jmax int `json:"jmax,omitempty"` // максимальный размер мусорного пакета
	S1   int `json:"s1,omitempty"`   // добавка к размеру init-пакета
	S2   int `json:"s2,omitempty"`   // добавка к размеру ответного пакета
	S3   int `json:"s3,omitempty"`   // (AWG 2.0) добавка к cookie-пакету
	S4   int `json:"s4,omitempty"`   // (AWG 2.0) добавка к пакету данных
	// H1…H4 — заголовки типов пакетов: число (AWG 1.x) или диапазон «a-b» (AWG 2.0).
	H1 string `json:"h1,omitempty"`
	H2 string `json:"h2,omitempty"`
	H3 string `json:"h3,omitempty"`
	H4 string `json:"h4,omitempty"`
	// I1…I5 — «сигнатурные» пакеты (AWG 1.5), например <b 0x…><r 16><t>.
	I [5]string `json:"i,omitempty"`
}

// Empty — ни один параметр не задан.
func (a *AWG) Empty() bool {
	if a == nil {
		return true
	}
	return *a == AWG{}
}

func (a *AWG) set(key, v string) error {
	num := func(dst *int) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return fmt.Errorf("«%s» — не неотрицательное число", v)
		}
		*dst = n
		return nil
	}
	switch key {
	case "jc":
		return num(&a.Jc)
	case "jmin":
		return num(&a.Jmin)
	case "jmax":
		return num(&a.Jmax)
	case "s1":
		return num(&a.S1)
	case "s2":
		return num(&a.S2)
	case "s3":
		return num(&a.S3)
	case "s4":
		return num(&a.S4)
	case "h1":
		a.H1 = v
	case "h2":
		a.H2 = v
	case "h3":
		a.H3 = v
	case "h4":
		a.H4 = v
	default:
		if len(key) == 2 && key[0] == 'i' && key[1] >= '1' && key[1] <= '5' {
			a.I[key[1]-'1'] = v
			return nil
		}
		return fmt.Errorf("неизвестный параметр %s", key)
	}
	return nil
}

// Validate проверяет допустимость сочетания параметров.
func (a *AWG) Validate() error {
	if a == nil {
		return nil
	}
	switch {
	case a.Jc > 128:
		return errors.New("Jc должен быть от 0 до 128")
	case a.Jc > 0 && a.Jmax == 0:
		return errors.New("при Jc > 0 нужен Jmax")
	case a.Jmin > a.Jmax:
		return errors.New("Jmin не может быть больше Jmax")
	case a.Jmax > 1280:
		return errors.New("Jmax должен быть не больше 1280")
	case a.S1 > 1132 || a.S2 > 1188:
		return errors.New("S1 должен быть не больше 1132, S2 — не больше 1188")
	case a.S1 > 0 && a.S1+56 == a.S2:
		return errors.New("S1 + 56 не должно равняться S2")
	}
	seen := map[string]bool{}
	for _, h := range []string{a.H1, a.H2, a.H3, a.H4} {
		if h == "" {
			continue
		}
		if seen[h] {
			return errors.New("значения H1…H4 должны быть разными")
		}
		seen[h] = true
		if !strings.Contains(h, "-") {
			n, err := strconv.ParseUint(h, 10, 32)
			if err != nil || n <= 4 {
				return fmt.Errorf("H «%s»: нужно число от 5 до 4294967295", h)
			}
		}
	}
	return nil
}

// HasV2 — заданы параметры, которых нет в AmneziaWG 1.x/1.5 (S3, S4, диапазоны H).
func (a *AWG) HasV2() bool {
	if a == nil {
		return false
	}
	if a.S3 != 0 || a.S4 != 0 {
		return true
	}
	for _, h := range []string{a.H1, a.H2, a.H3, a.H4} {
		if strings.Contains(h, "-") {
			return true
		}
	}
	return false
}

func (a *AWG) write(b *strings.Builder) {
	n := func(k string, v int) {
		if v != 0 {
			fmt.Fprintf(b, "%s = %d\n", k, v)
		}
	}
	s := func(k, v string) {
		if v != "" {
			fmt.Fprintf(b, "%s = %s\n", k, v)
		}
	}
	n("Jc", a.Jc)
	n("Jmin", a.Jmin)
	n("Jmax", a.Jmax)
	n("S1", a.S1)
	n("S2", a.S2)
	n("S3", a.S3)
	n("S4", a.S4)
	s("H1", a.H1)
	s("H2", a.H2)
	s("H3", a.H3)
	s("H4", a.H4)
	for i, v := range a.I {
		s(fmt.Sprintf("I%d", i+1), v)
	}
}

// ParseAWGInterface разбирает раздел [Interface] серверного конфига AmneziaWG (порт и параметры обфускации).
func ParseAWGInterface(text string) (a *AWG, port int) {
	a = &AWG{}
	in := false
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") {
			in = strings.EqualFold(line, "[interface]")
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !in || !ok {
			continue
		}
		k, v = strings.ToLower(strings.TrimSpace(k)), strings.TrimSpace(v)
		if k == "listenport" {
			port, _ = strconv.Atoi(v)
			continue
		}
		_ = a.set(k, v)
	}
	return a, port
}
