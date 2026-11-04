//go:build !windows

package gui

import (
	"os"
	"path/filepath"
	"syscall"

	"veil/internal/settings"
)

// acquireSingleInstance берёт блокировку файла в каталоге настроек; она снимается сама,
// когда процесс завершается. false — приложение уже запущено.
func acquireSingleInstance() (release func(), ok bool) {
	if os.Getenv("VEIL_MULTI") != "" {
		return func() {}, true
	}
	dir, err := settings.Dir()
	if err != nil {
		return func() {}, true
	}
	dir = filepath.Join(dir, "run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return func() {}, true
	}
	f, err := os.OpenFile(filepath.Join(dir, "app.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}, true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, false
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, true
}

// activateExisting без дополнительных средств окно другого экземпляра не поднять.
func activateExisting() {}
