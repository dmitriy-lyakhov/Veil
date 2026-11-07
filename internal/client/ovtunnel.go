package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"veil/internal/client/netsys"
	"veil/internal/config"
	"veil/internal/routing"
	"veil/internal/vpnconf/ovpnconf"
)

// OpenVPN в приложение не встроен (лицензия GPL и собственный драйвер): используется установленный
// openvpn. Он сам создаёт адаптер, ставит адрес, маршруты (в том числе маршрут до сервера в обход
// туннеля при redirect-gateway) и DNS; Veil запускает его, читает состояние через управляющий
// интерфейс (management) на 127.0.0.1 и останавливает командой signal SIGTERM.

const (
	OpenVPNDownloadURL = "https://openvpn.net/community-downloads/"
	ovConnectTimeout   = 45 * time.Second
	ovStopTimeout      = 8 * time.Second
)

// ErrOpenVPNMissing — на компьютере нет установленного OpenVPN.
var ErrOpenVPNMissing = errors.New("OpenVPN не установлен на этом компьютере. Скачайте и установите OpenVPN (версии 2.5 и новее): " + OpenVPNDownloadURL + " — затем подключитесь снова")

func ovCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		var out []string
		for _, env := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"} {
			if d := os.Getenv(env); d != "" {
				out = append(out, filepath.Join(d, "OpenVPN", "bin", "openvpn.exe"))
			}
		}
		return append(out, `C:\Program Files\OpenVPN\bin\openvpn.exe`, `C:\Program Files (x86)\OpenVPN\bin\openvpn.exe`)
	case "darwin":
		return []string{"/opt/homebrew/sbin/openvpn", "/usr/local/sbin/openvpn", "/opt/homebrew/bin/openvpn", "/usr/local/bin/openvpn"}
	default:
		return []string{"/usr/sbin/openvpn", "/usr/local/sbin/openvpn", "/usr/bin/openvpn"}
	}
}

func findOpenVPN(cands []string, exists func(string) bool, lookPath func(string) (string, error)) (string, error) {
	for _, c := range cands {
		if exists(c) {
			return c, nil
		}
	}
	if p, err := lookPath("openvpn"); err == nil && p != "" {
		return p, nil
	}
	return "", ErrOpenVPNMissing
}

// OpenVPNPath возвращает путь к установленному openvpn или ErrOpenVPNMissing.
func OpenVPNPath() (string, error) {
	return findOpenVPN(ovCandidates(), func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && !st.IsDir()
	}, exec.LookPath)
}

// ---------- разбор вывода управляющего интерфейса ----------

type ovEvent struct {
	kind   string // "state", "bytecount", "fatal", "other"
	state  string // CONNECTING, WAIT, AUTH, GET_CONFIG, ASSIGN_IP, ADD_ROUTES, CONNECTED, RECONNECTING, EXITING, TCP_CONNECT, RESOLVE
	detail string
	local  string
	remote string
	rx, tx uint64
}

func parseMgmtLine(line string) ovEvent {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, ">STATE:"):
		f := strings.Split(strings.TrimPrefix(line, ">STATE:"), ",")
		ev := ovEvent{kind: "state"}
		if len(f) > 1 {
			ev.state = f[1]
		}
		if len(f) > 2 {
			ev.detail = f[2]
		}
		if len(f) > 3 {
			ev.local = f[3]
		}
		if len(f) > 4 {
			ev.remote = f[4]
		}
		return ev
	case strings.HasPrefix(line, ">BYTECOUNT:"):
		f := strings.Split(strings.TrimPrefix(line, ">BYTECOUNT:"), ",")
		if len(f) == 2 {
			rx, e1 := strconv.ParseUint(strings.TrimSpace(f[0]), 10, 64)
			tx, e2 := strconv.ParseUint(strings.TrimSpace(f[1]), 10, 64)
			if e1 == nil && e2 == nil {
				return ovEvent{kind: "bytecount", rx: rx, tx: tx}
			}
		}
	case strings.HasPrefix(line, ">FATAL:"):
		return ovEvent{kind: "fatal", detail: strings.TrimSpace(strings.TrimPrefix(line, ">FATAL:"))}
	}
	return ovEvent{kind: "other"}
}

// ovStateToClient переводит состояние OpenVPN в состояние приложения.
func ovStateToClient(s string) State {
	switch s {
	case "CONNECTED":
		return StateConnected
	case "RECONNECTING":
		return StateReconnecting
	case "EXITING":
		return StateStopped
	default:
		return StateConnecting
	}
}

