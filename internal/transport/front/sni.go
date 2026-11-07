package front

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

var sniPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{1,62}$`)

// Presets — готовые варианты «под какой сайт маскироваться».
var Presets = []string{"ya.ru", "yandex.ru", "vk.com", "mail.ru"}

// NormalizeSNI приводит введённый домен к виду, пригодному для SNI: без схемы, пути и порта,
// в нижнем регистре, IDN в punycode. Пустая строка означает «без маскировки».
func NormalizeSNI(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimSuffix(strings.SplitN(s, "/", 2)[0], ".")
	if s == "" {
		return "", nil
	}
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil || !sniPattern.MatchString(ascii) || len(ascii) > 253 {
		return "", fmt.Errorf("«%s» не похоже на доменное имя сайта, например ya.ru", s)
	}
	return ascii, nil
}

// NewKey создаёт общий ключ метки в виде base64.
func NewKey() (string, error) {
	b := make([]byte, KeyLen)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("не удалось создать ключ маскировки")
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
