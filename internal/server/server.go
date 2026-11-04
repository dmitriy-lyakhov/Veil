package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"golang.org/x/crypto/acme"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"veil/internal/client"
	"veil/internal/config"
	"veil/internal/proto"
	"veil/internal/site"
	"veil/internal/streambind"
	"veil/internal/wgkey"
)

const (
	Version         = "0.5.0"
	quicIdleTimeout = 30 * time.Second
	quicKeepAlive   = 10 * time.Second
	defaultSiteLang = "ru"
	relayPrefix     = "relay:"
	relayAPISuffix  = "/relay"
)

type Server struct {
	cfgPath  string
	bind     *streambind.Bind
	dev      *device.Device
	verifier *proto.Verifier
	log      *slog.Logger
	started  time.Time
	web      *webSite
	admin    *adminState
	selfCert tls.Certificate
	certs    *certManager

	mu  sync.Mutex
	cfg *config.Server

	sessMu   sync.Mutex
	sessions map[*session]struct{}
	counters map[string]*relayCounter
	hopKinds map[string]client.Kind

	tunnelNets []netip.Prefix

	lat           latencyBook
	tuneMu        sync.Mutex
	probeEvery    time.Duration
	optimizeEvery time.Duration
	autoMu        sync.Mutex
	candidates    map[string]string
	autoCost      map[string]float64
	stopLoops     context.CancelFunc
}

type session struct {
	peer      string
	remote    string
	transport string
	since     time.Time
	kill      func()
}

func New(cfg *config.Server, cfgPath string, tunDev tun.Device, log *slog.Logger) (*Server, error) {
	dirty := false
	if cfg.Site == nil || !cfg.Site.Valid() {
		gen := site.Generate(defaultSiteLang)
		cfg.Site = &gen
		dirty = true
	}
	cert, err := tls.X509KeyPair([]byte(cfg.CertPEM), []byte(cfg.KeyPEM))
	if err != nil {
		return nil, fmt.Errorf("certificate: %w", err)
	}
	uapi, err := deviceConfig(cfg)
	if err != nil {
		return nil, err
	}
	bind := streambind.New()
	dev := device.NewDevice(tunDev, bind, wgLogger(wgLogLevel()))
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("wireguard config: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, err
	}
	s := &Server{
		cfgPath:  cfgPath,
		cfg:      cfg,
		bind:     bind,
		dev:      dev,
		verifier: proto.NewVerifier(nil),
		log:      log,
		started:  time.Now(),
		web:      newWebSite(cfg.Site),
		admin:    newAdminState(),
		selfCert: cert,
		sessions: map[*session]struct{}{},
		counters: map[string]*relayCounter{},
		hopKinds: map[string]client.Kind{},

		lat:           latencyBook{links: map[string]*LinkLatency{}},
		probeEvery:    defaultProbeInterval,
		optimizeEvery: defaultOptimizeInterval,
		candidates:    map[string]string{},
		autoCost:      map[string]float64{},
	}
	for _, p := range cfg.Peers {
		if !p.Disabled {
			if err := s.authorize(p.Name, p.AuthKey); err != nil {
				dev.Close()
				return nil, err
			}
		}
	}
	for _, r := range cfg.Relays {
		if err := s.authorize(relayPrefix+r.Name, r.AuthKey); err != nil {
			dev.Close()
			return nil, err
		}
	}
	for _, a := range []string{cfg.Address4, cfg.Address6} {
		if p, err := netip.ParsePrefix(a); err == nil {
			s.tunnelNets = append(s.tunnelNets, p.Masked())
		}
	}
	cacheDir := ""
	if cfgPath != "" {
		cacheDir = filepath.Join(filepath.Dir(cfgPath), "acme")
	}
	s.certs = newCertManager(cacheDir, s.domain, log)
	if dirty {
		if err := s.saveLocked(); err != nil {
			log.Warn("save generated site", "err", err)
		}
	}
	if d := s.domain(); d != "" {
		go s.certs.warm(d)
	}
	loopCtx, stop := context.WithCancel(context.Background())
	s.stopLoops = stop
	s.backgroundLoops(loopCtx)
	return s, nil
}

