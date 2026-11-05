//go:build !windows

package client

import (
	"os/exec"
)

func hideWindow(*exec.Cmd) {}

func decodeOutput(b []byte) string { return string(b) }

func prepareNetwork(NetConfig) error { return nil }
