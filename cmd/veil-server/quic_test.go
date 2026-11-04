package main

import (
	"io"
	"log/slog"
	"net"
	"net/netip"
	"testing"

	"golang.zx2c4.com/wireguard/tun/netstack"

	"veil/internal/provision"
	"veil/internal/server"
)

func udpPortBusy(addr string) bool {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return true
	}
	pc.Close()
	return false
}

func TestQUICListenerFollowsMasking(t *testing.T) {
	cfg, err := provision.NewServer("en")
	if err != nil {
		t.Fatal(err)
	}
	tunDev, _, err := netstack.CreateNetTUN([]netip.Addr{netip.MustParseAddr("10.66.0.1")}, nil, cfg.MTU)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := server.New(cfg, "", tunDev, log)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	probe.Close()

	q := &quicService{srv: srv, addr: addr, log: log}
	srv.OnFrontChange(q.Sync)
	defer q.Close()
	q.Sync()
	if !udpPortBusy(addr) {
		t.Fatal("QUIC must listen while masking is off")
	}
	if err := srv.SetFront("ya.example", ""); err != nil {
		t.Fatal(err)
	}
	if udpPortBusy(addr) {
		t.Fatal("QUIC still answers with our certificate while masking is on")
	}
	if err := srv.SetFront("", ""); err != nil {
		t.Fatal(err)
	}
	if !udpPortBusy(addr) {
		t.Fatal("QUIC must come back when masking is turned off")
	}
}
