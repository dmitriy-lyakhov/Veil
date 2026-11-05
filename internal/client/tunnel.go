package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/config"
	"veil/internal/streambind"
	"veil/internal/wgkey"
)

const (
	keepaliveSeconds = 25
	rotationMin      = 5 * time.Minute
	rotationJitter   = 10 * time.Minute
	rotationRetry    = 30 * time.Second
	rotationGrace    = 5 * time.Second
	backoffMin       = time.Second
	backoffMax       = 30 * time.Second
)

type State string

const (
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateReconnecting State = "reconnecting"
	StateStopped      State = "stopped"
)

type Status struct {
	State     State     `json:"state"`
	Family    string    `json:"family,omitempty"`
	Transport string    `json:"transport,omitempty"`
	Entry     string    `json:"entry,omitempty"`
	EntryRTT  float64   `json:"entry_rtt_ms,omitempty"`
	Exit      string    `json:"exit,omitempty"`
	Remote    string    `json:"remote,omitempty"`
	Since     time.Time `json:"since,omitempty"`
	Handshake time.Time `json:"handshake,omitempty"`
	RxBytes   uint64    `json:"rx_bytes"`
	TxBytes   uint64    `json:"tx_bytes"`
	LastError string    `json:"last_error,omitempty"`
	// LatencyMS — задержка до входа по открытому соединению (HTTP/2 PING); 0 — не измерена.
	LatencyMS float64 `json:"latency_ms,omitempty"`
	// Skipped — входы каскада, которые не удалось использовать перед этим подключением.
	Skipped []string `json:"skipped_entries,omitempty"`
}

type Tunnel struct {
	dialer     *Dialer
	bind       *streambind.Bind
	dev        *device.Device
	log        *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	upOnce     sync.Once
	reselect   chan struct{}
	attachedAt time.Time // когда подключён текущий транспорт (под mu)
	peer       string
	rx, tx     atomic.Uint64

	mu     sync.Mutex
	status Status
}

type countingSink struct {
	t *Tunnel
	c Transport
}

func (s countingSink) WritePacket(p []byte) error {
	if err := s.c.WritePacket(p); err != nil {
		return err
	}
	s.t.tx.Add(uint64(len(p)))
	return nil
}

func Start(prof *config.Profile, tunDev tun.Device, dial DialFunc, log *slog.Logger) (*Tunnel, error) {
	uapi, err := deviceConfig(prof)
	if err != nil {
		return nil, err
	}
	bind := streambind.New()
	dev := device.NewDevice(holdUp(tunDev), bind, quietLogger())
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("wireguard config: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tunnel{
		dialer: &Dialer{Profile: prof, Dial: dial},
		bind:   bind,
		dev:    dev,
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),

		reselect: make(chan struct{}, 1),
		status:   Status{State: StateConnecting},
	}
	t.peer, _ = wgkey.Hex(prof.ServerPublicKey)
	go t.run()
	return t, nil
}

func quietLogger() *device.Logger {
	l := device.NewLogger(device.LogLevelError, "wireguard: ")
	errorf := l.Errorf
	l.Errorf = func(format string, args ...any) {
		for _, a := range args {
			if err, ok := a.(error); ok && (errors.Is(err, streambind.ErrNoTransport) || errors.Is(err, net.ErrClosed)) {
				return
			}
		}
		errorf(format, args...)
	}
	return l
}

func deviceConfig(p *config.Profile) (string, error) {
	priv, err := wgkey.Hex(p.PrivateKey)
	if err != nil {
		return "", err
	}
	pub, err := wgkey.Hex(p.ServerPublicKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("private_key=%s\npublic_key=%s\nendpoint=veil\npersistent_keepalive_interval=%d\nallowed_ip=0.0.0.0/0\nallowed_ip=::/0\n",
		priv, pub, keepaliveSeconds), nil
}

func (t *Tunnel) run() {
	defer close(t.done)
	backoff := backoffMin
	for t.ctx.Err() == nil {
		c, err := t.dialer.Connect(t.ctx)
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			t.log.Warn("connect failed", "err", err)
			t.setError(err)
			select {
			case <-t.ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, backoffMax)
			continue
		}
		backoff = backoffMin
		t.attach(c)
		t.serve(c)
	}
}

