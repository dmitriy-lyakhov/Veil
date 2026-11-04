package client

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"

	"veil/internal/ipsecconf"
)

const testCA = "-----BEGIN CERTIFICATE-----\nQUJD\nREVG\n-----END CERTIFICATE-----\n"

func eapCfg() *ipsecconf.Config {
	return &ipsecconf.Config{Server: "203.0.113.5", CAPEM: testCA, Auth: ipsecconf.AuthEAP, Username: "user1", Password: "pa'ss"}
}

func TestPemBody(t *testing.T) {
	if got := pemBody(testCA); got != "QUJDREVG" {
		t.Fatalf("%q", got)
	}
}

func TestIPsecSetupScriptEAP(t *testing.T) {
	c := eapCfg()
	s := ipsecSetupScript(c, "C:/t/ca.crt", "", false)
	for _, want := range []string{
		"-TunnelType Ikev2", "-AuthenticationMethod MSChapv2", "-ServerAddress '203.0.113.5'",
		"-SplitTunneling:$false", "-AllUserConnection", "Set-VpnConnectionIPsecConfiguration",
		"-DHGroup Group14", "FromBase64String('QUJDREVG')", "throw 'CA_NOT_TRUSTED'",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("нет %q в скрипте:\n%s", want, s)
		}
	}
	if strings.Contains(s, "Import-Certificate") {
		t.Error("без разрешения сертификат не должен импортироваться")
	}
	if strings.Contains(s, "pa'ss") || strings.Contains(s, "user1") {
		t.Error("логин и пароль не должны попадать в скрипт настройки")
	}
}

func TestIPsecSetupScriptTrustSplitCert(t *testing.T) {
	c := eapCfg()
	c.TrustCA = true
	s := ipsecSetupScript(c, "C:/t/it's.crt", "", true)
	if !strings.Contains(s, `Import-Certificate -FilePath 'C:/t/it''s.crt' -CertStoreLocation Cert:\LocalMachine\Root`) {
		t.Error(s)
	}
	if strings.Contains(s, "CA_NOT_TRUSTED") || !strings.Contains(s, "-SplitTunneling:$true") {
		t.Error(s)
	}
	c.Auth, c.P12, c.P12Pass = ipsecconf.AuthCert, "AAAA", "p'w"
	s = ipsecSetupScript(c, "ca", "C:/t/c.p12", false)
	if !strings.Contains(s, "-AuthenticationMethod MachineCertificate") ||
		!strings.Contains(s, `Import-PfxCertificate -FilePath 'C:/t/c.p12' -CertStoreLocation Cert:\LocalMachine\My -Password (ConvertTo-SecureString 'p''w' -AsPlainText -Force)`) {
		t.Error(s)
	}
}

func TestRasdialArgs(t *testing.T) {
	a := rasdialArgs(eapCfg())
	if strings.Join(a, "|") != "rasdial|Veil IKEv2|user1|pa'ss" {
		t.Fatal(a)
	}
	c := eapCfg()
	c.Auth = ipsecconf.AuthCert
	if len(rasdialArgs(c)) != 2 {
		t.Fatal(rasdialArgs(c))
	}
}

func TestParseIPsecStatus(t *testing.T) {
	s := parseIPsecStatus("STATE=Connected\r\nRX=1234\r\nTX=99\r\n")
	if s.State != "Connected" || s.Rx != 1234 || s.Tx != 99 {
		t.Fatalf("%+v", s)
	}
	if s := parseIPsecStatus("STATE=Missing"); s.State != "Missing" || s.Rx != 0 {
		t.Fatalf("%+v", s)
	}
	if s := parseIPsecStatus("мусор"); s.State != "" {
		t.Fatalf("%+v", s)
	}
}

func TestIPsecEngineStatus(t *testing.T) {
	e := &ipsecEngine{cfg: eapCfg()}
	if e.Status().State != StateConnecting {
		t.Fatal("до подключения — connecting")
	}
	e.st = ipsecStatus{State: "Connected", Rx: 5, Tx: 6}
	e.seenUp = true
	if st := e.Status(); st.State != StateConnected || st.RxBytes != 5 || st.TxBytes != 6 || st.Transport != "IPsec/IKEv2" {
		t.Fatalf("%+v", st)
	}
	e.st = ipsecStatus{State: "Disconnected"}
	if e.Status().State != StateStopped {
		t.Fatal("после обрыва — stopped")
	}
}

func TestRasdialError(t *testing.T) {
	cases := map[string]string{
		"rasdial: exit status 691: Remote Access error 691": "логин",
		"rasdial: exit status 809: ":                        "UDP 500",
		"rasdial: exit status 13801: ":                      "сертификат",
		"rasdial: exit status 1: boom":                      "boom",
	}
	for in, want := range cases {
		if err := rasdialError(in); !strings.Contains(err.Error(), want) {
			t.Errorf("%q → %v", in, err)
		}
	}
}

func TestPsEncoded(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(psEncoded("Ж a"))
	if err != nil || len(raw) != 6 {
		t.Fatal(raw, err)
	}
	u := make([]uint16, 3)
	for i := range u {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	if string(utf16.Decode(u)) != "Ж a" {
		t.Fatal(u)
	}
}
