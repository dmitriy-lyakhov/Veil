package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"veil/internal/ovpnconf"
)

func TestParseMgmtLine(t *testing.T) {
	ev := parseMgmtLine(">STATE:1700000000,CONNECTED,SUCCESS,10.9.0.2,203.0.113.5,1194,,\r\n")
	if ev.kind != "state" || ev.state != "CONNECTED" || ev.local != "10.9.0.2" || ev.remote != "203.0.113.5" {
		t.Fatalf("%+v", ev)
	}
	if ev := parseMgmtLine(">BYTECOUNT:1234,5678"); ev.kind != "bytecount" || ev.rx != 1234 || ev.tx != 5678 {
		t.Fatalf("%+v", ev)
	}
	if ev := parseMgmtLine(">FATAL:Cannot open TUN"); ev.kind != "fatal" || ev.detail != "Cannot open TUN" {
		t.Fatalf("%+v", ev)
	}
	for _, other := range []string{"SUCCESS: password is correct", ">INFO:x", ">BYTECOUNT:x,y", ""} {
		if parseMgmtLine(other).kind != "other" {
			t.Errorf("%q должна быть «other»", other)
		}
	}
}

func TestOVStateToClient(t *testing.T) {
	cases := map[string]State{
		"CONNECTED": StateConnected, "RECONNECTING": StateReconnecting, "EXITING": StateStopped,
		"CONNECTING": StateConnecting, "WAIT": StateConnecting, "AUTH": StateConnecting,
		"GET_CONFIG": StateConnecting, "ASSIGN_IP": StateConnecting, "ADD_ROUTES": StateConnecting, "TCP_CONNECT": StateConnecting,
	}
	for in, want := range cases {
		if got := ovStateToClient(in); got != want {
			t.Errorf("%s: %s, ожидалось %s", in, got, want)
		}
	}
}

func TestFindOpenVPN(t *testing.T) {
	has := func(set ...string) func(string) bool {
		return func(p string) bool {
			for _, s := range set {
				if s == p {
					return true
				}
			}
			return false
		}
	}
	noPath := func(string) (string, error) { return "", errors.New("нет") }
	if p, err := findOpenVPN([]string{`C:\a\openvpn.exe`, `C:\b\openvpn.exe`}, has(`C:\b\openvpn.exe`), noPath); err != nil || p != `C:\b\openvpn.exe` {
		t.Fatal(p, err)
	}
	if p, err := findOpenVPN(nil, has(), func(string) (string, error) { return "/usr/bin/openvpn", nil }); err != nil || p != "/usr/bin/openvpn" {
		t.Fatal(p, err)
	}
	_, err := findOpenVPN([]string{"x"}, has(), noPath)
	if !errors.Is(err, ErrOpenVPNMissing) || !strings.Contains(err.Error(), "openvpn.net") {
		t.Fatalf("ожидалось сообщение со ссылкой, получено %v", err)
	}
}

func TestOVFailure(t *testing.T) {
	cases := map[string]string{
		"TLS Error: TLS key negotiation failed to occur within 60 seconds": "рукопожатие",
		"VERIFY ERROR: certificate verify failed":                          "сертификат",
		"Options error: Unrecognized option":                               "настроек",
		"There are no TAP-Windows adapters on this system":                 "драйвера",
	}
	for in, want := range cases {
		if got := ovFailure(in); !strings.Contains(got, want) {
			t.Errorf("%q → %q, ожидалось «%s»", in, got, want)
		}
	}
	if ovFailure("всё хорошо") != "" {
		t.Error("для обычных строк объяснения быть не должно")
	}
}

func TestOVArgsAndConfig(t *testing.T) {
	a := strings.Join(ovArgs("c.ovpn", "l.log", 7505, "pw"), " ")
	for _, want := range []string{"--config c.ovpn", "--management 127.0.0.1 7505 pw", "--log l.log"} {
		if !strings.Contains(a, want) {
			t.Errorf("нет %q в %q", want, a)
		}
	}
	c, err := ovpnconf.Parse("client\nremote 203.0.113.5 1194\n<ca>\nx\n</ca>\n")
	if err != nil {
		t.Fatal(err)
	}
	if ovConfigText(c, nil) != c.Text {
		t.Error("без DNS текст не меняется")
	}
	txt := ovConfigText(c, []string{"9.9.9.9"})
	if !strings.Contains(txt, "dhcp-option DNS 9.9.9.9") || !strings.HasPrefix(txt, "pull-filter ignore") {
		t.Errorf("DNS настроек должен заменять серверные:\n%s", txt)
	}
}

// Поддельный управляющий интерфейс: проверяет пароль, приём состояния и счётчиков, команду остановки.
func TestManageAgainstFakeMgmt(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	got := make(chan string, 8)
	var tn *ovTunnel
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		io.WriteString(c, "ENTER PASSWORD:")
		r := bufio.NewReader(c)
		ln1, _ := r.ReadString('\n')
		got <- strings.TrimSpace(ln1)
		io.WriteString(c, "SUCCESS: password is correct\r\n>STATE:1,CONNECTING,,,,,,\r\n>STATE:2,CONNECTED,SUCCESS,10.9.0.2,203.0.113.5,1194,,\r\n>BYTECOUNT:100,200\r\n")
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			got <- strings.TrimSpace(l)
			if strings.HasPrefix(l, "signal SIGTERM") {
				close(tn.done)
				return
			}
		}
	}()
	tn = &ovTunnel{log: slog.New(slog.NewTextHandler(io.Discard, nil)), state: "CONNECTING", done: make(chan struct{}), started: time.Now(), family: "IPv4", proto: "UDP", remote: "203.0.113.5:1194"}
	go tn.manage(port, "secret")
	if p := <-got; p != "secret" {
		t.Fatalf("пароль: %q", p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && tn.Status().RxBytes == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	st := tn.Status()
	if st.State != StateConnected || st.RxBytes != 100 || st.TxBytes != 200 || st.Transport != "OpenVPN UDP" {
		t.Fatalf("%+v", st)
	}
	// штатная остановка: Close шлёт signal SIGTERM и ждёт завершения
	tn.Close()
	select {
	case <-tn.done:
	default:
		t.Fatal("Close не дождался завершения")
	}
	if tn.Status().State != StateStopped {
		t.Fatal("после остановки состояние должно быть «stopped»")
	}
}
