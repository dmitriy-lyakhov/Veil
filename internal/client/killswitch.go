package client

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
)

// «Мягкий» KillSwitch: пока туннель поднят, рядом с его маршрутами лежат запасные
// маршруты в никуда (с большой метрикой). Пока работает туннель, побеждают его маршруты.
// Если помощник упал, а адаптер исчез, остаются только запасные — и трафик, который
// шёл бы в туннель, блокируется, а не уходит в открытую сеть. При обычном отключении
// запасные маршруты убираются. Список хранится в файле, чтобы снять блокировку после сбоя.

type killState struct {
	Prefixes []string `json:"prefixes"`
}

func killStatePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Veil", "run", "killswitch.json"), nil
}

// killSwitchPrefixes — что перекрывать: общие половинки при полном туннеле либо
// выбранные подсети при выборочном (напрямую идущий трафик не трогаем).
func killSwitchPrefixes(nc NetConfig) []netip.Prefix {
	if fullTunnel(nc) {
		return []netip.Prefix{
			netip.MustParsePrefix("0.0.0.0/1"), netip.MustParsePrefix("128.0.0.0/1"),
			netip.MustParsePrefix("::/1"), netip.MustParsePrefix("8000::/1"),
		}
	}
	return append([]netip.Prefix(nil), nc.Plan.Via...)
}

func armKillSwitch(nc NetConfig, u *undoStack, log *slog.Logger) error {
	prefixes := killSwitchPrefixes(nc)
	path, err := killStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	strs := make([]string, len(prefixes))
	for i, p := range prefixes {
		strs[i] = p.String()
	}
	b, _ := json.Marshal(killState{Prefixes: strs})
	// сначала запись о том, что будет добавлено: сбой на полпути не оставит неучтённых маршрутов
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	if err := addBlackholes(prefixes, log); err != nil {
		removeBlackholes(prefixes)
		_ = os.Remove(path)
		return err
	}
	u.push(func() {
		removeBlackholes(prefixes)
		_ = os.Remove(path)
	})
	log.Info("kill switch armed", "routes", len(prefixes))
	return nil
}

// Unblock снимает запасные маршруты, оставшиеся после аварийного завершения. Безопасно вызывать всегда.
func Unblock() error {
	path, err := killStatePath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var st killState
	if err := json.Unmarshal(b, &st); err == nil {
		var ps []netip.Prefix
		for _, s := range st.Prefixes {
			if p, err := netip.ParsePrefix(s); err == nil {
				ps = append(ps, p)
			}
		}
		removeBlackholes(ps)
	}
	return os.Remove(path)
}

// Blocked сообщает, остались ли следы блокировки (для интерфейса).
func Blocked() bool {
	path, err := killStatePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
