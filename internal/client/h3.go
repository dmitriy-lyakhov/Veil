package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"veil/internal/netpin"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go/quicvarint"
	quic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"

	"veil/internal/config"
	"veil/internal/proto"
)

const (
	quicIdleTimeout = 30 * time.Second
	quicKeepAlive   = 10 * time.Second

	uquicDatagramLimit = 1150

	h3StreamControl      = 0x00
	h3StreamQPACKEncoder = 0x02
	h3StreamQPACKDecoder = 0x03
	h3FrameHeaders       = 0x01
	h3FrameSettings      = 0x04
	h3SettingQPACKTable  = 0x01
	h3SettingMaxField    = 0x06
	h3SettingQPACKBlock  = 0x07
	h3SettingDatagram    = 0x33
	h3ErrNoError         = 0x100
	maxResponseHeaders   = 64 << 10
)

type h3Conn struct {
	target config.Target
	udp    *net.UDPConn
	qc     quic.Connection
	str    quic.Stream
	prefix []byte
	sender *proto.DatagramSender
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func (c *h3Conn) Target() config.Target { return c.target }
func (c *h3Conn) Kind() Kind            { return KindH3 }

func (c *h3Conn) WritePacket(p []byte) error { return c.sender.WritePacket(p) }

func (c *h3Conn) sendDatagram(b []byte) error {
	d := make([]byte, 0, len(c.prefix)+len(b))
	d = append(append(d, c.prefix...), b...)
	return c.qc.SendDatagram(d)
}

func (c *h3Conn) ReadLoop(deliver func([]byte)) error {
	rx := proto.NewDatagramReceiver()
	for {
		d, err := c.qc.ReceiveDatagram(c.ctx)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(d, c.prefix) {
			continue
		}
		if p, err := rx.Decode(d[len(c.prefix):]); err == nil && p != nil {
			deliver(p)
		}
	}
}

func (c *h3Conn) watchStream() {
	_, _ = io.Copy(io.Discard, c.str)
	c.Close()
}

func (c *h3Conn) Close() {
	c.once.Do(func() {
		c.cancel()
		if c.qc != nil {
			_ = c.qc.CloseWithError(h3ErrNoError, "")
		}
		_ = c.udp.Close()
	})
}

func chromeQUICSpec(t config.Target) (*quic.QUICSpec, error) {
	id := quic.QUICChrome_115_IPv4
	if t.Addr.Addr().Is6() {
		id = quic.QUICChrome_115_IPv6
	}
	spec, err := quic.QUICID2Spec(id)
	if err != nil {
		return nil, err
	}
	modernizeChromeQUIC(&spec)
	guardFrameBuilder(&spec)
	return &spec, nil
}

// modernizeChromeQUIC добавляет к пресету uquic (Chrome 115) ECH GREASE, как у Chrome с 117.
// Постквантовый key share (X25519MLKEM768, Chrome с 131) добавить нельзя: ClientHello
// перестаёт помещаться в один Initial-пакет, а uquic не умеет делить его на два — HTTP/3
// не устанавливается (проверено).
func modernizeChromeQUIC(spec *quic.QUICSpec) {
	exts := spec.ClientHelloSpec.Extensions
	pos := rand.IntN(len(exts) + 1)
	exts = append(exts, nil)
	copy(exts[pos+1:], exts[pos:])
	exts[pos] = &utls.GREASEEncryptedClientHelloExtension{}
	spec.ClientHelloSpec.Extensions = exts
}

func (d *Dialer) dialH3(ctx context.Context, t config.Target, authKey []byte) (Transport, error) {
	network := "udp4"
	if t.Addr.Addr().Is6() {
		network = "udp6"
	}
	lc := net.ListenConfig{Control: netpin.Control}
	laddr := "0.0.0.0:0"
	if network == "udp6" {
		laddr = "[::]:0"
	}
	pc, err := lc.ListenPacket(ctx, network, laddr)
	if err != nil {
		return nil, err
	}
	udp := pc.(*net.UDPConn)
	connCtx, cancel := context.WithCancel(context.Background())
	c := &h3Conn{target: t, udp: udp, ctx: connCtx, cancel: cancel}
	stop := context.AfterFunc(ctx, c.Close)
	err = d.h3Handshake(ctx, c, authKey)
	if !stop() {
		c.Close()
		return nil, ctx.Err()
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (d *Dialer) h3Handshake(ctx context.Context, c *h3Conn, authKey []byte) error {
	spec, err := chromeQUICSpec(c.target)
	if err != nil {
		return err
	}
	ut := &quic.UTransport{Transport: &quic.Transport{Conn: c.udp}, QUICSpec: spec}
	tlsConf := &utls.Config{
		ServerName:            d.Profile.TLSName(),
		InsecureSkipVerify:    d.pinned(),
		NextProtos:            []string{"h3"},
		VerifyPeerCertificate: d.verifyPin,
	}
	c.qc, err = ut.Dial(ctx, net.UDPAddrFromAddrPort(c.target.Addr), tlsConf, &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  quicIdleTimeout,
		KeepAlivePeriod: quicKeepAlive,
	})
	if err != nil {
		return fmt.Errorf("quic: %w", err)
	}
	if err := openH3UniStreams(c.qc); err != nil {
		return err
	}
	settings := make(chan bool, 1)
	go acceptH3UniStreams(c.ctx, c.qc, settings)

	c.str, err = c.qc.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	req, err := d.tunnelRequest(c.target, authKey, nil)
	if err != nil {
		return err
	}
	if _, err := c.str.Write(encodeH3Headers(req)); err != nil {
		return err
	}
	status, ctype, err := readH3Response(c.str)
	if err != nil {
		return err
	}
	if status != http.StatusOK || ctype != "application/octet-stream" {
		return fmt.Errorf("сервер отклонил подключение (HTTP %d)", status)
	}
	select {
	case ok := <-settings:
		if !ok {
			return errors.New("сервер не поддерживает HTTP/3 датаграммы")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	c.prefix = quicvarint.Append(nil, uint64(c.str.StreamID())/4)
	c.sender = proto.NewFixedDatagramSender(c.sendDatagram, uquicDatagramLimit)
	go c.watchStream()
	return nil
}

func openH3UniStreams(qc quic.Connection) error {
	ctrl, err := qc.OpenUniStream()
	if err != nil {
		return err
	}
	var settings []byte
	for _, kv := range [][2]uint64{
		{h3SettingQPACKTable, 65536},
		{h3SettingMaxField, 262144},
		{h3SettingQPACKBlock, 100},
		{h3SettingDatagram, 1},
		{0x1f*uint64(rand.IntN(1<<16)) + 0x21, uint64(rand.Uint32())},
	} {
		settings = quicvarint.Append(settings, kv[0])
		settings = quicvarint.Append(settings, kv[1])
	}
	b := quicvarint.Append(nil, h3StreamControl)
	b = appendH3Frame(b, h3FrameSettings, settings)
	if _, err := ctrl.Write(b); err != nil {
		return err
	}
	for _, typ := range []uint64{h3StreamQPACKEncoder, h3StreamQPACKDecoder} {
		s, err := qc.OpenUniStream()
		if err != nil {
			return err
		}
		if _, err := s.Write(quicvarint.Append(nil, typ)); err != nil {
			return err
		}
	}
	return nil
}

func acceptH3UniStreams(ctx context.Context, qc quic.Connection, settings chan<- bool) {
	for {
		s, err := qc.AcceptUniStream(ctx)
		if err != nil {
			select {
			case settings <- false:
			default:
			}
			return
		}
		go func() {
			r := bufio.NewReader(s)
			typ, err := quicvarint.Read(r)
			if err == nil && typ == h3StreamControl {
				ft, payload, err := readH3Frame(r, maxResponseHeaders)
				ok := err == nil && ft == h3FrameSettings && settingEnabled(payload, h3SettingDatagram)
				select {
				case settings <- ok:
				default:
				}
			}
			_, _ = io.Copy(io.Discard, r)
		}()
	}
}

func settingEnabled(payload []byte, id uint64) bool {
	r := bytes.NewReader(payload)
	for r.Len() > 0 {
		k, err := quicvarint.Read(r)
		if err != nil {
			return false
		}
		v, err := quicvarint.Read(r)
		if err != nil {
			return false
		}
		if k == id {
			return v == 1
		}
	}
	return false
}

func encodeH3Headers(req *http.Request) []byte {
	var block bytes.Buffer
	enc := qpack.NewEncoder(&block)
	fields := []qpack.HeaderField{
		{Name: ":method", Value: req.Method},
		{Name: ":authority", Value: req.Host},
		{Name: ":scheme", Value: "https"},
		{Name: ":path", Value: req.URL.RequestURI()},
		{Name: "content-type", Value: req.Header.Get("Content-Type")},
		{Name: "authorization", Value: req.Header.Get("Authorization")},
		{Name: "user-agent", Value: req.Header.Get("User-Agent")},
		{Name: "accept", Value: req.Header.Get("Accept")},
		{Name: "accept-encoding", Value: "gzip, deflate, br, zstd"},
		{Name: "accept-language", Value: req.Header.Get("Accept-Language")},
		{Name: "priority", Value: "u=1, i"},
	}
	if req.Host == "" {
		fields[1].Value = req.URL.Host
	}
	for _, f := range fields {
		_ = enc.WriteField(f)
	}
	_ = enc.Close()
	return appendH3Frame(nil, h3FrameHeaders, block.Bytes())
}

func readH3Response(s io.Reader) (int, string, error) {
	r := bufio.NewReader(s)
	for {
		ft, payload, err := readH3Frame(r, maxResponseHeaders)
		if err != nil {
			return 0, "", err
		}
		if ft != h3FrameHeaders {
			continue
		}
		status, ctype := 0, ""
		next := qpack.NewDecoder().Decode(payload)
		for {
			hf, err := next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return 0, "", err
			}
			switch strings.ToLower(hf.Name) {
			case ":status":
				status, _ = strconv.Atoi(hf.Value)
			case "content-type":
				ctype = hf.Value
			}
		}
		return status, ctype, nil
	}
}

func appendH3Frame(b []byte, typ uint64, payload []byte) []byte {
	b = quicvarint.Append(b, typ)
	b = quicvarint.Append(b, uint64(len(payload)))
	return append(b, payload...)
}

func readH3Frame(r *bufio.Reader, limit int) (uint64, []byte, error) {
	typ, err := quicvarint.Read(r)
	if err != nil {
		return 0, nil, err
	}
	n, err := quicvarint.Read(r)
	if err != nil {
		return 0, nil, err
	}
	if n > uint64(limit) {
		return 0, nil, fmt.Errorf("http3 frame of %d bytes too large", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return typ, payload, nil
}
