package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/conn"
	awgdevice "github.com/amnezia-vpn/amneziawg-go/device"
	awgtuntest "github.com/amnezia-vpn/amneziawg-go/tun/tuntest"
	wgtuntest "golang.zx2c4.com/wireguard/tun/tuntest"

	"veil/internal/config"
	"veil/internal/vpnconf/wgconf"
	"veil/internal/vpnconf/wgkey"
)

func freeUDPPort(t *testing.T) int {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

// startAWGServer поднимает настоящее устройство amneziawg-go на loopback с заданной обфускацией.
func startAWGServer(t *testing.T, srvPriv, cliPub string, port int, obfs string) {
	srvTun := awgtuntest.NewChannelTUN()
	srv := awgdevice.NewDevice(srvTun.TUN(), awgconn.NewDefaultBind(), awgdevice.NewLogger(awgdevice.LogLevelSilent, ""))
	t.Cleanup(srv.Close)
	sp, _ := wgkey.Hex(srvPriv)
	cp, _ := wgkey.Hex(cliPub)
	if err := srv.IpcSet(fmt.Sprintf("private_key=%s\nlisten_port=%d\n%spublic_key=%s\nallowed_ip=10.9.1.2/32\n", sp, port, obfs, cp)); err != nil {
		t.Fatal(err)
	}
	if err := srv.Up(); err != nil {
		t.Fatal(err)
	}
}

// Клиентский движок AmneziaWG против настоящего устройства amneziawg-go: обфускация Jc/S1/S2/H1–H4 и I1.
func TestAmneziaWGEngineHandshake(t *testing.T) {
	srvPriv, srvPub, _ := wgkey.Generate()
	cliPriv, cliPub, _ := wgkey.Generate()
	port := freeUDPPort(t)
	startAWGServer(t, srvPriv, cliPub, port,
		"jc=3\njmin=10\njmax=40\ns1=31\ns2=47\nh1=1000001\nh2=2000002\nh3=3000003\nh4=4000004\n")

	cfg := &wgconf.Config{
		PrivateKey: cliPriv, Address: []netip.Prefix{netip.MustParsePrefix("10.9.1.2/32")},
		PublicKey: srvPub, Endpoint: fmt.Sprintf("127.0.0.1:%d", port), Keepalive: 1,
		AWG: &wgconf.AWG{Jc: 3, Jmin: 10, Jmax: 40, S1: 31, S2: 47, H1: "1000001", H2: "2000002", H3: "3000003", H4: "4000004",
			I: [5]string{"<b 0xf6ab3267fa><r 12><t>"}},
	}
	prof := &config.Profile{Name: "t", Protocol: "amneziawg", WG: cfg}
	nc, err := netConfigFor(prof)
	if err != nil {
		t.Fatal(err)
	}
	if !nc.BypassServers || nc.Addr4.String() != "10.9.1.2/32" || len(nc.Servers) != 1 || nc.Plan != nil {
		t.Fatalf("netconfig: %+v", nc)
	}
	tn, err := startAmneziaWG(prof, wgtuntest.NewChannelTUN().TUN(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatalf("рукопожатие не состоялось: %v", err)
	}
	if st := tn.Status(); st.State != StateConnected || st.Transport != "AmneziaWG" || st.Family != "IPv4" {
		t.Fatalf("status: %+v", st)
	}
}

// Если параметры клиента не совпадают с серверными, рукопожатия быть не должно.
func TestAmneziaWGEngineMismatch(t *testing.T) {
	srvPriv, srvPub, _ := wgkey.Generate()
	cliPriv, cliPub, _ := wgkey.Generate()
	port := freeUDPPort(t)
	startAWGServer(t, srvPriv, cliPub, port, "s1=31\ns2=47\nh1=1000001\nh2=2000002\nh3=3000003\nh4=4000004\n")
	cfg := &wgconf.Config{
		PrivateKey: cliPriv, Address: []netip.Prefix{netip.MustParsePrefix("10.9.1.2/32")},
		PublicKey: srvPub, Endpoint: fmt.Sprintf("127.0.0.1:%d", port), Keepalive: 1,
		AWG: &wgconf.AWG{S1: 31, S2: 47, H1: "1000999", H2: "2000002", H3: "3000003", H4: "4000004"},
	}
	tn, err := startAmneziaWG(&config.Profile{Protocol: "amneziawg", WG: cfg}, wgtuntest.NewChannelTUN().TUN(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err == nil {
		t.Fatal("с другим H1 рукопожатие состояться не должно")
	}
}

func TestAWGUAPI(t *testing.T) {
	_, pub, _ := wgkey.Generate()
	priv, _, _ := wgkey.Generate()
	ep := netip.MustParseAddrPort("198.51.100.1:1234")
	cfg := &wgconf.Config{PrivateKey: priv, PublicKey: pub, Endpoint: ep.String(),
		AWG: &wgconf.AWG{Jc: 4, Jmin: 10, Jmax: 50, S1: 20, S2: 30, H1: "100", H2: "200", H3: "300", H4: "400", I: [5]string{"<t>", "", "<r 5>"}}}
	s, err := awgUAPI(cfg, ep)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"jc=4\n", "jmin=10\n", "jmax=50\n", "s1=20\n", "s2=30\n", "h4=400\n", "i1=<t>\n", "i3=<r 5>\n", "endpoint=198.51.100.1:1234\n"} {
		if !strings.Contains(s, want) {
			t.Errorf("нет %q в\n%s", want, s)
		}
	}
	if strings.Contains(s, "i2=") || strings.Index(s, "jc=") > strings.Index(s, "public_key=") {
		t.Errorf("порядок или лишние ключи:\n%s", s)
	}
	cfg.AWG.S3, cfg.AWG.S4, cfg.AWG.H1 = 5, 6, "100-200"
	s, err = awgUAPI(cfg, ep)
	if err != nil || !strings.Contains(s, "s3=5\n") || !strings.Contains(s, "s4=6\n") || !strings.Contains(s, "h1=100-200\n") {
		t.Fatalf("AWG 2.0: %v\n%s", err, s)
	}
}

// AmneziaWG 2.0: S3/S4 и диапазоны заголовков.
func TestAmneziaWGEngineHandshakeV2(t *testing.T) {
	srvPriv, srvPub, _ := wgkey.Generate()
	cliPriv, cliPub, _ := wgkey.Generate()
	port := freeUDPPort(t)
	startAWGServer(t, srvPriv, cliPub, port,
		"jc=2\njmin=10\njmax=40\ns1=31\ns2=47\ns3=20\ns4=9\nh1=100000-200000\nh2=300000-400000\nh3=500000-600000\nh4=700000-800000\n")
	cfg := &wgconf.Config{
		PrivateKey: cliPriv, Address: []netip.Prefix{netip.MustParsePrefix("10.9.1.2/32")},
		PublicKey: srvPub, Endpoint: fmt.Sprintf("127.0.0.1:%d", port), Keepalive: 1,
		AWG: &wgconf.AWG{Jc: 2, Jmin: 10, Jmax: 40, S1: 31, S2: 47, S3: 20, S4: 9,
			H1: "100000-200000", H2: "300000-400000", H3: "500000-600000", H4: "700000-800000"},
	}
	tn, err := startAmneziaWG(&config.Profile{Protocol: "amneziawg", WG: cfg}, wgtuntest.NewChannelTUN().TUN(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatalf("рукопожатие AWG 2.0 не состоялось: %v", err)
	}
}