func (t *Tunnel) serve(c Transport) {
	errc := t.startReader(c)
	rotate := time.NewTimer(rotationInterval())
	defer rotate.Stop()
	watch := time.NewTicker(watchInterval)
	defer watch.Stop()
	ticks := 0
	for {
		select {
		case <-t.ctx.Done():
			c.Close()
			return
		case err := <-errc:
			t.log.Info("transport closed", "err", err)
			t.bind.SetDefault(nil)
			t.setError(err)
			c.Close()
			return
		case <-t.reselect:
			nc, err := t.dialer.Connect(t.ctx)
			if err != nil {
				t.log.Warn("reselect failed", "err", err)
				continue
			}
			t.attach(nc)
			time.AfterFunc(rotationGrace, c.Close)
			c, errc = nc, t.startReader(nc)
			rotate.Reset(rotationInterval())
		case <-watch.C:
			if ticks++; ticks%2 == 0 {
				t.sampleRTT(c)
			}
			if !t.silent() {
				continue
			}
			name := t.dialer.LastEntry
			t.log.Warn("entry accepts connections but passes no traffic, switching", "entry", name)
			t.dialer.MarkFailed(name)
			nc, err := t.dialer.Connect(t.ctx)
			if err != nil {
				t.log.Warn("failover failed", "err", err)
				t.touchAttached()
				continue
			}
			t.attach(nc)
			time.AfterFunc(rotationGrace, c.Close)
			c, errc = nc, t.startReader(nc)
			rotate.Reset(rotationInterval())
		case <-rotate.C:
			nc, err := t.dialer.Connect(t.ctx)
			if err != nil {
				rotate.Reset(rotationRetry)
				continue
			}
			t.attach(nc)
			time.AfterFunc(rotationGrace, c.Close)
			c, errc = nc, t.startReader(nc)
			rotate.Reset(rotationInterval())
		}
	}
}

func (t *Tunnel) startReader(c Transport) chan error {
	errc := make(chan error, 1)
	ep := t.bind.Virtual()
	go func() {
		errc <- c.ReadLoop(func(p []byte) {
			t.rx.Add(uint64(len(p)))
			t.bind.Deliver(ep, p)
		})
	}()
	return errc
}

func (t *Tunnel) attach(c Transport) {
	t.bind.SetDefault(countingSink{t, c})
	t.mu.Lock()
	t.status.State = StateConnected
	t.status.Family = c.Target().Family
	t.status.Transport = string(c.Kind())
	t.status.Remote = c.Target().Addr.String()
	t.status.Entry = t.dialer.LastEntry
	t.status.EntryRTT = float64(t.dialer.LastEntryRTT.Microseconds()) / 1000
	t.status.Exit = t.dialer.Profile.ExitName
	t.status.Skipped = slices.Clone(t.dialer.LastSkipped)
	t.status.Since = time.Now()
	t.attachedAt = time.Now()
	t.status.LastError = ""
	t.mu.Unlock()
	t.log.Info("transport connected", "transport", c.Kind(), "family", c.Target().Family, "remote", c.Target().Addr)
	first := false
	t.upOnce.Do(func() {
		first = true
		if err := t.dev.Up(); err != nil {
			t.log.Error("wireguard up", "err", err)
		}
	})
	if !first {
		t.kick()
	}
}

func (t *Tunnel) kick() {
	for _, secs := range []int{0, keepaliveSeconds} {
		if err := t.dev.IpcSet(fmt.Sprintf("public_key=%s\npersistent_keepalive_interval=%d\n", t.peer, secs)); err != nil {
			t.log.Warn("keepalive kick", "err", err)
			return
		}
	}
}

