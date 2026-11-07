package client

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"veil/internal/client/netsys"
	"veil/internal/config"
	"veil/internal/routing"
	"veil/internal/vpnconf/ipsecconf"
)

// IPsec/IKEv2 в Windows — встроенный системный VPN (без wintun): подключение создаётся через
// PowerShell (Add-VpnConnection), соединяется rasdial, состояние опрашивается Get-VpnConnection.
// Все команды выполняет помощник туннеля с правами администратора.

// IPsecConnName — имя VPN-подключения в Windows.
const IPsecConnName = "Veil IKEv2"

const (
	ipsecConnectTimeout = 60 * time.Second
	ipsecPollEvery      = 3 * time.Second
	errCANotTrusted     = "CA_NOT_TRUSTED"
)

func psq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// pemBody — тело сертификата PEM без заголовков (base64).
func pemBody(pem string) string {
	var b strings.Builder
	for _, ln := range strings.Split(strings.ReplaceAll(pem, "\r", ""), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "-----") {
			continue
		}
		b.WriteString(ln)
	}
	return b.String()
}

// ipsecSetupScript — PowerShell: проверка/установка сертификата центра, (пере)создание подключения.
// caFile и p12File — пути уже записанных файлов (p12File пуст при входе по паролю).
func ipsecSetupScript(c *ipsecconf.Config, caFile, p12File string, split bool) string {
	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	w(`$ErrorActionPreference = 'Stop'`)
	w(`$n = %s`, psq(IPsecConnName))
	w(`$ca = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2 -ArgumentList (,[Convert]::FromBase64String(%s))`, psq(pemBody(c.CAPEM)))
	w(`if (-not (Get-ChildItem Cert:\LocalMachine\Root | Where-Object { $_.Thumbprint -eq $ca.Thumbprint })) {`)
	if c.TrustCA {
		w(`  Import-Certificate -FilePath %s -CertStoreLocation Cert:\LocalMachine\Root | Out-Null`, psq(caFile))
	} else {
		w(`  throw %s`, psq(errCANotTrusted))
	}
	w(`}`)
	if c.Auth == ipsecconf.AuthCert {
		w(`Import-PfxCertificate -FilePath %s -CertStoreLocation Cert:\LocalMachine\My -Password (ConvertTo-SecureString %s -AsPlainText -Force) | Out-Null`, psq(p12File), psq(c.P12Pass))
	}
	w(`try { Remove-VpnConnection -Name $n -AllUserConnection -Force } catch {}`)
	method := "MSChapv2"
	if c.Auth == ipsecconf.AuthCert {
		method = "MachineCertificate"
	}
	split_ := "$false"
	if split {
		split_ = "$true"
	}
	w(`Add-VpnConnection -Name $n -ServerAddress %s -TunnelType Ikev2 -AuthenticationMethod %s -EncryptionLevel Required -SplitTunneling:%s -RememberCredential:$false -AllUserConnection -Force`, psq(c.Server), method, split_)
	w(`Set-VpnConnectionIPsecConfiguration -ConnectionName $n -AuthenticationTransformConstants SHA256128 -CipherTransformConstants AES256 -EncryptionMethod AES256 -IntegrityCheckMethod SHA256 -DHGroup Group14 -PfsGroup None -AllUserConnection -Force`)
	return b.String()
}

const ipsecStatusScript = `$c = Get-VpnConnection -Name ` + "'" + IPsecConnName + "'" + ` -AllUserConnection -ErrorAction SilentlyContinue
if (-not $c) { 'STATE=Missing'; exit }
"STATE=$($c.ConnectionStatus)"
$a = Get-NetAdapterStatistics -Name ` + "'" + IPsecConnName + "'" + ` -ErrorAction SilentlyContinue
if ($a) { "RX=$($a.ReceivedBytes)"; "TX=$($a.SentBytes)" }`

const ipsecRemoveScript = `try { Remove-VpnConnection -Name ` + "'" + IPsecConnName + "'" + ` -AllUserConnection -Force } catch {}`

// ipsecStatus — разобранный вывод ipsecStatusScript.
type ipsecStatus struct {
	State  string // Connected, Connecting, Disconnected, Disconnecting, Missing
	Rx, Tx uint64
}

func parseIPsecStatus(out string) ipsecStatus {
	var s ipsecStatus
	for _, ln := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "STATE":
			s.State = v
		case "RX":
			s.Rx, _ = strconv.ParseUint(v, 10, 64)
		case "TX":
			s.Tx, _ = strconv.ParseUint(v, 10, 64)
		}
	}
	return s
}

