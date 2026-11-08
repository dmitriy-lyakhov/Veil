//go:build !linux

package netsys

import "errors"

// EnableGateway — режим шлюза сайта поддерживается только на Linux (роутеры, серверы в офисе).
func EnableGateway(string) (func(), []string, error) {
	return nil, nil, errors.New("режим шлюза (-gateway) поддерживается только на Linux")
}
