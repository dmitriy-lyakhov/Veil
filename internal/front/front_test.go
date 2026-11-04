package front

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

// helloBytes снимает ClientHello настоящего crypto/tls клиента.
func helloBytes(t *testing.T, sni string) []byte {
	t.Helper()
	a, b := net.Pipe()
	go func() {
		c := tls.Client(a, &tls.Config{ServerName: sni, InsecureSkipVerify: true})
		_ = c.Handshake()
	}()
	buf := make([]byte, 5)
	if _, err := io.ReadFull(b, buf); err != nil {
		t.Fatal(err)
	}
	n := int(buf[3])<<8 | int(buf[4])
	body := make([]byte, n)
	if _, err := io.ReadFull(b, body); err != nil {
		t.Fatal(err)
	}
	b.Close()
	return append(buf, body...)
}

func TestParseClientHello(t *testing.T) {
	h, err := ParseClientHello(helloBytes(t, "Ya.RU"))
	if err != nil {
		t.Fatal(err)
	}
	if h.SNI != "ya.ru" || len(h.Random) != 32 || len(h.SessionID) != 32 && len(h.SessionID) != 0 {
		t.Fatalf("bad parse: %+v", h)
	}
}

func TestVerify(t *testing.T) {
	key := make([]byte, KeyLen)
	_, _ = rand.Read(key)
	cfg := &Config{SNI: "ya.ru", Key: key}
	rnd := make([]byte, 32)
	_, _ = rand.Read(rnd)
	now := time.Now()
	h := &Hello{Random: rnd, SessionID: Tag(key, rnd, now)}
	if !cfg.Verify(h, now) || !cfg.Verify(h, now.Add(time.Minute)) {
		t.Fatal("valid tag rejected")
	}
	if cfg.Verify(h, now.Add(10*time.Minute)) {
		t.Fatal("stale tag accepted")
	}
	h.SessionID[0] ^= 1
	if cfg.Verify(h, now) {
		t.Fatal("forged tag accepted")
	}
}

// Чужой клиент попадает на «настоящий сайт», клиент с меткой — к нашему серверу.
func TestListenerSplice(t *testing.T) {
	dest, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	go func() {
		for {
			c, err := dest.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); b := make([]byte, 4096); _, _ = c.Read(b); _, _ = c.Write([]byte("REAL")) }()
		}
	}()
	key := make([]byte, KeyLen)
	_, _ = rand.Read(key)
	cfg := &Config{SNI: "ya.ru", Dest: dest.Addr().String(), Key: key}
	raw, _ := net.Listen("tcp", "127.0.0.1:0")
	fl := NewListener(raw, func() *Config { return cfg }, nil)
	defer fl.Close()

	// чужой: ClientHello без метки
	c, _ := net.Dial("tcp", raw.Addr().String())
	hello := helloBytes(t, "ya.ru")
	_, _ = c.Write(hello)
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, 4)
	if _, err := io.ReadFull(c, got); err != nil || string(got) != "REAL" {
		t.Fatalf("stranger not spliced: %q %v", got, err)
	}
	c.Close()

	// свой: метка внутри session_id
	h, _ := ParseClientHello(hello)
	own := bytes.Clone(hello)
	// session_id начинается после заголовка(5)+handshake(4)+версия(2)+random(32)+длина(1)
	copy(own[5+4+2+32+1:], Tag(key, h.Random, time.Now()))
	if len(h.SessionID) != 32 {
		t.Skip("crypto/tls не шлёт session_id в этом режиме")
	}
	c2, _ := net.Dial("tcp", raw.Addr().String())
	defer c2.Close()
	_, _ = c2.Write(own)
	acc := make(chan net.Conn, 1)
	go func() { a, _ := fl.Accept(); acc <- a }()
	select {
	case a := <-acc:
		buf := make([]byte, len(own))
		_, _ = io.ReadFull(a, buf)
		if !bytes.Equal(buf, own) {
			t.Fatal("prefix not replayed to server")
		}
		a.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("authorised client not delivered")
	}
	_ = tls.VersionTLS13
}