// psEncoded — команда для powershell -EncodedCommand (UTF-16LE в base64), без проблем с кавычками и кодировкой.
func psEncoded(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 0, len(u)*2)
	for _, r := range u {
		b = append(b, byte(r), byte(r>>8))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// runQuiet запускает команду; в тексте ошибки нет аргументов (в них могут быть пароли).
func runQuiet(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	netsys.HideWindow(cmd)
	raw, err := cmd.CombinedOutput()
	out := strings.TrimSpace(netsys.DecodeOutput(raw))
	if err != nil {
		return out, fmt.Errorf("%s: %v: %s", args[0], err, out)
	}
	return out, nil
}

func powershell(script string) (string, error) {
	return runQuiet("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", psEncoded(script))
}

// rasdialArgs — аргументы rasdial для подключения (для входа по сертификату логин не нужен).
func rasdialArgs(c *ipsecconf.Config) []string {
	if c.Auth == ipsecconf.AuthCert {
		return []string{"rasdial", IPsecConnName}
	}
	return []string{"rasdial", IPsecConnName, c.Username, c.Password}
}

var (
	rasExitRe = regexp.MustCompile(`exit status (\d+)`) // rasdial возвращает номер ошибки RAS кодом выхода
	rasCodeRe = regexp.MustCompile(`(?i)(?:код ошибки|error|ошибка)\D{0,3}(\d{3,5})`)
)

// rasdialError переводит ответ rasdial (в нём есть «exit status N») в понятное сообщение.
func rasdialError(out string) error {
	code := ""
	if m := rasExitRe.FindStringSubmatch(out); m != nil && len(m[1]) >= 3 {
		code = m[1]
	} else if m := rasCodeRe.FindStringSubmatch(out); m != nil {
		code = m[1]
	}
	var msg string
	switch code {
	case "691":
		msg = "неверные логин или пароль (ошибка 691)"
	case "809":
		msg = "сервер не отвечает: проверьте адрес и что открыты порты UDP 500 и 4500 (ошибка 809)"
	case "13801", "13806", "13807", "13813":
		msg = "сертификат сервера не принят: центр сертификации не доверенный или адрес сервера не совпадает с сертификатом (ошибка " + code + ")"
	case "13868", "13869", "13870":
		msg = "для входа по сертификату не найден подходящий сертификат клиента (ошибка " + code + ")"
	case "":
		msg = strings.TrimSpace(out)
	default:
		msg = "ошибка " + code
	}
	return fmt.Errorf("подключение IPsec/IKEv2 не удалось: %s", msg)
}

type ipsecEngine struct {
	cfg *ipsecconf.Config
	log *slog.Logger

	mu       sync.Mutex
	st       ipsecStatus
	started  time.Time
	seenUp   bool
	lastErr  string
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
}

func (e *ipsecEngine) poll() {
	defer close(e.done)
	t := time.NewTicker(ipsecPollEvery)
	defer t.Stop()
	for {
		out, err := powershell(ipsecStatusScript)
		e.mu.Lock()
		if err != nil {
			e.lastErr = shortLine(err.Error())
		} else {
			e.st = parseIPsecStatus(out)
			if e.st.State == "Connected" {
				e.seenUp = true
				e.lastErr = ""
			}
		}
		e.mu.Unlock()
		select {
		case <-e.stop:
			return
		case <-t.C:
		}
	}
}

func shortLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	return s
}

func (e *ipsecEngine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := Status{State: StateConnecting, Family: "IPv4", Transport: "IPsec/IKEv2", Remote: e.cfg.Server,
		Since: e.started, RxBytes: e.st.Rx, TxBytes: e.st.Tx, LastError: e.lastErr}
	switch e.st.State {
	case "Connected":
		st.State = StateConnected
		st.Handshake = time.Now()
	case "Disconnected", "Missing":
		if e.seenUp {
			st.State = StateStopped
			if st.LastError == "" {
				st.LastError = "соединение IPsec разорвано"
			}
		}
	}
	if net.ParseIP(e.cfg.Server) != nil && strings.Contains(e.cfg.Server, ":") {
		st.Family = "IPv6"
	}
	return st
}