func (t *Tunnel) setError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.status.State == StateConnected {
		t.status.State = StateReconnecting
	}
	t.status.LastError = err.Error()
}

func (t *Tunnel) Status() Status {
	t.mu.Lock()
	s := t.status
	t.mu.Unlock()
	if t.ctx.Err() != nil {
		s.State = StateStopped
	}
	s.RxBytes, s.TxBytes = t.rx.Load(), t.tx.Load()
	s.Handshake = t.lastHandshake()
	return s
}

func (t *Tunnel) lastHandshake() time.Time {
	cfg, err := t.dev.IpcGet()
	if err != nil {
		return time.Time{}
	}
	var sec, nsec int64
	sc := bufio.NewScanner(strings.NewReader(cfg))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "last_handshake_time_sec":
			fmt.Sscan(v, &sec)
		case "last_handshake_time_nsec":
			fmt.Sscan(v, &nsec)
		}
	}
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, nsec)
}

func (t *Tunnel) WaitHandshake(ctx context.Context) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if !t.lastHandshake().IsZero() {
			return nil
		}
		select {
		case <-ctx.Done():
			t.mu.Lock()
			last := t.status.LastError
			t.mu.Unlock()
			if last != "" {
				return errors.New(last)
			}
			return errors.New("сервер не ответил на рукопожатие WireGuard")
		case <-t.done:
			return errors.New("туннель остановлен")
		case <-tick.C:
		}
	}
}

func (t *Tunnel) Close() {
	t.cancel()
	<-t.done
	t.bind.SetDefault(nil)
	t.dev.Close()
}

func (t *Tunnel) Reselect() {
	select {
	case t.reselect <- struct{}{}:
	default:
	}
}

func rotationInterval() time.Duration {
	return rotationMin + rand.N(rotationJitter)
}

const (
	watchInterval = 2 * time.Second
	// silentFirst — сколько ждать первого рукопожатия WireGuard на новом транспорте, прежде чем считать вход «немым».
	silentFirst = 20 * time.Second
	// silentStale — предельный возраст рукопожатия: при keepalive WireGuard обновляет его примерно раз в две минуты.
	silentStale = 210 * time.Second
)

func (t *Tunnel) touchAttached() {
	t.mu.Lock()
	t.attachedAt = time.Now()
	t.mu.Unlock()
}

// silent — вход принимает соединения, но трафик через него не идёт (его путь дальше оборван):
// рукопожатия нет слишком долго. Проверяется только при нескольких входах: переключаться некуда иначе.
func (t *Tunnel) silent() bool {
	if len(t.dialer.Profile.Entries) < 2 {
		return false
	}
	t.mu.Lock()
	since := time.Since(t.attachedAt)
	t.mu.Unlock()
	if since < silentFirst {
		return false
	}
	hs := t.lastHandshake()
	if hs.IsZero() {
		return true
	}
	return time.Since(hs) > silentStale
}

// rttProber — транспорт, умеющий измерять задержку по уже открытому соединению.
type rttProber interface {
	Ping(ctx context.Context) (time.Duration, error)
}

// sampleRTT измеряет задержку до входа в фоне и кладёт в статус. Для транспортов без PING остаётся замер входа.
func (t *Tunnel) sampleRTT(c Transport) {
	p, ok := c.(rttProber)
	if !ok {
		t.mu.Lock()
		if t.status.LatencyMS == 0 {
			t.status.LatencyMS = t.status.EntryRTT
		}
		t.mu.Unlock()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(t.ctx, 3*time.Second)
		defer cancel()
		d, err := p.Ping(ctx)
		if err != nil {
			return
		}
		t.mu.Lock()
		t.status.LatencyMS = float64(d.Microseconds()) / 1000
		t.mu.Unlock()
	}()
}
