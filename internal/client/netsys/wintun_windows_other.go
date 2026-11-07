//go:build windows && !amd64

package netsys

import "errors"

func ensureWintun() error {
	return errors.New("встроенный wintun.dll есть только для Windows amd64: положите wintun.dll рядом с программой")
}
