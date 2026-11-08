package deploy

// Установка в Docker. Порядок при подключении по SSH:
//  1. Linux? Есть ли docker и docker compose?
//  2. Нет — дистрибутив поддерживается (Debian/Ubuntu, Fedora, RHEL/CentOS/Rocky/Alma) — ставим Docker;
//     не поддерживается — errDockerUnsupported, и протокол ставится по-старому, службой systemd.
//  3. Контейнер протокола уже есть — только выдаём клиента; нет — выкладываем Docker-файлы
//     (встроены в приложение) и готовые бинарники в /opt/veil, собираем образ и поднимаем контейнер.
// Все действия с клиентами идут через «docker exec» в контейнер: пути и команды внутри те же,
// что при установке без Docker, поэтому остальной код установщика не различает эти режимы.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	dockerfiles "veil/docker"
	"veil/internal/config"
	"veil/internal/vpnconf/ipsecconf"
	"veil/internal/vpnconf/ovpnconf"
	"veil/internal/vpnconf/wgconf"
)

const (
	dockerRoot     = "/opt/veil"
	containerStart = 3 * time.Minute
)

// errDockerUnsupported — Docker на этой системе не поставить: установщик переходит на установку без Docker.
var errDockerUnsupported = errors.New("docker не поддерживается на этой системе")

// container — протокол в контейнере: каталог в docker/, имя контейнера, нужные готовые бинарники.
type container struct {
	dir   string
	name  string
	bins  []string
	image string // имя образа: veil/<image>:latest (как в compose.yml)
}

var containers = map[string]container{
	"veil":      {"veil", "veil", []string{"veil-server"}, "veil-server"},
	"wireguard": {"wireguard", "veil-wireguard", []string{"wireguard-go"}, "wireguard"},
	"amneziawg": {"amneziawg", "veil-amneziawg", []string{"amneziawg-go"}, "amneziawg"},
	"openvpn":   {"openvpn", "veil-openvpn", nil, "openvpn"},
	"ipsec":     {"ipsec", "veil-ipsec", nil, "ipsec"},
}

// in возвращает remote, выполняющий команды внутри контейнера name.
func (r *remote) in(name string) *remote {
	c := *r
	c.ctr = name
	return &c
}

func (r *remote) containerExists(name string) bool {
	if r.ctr != "" {
		r = r.in("")
	}
	_, err := r.exec("docker inspect --format '{{.Id}}' "+shellQuote(name)+" >/dev/null 2>&1", nil, true)
	return err == nil
}

func (r *remote) dockerReady() bool {
	_, err := r.exec("docker version --format '{{.Server.Version}}' >/dev/null 2>&1 && docker compose version >/dev/null 2>&1", nil, true)
	return err == nil
}

