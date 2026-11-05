package client

import (
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"veil/internal/config"
)

const hostsMarker = "# veil-admin"

func hostsPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// rewriteHosts убирает из hosts прежнюю запись Veil и, если ip задан, дописывает «ip admin».
func rewriteHosts(ip string) error {
	path := hostsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	eol := "\n"
	if strings.Contains(string(b), "\r\n") {
		eol = "\r\n"
	}
	var keep []string
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if !strings.Contains(line, hostsMarker) {
			keep = append(keep, line)
		}
	}
	for len(keep) > 0 && keep[len(keep)-1] == "" {
		keep = keep[:len(keep)-1]
	}
	if ip != "" {
		keep = append(keep, ip+" admin "+hostsMarker)
	}
	out := strings.Join(keep, eol) + eol
	if out == string(b) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// adminAliasIP — адрес админки из профиля, если это IP-адрес.
func adminAliasIP(p *config.Profile) string {
	if p == nil || p.AdminURL == "" {
		return ""
	}
	u, err := url.Parse(p.AdminURL)
	if err != nil {
		return ""
	}
	if a, err := netip.ParseAddr(u.Hostname()); err == nil {
		return a.String()
	}
	return ""
}

// installAdminAlias прописывает имя «admin» на адрес админки сервера на время подключения,
// чтобы внутри туннеля она открывалась по https://admin. Без прав на запись в hosts — только предупреждение.
// Возвращает AdminHostsOK, AdminHostsFailed или "" (у профиля нет админки по IP-адресу).
func installAdminAlias(p *config.Profile, u *undoStack, log *slog.Logger) string {
	ip := adminAliasIP(p)
	if ip == "" {
		return ""
	}
	if err := rewriteHosts(ip); err != nil {
		log.Warn("hosts: имя admin не прописано", "err", err)
		return AdminHostsFailed
	}
	u.push(func() { _ = rewriteHosts("") })
	return AdminHostsOK
}

// removeStaleAdminAlias убирает запись от прошлого сеанса, завершённого аварийно.
func removeStaleAdminAlias() { _ = rewriteHosts("") }
