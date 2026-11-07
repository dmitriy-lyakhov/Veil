//go:build !windows

package netsys

import (
	"os/exec"
)

func HideWindow(*exec.Cmd) {}

func DecodeOutput(b []byte) string { return string(b) }

func PrepareNetwork(NetConfig) error { return nil }