// Скрипт установки Docker. Коды выхода: 0 — готово, 3 — система не поддерживается.
const dockerInstallScript = `set -e
log() { echo "$*"; }
ready() { docker version >/dev/null 2>&1 && docker compose version >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1; }
if ready; then echo READY; exit 0; fi
[ "$(uname -s)" = Linux ] || { echo "REASON=не Linux"; exit 3; }
[ -r /etc/os-release ] && . /etc/os-release
FAMILY=
case " ${ID:-} ${ID_LIKE:-} " in
  *" debian "*|*" ubuntu "*|*" raspbian "*) FAMILY=deb ;;
  *" fedora "*|*" rhel "*|*" centos "*|*" rocky "*|*" almalinux "*) FAMILY=rpm ;;
esac
[ -n "$FAMILY" ] || { echo "REASON=${PRETTY_NAME:-неизвестный дистрибутив}"; exit 3; }
if [ "$FAMILY" = deb ]; then
  export DEBIAN_FRONTEND=noninteractive
  command -v curl >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq curl ca-certificates; }
else
  command -v curl >/dev/null 2>&1 || dnf install -y curl || yum install -y curl
fi
if ! command -v docker >/dev/null 2>&1; then
  log "Установка Docker (get.docker.com)…"
  if ! curl -fsSL https://get.docker.com -o /tmp/veil-get-docker.sh || ! sh /tmp/veil-get-docker.sh >/tmp/veil-get-docker.log 2>&1; then
    log "get.docker.com не подошёл — ставлю пакеты дистрибутива"
    if [ "$FAMILY" = deb ]; then
      apt-get update -qq
      apt-get install -y -qq docker.io
    else
      REPO=centos; [ "${ID:-}" = fedora ] && REPO=fedora; [ "${ID:-}" = rhel ] && REPO=rhel
      (dnf install -y dnf-plugins-core && dnf config-manager --add-repo "https://download.docker.com/linux/$REPO/docker-ce.repo" \
        && dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin docker-buildx-plugin) ||
      (yum install -y yum-utils && yum-config-manager --add-repo "https://download.docker.com/linux/$REPO/docker-ce.repo" \
        && yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin docker-buildx-plugin)
    fi
  fi
  rm -f /tmp/veil-get-docker.sh
fi
if ! docker compose version >/dev/null 2>&1 || ! docker buildx version >/dev/null 2>&1; then
  if [ "$FAMILY" = deb ]; then
    apt-get update -qq
    apt-get install -y -qq docker-compose-plugin docker-buildx-plugin 2>/dev/null ||
      { apt-get install -y -qq docker-compose-v2 2>/dev/null || apt-get install -y -qq docker-compose 2>/dev/null || true; apt-get install -y -qq docker-buildx 2>/dev/null || true; }
  else
    dnf install -y docker-compose-plugin docker-buildx-plugin 2>/dev/null || yum install -y docker-compose-plugin docker-buildx-plugin || true
  fi
fi
command -v systemctl >/dev/null 2>&1 && systemctl enable --now docker >/dev/null 2>&1 || service docker start >/dev/null 2>&1 || true
# публикация портов по IPv6 (NAT66 для контейнеров) включена по умолчанию только с Docker 27
VER=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 0)
if [ "${VER%%.*}" -lt 27 ] 2>/dev/null && [ ! -s /etc/docker/daemon.json ]; then
  mkdir -p /etc/docker
  echo '{"ip6tables": true, "experimental": true}' > /etc/docker/daemon.json
  systemctl restart docker 2>/dev/null || service docker restart || true
fi
docker compose version >/dev/null 2>&1 || { echo "не удалось установить docker compose" >&2; exit 1; }
docker buildx version >/dev/null 2>&1 || { echo "не удалось установить docker buildx (нужен для сборки образов)" >&2; exit 1; }
echo "READY $VER"
`

// ensureDocker проверяет Docker и при необходимости ставит его. Для неподдерживаемых систем — errDockerUnsupported.
func (r *remote) ensureDocker(logf Logf) error {
	r = r.in("")
	if r.dockerReady() {
		if _, err := r.exec("docker buildx version >/dev/null 2>&1", nil, true); err == nil {
			ver, _ := r.exec("docker version --format '{{.Server.Version}}'", nil, true)
			r.step("docker", "Docker найден", "Docker "+strings.TrimSpace(ver)+", compose, buildx", "done")
			return nil
		}
	}
	logf("Docker не найден или неполный — установка (может занять несколько минут)…")
	r.step("docker", "Установка Docker", "get.docker.com или пакеты дистрибутива", "active")
	out, err := r.exec(dockerInstallScript, nil, true)
	if err != nil {
		if reason := parseKV(out)["REASON"]; reason != "" {
			return fmt.Errorf("%w: %s", errDockerUnsupported, reason)
		}
		return fmt.Errorf("установка Docker: %w", err)
	}
	ver := strings.TrimSpace(strings.TrimPrefix(out[strings.LastIndex(out, "READY"):], "READY"))
	logf("Docker готов: %s", ver)
	r.step("docker", "Установка Docker", "Docker "+ver+", compose, buildx", "done")
	return nil
}

