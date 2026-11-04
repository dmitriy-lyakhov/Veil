package client

import (
	"veil/internal/netpin"
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"

	"veil/internal/config"
	"veil/internal/proto"
	"veil/internal/provision"
)

const (
	FallbackDelay = 250 * time.Millisecond
	dialTimeout   = 15 * time.Second
	userAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
)

type Kind string

const (
	KindH3 Kind = "HTTP/3"
	KindH2 Kind = "HTTP/2"
)

type Transport interface {
	WritePacket([]byte) error
	ReadLoop(deliver func([]byte)) error
	Close()
	Target() config.Target
	Kind() Kind
}

type candidate struct {
	target config.Target
	kind   Kind
}

func (c candidate) String() string {
	return fmt.Sprintf("%s %s %s", c.target.Family, c.kind, c.target.Addr)
}

type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

type Dialer struct {
	Profile      *config.Profile
	Dial         DialFunc
	Prefer       Kind
	LastEntry    string
	LastEntryRTT time.Duration
}

type dialResult struct {
	conn Transport
	err  error
}

func (d *Dialer) candidates() ([]candidate, error) {
	targets, err := d.Profile.Targets()
	if err != nil {
		return nil, err
	}
	kinds := []Kind{KindH3, KindH2}
	if len(d.Profile.Transports) > 0 {
		kinds = kinds[:0]
		for _, k := range d.Profile.Transports {
			switch k {
			case "h3":
				kinds = append(kinds, KindH3)
			case "h2":
				kinds = append(kinds, KindH2)
			}
		}
		if len(kinds) == 0 {
			return nil, errors.New("в профиле не указан ни один известный транспорт (h3, h2)")
		}
	}
	if d.Prefer != "" && slices.Contains(kinds, d.Prefer) {
		kinds = append([]Kind{d.Prefer}, slices.DeleteFunc(slices.Clone(kinds), func(k Kind) bool { return k == d.Prefer })...)
	}
	var out []candidate
	for _, t := range targets {
		for _, k := range kinds {
			out = append(out, candidate{t, k})
		}
	}
	return out, nil
}

