package netsys

import (
	"net/netip"
	"runtime"
	"strings"
)

// DefaultGateways — шлюзы по умолчанию (IPv4) по именам сетевых интерфейсов. Нужны странице выбора интерфейса:
// трафик до сервера идёт через шлюз выбранного интерфейса. Интерфейсы без шлюза в результате отсутствуют.
func DefaultGateways() map[string]string {
	var out string
	var err error
	switch runtime.GOOS {
	case "windows":
		out, err = Run("powershell", "-NoProfile", "-NonInteractive", "-Command",
			`Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -ne '0.0.0.0' } | ForEach-Object { "$($_.InterfaceAlias)|$($_.NextHop)" }`)
	case "darwin":
		out, err = Run("netstat", "-rn", "-f", "inet")
	default:
		out, err = Run("ip", "-4", "route", "show", "default")
	}
	if err != nil {
		return nil
	}
	return parseGateways(runtime.GOOS, out)
}

func parseGateways(goos, out string) map[string]string {
	res := map[string]string{}
	add := func(iface, gw string) {
		if _, err := netip.ParseAddr(gw); err == nil && iface != "" && res[iface] == "" {
			res[iface] = gw
		}
	}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		switch goos {
		case "windows":
			if i := strings.LastIndex(line, "|"); i > 0 {
				add(strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]))
			}
		case "darwin":
			if len(f) >= 4 && f[0] == "default" {
				add(f[len(f)-1], f[1])
			}
		default:
			if len(f) >= 5 && f[0] == "default" {
				for i := 1; i+1 < len(f); i++ {
					if f[i] == "dev" {
						add(f[i+1], f[2])
					}
				}
			}
		}
	}
	return res
}
