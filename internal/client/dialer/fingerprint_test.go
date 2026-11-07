package dialer

import (
	"context"
	"crypto/tls"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/gaukas/clienthellod"
	gquic "github.com/quic-go/quic-go"

	"veil/internal/config"
)

const (
	extCompressCertificate = 27
	extApplicationSettings = 17513

	qtpMaxDatagramFrameSize = 0x20
)

var goOnlyExtensions = []uint16{11, 23, 50, 65281}

func captureInitial(t *testing.T, dial func(addr netip.AddrPort)) *clienthellod.ClientInitialPacket {
	t.Helper()
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go dial(pc.LocalAddr().(*net.UDPAddr).AddrPort())
	buf := make([]byte, 2048)
	_ = pc.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := pc.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	cip, err := clienthellod.ParseQUICCIP(buf[:n])
	if err != nil {
		t.Fatalf("parse initial: %v", err)
	}
	if cip.QCH == nil {
		t.Fatal("no ClientHello in first Initial packet")
	}
	return cip
}

func TestH3ClientLooksLikeChrome(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ours := captureInitial(t, func(addr netip.AddrPort) {
		d := &Dialer{Profile: &config.Profile{}}
		_, _ = d.dialH3(ctx, config.Target{Family: "IPv4", Addr: addr}, nil)
	})
	stock := captureInitial(t, func(addr netip.AddrPort) {
		_, _ = gquic.DialAddr(ctx, addr.String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}, CurvePreferences: []tls.CurveID{tls.X25519}}, nil)
	})
	ch := ours.QCH.ClientHello
	t.Logf("ours:  %d bytes, ciphers %v, extensions %v, alpn %v", len(ours.QCH.Raw()), ch.CipherSuites, ch.Extensions, ch.ALPN)
	t.Logf("stock: ciphers %v, extensions %v", stock.QCH.ClientHello.CipherSuites, stock.QCH.ClientHello.Extensions)
	if !slices.Equal(ch.ALPN, []string{"h3"}) {
		t.Fatalf("alpn %v", ch.ALPN)
	}
	for _, ext := range goOnlyExtensions {
		if slices.Contains(ch.Extensions, ext) {
			t.Fatalf("extension %d is a Go TLS marker, Chrome QUIC never sends it", ext)
		}
		if !slices.Contains(stock.QCH.ClientHello.Extensions, ext) {
			t.Fatalf("stock quic-go no longer sends %d, update the marker list", ext)
		}
	}
	for _, ext := range []uint16{extCompressCertificate, extApplicationSettings} {
		if !slices.Contains(ch.Extensions, ext) {
			t.Fatalf("Chrome extension %d missing", ext)
		}
		if slices.Contains(stock.QCH.ClientHello.Extensions, ext) {
			t.Fatalf("stock quic-go unexpectedly sends extension %d", ext)
		}
	}
	if ours.QTP == nil || !slices.Contains(ours.QTP.QTPIDs, qtpMaxDatagramFrameSize) {
		t.Fatal("max_datagram_frame_size transport parameter missing")
	}
}
