// Package front — маскировка входа под чужой сайт (в духе REALITY, но без собственной реализации TLS).
//
// Клиент отправляет настоящий ClientHello браузера с SNI популярного сайта (например ya.ru), а в поле
// session_id кладёт метку HMAC от общего ключа и случайного числа из того же ClientHello.
// Сервер читает ClientHello до TLS-рукопожатия: если метка верна — отдаёт соединение туннельному
// TLS-серверу, иначе прозрачно проксирует TCP на настоящий сайт. Любой, кто не знает ключа
// (в том числе активный пробник), видит подлинный сертификат и ответы чужого сайта.
package front

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// SessionIDLen — длина session_id в ClientHello TLS 1.3 у браузеров.
	SessionIDLen = 32
	// KeyLen — длина общего ключа.
	KeyLen = 32

	maxRecord     = 16384 + 256
	helloDeadline = 10 * time.Second
	spliceIdle    = 2 * time.Minute
	skew          = 1 // допустимое расхождение часов клиента и сервера, минут
	replayWindow  = 5 * time.Minute
)

// Tag — метка клиента для ClientHello с данным случайным числом и моментом времени.
func Tag(key, random []byte, t time.Time) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("veil-front-v1"))
	m.Write(random)
	var min [8]byte
	binary.BigEndian.PutUint64(min[:], uint64(t.Unix()/60))
	m.Write(min[:])
	return m.Sum(nil)
}

// Hello — разобранные поля ClientHello, нужные для решения.
type Hello struct {
	SNI       string
	Random    []byte
	SessionID []byte
}

// ParseClientHello разбирает запись TLS (с заголовком) с ClientHello.
func ParseClientHello(rec []byte) (*Hello, error) {
	if len(rec) < 5 || rec[0] != 0x16 {
		return nil, errors.New("not a TLS handshake record")
	}
	b := rec[5:]
	if len(b) < 4 || b[0] != 0x01 {
		return nil, errors.New("not a ClientHello")
	}
	n := int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	b = b[4:]
	if len(b) < n {
		return nil, errors.New("fragmented ClientHello")
	}
	b = b[:n]
	if len(b) < 2+32+1 {
		return nil, errors.New("short ClientHello")
	}
	h := &Hello{Random: b[2:34]}
	b = b[34:]
	sl := int(b[0])
	if len(b) < 1+sl+2 {
		return nil, errors.New("short ClientHello")
	}
	h.SessionID = b[1 : 1+sl]
	b = b[1+sl:]
	cs := int(binary.BigEndian.Uint16(b))
	if len(b) < 2+cs+1 {
		return nil, errors.New("short ClientHello")
	}
	b = b[2+cs:]
	cm := int(b[0])
	if len(b) < 1+cm+2 {
		return nil, errors.New("short ClientHello")
	}
	b = b[1+cm:]
	el := int(binary.BigEndian.Uint16(b))
	b = b[2:]
	if len(b) < el {
		return nil, errors.New("short extensions")
	}
	b = b[:el]
	for len(b) >= 4 {
		typ, l := binary.BigEndian.Uint16(b), int(binary.BigEndian.Uint16(b[2:]))
		b = b[4:]
		if len(b) < l {
			break
		}
		if typ == 0 && l >= 5 { // server_name
			d := b[:l]
			if nl := int(binary.BigEndian.Uint16(d[3:])); len(d) >= 5+nl && d[2] == 0 {
				h.SNI = strings.ToLower(string(d[5 : 5+nl]))
			}
		}
		b = b[l:]
	}
	return h, nil
}

// Config — настройки маскировки сервера.
type Config struct {
	SNI  string // под какой сайт маскируемся (ожидаемый SNI клиентов)
	Dest string // куда проксировать чужих, host:port; по умолчанию SNI:443
	Key  []byte // общий ключ, KeyLen байт
}

func (c *Config) dest() string {
	if c.Dest != "" {
		return c.Dest
	}
	if strings.Contains(c.SNI, ":") {
		return c.SNI
	}
	return net.JoinHostPort(c.SNI, "443")
}

