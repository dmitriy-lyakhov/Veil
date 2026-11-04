// Package settings — настройки приложения (settings.json в каталоге конфигурации Veil).
package settings

import (
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	DNS            []string  `json:"dns,omitempty"` // подмена DNS сервера из профиля (основной, запасной)
	KillSwitch     bool      `json:"kill_switch,omitempty"`
	AutoStart      bool      `json:"auto_start,omitempty"`
	AutoConnect    bool      `json:"auto_connect,omitempty"`
	StartMinimized bool      `json:"start_minimized,omitempty"`
	NoUpdateCheck  bool      `json:"no_update_check,omitempty"`
	LogEnabled     bool      `json:"log_enabled,omitempty"`
	LogSince       time.Time `json:"log_since,omitempty"`
	Theme          string    `json:"theme,omitempty"` // "dark" (по умолчанию) или "light"
	Lang           string    `json:"lang,omitempty"`  // "ru" (по умолчанию) или "en"
}

// LogLifetime — через сколько дней запись журналов выключается сама.
const LogLifetime = 14 * 24 * time.Hour

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Veil"), nil
}

func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "settings.json"), nil
}

func Load(path string) (*Settings, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Settings{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := &Settings{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Settings) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DNSAddrs возвращает корректные адреса подменённых DNS-серверов.
func (s *Settings) DNSAddrs() []netip.Addr {
	var out []netip.Addr
	for _, d := range s.DNS {
		if a, err := netip.ParseAddr(d); err == nil {
			out = append(out, a)
		}
	}
	return out
}

// LogExpired — включённая запись журналов просрочена и должна быть выключена.
func (s *Settings) LogExpired(now time.Time) bool {
	return s.LogEnabled && !s.LogSince.IsZero() && now.Sub(s.LogSince) > LogLifetime
}

// Dark — включена ли тёмная тема (по умолчанию да).
func (s *Settings) Dark() bool { return s.Theme != "light" }

// Language — язык интерфейса: "ru" или "en".
func (s *Settings) Language() string {
	if s.Lang == "en" {
		return "en"
	}
	return "ru"
}