func (s *Server) authorize(name, authKey string) error {
	k, err := base64.StdEncoding.DecodeString(authKey)
	if err != nil {
		return fmt.Errorf("%s: auth key: %w", name, err)
	}
	s.verifier.Set(name, k)
	return nil
}

func (s *Server) saveLocked() error {
	if s.cfgPath == "" {
		return nil
	}
	return config.Save(s.cfgPath, s.cfg)
}

func (s *Server) domain() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Domain
}

func wgLogLevel() int {
	if os.Getenv("VEIL_DEBUG") != "" {
		return device.LogLevelVerbose
	}
	return device.LogLevelError
}

func wgLogger(level int) *device.Logger {
	l := device.NewLogger(level, "wireguard: ")
	errorf := l.Errorf
	l.Errorf = func(format string, args ...any) {
		for _, a := range args {
			if err, ok := a.(error); ok && errors.Is(err, net.ErrClosed) {
				return
			}
		}
		errorf(format, args...)
	}
	return l
}

func localPeer(p config.Peer) bool { return len(p.Route) == 0 && p.PublicKey != "" }

func deviceConfig(cfg *config.Server) (string, error) {
	var b strings.Builder
	priv, err := wgkey.Hex(cfg.PrivateKey)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "private_key=%s\n", priv)
	seen := map[string]bool{}
	for _, p := range cfg.Peers {
		if !localPeer(p) || seen[p.PublicKey] {
			continue
		}
		seen[p.PublicKey] = true
		cfgText, err := peerConfig(p)
		if err != nil {
			return "", err
		}
		b.WriteString(cfgText)
	}
	return b.String(), nil
}

func peerConfig(p config.Peer) (string, error) {
	pub, err := wgkey.Hex(p.PublicKey)
	if err != nil {
		return "", fmt.Errorf("peer %s: %w", p.Name, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "public_key=%s\nreplace_allowed_ips=true\n", pub)
	for _, a := range []string{p.Address4, p.Address6} {
		if a != "" {
			fmt.Fprintf(&b, "allowed_ip=%s\n", a)
		}
	}
	return b.String(), nil
}

func (s *Server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if d := s.domain(); d != "" && strings.EqualFold(strings.TrimSuffix(hello.ServerName, "."), d) {
		if c, err := s.certs.get(hello); err == nil {
			return c, nil
		}
	}
	return &s.selfCert, nil
}

func (s *Server) TLSConfig() (*tls.Config, error) {
	return &tls.Config{
		GetCertificate: s.getCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"h2", "http/1.1", acme.ALPNProto},
	}, nil
}

func (s *Server) HTTPServer() (*http.Server, error) {
	tc, err := s.TLSConfig()
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Handler:           s,
		TLSConfig:         tc,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ErrorLog:          slog.NewLogLogger(s.log.Handler(), slog.LevelDebug),
	}, nil
}

func (s *Server) HTTP3Server() (*http3.Server, error) {
	tc, err := s.TLSConfig()
	if err != nil {
		return nil, err
	}
	tc.NextProtos = []string{http3.NextProtoH3}
	tc.MinVersion = tls.VersionTLS13
	return &http3.Server{
		Handler:         s,
		TLSConfig:       tc,
		EnableDatagrams: true,
		IdleTimeout:     2 * time.Minute,
		QUICConfig: &quic.Config{
			EnableDatagrams: true,
			MaxIdleTimeout:  quicIdleTimeout,
			KeepAlivePeriod: quicKeepAlive,
		},
	}, nil
}

func bearer(r *http.Request) string {
	tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tunnelPath := s.cfg.Path
	s.mu.Unlock()
	if r.Method == http.MethodPost && (r.ProtoMajor == 2 || r.ProtoMajor == 3) {
		switch r.URL.Path {
		case tunnelPath:
			if name, err := s.verifier.Verify(bearer(r)); err == nil && !strings.HasPrefix(name, relayPrefix) {
				if r.ProtoMajor == 3 {
					s.serveTunnelH3(w, r, name)
				} else {
					s.serveTunnel(w, r, name)
				}
				return
			}
		case tunnelPath + relayAPISuffix:
			if name, err := s.verifier.Verify(bearer(r)); err == nil && strings.HasPrefix(name, relayPrefix) {
				s.serveRelayAPI(w, r, strings.TrimPrefix(name, relayPrefix))
				return
			}
		}
	}
	if r.ProtoMajor < 3 {
		if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			if _, port, err := net.SplitHostPort(addr.String()); err == nil {
				w.Header().Set("Alt-Svc", `h3=":`+port+`"; ma=86400`)
			}
		}
	}
	w.Header().Set("Server", serverHeader)
	if (r.URL.Path == adminPrefix || strings.HasPrefix(r.URL.Path, adminPrefix+"/")) && s.fromTunnel(r) {
		s.serveAdmin(w, r)
		return
	}
	s.web.ServeHTTP(w, r)
}