// ovFailure — понятное объяснение по последним строкам журнала openvpn.
func ovFailure(logTail string) string {
	l := strings.ToLower(logTail)
	switch {
	case strings.Contains(l, "tls key negotiation failed") || strings.Contains(l, "tls handshake failed"):
		return "сервер не ответил на рукопожатие OpenVPN: проверьте адрес, порт, UDP/TCP и что порт открыт на сервере"
	case strings.Contains(l, "certificate verify failed") || strings.Contains(l, "cert_verify"):
		return "сертификат не прошёл проверку: возможно, сертификат клиента отозван или просрочен"
	case strings.Contains(l, "auth_failed"):
		return "сервер отклонил авторизацию"
	case strings.Contains(l, "options error"):
		return "ошибка в файле настроек OpenVPN"
	case strings.Contains(l, "wintun") || strings.Contains(l, "no tap-windows") || strings.Contains(l, "tap adapter") || strings.Contains(l, "there are no tap"):
		return "в системе нет драйвера адаптера OpenVPN: переустановите OpenVPN с драйвером"
	case strings.Contains(l, "access is denied") || strings.Contains(l, "permission denied"):
		return "не хватает прав: OpenVPN нужно запускать от администратора"
	}
	return ""
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ovArgs — аргументы запуска openvpn.
func ovArgs(cfgPath, logPath string, port int, pwFile string) []string {
	return []string{
		"--config", cfgPath,
		"--management", "127.0.0.1", strconv.Itoa(port), pwFile,
		"--log", logPath,
		"--auth-retry", "nointeract",
	}
}

// ovConfigText — текст конфигурации для запуска: DNS из настроек приложения заменяют полученные от сервера.
func ovConfigText(c *ovpnconf.Config, dns []string) string {
	if len(dns) == 0 {
		return c.Text
	}
	return "pull-filter ignore \"dhcp-option DNS\"\n" + c.WithDNS(dns)
}

// ---------- сам движок ----------

type ovTunnel struct {
	cmd     *exec.Cmd
	log     *slog.Logger
	dir     string
	logPath string
	remote  string
	family  string
	proto   string
	started time.Time

	mu     sync.Mutex
	conn   net.Conn
	state  string
	local  string
	since  time.Time
	rx, tx uint64
	fatal  string

	done    chan struct{}
	exitErr error
	closed  sync.Once
}

func randToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func startOpenVPN(prof *config.Profile, dns []string, log *slog.Logger) (*ovTunnel, error) {
	if prof.OVPN == nil {
		return nil, errors.New("в профиле нет настроек OpenVPN")
	}
	exe, err := OpenVPNPath()
	if err != nil {
		return nil, err
	}
	host, port, proto, err := prof.OVPN.Remote()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "veil-ovpn-")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*ovTunnel, error) { os.RemoveAll(dir); return nil, err }
	cfgPath, logPath, pwPath := filepath.Join(dir, "client.ovpn"), filepath.Join(dir, "openvpn.log"), filepath.Join(dir, "mgmt.pw")
	pw := randToken()
	if err := os.WriteFile(cfgPath, []byte(ovConfigText(prof.OVPN, dns)), 0o600); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(pwPath, []byte(pw+"\n"), 0o600); err != nil {
		return fail(err)
	}
	mport, err := freePort()
	if err != nil {
		return fail(err)
	}
	cmd := exec.Command(exe, ovArgs(cfgPath, logPath, mport, pwPath)...)
	cmd.Dir = filepath.Dir(exe)
	netsys.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return fail(fmt.Errorf("не удалось запустить OpenVPN: %w", err))
	}
	family := "IPv4"
	if a := net.ParseIP(host); a != nil && a.To4() == nil {
		family = "IPv6"
	}
	t := &ovTunnel{cmd: cmd, log: log, dir: dir, logPath: logPath, remote: net.JoinHostPort(host, strconv.Itoa(port)),
		family: family, proto: strings.ToUpper(proto), started: time.Now(), state: "CONNECTING", done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		t.mu.Lock()
		t.exitErr = err
		t.state = "EXITING"
		t.mu.Unlock()
		close(t.done)
	}()
	go t.manage(mport, pw)
	return t, nil
}

// readPrompt читает ответ до заданного окончания (приглашение пароля приходит без перевода строки).
func readPrompt(r *bufio.Reader, suffix string) error {
	var buf []byte
	for len(buf) < 4096 {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		buf = append(buf, b)
		if strings.HasSuffix(string(buf), suffix) {
			return nil
		}
	}
	return errors.New("неожиданный ответ управляющего интерфейса")
}

