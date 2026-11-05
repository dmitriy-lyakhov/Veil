package helper

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/netpin"
	"veil/internal/routing"
	"veil/internal/settings"
)

const (
	tokenHeader     = "X-Veil-Token"
	watchdogTimeout = 30 * time.Second
	errorLinger     = 15 * time.Second
	pollTimeout     = 3 * time.Second
)

type Report struct {
	client.Status
	Iface string `json:"iface,omitempty"`
	Error string `json:"error,omitempty"`
	PID   int    `json:"pid"`
	PinV4 uint32 `json:"pin_v4,omitempty"`
	PinV6 uint32 `json:"pin_v6,omitempty"`
}

type service struct {
	mu       sync.Mutex
	tunnel   *client.SystemTunnel
	err      error
	lastPoll time.Time
	stop     chan struct{}
	stopOnce sync.Once
}

func (s *service) report() Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPoll = time.Now()
	r := Report{PID: os.Getpid(), Status: client.Status{State: client.StateConnecting}}
	r.PinV4, r.PinV6 = netpin.Get()
	switch {
	case s.err != nil:
		r.State = client.StateStopped
		r.Error = s.err.Error()
	case s.tunnel != nil:
		r.Status = s.tunnel.Status()
		r.Iface = s.tunnel.Iface
	}
	return r
}

func (s *service) shutdown() { s.stopOnce.Do(func() { close(s.stop) }) }

func Serve(profilePath, controlAddr, tokenFile, routesPath, settingsPath string, log *slog.Logger) error {
	prof, err := config.Load[config.Profile](profilePath)
	if err != nil {
		return err
	}
	var opt client.Options
	if settingsPath != "" {
		if st, err := settings.Load(settingsPath); err != nil {
			log.Warn("settings ignored", "err", err)
		} else {
			opt.KillSwitch = st.KillSwitch
			opt.Interface = st.Interface
			if len(st.DNS) > 0 {
				prof.DNS = st.DNS
			}
		}
	}
	var rc *routing.Config
	if routesPath != "" {
		if rc, err = routing.Load(routesPath); err != nil {
			log.Warn("routing config ignored", "err", err)
			rc = nil
		}
	}
	tokenBytes, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(string(tokenBytes))
	if len(token) < 32 {
		return errors.New("token too short")
	}
	ln, err := net.Listen("tcp", controlAddr)
	if err != nil {
		return err
	}
	svc := &service{stop: make(chan struct{}), lastPoll: time.Now()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(svc.report())
	})
	mux.HandleFunc("POST /reselect", func(w http.ResponseWriter, r *http.Request) {
		svc.mu.Lock()
		tn := svc.tunnel
		svc.mu.Unlock()
		if tn != nil {
			tn.Reselect()
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		svc.shutdown()
		w.WriteHeader(http.StatusNoContent)
	})
	hs := &http.Server{Handler: requireToken(token, mux), ReadHeaderTimeout: 5 * time.Second}
	go hs.Serve(ln)
	defer hs.Close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		svc.shutdown()
	}()

	go func() {
		tn, err := client.StartSystem(prof, rc, opt, log)
		svc.mu.Lock()
		defer svc.mu.Unlock()
		if err != nil {
			svc.err = err
			log.Error("tunnel start failed", "err", err)
			time.AfterFunc(errorLinger, svc.shutdown)
			return
		}
		select {
		case <-svc.stop:
			tn.Close()
		default:
			svc.tunnel = tn
		}
	}()

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-svc.stop:
			svc.mu.Lock()
			tn := svc.tunnel
			svc.tunnel = nil
			svc.mu.Unlock()
			if tn != nil {
				tn.Close()
				log.Info("tunnel stopped")
			}
			return nil
		case <-tick.C:
			svc.mu.Lock()
			idle := time.Since(svc.lastPoll)
			svc.mu.Unlock()
			if idle > watchdogTimeout {
				log.Warn("controller went away, stopping tunnel")
				svc.shutdown()
			}
		}
	}
}

func requireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type Handle struct {
	addr  string
	token string
	hc    *http.Client
	Log   string
}

func Launch(profilePath string) (*Handle, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return nil, err
	}
	addr, err := freeLocalAddr()
	if err != nil {
		return nil, err
	}
	runDir, err := runtimeDir()
	if err != nil {
		return nil, err
	}
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tok)
	tokenFile := filepath.Join(runDir, "control.token")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		return nil, err
	}
	logFile := filepath.Join(runDir, "tunnel.log")
	args := []string{exe, "tun", "--profile", profilePath, "--control", addr, "--token-file", tokenFile}
	if rp, err := routing.Path(); err == nil {
		args = append(args, "--routes", rp)
	}
	if sp, err := settings.Path(); err == nil {
		args = append(args, "--settings", sp)
	}
	if err := elevate(args, logFile); err != nil {
		return nil, err
	}
	return &Handle{addr: addr, token: token, hc: &http.Client{Timeout: pollTimeout}, Log: logFile}, nil
}

func (h *Handle) Status() (*Report, error) {
	req, _ := http.NewRequest(http.MethodGet, "http://"+h.addr+"/status", nil)
	req.Header.Set(tokenHeader, h.token)
	resp, err := h.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("helper: HTTP %d", resp.StatusCode)
	}
	var r Report
	return &r, json.NewDecoder(resp.Body).Decode(&r)
}

func (h *Handle) Reselect() error {
	req, _ := http.NewRequest(http.MethodPost, "http://"+h.addr+"/reselect", nil)
	req.Header.Set(tokenHeader, h.token)
	resp, err := h.hc.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (h *Handle) Stop() error {
	req, _ := http.NewRequest(http.MethodPost, "http://"+h.addr+"/stop", nil)
	req.Header.Set(tokenHeader, h.token)
	resp, err := h.hc.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (h *Handle) WaitReady(ctx context.Context) error {
	for {
		if _, err := h.Status(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("помощник туннеля не запустился (отменён запрос прав администратора?)")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func freeLocalAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	return ln.Addr().String(), nil
}

func runtimeDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Veil", "run")
	return dir, os.MkdirAll(dir, 0o700)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// RunDir — каталог служебных файлов (журналы, токен).
func RunDir() (string, error) { return runtimeDir() }

// Unblock запускает с правами администратора снятие остатков блокировки KillSwitch.
func Unblock() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	dir, err := runtimeDir()
	if err != nil {
		return err
	}
	return elevate([]string{exe, "unblock"}, filepath.Join(dir, "unblock.log"))
}
