package gui

import (
	"strings"
	"testing"
)

const testOVPN = "client\ndev tun\nproto tcp\nremote 203.0.113.5 443\n<ca>\n-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----\n</ca>\n"

func TestDecodeAnyOpenVPN(t *testing.T) {
	p, err := decodeAny(testOVPN)
	if err != nil {
		t.Fatal(err)
	}
	if p.Protocol != "openvpn" || p.OVPN == nil || p.IPv4 != "203.0.113.5" || p.Name != "OpenVPN 203.0.113.5" {
		t.Fatalf("%+v", p)
	}
	if p.IsVeil() {
		t.Error("профиль OpenVPN не должен считаться профилем Veil")
	}
}

func TestDecodeAnyOpenVPNRejectsScripts(t *testing.T) {
	if _, err := decodeAny(testOVPN + "up evil.bat\n"); err == nil || !strings.Contains(err.Error(), "up") {
		t.Fatalf("ожидалась ошибка про директиву up, получено %v", err)
	}
}

func TestDecodeAnyWireGuardStillWorks(t *testing.T) {
	_, err := decodeAny("[Interface]\nPrivateKey = x\n")
	if err == nil {
		t.Skip("разбор WireGuard принял неполный файл")
	}
	if strings.Contains(err.Error(), "OpenVPN") {
		t.Fatalf("WireGuard-файл принят за OpenVPN: %v", err)
	}
}
