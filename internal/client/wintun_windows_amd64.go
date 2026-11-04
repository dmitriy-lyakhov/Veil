//go:build windows && amd64

package client

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// wintun.dll 0.14.1 (WireGuard LLC, подписана) встроен в приложение и
// раскладывается рядом с exe: wintun ищет библиотеку только там и в System32.
//
//go:embed wintun_amd64.dll
var wintunDLL []byte

func ensureWintun() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	p := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if b, err := os.ReadFile(p); err == nil && bytes.Equal(b, wintunDLL) {
		return nil
	}
	if err := os.WriteFile(p, wintunDLL, 0o644); err != nil {
		return fmt.Errorf("не удалось записать wintun.dll рядом с программой (%s): %w", filepath.Dir(exe), err)
	}
	return nil
}
