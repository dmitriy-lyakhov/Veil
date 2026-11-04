package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/tuntest"

	"veil/internal/config"
	"veil/internal/wgconf"
	"veil/internal/wgkey"
)

// Клиентский движок WireGuard против настоящего устройства wireguard-go на loopback:
// проверяем разбор профиля, настройку и рукопожатие.
func TestWireGuardEngineHandshake(t *testing.T) {
	srvPriv, srvPub, _ := wgkey.Generate()
	cliPriv, cliPub, _ := wgkey.Generate()

	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()

	srvTun := tuntest.NewChannelTUN()
	srv := device.NewDevice(srvTun.TUN(), conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer srv.Close()
	sp, _ := wgkey.Hex(srvPriv)
	cp, _ := wgkey.Hex(cliPub)
	if err := srv.IpcSet(fmt.Sprintf("private_key=%s\nlisten_port=%d\npublic_key=%s\nallowed_ip=10.8.1.2/32\n", sp, port, cp)); err != nil {
		t.Fatal(err)
	}
	if err := srv.Up(); err != nil {
		t.Fatal(err)
	}

	cfg := &wgconf.Config{
		PrivateKey: cliPriv, Address: []netip.Prefix{netip.MustParsePrefix("10.8.1.2/32")},
		PublicKey: srvPub, Endpoint: fmt.Sprintf("127.0.0.1:%d", port), Keepalive: 1,
	}
	prof := &config.Profile{Name: "t", Protocol: "wireguard", WG: cfg}
	nc, err := netConfigFor(prof)
	if err != nil {
		t.Fatal(err)
	}
	if !nc.BypassServers || nc.Addr4.String() != "10.8.1.2/32" || len(nc.Servers) != 1 || nc.Plan != nil {
		t.Fatalf("netconfig: %+v", nc)
	}

	cliTun := tuntest.NewChannelTUN()
	tn, err := startWireGuard(prof, cliTun.TUN(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := tn.WaitHandshake(ctx); err != nil {
		t.Fatalf("рукопожатие не состоялось: %v", err)
	}
	st := tn.Status()
	if st.State != StateConnected || st.Transport != "WireGuard" || st.Family != "IPv4" {
		t.Fatalf("status: %+v", st)
	}
}

func TestProtocolNetConfigSelective(t *testing.T) {
	cfg := &wgconf.Config{
		PrivateKey: "a", PublicKey: "b", Endpoint: "198.51.100.1:51820",
		Address:    []netip.Prefix{netip.MustParsePrefix("10.0.0.2/32")},
		AllowedIPs: []netip.Prefix{netip.MustParsePrefix("192.168.0.0/16")},
	}
	nc, err := netConfigFor(&config.Profile{Protocol: "wireguard", WG: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if nc.Plan == nil || !nc.Plan.Selective || len(nc.Plan.Via) != 1 {
		t.Fatalf("AllowedIPs не «всё» должны давать выборочный режим: %+v", nc.Plan)
	}
	if _, err := netConfigFor(&config.Profile{Protocol: "openvpn"}); err == nil {
		t.Fatal("неподдерживаемый протокол должен давать ошибку")
	}
}
