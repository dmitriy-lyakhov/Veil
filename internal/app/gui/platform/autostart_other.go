//go:build !windows

package platform

import "errors"

func SetAutostart(bool) error {
	return errors.New("автозапуск пока поддерживается только в Windows")
}
