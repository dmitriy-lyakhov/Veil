package deploy

// Шаги установки для экрана прогресса и управление контейнерами сервера из приложения.

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Step — шаг установки. ID: ssh, system, docker, files, build, start, profile.
// State: "active", "done", "skip" (например, Docker недоступен — служба systemd), "fail".
type Step struct {
	ID     string
	Title  string
	Detail string
	State  string
}

// Progress получает шаги установки по мере выполнения (может быть nil).
type Progress func(Step)

func (r *remote) step(id, title, detail, state string) {
	if r.prog != nil {
		r.prog(Step{ID: id, Title: title, Detail: detail, State: state})
	}
}

// ContainerInfo — контейнер Veil на сервере (метка net.veil.protocol).
type ContainerInfo struct {
	Name     string // veil, veil-admin, veil-wireguard, …
	Protocol string // veil, admin, wireguard, amneziawg, openvpn, ipsec
	Image    string
	State    string // running, restarting, exited, …
	Status   string // «Up 2 days», как в docker ps
	Ports    string
}

// Running — контейнер работает.
func (c ContainerInfo) Running() bool { return c.State == "running" }

// DockerVersion — версия Docker на сервере; пусто, если Docker нет.
func (s *Session) DockerVersion() string {
	out, err := s.r.exec("docker version --format '{{.Server.Version}}' 2>/dev/null", nil, true)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Containers — контейнеры Veil на сервере (пусто, если Docker нет или Veil стоит без него).
func (s *Session) Containers() ([]ContainerInfo, error) {
	if s.DockerVersion() == "" {
		return nil, nil
	}
	out, err := s.r.exec(`docker ps -a --filter label=net.veil.protocol --format '{{.Names}}|{{.Label "net.veil.protocol"}}|{{.Image}}|{{.State}}|{{.Status}}|{{.Ports}}'`, nil, true)
	if err != nil {
		return nil, err
	}
	var list []ContainerInfo
	for _, ln := range strings.Split(out, "\n") {
		f := strings.SplitN(strings.TrimSpace(ln), "|", 6)
		if len(f) < 6 {
			continue
		}
		list = append(list, ContainerInfo{Name: f[0], Protocol: f[1], Image: f[2], State: f[3], Status: f[4], Ports: f[5]})
	}
	return list, nil
}

var containerNameRe = regexp.MustCompile(`^veil(-[a-z0-9]+)?$`)

// ContainerLogs — последние строки журнала контейнера Veil.
func (s *Session) ContainerLogs(name string, lines int) (string, error) {
	if !containerNameRe.MatchString(name) {
		return "", errors.New("недопустимое имя контейнера")
	}
	if lines <= 0 || lines > 2000 {
		lines = 200
	}
	return s.r.exec(fmt.Sprintf("docker logs --tail %d %s 2>&1", lines, name), nil, true)
}

// RestartContainer перезапускает контейнер Veil и ждёт, пока он поднимется.
func (s *Session) RestartContainer(name string) error {
	if !containerNameRe.MatchString(name) {
		return errors.New("недопустимое имя контейнера")
	}
	if _, err := s.r.exec("docker restart "+name, nil, true); err != nil {
		return err
	}
	return s.r.waitContainer(name)
}

// AdminPanel сообщает, работает ли веб-панель в контейнере veil-admin, и можно ли её включать
// и выключать (только когда сам Veil стоит в Docker).
func (s *Session) AdminPanel() (on, available bool) {
	if !s.inDocker("veil") {
		return false, false
	}
	state, err := s.r.exec("docker inspect --format '{{.State.Status}}' veil-admin 2>/dev/null", nil, true)
	return err == nil && strings.TrimSpace(state) == "running", true
}

// SetAdminPanel включает или выключает контейнер veil-admin и переключает режим панели в veil-server
// (выключенная панель — /admin отвечает 404).
func (s *Session) SetAdminPanel(on bool, logf Logf) error {
	if !s.inDocker("veil") {
		return errors.New("админ-панель в отдельном контейнере доступна, только когда Veil установлен в Docker")
	}
	dir := dockerRoot + "/docker/veil"
	profile, mode := "", "off"
	if on {
		profile, mode = "admin", "container"
	}
	// .env пишет приложение: меняем только две переменные, остальное оставляем как есть
	script := fmt.Sprintf(`set -e
cd %s
touch .env
sed -i '/^COMPOSE_PROFILES=/d; /^VEIL_ADMIN=/d' .env
echo "COMPOSE_PROFILES='%s'" >> .env
echo "VEIL_ADMIN='%s'" >> .env
`, dir, profile, mode)
	if _, err := s.r.exec(script, nil, true); err != nil {
		return err
	}
	if on {
		logf("Запуск контейнера veil-admin…")
		if out, err := s.r.exec("cd "+dir+" && docker compose up -d --no-build --remove-orphans 2>&1", nil, true); err != nil {
			return fmt.Errorf("docker compose: %w\n%s", err, tail(out, 20))
		}
		if err := s.r.waitContainer("veil-admin"); err != nil {
			return err
		}
	} else {
		logf("Остановка контейнера veil-admin…")
		if out, err := s.r.exec("docker rm -f veil-admin 2>&1 || true", nil, true); err != nil {
			return fmt.Errorf("%w\n%s", err, out)
		}
	}
	return s.Call("POST", "settings/admin", map[string]string{"mode": mode}, nil)
}