func (s *Server) fromTunnel(r *http.Request) bool {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := ap.Addr().Unmap()
	for _, p := range s.tunnelNets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) track(sess *session) func() {
	s.sessMu.Lock()
	s.sessions[sess] = struct{}{}
	n := len(s.sessions)
	s.sessMu.Unlock()
	s.log.Info("session open", "peer", sess.peer, "remote", sess.remote, "transport", sess.transport, "active", n)
	return func() {
		s.sessMu.Lock()
		delete(s.sessions, sess)
		s.sessMu.Unlock()
	}
}

func (s *Server) killSessions(match func(peer string) bool) int {
	s.sessMu.Lock()
	var kills []func()
	for sess := range s.sessions {
		if match(sess.peer) {
			kills = append(kills, sess.kill)
		}
	}
	s.sessMu.Unlock()
	for _, k := range kills {
		k()
	}
	return len(kills)
}

type downstream struct {
	sink streambind.Sink
	read func(deliver func([]byte)) error
	kill func()
}

func (s *Server) serveTunnelH3(w http.ResponseWriter, r *http.Request, peer string) {
	h := w.Header()
	h.Set("Server", serverHeader)
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	streamer, ok := w.(http3.HTTPStreamer)
	if !ok {
		return
	}
	str, err := streamer.HTTPStream()
	if err != nil {
		return
	}
	defer str.Close()
	sink := &closableSink{next: proto.NewDatagramSender(str.SendDatagram)}
	defer sink.Close()
	ctx, cancel := contextWithCancel(r)
	defer cancel()
	rx := proto.NewDatagramReceiver()
	s.runSession(r, peer, "HTTP/3", downstream{sink: sink, kill: cancel, read: func(deliver func([]byte)) error {
		for {
			d, err := str.ReceiveDatagram(ctx)
			if err != nil {
				return err
			}
			if p, err := rx.Decode(d); err == nil && p != nil {
				deliver(p)
			}
		}
	}})
}

func (s *Server) serveTunnel(w http.ResponseWriter, r *http.Request, peer string) {
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Server", serverHeader)
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}
	fw := proto.NewFrameWriter(w, func() { _ = rc.Flush() })
	defer fw.Close()
	fr := proto.NewFrameReader(r.Body)
	s.runSession(r, peer, "HTTP/2", downstream{sink: fw, kill: func() { _ = r.Body.Close() }, read: func(deliver func([]byte)) error {
		for {
			p, err := fr.ReadPacket()
			if err != nil {
				return err
			}
			deliver(p)
		}
	}})
}

func (s *Server) runSession(r *http.Request, peer, transport string, ds downstream) {
	defer s.track(&session{peer: peer, remote: r.RemoteAddr, transport: transport, since: time.Now(), kill: ds.kill})()
	if hop, ok := s.nextHop(peer); ok {
		err := s.relaySession(r.Context(), peer, hop, ds)
		s.log.Info("relay session closed", "peer", peer, "next", hop.name, "reason", err)
		return
	}
	ep := streambind.NewEndpoint(peer+"@"+transport+"/"+r.RemoteAddr, ds.sink)
	err := ds.read(func(p []byte) { s.bind.Deliver(ep, p) })
	s.log.Info("session closed", "peer", peer, "remote", r.RemoteAddr, "transport", transport, "reason", err)
}

type closableSink struct {
	mu     sync.Mutex
	next   streambind.Sink
	closed bool
}

func (c *closableSink) WritePacket(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	return c.next.WritePacket(p)
}

func (c *closableSink) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

func (s *Server) Close() {
	s.stopLoops()
	s.dev.Close()
}
