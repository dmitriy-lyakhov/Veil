package deploy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"veil/internal/config"
	"veil/internal/provision"
	"veil/internal/site"
	"veil/internal/transport/front"
	"veil/internal/transport/netpin"
)

const (
	remoteBinary = "/usr/local/bin/veil-server"
	remoteConfig = "/etc/veil/server.json"
	remoteUnit   = "/etc/systemd/system/veil.service"
	sshTimeout   = 10 * time.Second
	startWait    = 8 * time.Second
	adminUser    = "admin"
)

const unitFile = `[Unit]
Description=Veil VPN server
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/veil-server -config /etc/veil/server.json
Restart=on-failure
RestartSec=2
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`

type Request struct {
	Name          string
	IPv4          string
	IPv6          string
	SSHPort       int
	User          string
	Password      string
	KeyPEM        []byte
	KeyPath       string
	KeyPassphrase string
	KnownHostKey  string
	ServerBinDir  string
	SiteLang      string
	FrontSNI      string
}

type Logf func(format string, args ...any)

func Deploy(ctx context.Context, req Request, logf Logf) (*config.Profile, error) {
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.User == "" {
		req.User = "root"
	}
	if req.Name == "" {
		req.Name = firstNonEmpty(req.IPv4, req.IPv6)
	}
	client, hostKey, err := connect(ctx, req, logf)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	r := &remote{c: client, password: req.Password}
	return r.install(ctx, req, hostKey, logf)
}

func connect(ctx context.Context, req Request, logf Logf) (*ssh.Client, string, error) {
	targets, err := (&config.Profile{IPv4: req.IPv4, IPv6: req.IPv6}).Targets()
	if err != nil {
		return nil, "", err
	}
	auth, err := authMethods(req)
	if err != nil {
		return nil, "", err
	}
	var hostKey string
	sshCfg := &ssh.ClientConfig{
		User:    req.User,
		Auth:    auth,
		Timeout: sshTimeout,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			hostKey = ssh.FingerprintSHA256(key)
			if req.KnownHostKey != "" && req.KnownHostKey != hostKey {
				return fmt.Errorf("ключ хоста изменился: ожидался %s, получен %s", req.KnownHostKey, hostKey)
			}
			return nil
		},
	}
	var errs []error
	for _, t := range targets {
		if !netpin.Reachable(t.Addr.Addr().Is6()) {
			// VPN включён, а до адресов этого семейства нет прямого пути: SSH ушёл бы в туннель и оборвался бы
			logf("SSH: %s пропущен — до него нет прямого пути мимо туннеля", t.Family)
			errs = append(errs, fmt.Errorf("%s: нет прямого пути мимо туннеля", t.Family))
			continue
		}
		addr := netip.AddrPortFrom(t.Addr.Addr(), uint16(req.SSHPort)).String()
		logf("SSH: подключение к %s (%s)…", addr, t.Family)
		client, err := dialSSH(ctx, addr, sshCfg)
		if err == nil {
			logf("SSH: подключено через %s, ключ хоста %s", t.Family, hostKey)
			return client, hostKey, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", t.Family, err))
	}
	return nil, "", fmt.Errorf("не удалось подключиться по SSH: %w", errors.Join(errs...))
}

func dialSSH(ctx context.Context, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	d := net.Dialer{Timeout: sshTimeout, Control: netpin.Control}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(2 * sshTimeout))
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(c, chans, reqs), nil
}

func authMethods(req Request) ([]ssh.AuthMethod, error) {
	var out []ssh.AuthMethod
	keyPEM := req.KeyPEM
	if len(keyPEM) == 0 && req.KeyPath != "" {
		b, err := os.ReadFile(expandHome(req.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("чтение SSH-ключа: %w", err)
		}
		keyPEM = b
	}
	if len(keyPEM) > 0 {
		var signer ssh.Signer
		var err error
		if req.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyPEM, []byte(req.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyPEM)
		}
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, errors.New("SSH-ключ защищён паролем — укажите его")
			}
			return nil, fmt.Errorf("SSH-ключ: %w", err)
		}
		out = append(out, ssh.PublicKeys(signer))
	}
	if req.Password != "" {
		pw := req.Password
		out = append(out, ssh.Password(pw), ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
			ans := make([]string, len(qs))
			for i := range ans {
				ans[i] = pw
			}
			return ans, nil
		}))
	}
	if len(out) == 0 {
		return nil, errors.New("укажите пароль или SSH-ключ")
	}
	return out, nil
}

type remote struct {
	c        *ssh.Client
	password string
	root     bool
}