func (d *Dialer) Connect(ctx context.Context) (Transport, error) {
	if len(d.Profile.Entries) > 0 {
		return d.connectEntries(ctx)
	}
	cands, err := d.candidates()
	if err != nil {
		return nil, err
	}
	authKey, err := base64.StdEncoding.DecodeString(d.Profile.AuthKey)
	if err != nil {
		return nil, fmt.Errorf("auth key: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	results := make(chan dialResult, len(cands))
	started, received := 0, 0
	launch := func() {
		c := cands[started]
		started++
		go func() {
			conn, err := d.dialOne(ctx, c, authKey)
			if err != nil {
				err = fmt.Errorf("%s: %w", c, err)
			}
			results <- dialResult{conn, err}
		}()
	}
	launch()
	timer := time.NewTimer(FallbackDelay)
	defer timer.Stop()
	var errs []error
	for {
		select {
		case <-timer.C:
			if started < len(cands) {
				launch()
				timer.Reset(FallbackDelay)
			}
		case r := <-results:
			received++
			if r.err == nil {
				go drain(results, started-received)
				return r.conn, nil
			}
			errs = append(errs, r.err)
			if started < len(cands) {
				launch()
				timer.Reset(FallbackDelay)
			} else if received == started {
				return nil, errors.Join(errs...)
			}
		}
	}
}

func drain(results <-chan dialResult, n int) {
	for range n {
		if r := <-results; r.conn != nil {
			r.conn.Close()
		}
	}
}

func (d *Dialer) dialOne(ctx context.Context, c candidate, authKey []byte) (Transport, error) {
	if c.kind == KindH3 {
		return d.dialH3(ctx, c.target, authKey)
	}
	return d.dialH2(ctx, c.target, authKey)
}

func (d *Dialer) pinned() bool { return d.Profile.CertPin != "" }

func (d *Dialer) verifyPin(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if !d.pinned() {
		return nil
	}
	if len(rawCerts) == 0 {
		return errors.New("no certificate")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return err
	}
	if provision.SPKIPin(cert) != d.Profile.CertPin {
		return errors.New("certificate pin mismatch")
	}
	return nil
}

func (d *Dialer) tunnelRequest(t config.Target, authKey []byte, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, "https://"+d.Profile.HostHeader(t)+d.Profile.Path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+proto.NewToken(authKey, time.Now()))
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/octet-stream")
	return req, nil
}

type h2Conn struct {
	target config.Target
	raw    net.Conn
	cc     *http2.ClientConn
	pw     *io.PipeWriter
	body   io.ReadCloser
	fw     *proto.FrameWriter
	once   sync.Once
}

func (c *h2Conn) Target() config.Target { return c.target }
func (c *h2Conn) Kind() Kind            { return KindH2 }

func (c *h2Conn) WritePacket(p []byte) error { return c.fw.WritePacket(p) }

func (c *h2Conn) ReadLoop(deliver func([]byte)) error {
	fr := proto.NewFrameReader(c.body)
	for {
		p, err := fr.ReadPacket()
		if err != nil {
			return err
		}
		deliver(p)
	}
}

func (c *h2Conn) Close() {
	c.once.Do(func() {
		c.fw.Close()
		_ = c.pw.Close()
		if c.body != nil {
			_ = c.body.Close()
		}
		_ = c.cc.Close()
		_ = c.raw.Close()
	})
}

func (d *Dialer) dialH2(ctx context.Context, t config.Target, authKey []byte) (Transport, error) {
	dial := d.Dial
	if dial == nil {
		dial = (&net.Dialer{Control: netpin.Control}).DialContext
	}
	raw, err := dial(ctx, "tcp", t.Addr.String())
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	c, err := d.h2Handshake(raw, t, authKey)
	if !stop() {
		if c != nil {
			c.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return c, nil
}

func (d *Dialer) tlsOnly(raw net.Conn) (*utls.UConn, error) {
	uc := utls.UClient(raw, &utls.Config{
		ServerName:            d.Profile.ServerName,
		InsecureSkipVerify:    d.pinned(),
		NextProtos:            []string{"h2", "http/1.1"},
		VerifyPeerCertificate: d.verifyPin,
	}, utls.HelloChrome_120) // совпадает с QUIC-пресетом: ECH GREASE, без PQ, ALPS 17513
	if err := uc.Handshake(); err != nil {
		return nil, fmt.Errorf("tls: %w", err)
	}
	return uc, nil
}

func (d *Dialer) h2Client(raw net.Conn) (*utls.UConn, *http2.ClientConn, error) {
	uc, err := d.tlsOnly(raw)
	if err != nil {
		return nil, nil, err
	}
	if proto := uc.ConnectionState().NegotiatedProtocol; proto != "h2" {
		return nil, nil, fmt.Errorf("server negotiated %q instead of h2", proto)
	}
	tr := &http2.Transport{ReadIdleTimeout: 30 * time.Second, PingTimeout: 15 * time.Second}
	cc, err := tr.NewClientConn(uc)
	if err != nil {
		return nil, nil, err
	}
	return uc, cc, nil
}

func (d *Dialer) h2Handshake(raw net.Conn, t config.Target, authKey []byte) (*h2Conn, error) {
	_, cc, err := d.h2Client(raw)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	req, err := d.tunnelRequest(t, authKey, pr)
	if err != nil {
		_ = cc.Close()
		return nil, err
	}
	c := &h2Conn{target: t, raw: raw, cc: cc, pw: pw, fw: proto.NewFrameWriter(pw, nil)}
	resp, err := cc.RoundTrip(req)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.body = resp.Body
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/octet-stream" {
		c.Close()
		return nil, fmt.Errorf("сервер отклонил подключение (HTTP %d)", resp.StatusCode)
	}
	return c, nil
}
