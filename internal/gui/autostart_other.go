//go:build !windows

package gui

import "errors"

func setAutostart(bool) error {
	return errors.New("автозапуск пока поддерживается только в Windows")
}
