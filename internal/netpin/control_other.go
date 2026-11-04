//go:build !windows

package netpin

import "syscall"

func Control(string, string, syscall.RawConn) error { return nil }