// Verify сообщает, подходит ли ClientHello нашему клиенту.
func (c *Config) Verify(h *Hello, now time.Time) bool {
	if len(h.SessionID) != SessionIDLen || len(c.Key) != KeyLen {
		return false
	}
	ok := false
	for d := -skew; d <= skew; d++ {
		if hmac.Equal(h.SessionID, Tag(c.Key, h.Random, now.Add(time.Duration(d)*time.Minute))) {
			ok = true
		}
	}
	return ok
}

// Listener оборачивает TCP-слушатель: «свои» соединения возвращает из Accept, чужие проксирует.
type Listener struct {
	net.Listener
	get    func() *Config
	bypass func(net.Conn) bool
	ch     chan net.Conn
	done   chan struct{}
	once   sync.Once

	mu   sync.Mutex
	err  error
	seen map[[32]byte]time.Time
}

// NewListener — get возвращает текущие настройки (nil — маскировка выключена и всё идёт как есть),
// bypass — соединения, которые никогда не проксируются (например, изнутри туннеля к панели).
func NewListener(l net.Listener, get func() *Config, bypass func(net.Conn) bool) *Listener {
	fl := &Listener{Listener: l, get: get, bypass: bypass, ch: make(chan net.Conn), done: make(chan struct{}), seen: map[[32]byte]time.Time{}}
	go fl.loop()
	return fl
}

func (l *Listener) loop() {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			l.once.Do(func() { close(l.done) })
			l.fail(err)
			return
		}
		go l.handle(c)
	}
}

func (l *Listener) fail(err error) {
	l.mu.Lock()
	l.err = err
	l.mu.Unlock()
}

// Accept возвращает следующее «своё» соединение.
func (l *Listener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.err != nil {
			return nil, l.err
		}
		return nil, net.ErrClosed
	}
}

func (l *Listener) deliver(c net.Conn) {
	select {
	case l.ch <- c:
	case <-l.done:
		_ = c.Close()
	}
}

func (l *Listener) handle(c net.Conn) {
	cfg := l.get()
	if cfg == nil || (l.bypass != nil && l.bypass(c)) {
		l.deliver(c)
		return
	}
	_ = c.SetReadDeadline(time.Now().Add(helloDeadline))
	var hdr [5]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		_ = c.Close()
		return
	}
	buf := append([]byte(nil), hdr[:]...)
	if hdr[0] == 0x16 {
		if n := int(binary.BigEndian.Uint16(hdr[3:])); n > 0 && n <= maxRecord {
			body := make([]byte, n)
			if m, err := io.ReadFull(c, body); err != nil {
				buf = append(buf, body[:m]...)
				l.splice(c, buf, cfg)
				return
			}
			buf = append(buf, body...)
			if h, err := ParseClientHello(buf); err == nil && cfg.Verify(h, time.Now()) && l.fresh(h.Random) {
				_ = c.SetReadDeadline(time.Time{})
				l.deliver(&prefixed{Conn: c, r: bytes.NewReader(buf)})
				return
			}
		}
	}
	l.splice(c, buf, cfg)
}

// fresh отвергает повторное использование одной и той же метки.
func (l *Listener) fresh(random []byte) bool {
	var k [32]byte
	copy(k[:], random)
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for r, t := range l.seen {
		if now.Sub(t) > replayWindow {
			delete(l.seen, r)
		}
	}
	if _, dup := l.seen[k]; dup {
		return false
	}
	l.seen[k] = now
	return true
}

// splice проксирует соединение на настоящий сайт, начав с уже прочитанных байтов.
func (l *Listener) splice(c net.Conn, prefix []byte, cfg *Config) {
	defer c.Close()
	up, err := net.DialTimeout("tcp", cfg.dest(), 5*time.Second)
	if err != nil {
		return
	}
	defer up.Close()
	if _, err := up.Write(prefix); err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		buf := make([]byte, 32*1024)
		for {
			_ = src.SetReadDeadline(time.Now().Add(spliceIdle))
			n, err := src.Read(buf)
			if n > 0 {
				if _, werr := dst.Write(buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}
	go pipe(up, c)
	go pipe(c, up)
	<-done
}

type prefixed struct {
	net.Conn
	r *bytes.Reader
}

func (p *prefixed) Read(b []byte) (int, error) {
	if p.r.Len() > 0 {
		return p.r.Read(b)
	}
	return p.Conn.Read(b)
}
