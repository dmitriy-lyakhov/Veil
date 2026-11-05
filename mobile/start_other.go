//go:build !linux

package veilcore

import "errors"

// Start доступен только на Linux/Android.
func Start(link string, tunFd int32, protector Protector) error {
	return errors.New("Start: только Android/Linux")
}

func releaseProtect() {}