func (e *ipsecEngine) WaitHandshake(ctx context.Context) error {
	for {
		e.mu.Lock()
		up := e.st.State == "Connected"
		e.mu.Unlock()
		if up {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("подключение IPsec не установилось")
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (e *ipsecEngine) Close() {
	e.stopOnce.Do(func() {
		close(e.stop)
		<-e.done
		if out, err := runQuiet("rasdial", IPsecConnName, "/disconnect"); err != nil {
			e.log.Warn("rasdial disconnect", "err", err, "out", out)
		}
		if _, err := powershell(ipsecRemoveScript); err != nil {
			e.log.Warn("remove vpn connection", "err", err)
		}
	})
}

func (e *ipsecEngine) Reselect() {}

// writeTemp пишет секретный файл с случайным именем и возвращает путь.
func writeTemp(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// startIPsec поднимает системное подключение IKEv2 и применяет списки «через VPN/мимо VPN».
func startIPsec(prof *config.Profile, rc *routing.Config, opt Options, log *slog.Logger) (*SystemTunnel, error) {
	c := prof.IPsec
	if c == nil {
		return nil, errors.New("в профиле нет настроек IPsec")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		return nil, errors.New("подключение IPsec/IKEv2 из приложения поддерживается только в Windows; на этом устройстве настройте встроенный IKEv2 по инструкции из профиля")
	}
	if err := netsys.Unblock(); err != nil {
		log.Warn("unblock", "err", err)
	}
	if opt.KillSwitch {
		log.Warn("kill switch не поддерживается для IPsec/IKEv2, защита отключена")
	}
	nc := netsys.NetConfig{Iface: IPsecConnName}
	for _, d := range append(append([]string(nil), c.DNS...), prof.DNS...) {
		if a, err := netip.ParseAddr(d); err == nil {
			nc.DNS = append(nc.DNS, a)
		}
	}
	if rc != nil && rc.Enabled && !rc.Empty() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		plan, err := routing.Build(ctx, rc, nil)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("списки адресов: %w", err)
		}
		for _, w := range plan.Warnings {
			log.Warn("routing list", "warning", w)
		}
		log.Info("routing plan", "summary", plan.Summary())
		nc.Plan = plan
	}
	netsys.PrepareRoutes(nc) // физический шлюз определяется до подъёма VPN

	caFile, err := writeTemp("veil-ca-*.crt", []byte(c.CAPEM))
	if err != nil {
		return nil, err
	}
	defer os.Remove(caFile)
	p12File := ""
	if c.Auth == ipsecconf.AuthCert {
		raw, err := base64.StdEncoding.DecodeString(c.P12)
		if err != nil {
			return nil, fmt.Errorf("сертификат клиента: %w", err)
		}
		if p12File, err = writeTemp("veil-client-*.p12", raw); err != nil {
			return nil, err
		}
		defer os.Remove(p12File)
	}
	if out, err := powershell(ipsecSetupScript(c, filepath.ToSlash(caFile), filepath.ToSlash(p12File), !netsys.FullTunnel(nc))); err != nil {
		if strings.Contains(out, errCANotTrusted) || strings.Contains(err.Error(), errCANotTrusted) {
			return nil, errors.New("сертификат центра сертификации Veil не добавлен в доверенные: разрешите это на странице профиля IPsec и подключитесь снова")
		}
		return nil, fmt.Errorf("настройка подключения Windows: %w", err)
	}

	eng := &ipsecEngine{cfg: c, log: log, started: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	cleanup := func() { _, _ = runQuiet("rasdial", IPsecConnName, "/disconnect"); _, _ = powershell(ipsecRemoveScript) }
	type res struct {
		out string
		err error
	}
	ch := make(chan res, 1)
	go func() {
		out, err := runQuiet(rasdialArgs(c)...)
		ch <- res{out, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			cleanup()
			return nil, rasdialError(r.err.Error())
		}
	case <-time.After(ipsecConnectTimeout):
		cleanup()
		return nil, errors.New("подключение IPsec/IKEv2: сервер не ответил за отведённое время")
	}
	go eng.poll()
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	if err := eng.WaitHandshake(ctx); err != nil {
		eng.Close()
		return nil, err
	}
	// адаптер подключения появляется не сразу
	for i := 0; i < 25; i++ {
		if _, err := net.InterfaceByName(IPsecConnName); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	u := &netsys.Undo{}
	if err := netsys.ApplyPlan(nc, u, log); err != nil {
		u.Run()
		eng.Close()
		return nil, fmt.Errorf("настройка маршрутов: %w", err)
	}
	st := &SystemTunnel{Engine: eng, Iface: IPsecConnName, nc: nc, log: log, u: u, rcfg: rc, stop: make(chan struct{}),
		appliedVia: map[netip.Prefix]bool{}, appliedDirect: map[netip.Prefix]bool{}}
	if nc.Plan != nil {
		for _, p := range nc.Plan.Via {
			st.appliedVia[p] = true
		}
		for _, p := range nc.Plan.Direct {
			st.appliedDirect[p] = true
		}
		if len(nc.Plan.ViaDomains)+len(nc.Plan.DirDomains) > 0 {
			go st.refreshLoop()
		}
	}
	log.Info("ipsec up", "status", eng.Status())
	return st, nil
}
