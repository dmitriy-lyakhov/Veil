//go:build linux

package client

import (
	"fmt"
	"log/slog"
	"net/netip"
)

func addBlackholes(prefixes []netip.Prefix, log *slog.Logger) error {
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("route replace blackhole %s metric 9000", p))
	}
	return ipBatch(lines)
}

func removeBlackholes(prefixes []netip.Prefix) {
	var lines []string
	for _, p := range prefixes {
		lines = append(lines, fmt.Sprintf("route delete blackhole %s metric 9000", p))
	}
	_ = ipBatch(lines)
}