func (t *ovTunnel) manage(port int, pw string) {
	var conn net.Conn
	var err error
	// openvpn открывает порт управления не сразу
	for i := 0; i < 100; i++ {
		select {
		case <-t.done:
			return
		default:
		}
		if conn, err = net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if conn == nil {
		t.log.Warn("openvpn management unavailable", "err", err)
		return
	}
	t.mu.Lock()
	t.conn = conn
	t.mu.Unlock()
	defer conn.Close()
	r := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := readPrompt(r, "ENTER PASSWORD:"); err != nil {
		t.log.Warn("openvpn management auth", "err", err)
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	fmt.Fprintf(conn, "%s\nstate on\nstate\nbytecount 2\n", pw)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		ev := parseMgmtLine(sc.Text())
		t.mu.Lock()
		switch ev.kind {
		case "state":
			if ev.state != t.state {
				t.since = time.Now()
			}
			t.state, t.local = ev.state, ev.local
		case "bytecount":
			t.rx, t.tx = ev.rx, ev.tx
		case "fatal":
			t.fatal = ev.detail
		}
		t.mu.Unlock()
	}
}

func (t *ovTunnel) logTail() string {
	raw, err := os.ReadFile(t.logPath)
	if err != nil {
		return ""
	}
	return lastLines(string(raw), 8)
}

func (t *ovTunnel) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	st := Status{State: ovStateToClient(t.state), Family: t.family, Transport: "OpenVPN " + t.proto,
		Remote: t.remote, Since: t.started, RxBytes: t.rx, TxBytes: t.tx}
	if st.State == StateConnected {
		st.Handshake = t.since
	}
	select {
	case <-t.done:
		st.State = StateStopped
		if t.fatal != "" {
			st.LastError = t.fatal
		}
	default:
	}
	return st
}

func (t *ovTunnel) WaitHandshake(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, ovConnectTimeout)
	defer cancel()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for {
		t.mu.Lock()
		state := t.state
		t.mu.Unlock()
		if state == "CONNECTED" {
			return nil
		}
		select {
		case <-t.done:
			tail := t.logTail()
			msg := "OpenVPN завершился при подключении"
			if why := ovFailure(tail); why != "" {
				msg += ": " + why
			}
			if tail != "" {
				msg += "\n" + tail
			}
			return errors.New(msg)
		case <-ctx.Done():
			msg := "сервер не ответил за отведённое время"
			if why := ovFailure(t.logTail()); why != "" {
				msg = why
			}
			return errors.New(msg + " (OpenVPN, " + t.remote + ")")
		case <-tick.C:
		}
	}
}

// Close просит openvpn завершиться штатно (он сам убирает маршруты и DNS), при неудаче завершает процесс.
func (t *ovTunnel) Close() {
	t.closed.Do(func() {
		t.mu.Lock()
		conn := t.conn
		t.mu.Unlock()
		if conn != nil {
			_, _ = fmt.Fprint(conn, "signal SIGTERM\n")
		}
		select {
		case <-t.done:
		case <-time.After(ovStopTimeout):
			t.log.Warn("openvpn did not stop, killing")
			_ = t.cmd.Process.Kill()
			select {
			case <-t.done:
			case <-time.After(3 * time.Second):
			}
		}
		_ = os.RemoveAll(t.dir)
	})
}

func (t *ovTunnel) Reselect() {}

// startOpenVPNSystem — подключение по OpenVPN вместо создания TUN самим приложением.
func startOpenVPNSystem(prof *config.Profile, rc *routing.Config, opt Options, log *slog.Logger) (*SystemTunnel, error) {
	if opt.KillSwitch {
		log.Warn("kill switch не применяется к OpenVPN: блокировка при обрыве не включена")
	}
	if rc != nil && rc.Enabled && !rc.Empty() {
		log.Warn("списки маршрутизации не применяются к OpenVPN: весь трафик идёт по настройкам сервера")
	}
	var dns []string
	for _, d := range prof.DNS {
		if net.ParseIP(d) != nil {
			dns = append(dns, d)
		}
	}
	tn, err := startOpenVPN(prof, dns, log)
	if err != nil {
		return nil, err
	}
	if err := tn.WaitHandshake(context.Background()); err != nil {
		tn.Close()
		return nil, err
	}
	st := &SystemTunnel{Engine: tn, Iface: "OpenVPN", log: log, u: &netsys.Undo{}, stop: make(chan struct{}),
		appliedVia: map[netip.Prefix]bool{}, appliedDirect: map[netip.Prefix]bool{}}
	log.Info("openvpn up", "status", tn.Status())
	return st, nil
}