func (r *remote) arch() (string, error) {
	machine, err := r.in("").exec("uname -sm", nil, false)
	if err != nil {
		return "", err
	}
	return goArch(machine)
}

// dockerBundle — архив для /opt/veil: docker/… (встроенные файлы), bin/… (бинарники), docker/<dir>/.env.
func dockerBundle(c container, arch, binDir string, env map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	add := func(name string, data []byte, mode int64) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: time.Now()}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	err := fs.WalkDir(dockerfiles.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := dockerfiles.FS.ReadFile(p)
		if err != nil {
			return err
		}
		mode := int64(0o644)
		if strings.HasSuffix(p, ".sh") {
			mode = 0o755
		}
		return add("docker/"+p, b, mode)
	})
	if err != nil {
		return nil, err
	}
	for _, name := range c.bins {
		b, err := findBinary(binDir, name, arch)
		if err != nil {
			return nil, err
		}
		if err := add("bin/"+name+"-linux-"+arch, b, 0o755); err != nil {
			return nil, err
		}
	}
	if err := add("docker/"+c.dir+"/.env", []byte(envFile(env)), 0o600); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// envFile — файл .env для docker compose; значения в одинарных кавычках (без подстановок).
func envFile(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# создано приложением Veil при установке; меняется при обновлении\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s='%s'\n", k, strings.ReplaceAll(env[k], "'", ""))
	}
	return b.String()
}

// startContainer выкладывает файлы и бинарники в /opt/veil, собирает образ и поднимает контейнер.
func (r *remote) startContainer(proto, binDir string, env map[string]string, recreate bool, logf Logf) error {
	r = r.in("")
	c, ok := containers[proto]
	if !ok {
		return fmt.Errorf("нет контейнера для протокола %s", proto)
	}
	arch, err := r.arch()
	if err != nil {
		return err
	}
	env["VEIL_BIN_FROM"] = "prebuilt"
	// MTU сети контейнера = MTU внешнего интерфейса: иначе на VPS с MTU < 1500 рвутся TLS и большие пакеты
	if mtu, err := r.exec(`d=$(ip -o route get 1.1.1.1 2>/dev/null | sed -n 's/.* dev \([^ ]*\).*/\1/p'); [ -n "$d" ] && cat /sys/class/net/$d/mtu`, nil, false); err == nil && mtu != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(mtu)); err == nil && n >= 1280 && n <= 9000 {
			env["VEIL_DOCKER_MTU"] = strconv.Itoa(n)
		}
	}
	bundle, err := dockerBundle(c, arch, binDir, env)
	if err != nil {
		return err
	}
	logf("Загрузка файлов Docker в %s (%d КБ)…", dockerRoot, len(bundle)/1024)
	r.step("files", "Файлы контейнеров", fmt.Sprintf("%s: docker/, bin/ (%d КБ)", dockerRoot, len(bundle)/1024), "active")
	tmp := dockerRoot + "/.upload.tgz"
	if err := r.upload(bundle, tmp, "0600"); err != nil {
		return err
	}
	if _, err := r.exec("tar -xzf "+tmp+" -C "+dockerRoot+" && rm -f "+tmp, nil, true); err != nil {
		return err
	}
	r.step("files", "Файлы контейнеров", fmt.Sprintf("%s: docker/, bin/ (%d КБ)", dockerRoot, len(bundle)/1024), "done")
	logf("Сборка образа %s (первый раз — несколько минут)…", c.image)
	r.step("build", "Сборка образа", "veil/"+c.image+":latest", "active")
	// образ собирается отдельно, в сети хоста: шаги RUN через мост Docker (MTU 1500) на VPS с меньшим MTU
	// подвешивают скачивание пакетов; compose потом только запускает готовый образ
	build := fmt.Sprintf("cd %s && docker build --network host --build-arg BIN_FROM=prebuilt -f docker/%s/Dockerfile -t veil/%s:latest .", dockerRoot, c.dir, c.image)
	out, err := r.exec(build+" 2>&1", nil, true)
	if err != nil {
		// сборка скачивает базовые образы и пакеты: при сетевом сбое пробуем ещё раз
		logf("Сборка не удалась, повтор…")
		if out, err = r.exec(build+" 2>&1", nil, true); err != nil {
			return fmt.Errorf("docker build: %w\n%s", err, tail(out, 30))
		}
	}
	r.step("build", "Сборка образа", "veil/"+c.image+":latest", "done")
	logf("Запуск контейнера %s…", c.name)
	names := c.name
	if proto == "veil" && env["COMPOSE_PROFILES"] == "admin" {
		names += ", veil-admin"
	}
	r.step("start", "Запуск контейнеров", names, "active")
	up := "cd " + dockerRoot + "/docker/" + c.dir + " && docker compose up -d --no-build --remove-orphans"
	if recreate {
		up += " --force-recreate"
	}
	if out, err := r.exec(up+" 2>&1", nil, true); err != nil {
		return fmt.Errorf("docker compose: %w\n%s", err, tail(out, 30))
	}
	if err := r.waitContainer(c.name); err != nil {
		r.step("start", "Запуск контейнеров", err.Error(), "fail")
		return err
	}
	if proto == "veil" && env["COMPOSE_PROFILES"] == "admin" {
		if err := r.waitContainer("veil-admin"); err != nil {
			r.step("start", "Запуск контейнеров", err.Error(), "fail")
			return err
		}
	}
	r.step("start", "Запуск контейнеров", names, "done")
	return nil
}