func (r *remote) exec(cmd string, stdin []byte, asRoot bool) (string, error) {
	sess, err := r.c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	full := cmd
	if asRoot && !r.root {
		if r.password != "" {
			full = "sudo -S -k -p '' sh -c " + shellQuote(cmd)
			stdin = append([]byte(r.password+"\n"), stdin...)
		} else {
			full = "sudo -n sh -c " + shellQuote(cmd)
		}
	}
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	if stdin != nil {
		sess.Stdin = bytes.NewReader(stdin)
	}
	err = sess.Run(full)
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		return out, fmt.Errorf("%s: %w %s", cmd, err, strings.TrimSpace(stderr.String()+"\n"+out))
	}
	return out, nil
}

func (r *remote) upload(data []byte, dst string, mode string) error {
	tmp := "/tmp/veil-" + randomHex(6)
	if _, err := r.exec("umask 077 && cat > "+tmp, data, false); err != nil {
		return err
	}
	defer r.exec("rm -f "+tmp, nil, false)
	_, err := r.exec(fmt.Sprintf("install -D -m %s %s %s", mode, tmp, shellQuote(dst)), nil, true)
	return err
}

func (r *remote) install(ctx context.Context, req Request, hostKey string, logf Logf) (*config.Profile, error) {
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
	machine, err := r.exec("uname -sm", nil, false)
	if err != nil {
		return nil, err
	}
	arch, err := goArch(machine)
	if err != nil {
		return nil, err
	}
	logf("Сервер: %s → linux/%s", machine, arch)
	if _, err := r.exec("command -v systemctl", nil, false); err != nil {
		return nil, errors.New("на сервере нет systemd — пока поддерживаются только дистрибутивы с systemd")
	}
	if _, err := r.exec("test -c /dev/net/tun || (mkdir -p /dev/net && mknod /dev/net/tun c 10 200)", nil, true); err != nil {
		return nil, fmt.Errorf("на сервере недоступен /dev/net/tun (контейнер без TUN?): %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bin, err := ServerBinary(req.ServerBinDir, arch)
	if err != nil {
		return nil, err
	}

	srvCfg, existed, err := r.loadOrCreateConfig(req.SiteLang, logf)
	if err != nil {
		return nil, err
	}
	if !existed {
		if busy, _ := r.exec("ss -Hltn 'sport = :443' 2>/dev/null", nil, true); busy != "" {
			return nil, fmt.Errorf("порт 443 на сервере уже занят:\n%s", busy)
		}
	}
	if err := applyFront(srvCfg, req.FrontSNI); err != nil {
		return nil, err
	}
	prof, err := provision.AddPeer(srvCfg, uniquePeerName(srvCfg, req.Name))
	if err != nil {
		return nil, err
	}
	if srvCfg.Front != nil && srvCfg.Front.SNI != "" {
		prof.FrontSNI, prof.FrontKey = srvCfg.Front.SNI, srvCfg.Front.Key
		logf("Маскировка под %s: посторонние соединения уйдут на настоящий сайт", srvCfg.Front.SNI)
		r.checkFrontReachable(srvCfg.Front.SNI, logf)
	}
	prof.Name = req.Name
	prof.IPv4, prof.IPv6 = req.IPv4, req.IPv6
	prof.SSH = &config.SSHInfo{Port: req.SSHPort, User: req.User, HostKey: hostKey, KeyPath: req.KeyPath}
	if req.IPv4 != "" {
		srvCfg.PublicIPv4 = req.IPv4
	}
	if req.IPv6 != "" {
		srvCfg.PublicIPv6 = req.IPv6
	}
	if srvCfg.Site == nil || !srvCfg.Site.Valid() {
		cover := site.Generate(req.SiteLang)
		srvCfg.Site = &cover
	}
	logf("Сайт-прикрытие: «%s» — %s", srvCfg.Site.Company, srvCfg.Site.Tagline)
	prof.AdminURL = provision.AdminURL(srvCfg)
	if srvCfg.Admin.PasswordHash == "" {
		password := provision.NewAdminPassword()
		hash, err := provision.HashAdminPassword(password)
		if err != nil {
			return nil, err
		}
		srvCfg.Admin = config.Admin{User: adminUser, PasswordHash: hash}
		prof.AdminUser, prof.AdminPassword = adminUser, password
		logf("Админ-панель (доступна только через VPN): %s, логин %s, пароль %s", prof.AdminURL, adminUser, password)
	} else {
		prof.AdminUser = srvCfg.Admin.User
		logf("Админ-панель: %s — пароль не менялся (задан при первой установке)", prof.AdminURL)
	}

	logf("Загрузка серверного бинарника (%d КБ)…", len(bin)/1024)
	if err := r.upload(bin, remoteBinary, "0755"); err != nil {
		return nil, err
	}
	cfgJSON, err := json.MarshalIndent(srvCfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := r.upload(cfgJSON, remoteConfig, "0600"); err != nil {
		return nil, err
	}
	if err := r.upload([]byte(unitFile), remoteUnit, "0644"); err != nil {
		return nil, err
	}
	r.openFirewall(logf)
	logf("Запуск службы veil…")
	if _, err := r.exec("systemctl daemon-reload && systemctl enable veil >/dev/null 2>&1; systemctl restart veil", nil, true); err != nil {
		return nil, err
	}
	if err := r.waitActive(); err != nil {
		journal, _ := r.exec("journalctl -u veil -n 30 --no-pager", nil, true)
		return nil, fmt.Errorf("служба не запустилась: %w\n%s", err, journal)
	}
	logf("Служба veil запущена, устройств на сервере: %d", len(srvCfg.Peers))
	return prof, nil
}

func applyFront(cfg *config.Server, want string) error {
	sni, err := front.NormalizeSNI(want)
	if err != nil {
		return err
	}
	if sni == "" || (cfg.Front != nil && cfg.Front.SNI == sni) {
		return nil
	}
	key := ""
	if cfg.Front != nil {
		key = cfg.Front.Key
	}
	if key == "" {
		if key, err = front.NewKey(); err != nil {
			return err
		}
	}
	cfg.Front = &config.Front{SNI: sni, Key: key}
	return nil
}

func (r *remote) checkFrontReachable(sni string, logf Logf) {
	cmd := "timeout 8 bash -c 'exec 3<>/dev/tcp/" + sni + "/443'"
	if _, err := r.exec(cmd, nil, false); err != nil {
		logf("Внимание: сервер не достучался до %s:443 — посторонние соединения проксироваться не будут. Выберите другой сайт.", sni)
	}
}

func (r *remote) loadOrCreateConfig(siteLang string, logf Logf) (*config.Server, bool, error) {
	if out, err := r.exec("test -f "+remoteConfig+" && cat "+remoteConfig, nil, true); err == nil && out != "" {
		var cfg config.Server
		if err := json.Unmarshal([]byte(out), &cfg); err == nil {
			logf("Найдена существующая установка — добавляю новое устройство")
			return &cfg, true, nil
		}
		logf("Существующий конфиг повреждён — создаю новый")
	}
	cfg, err := provision.NewServer(siteLang)
	return cfg, false, err
}

func (r *remote) openFirewall(logf Logf) {
	if out, err := r.exec("command -v ufw >/dev/null && ufw status", nil, true); err == nil && strings.Contains(out, "Status: active") {
		if _, err := r.exec("ufw allow 80/tcp && ufw allow 443/tcp && ufw allow 443/udp", nil, true); err == nil {
			logf("ufw: открыты порты 80 и 443")
		}
	}
	if _, err := r.exec("command -v firewall-cmd >/dev/null && firewall-cmd --state", nil, true); err == nil {
		if _, err := r.exec("firewall-cmd --permanent --add-service=http && firewall-cmd --permanent --add-service=https && firewall-cmd --permanent --add-port=443/udp && firewall-cmd --reload", nil, true); err == nil {
			logf("firewalld: открыты порты 80 и 443")
		}
	}
}

func (r *remote) waitActive() error {
	deadline := time.Now().Add(startWait)
	var last string
	for time.Now().Before(deadline) {
		out, _ := r.exec("systemctl is-active veil", nil, true)
		last = out
		if out == "active" {
			time.Sleep(time.Second)
			if out, _ := r.exec("systemctl is-active veil", nil, true); out == "active" {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("состояние: %s", last)
}

func goArch(unameSM string) (string, error) {
	f := strings.Fields(unameSM)
	if len(f) != 2 || f[0] != "Linux" {
		return "", fmt.Errorf("поддерживается только Linux, сервер сообщил: %q", unameSM)
	}
	switch f[1] {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	}
	return "", fmt.Errorf("архитектура %s пока не поддерживается", f[1])
}

func ServerBinary(dir, arch string) ([]byte, error) {
	name := "veil-server-linux-" + arch
	var candidates []string
	if dir != "" {
		candidates = append(candidates, dir)
	}
	if env := os.Getenv("VEIL_SERVER_DIR"); env != "" {
		candidates = append(candidates, env)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "server"))
		if runtime.GOOS == "darwin" {
			candidates = append(candidates, filepath.Join(exeDir, "..", "Resources", "server"))
		}
	}
	candidates = append(candidates, filepath.Join("dist", "server"))
	for _, d := range candidates {
		if b, err := os.ReadFile(filepath.Join(d, name)); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("не найден серверный бинарник %s (искал в %s)", name, strings.Join(candidates, ", "))
}

func uniquePeerName(s *config.Server, name string) string {
	taken := map[string]bool{}
	for _, p := range s.Peers {
		taken[p.Name] = true
	}
	base := name + "@" + hostnameOr("device")
	cand := base
	for i := 2; taken[cand]; i++ {
		cand = base + "-" + strconv.Itoa(i)
	}
	return cand
}

func hostnameOr(def string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return def
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
