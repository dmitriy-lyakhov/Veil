//go:build !windows && !linux && !darwin

package netpin

import "syscall"

func Control(string, string, syscall.RawConn) error { return nil }