// osName — название дистрибутива сервера (PRETTY_NAME из /etc/os-release).
func (r *remote) osName() string {
	out, err := r.in("").exec(`. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME"`, nil, false)
	if err != nil || strings.TrimSpace(out) == "" {
		return "Linux"
	}
	return strings.TrimSpace(out)
}

// waitContainer ждёт, пока контейнер запустится и перестанет перезапускаться.
func (r *remote) waitContainer(name string) error {
	r = r.in("")
	deadline := time.Now().Add(containerStart)
	stable := 0
	var state string
	for time.Now().Before(deadline) {
		state, _ = r.exec("docker inspect --format '{{.State.Status}} {{.RestartCount}}' "+shellQuote(name), nil, true)
		if strings.HasPrefix(state, "running ") {
			if stable++; stable >= 3 {
				return nil
			}
		} else {
			stable = 0
		}
		time.Sleep(time.Second)
	}
	logs, _ := r.exec("docker logs --tail 40 "+shellQuote(name)+" 2>&1", nil, true)
	return fmt.Errorf("контейнер %s не запустился (%s):\n%s", name, state, logs)
}

// removeContainer останавливает контейнер протокола и удаляет его данные (том) и образ.
func (r *remote) removeContainer(proto string) error {
	r = r.in("")
	c := containers[proto]
	script := fmt.Sprintf(`if [ -f %[1]s/docker/%[2]s/compose.yml ]; then
  cd %[1]s/docker/%[2]s && docker compose down -v --rmi local --remove-orphans
else
  docker rm -f %[3]s
fi`, dockerRoot, c.dir, c.name)
	_, err := r.exec(script+" 2>&1", nil, true)
	return err
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ---------- обычный протокол «с нуля» ----------

// ProtocolResult — итог DeployProtocol: данные SSH для управления и конфигурация первого клиента.
type ProtocolResult struct {
	SSH   *config.SSHInfo
	WG    *wgconf.Config // WireGuard и AmneziaWG
	OVPN  *ovpnconf.Config
	IPsec *ipsecconf.Config
	// Existing — протокол на сервере уже был: ничего не ставилось, только выдан новый клиент.
	Existing bool
	// Container — контейнер протокола (пусто, если он стоит системной службой).
	Container string
	// Others — другие контейнеры Veil, которые уже работают на сервере (для экрана «Сервер уже настроен»).
	Others []ContainerInfo
}

// Protocols — протоколы, которые можно поставить на сервер сразу из формы «Свой сервер по SSH».
var Protocols = []string{"veil", "wireguard", "amneziawg", "openvpn", "ipsec"}

// DeployProtocol подключается по SSH и ставит на сервер обычный протокол (wireguard, amneziawg, openvpn, ipsec):
// в Docker, если он есть или его можно поставить, иначе службой systemd. Если протокол на сервере уже
// стоит — только выдаёт нового клиента. port — порт протокола (0 — по умолчанию).
func DeployProtocol(ctx context.Context, req Request, proto string, port int, logf Logf) (*ProtocolResult, error) {
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.User == "" {
		req.User = "root"
	}
	if req.Name == "" {
		req.Name = firstNonEmpty(req.IPv4, req.IPv6)
	}
	r := &remote{password: req.Password, prog: req.Progress}
	who := req.User + "@" + firstNonEmpty(req.IPv4, req.IPv6)
	r.step("ssh", "Подключение по SSH", who, "active")
	client, hostKey, err := connect(ctx, req, logf)
	if err != nil {
		r.step("ssh", "Подключение по SSH", err.Error(), "fail")
		return nil, err
	}
	defer client.Close()
	r.c = client
	r.step("ssh", "Подключение по SSH", who+", ключ хоста "+hostKey, "done")
	uid, err := r.exec("id -u", nil, false)
	if err != nil {
		return nil, err
	}
	r.root = uid == "0"
	if !r.root {
		if _, err := r.exec("true", nil, true); err != nil {
			return nil, fmt.Errorf("пользователь %s не может выполнить sudo: %w", req.User, err)
		}
		logf("Права: sudo")
	}
	arch, err := r.arch()
	if err != nil {
		return nil, err
	}
	logf("Сервер: linux/%s", arch)
	r.step("system", "Проверка системы", r.osName()+", linux/"+arch, "done")
	if proto != "ipsec" {
		if _, err := r.exec("test -c /dev/net/tun || (mkdir -p /dev/net && mknod /dev/net/tun c 10 200)", nil, true); err != nil {
			return nil, fmt.Errorf("на сервере недоступен /dev/net/tun (контейнер без TUN?): %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &Session{Profile: &config.Profile{Name: req.Name, IPv4: req.IPv4, IPv6: req.IPv6}, BinDir: req.ServerBinDir, r: r}
	host := firstNonEmpty(req.IPv4, req.IPv6)
	label := req.Name + "@" + hostnameOr("device")
	res := &ProtocolResult{SSH: &config.SSHInfo{Port: req.SSHPort, User: req.User, HostKey: hostKey, KeyPath: req.KeyPath}}
	if port == 0 {
		port = 20000 + randInt(0, 39999)
		if proto == "openvpn" {
			port = 1194
		}
	}
	installed := map[string]func() bool{"wireguard": s.WireGuardInstalled, "amneziawg": s.AmneziaWGInstalled, "openvpn": s.OpenVPNInstalled, "ipsec": s.IPsecInstalled}
	check, ok := installed[proto]
	if !ok {
		return nil, fmt.Errorf("неизвестный протокол %q", proto)
	}
	res.Existing = check()
	if res.Existing {
		logf("%s уже установлен — выдаю нового клиента", proto)
		r.step("client", "Протокол уже на сервере", "ничего не переустанавливается — выдаю нового клиента", "active")
	}
	switch proto {
	case "wireguard":
		if res.Existing {
			res.WG, err = s.WireGuardAddClient(label, host)
		} else {
			res.WG, err = s.InstallWireGuard(WGInstall{Port: port}, host, logf)
		}
	case "amneziawg":
		if res.Existing {
			res.WG, err = s.AmneziaWGAddClient(label, host)
		} else {
			res.WG, err = s.InstallAmneziaWG(AWGInstall{Port: port}, host, logf)
		}
	case "openvpn":
		if res.Existing {
			res.OVPN, err = s.OpenVPNAddClient(label, host)
		} else {
			res.OVPN, err = s.InstallOpenVPN(OVInstall{Port: port, Proto: "udp"}, host, logf)
		}
	case "ipsec":
		if res.Existing {
			res.IPsec, err = s.IPsecAddClient(label, host)
		} else {
			res.IPsec, err = s.InstallIPsec(host, logf)
		}
	}
	if err != nil {
		r.step("client", "Клиент", err.Error(), "fail")
		return nil, err
	}
	if s.inDocker(proto) {
		res.Container = containers[proto].name
	}
	if all, err := s.Containers(); err == nil {
		for _, c := range all {
			if c.Name != res.Container {
				res.Others = append(res.Others, c)
			}
		}
	}
	r.step("client", "Клиент выдан", label, "done")
	return res, nil
}

// ---------- сервер Veil в контейнере ----------

// veilEnv — переменные compose для контейнера veil, который ставит приложение: конфиг в /etc/veil хоста
// (его создаёт и дополняет установщик), первого клиента контейнер сам не создаёт.
func veilEnv(adminPanel bool) map[string]string {
	env := map[string]string{"VEIL_DATA": "/etc/veil", "VEIL_FIRST_CLIENT": "", "VEIL_DETECT_IP": "0", "VEIL_ADMIN": "off", "COMPOSE_PROFILES": ""}
	if adminPanel {
		env["VEIL_ADMIN"], env["COMPOSE_PROFILES"] = "container", "admin"
	}
	return env
}

// veilDockerMode решает, ставить ли Veil в Docker: нет, если он уже стоит службой systemd
// (такие установки обновляются как раньше) или Docker на системе не поставить.
func (r *remote) veilDockerMode(logf Logf) (bool, error) {
	if _, err := r.exec("test -f "+remoteUnit, nil, true); err == nil {
		logf("Veil уже установлен службой systemd — обновляю без Docker")
		r.step("docker", "Docker", "Veil уже стоит службой systemd — без Docker", "skip")
		return false, nil
	}
	if r.containerExists(containers["veil"].name) {
		logf("Найден контейнер veil")
		r.step("docker", "Docker найден", "контейнер veil уже работает", "done")
		return true, nil
	}
	err := r.ensureDocker(logf)
	if errors.Is(err, errDockerUnsupported) {
		logf("Docker недоступен (%v) — установка службой systemd", err)
		r.step("docker", "Docker", strings.TrimPrefix(err.Error(), errDockerUnsupported.Error()+": ")+" — Docker не поддерживается, ставлю службой systemd", "skip")
		return false, nil
	}
	return err == nil, err
}

// runVeilContainer поднимает контейнер veil или перезапускает его, чтобы он перечитал конфиг.
func (r *remote) runVeilContainer(binDir string, logf Logf) error {
	name := containers["veil"].name
	if r.containerExists(name) {
		// Veil уже в контейнере: конфиг с новым клиентом записан, контейнер только перечитывает его
		logf("Перезапуск контейнера veil…")
		r.step("start", "Перезапуск контейнера", "veil перечитывает конфиг с новым клиентом", "active")
		if _, err := r.exec("docker restart "+name, nil, true); err != nil {
			return err
		}
		if err := r.waitContainer(name); err != nil {
			r.step("start", "Перезапуск контейнера", err.Error(), "fail")
			return err
		}
		r.step("start", "Перезапуск контейнера", "veil", "done")
	} else if err := r.startContainer("veil", binDir, veilEnv(r.adminPanel), false, logf); err != nil {
		return err
	}
	in := r.in(name)
	for i := 0; i < 30; i++ {
		if _, err := in.exec("test -S "+controlSocket, nil, true); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	logs, _ := r.exec("docker logs --tail 30 "+name+" 2>&1", nil, true)
	return fmt.Errorf("сервер в контейнере не отвечает:\n%s", logs)
}

// ---------- Session: протоколы в контейнерах ----------

// proto возвращает remote для команд протокола: внутри его контейнера, если он есть, иначе на хосте.
func (s *Session) proto(name string) *remote {
	if s.inDocker(name) {
		return s.r.in(containers[name].name)
	}
	return s.r
}

// inDocker — протокол установлен в контейнере (результат запоминается на время сессии).
func (s *Session) inDocker(name string) bool {
	if s.docker == nil {
		s.docker = map[string]bool{}
	}
	if v, ok := s.docker[name]; ok {
		return v
	}
	v := s.r.containerExists(containers[name].name)
	if v {
		s.docker[name] = true
	}
	return v
}

// installInDocker ставит протокол в контейнер. Если Docker на системе не поставить —
// errDockerUnsupported (вызывающий ставит протокол без Docker).
func (s *Session) installInDocker(proto string, env map[string]string, logf Logf) error {
	if err := s.r.ensureDocker(logf); err != nil {
		return err
	}
	if err := s.r.startContainer(proto, s.BinDir, env, false, logf); err != nil {
		return err
	}
	if s.docker == nil {
		s.docker = map[string]bool{}
	}
	s.docker[proto] = true
	return nil
}

// tryDocker ставит протокол в Docker, если он ещё не стоит службой на хосте (nativeMarker — проверка этого).
// true — протокол в контейнере; false без ошибки — ставить без Docker (старая установка или система без Docker).
// proto — ключ протокола в containers (wireguard, …), title — его название для журнала.
func (s *Session) tryDocker(proto, title string, nativeMarker string, env map[string]string, logf Logf) (bool, error) {
	if _, err := s.r.exec(nativeMarker, nil, true); err == nil {
		logf("%s уже стоит на сервере без Docker — обновляю как есть", title)
		return false, nil
	}
	err := s.installInDocker(proto, env, logf)
	if errors.Is(err, errDockerUnsupported) {
		logf("Docker недоступен (%v) — установка без Docker", err)
		return false, nil
	}
	return err == nil, err
}

// ula6For — IPv6-адрес клиента в ULA-подсети интерфейса (если она есть в конфиге сервера):
// тот же номер хоста, что у IPv4. Контейнеры Veil дают туннелям WireGuard/AmneziaWG адреса IPv4 и IPv6.
func ula6For(conf string, ip4 netip.Addr, net4 netip.Prefix) (netip.Addr, bool) {
	var p6 netip.Prefix
	for _, ln := range strings.Split(strings.ReplaceAll(conf, "\r", ""), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(k), "address") {
			continue
		}
		for _, f := range strings.Split(v, ",") {
			if p, err := netip.ParsePrefix(strings.TrimSpace(f)); err == nil && p.Addr().Is6() && p.Bits() <= 120 {
				p6 = p.Masked()
			}
		}
		break
	}
	if !p6.IsValid() || !net4.IsValid() {
		return netip.Addr{}, false
	}
	a4, n4 := ip4.As4(), net4.Masked().Addr().As4()
	idx := (uint32(a4[0])<<24 | uint32(a4[1])<<16 | uint32(a4[2])<<8 | uint32(a4[3])) -
		(uint32(n4[0])<<24 | uint32(n4[1])<<16 | uint32(n4[2])<<8 | uint32(n4[3]))
	b := p6.Addr().As16()
	b[12], b[13], b[14], b[15] = byte(idx>>24), byte(idx>>16), byte(idx>>8), byte(idx)
	return netip.AddrFrom16(b), true
}
